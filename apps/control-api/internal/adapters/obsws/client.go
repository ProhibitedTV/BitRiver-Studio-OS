package obsws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

const (
	defaultConnectTimeout   = 4 * time.Second
	defaultRequestTimeout   = 5 * time.Second
	defaultReconnectInitial = 1 * time.Second
	defaultReconnectMax     = 20 * time.Second
	defaultRequestRetries   = 2

	subscriptionGeneral = 1
	subscriptionScenes  = 4
	subscriptionInputs  = 8
	subscriptionOutputs = 64
)

type Publisher interface {
	Publish(model.EventEnvelope)
}

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		if logger != nil {
			client.logger = logger
		}
	}
}

func WithEventPublisher(publisher Publisher) Option {
	return func(client *Client) {
		client.publisher = publisher
	}
}

func WithReconnect(initial time.Duration, max time.Duration) Option {
	return func(client *Client) {
		if initial > 0 {
			client.reconnectInitial = initial
		}
		if max > 0 {
			client.reconnectMax = max
		}
	}
}

func WithTimeouts(connectTimeout time.Duration, requestTimeout time.Duration) Option {
	return func(client *Client) {
		if connectTimeout > 0 {
			client.connectTimeout = connectTimeout
		}
		if requestTimeout > 0 {
			client.requestTimeout = requestTimeout
		}
	}
}

type Client struct {
	endpoint         string
	password         string
	logger           *slog.Logger
	publisher        Publisher
	connectTimeout   time.Duration
	requestTimeout   time.Duration
	reconnectInitial time.Duration
	reconnectMax     time.Duration
	requestRetries   int

	installMu   sync.Mutex
	mu          sync.RWMutex
	version     adapters.OBSVersion
	snapshot    adapters.OBSSnapshot
	cacheReady  bool
	connected   bool
	lastError   error
	refreshLock int32
}

func New(endpoint string, password string, options ...Option) *Client {
	client := &Client{
		endpoint:         endpoint,
		password:         password,
		logger:           slog.Default(),
		connectTimeout:   defaultConnectTimeout,
		requestTimeout:   defaultRequestTimeout,
		reconnectInitial: defaultReconnectInitial,
		reconnectMax:     defaultReconnectMax,
		requestRetries:   defaultRequestRetries,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func (client *Client) Run(ctx context.Context) {
	backoff := client.reconnectInitial
	for {
		if ctx.Err() != nil {
			return
		}

		err := client.runSession(ctx)
		if ctx.Err() != nil {
			return
		}

		client.setDisconnected(err)
		client.publishConnectionEvent("obs.connection.disconnected", err)
		client.logger.Warn("obs-websocket session ended", "error", errorString(err), "retryIn", backoff.String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > client.reconnectMax {
			backoff = client.reconnectMax
		}
	}
}

func (client *Client) Snapshot(ctx context.Context) (adapters.OBSSnapshot, error) {
	if snapshot, ok := client.cachedSnapshot(); ok {
		return snapshot, nil
	}

	snapshot, err := client.refreshSnapshot(ctx)
	if err != nil {
		if cached, ok := client.cachedSnapshot(); ok {
			return cached, nil
		}
		return adapters.OBSSnapshot{}, err
	}
	return snapshot, nil
}

func (client *Client) Version(ctx context.Context) (adapters.OBSVersion, error) {
	client.mu.RLock()
	version := cloneVersion(client.version)
	client.mu.RUnlock()
	if version.OBSVersion != "" {
		return version, nil
	}

	if _, err := client.refreshSnapshot(ctx); err != nil {
		return adapters.OBSVersion{}, err
	}

	client.mu.RLock()
	defer client.mu.RUnlock()
	return cloneVersion(client.version), nil
}

func (client *Client) Scenes(ctx context.Context) ([]adapters.OBSScene, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return append([]adapters.OBSScene(nil), snapshot.Scenes...), nil
}

func (client *Client) CurrentProgramScene(ctx context.Context) (string, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return snapshot.ActiveScene, nil
}

func (client *Client) Inputs(ctx context.Context) ([]adapters.OBSInput, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return cloneInputs(snapshot.Inputs), nil
}

func (client *Client) StreamStatus(ctx context.Context) (adapters.OBSStreamStatus, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return adapters.OBSStreamStatus{}, err
	}
	return cloneStreamStatus(snapshot.StreamStatus), nil
}

func (client *Client) RecordStatus(ctx context.Context) (adapters.OBSRecordStatus, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return adapters.OBSRecordStatus{}, err
	}
	return cloneRecordStatus(snapshot.RecordStatus), nil
}

func (client *Client) StartStream(ctx context.Context) error {
	return client.simpleRequest(ctx, "StartStream", nil)
}

func (client *Client) StopStream(ctx context.Context) error {
	return client.simpleRequest(ctx, "StopStream", nil)
}

func (client *Client) StartRecording(ctx context.Context) error {
	return client.simpleRequest(ctx, "StartRecord", nil)
}

func (client *Client) StopRecording(ctx context.Context) error {
	return client.simpleRequest(ctx, "StopRecord", nil)
}

func (client *Client) SetCurrentScene(ctx context.Context, name string) error {
	return client.simpleRequest(ctx, "SetCurrentProgramScene", map[string]any{"sceneName": name})
}

func (client *Client) SetInputMuted(ctx context.Context, name string, muted bool) error {
	return client.simpleRequest(ctx, "SetInputMute", map[string]any{
		"inputName":  name,
		"inputMuted": muted,
	})
}

func (client *Client) ToggleInputMuted(ctx context.Context, name string) (bool, error) {
	var muted bool
	err := client.withRequestRetry(ctx, func(connection *requestConnection) error {
		if _, err := connection.request("ToggleInputMute", map[string]any{"inputName": name}); err != nil {
			return err
		}
		payload, err := connection.request("GetInputMute", map[string]any{"inputName": name})
		if err != nil {
			return err
		}
		var response struct {
			InputMuted bool `json:"inputMuted"`
		}
		if err := decodeJSON(payload, &response); err != nil {
			return err
		}
		muted = response.InputMuted
		return nil
	})
	if err != nil {
		return false, err
	}

	if _, err := client.refreshSnapshot(ctx); err != nil {
		client.logger.Warn("obs refresh after toggle failed", "error", err)
		client.triggerRefresh()
	}
	return muted, nil
}

func (client *Client) SetInputVolume(ctx context.Context, name string, gainDB float64) error {
	return client.simpleRequest(ctx, "SetInputVolume", map[string]any{
		"inputName":     name,
		"inputVolumeDb": gainDB,
	})
}

func (client *Client) simpleRequest(ctx context.Context, requestType string, requestData any) error {
	err := client.withRequestRetry(ctx, func(connection *requestConnection) error {
		_, err := connection.request(requestType, requestData)
		return err
	})
	if err != nil {
		return err
	}

	if _, err := client.refreshSnapshot(ctx); err != nil {
		client.logger.Warn("obs refresh after mutation failed", "error", err)
		client.triggerRefresh()
	}
	return nil
}

func (client *Client) refreshSnapshot(ctx context.Context) (adapters.OBSSnapshot, error) {
	var snapshot adapters.OBSSnapshot
	err := client.withRequestRetry(ctx, func(connection *requestConnection) error {
		resolved, err := buildSnapshot(connection)
		if err != nil {
			return err
		}
		snapshot = resolved
		return nil
	})
	if err != nil {
		return adapters.OBSSnapshot{}, err
	}

	client.mu.Lock()
	client.version = cloneVersion(snapshot.Version)
	client.snapshot = cloneSnapshot(snapshot)
	client.cacheReady = true
	client.connected = true
	client.lastError = nil
	client.mu.Unlock()

	return snapshot, nil
}

func (client *Client) withRequestRetry(ctx context.Context, fn func(*requestConnection) error) error {
	var lastErr error
	for attempt := 0; attempt <= client.requestRetries; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, client.requestTimeout)
		connection, err := client.connectRequest(requestCtx)
		if err == nil {
			err = fn(connection)
			connection.close()
		}
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		client.logger.Warn("obs request failed", "attempt", attempt+1, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 150 * time.Millisecond):
		}
	}
	return lastErr
}

func (client *Client) cachedSnapshot() (adapters.OBSSnapshot, bool) {
	client.mu.RLock()
	defer client.mu.RUnlock()
	if !client.cacheReady {
		return adapters.OBSSnapshot{}, false
	}
	return cloneSnapshot(client.snapshot), true
}

func (client *Client) setDisconnected(err error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.connected = false
	client.lastError = err
}

func (client *Client) triggerRefresh() {
	if !atomic.CompareAndSwapInt32(&client.refreshLock, 0, 1) {
		return
	}

	go func() {
		defer atomic.StoreInt32(&client.refreshLock, 0)
		ctx, cancel := context.WithTimeout(context.Background(), client.requestTimeout)
		defer cancel()
		if _, err := client.refreshSnapshot(ctx); err != nil {
			client.logger.Warn("obs refresh after mutation failed", "error", err)
		}
	}()
}

func (client *Client) publish(eventType string, payload any) {
	if client.publisher == nil {
		return
	}
	client.publisher.Publish(model.EventEnvelope{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	})
}

func (client *Client) publishConnectionEvent(eventType string, err error) {
	payload := map[string]any{
		"endpoint": client.endpoint,
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	client.publish(eventType, payload)
}

func cloneSnapshot(in adapters.OBSSnapshot) adapters.OBSSnapshot {
	out := in
	out.Version = cloneVersion(in.Version)
	out.Scenes = append([]adapters.OBSScene(nil), in.Scenes...)
	out.Inputs = cloneInputs(in.Inputs)
	out.StreamStatus = cloneStreamStatus(in.StreamStatus)
	out.RecordStatus = cloneRecordStatus(in.RecordStatus)
	if in.LastChangedAt != nil {
		timestamp := *in.LastChangedAt
		out.LastChangedAt = &timestamp
	}
	return out
}

func cloneVersion(in adapters.OBSVersion) adapters.OBSVersion {
	out := in
	out.AvailableRequests = append([]string(nil), in.AvailableRequests...)
	out.AvailableEvents = append([]string(nil), in.AvailableEvents...)
	return out
}

func cloneInputs(in []adapters.OBSInput) []adapters.OBSInput {
	out := make([]adapters.OBSInput, len(in))
	copy(out, in)
	for index := range out {
		if in[index].LastChangedAt != nil {
			timestamp := *in[index].LastChangedAt
			out[index].LastChangedAt = &timestamp
		}
	}
	return out
}

func cloneStreamStatus(in adapters.OBSStreamStatus) adapters.OBSStreamStatus {
	out := in
	if in.LastChangedAt != nil {
		timestamp := *in.LastChangedAt
		out.LastChangedAt = &timestamp
	}
	return out
}

func cloneRecordStatus(in adapters.OBSRecordStatus) adapters.OBSRecordStatus {
	out := in
	if in.LastChangedAt != nil {
		timestamp := *in.LastChangedAt
		out.LastChangedAt = &timestamp
	}
	return out
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

var _ adapters.Runner = (*Client)(nil)
var _ adapters.OBS = (*Client)(nil)

func decodeJSON(payload []byte, target any) error {
	if len(payload) == 0 {
		return errors.New("empty OBS response payload")
	}
	return json.Unmarshal(payload, target)
}
