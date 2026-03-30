package mediamtx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type Option func(*Client)

type Client struct {
	baseURL          string
	httpClient       *http.Client
	logger           *slog.Logger
	configPath       string
	destinationsPath string
	serviceName      string
	runner           CommandRunner

	mu    sync.RWMutex
	state adapters.MediaMTXSnapshot
}

type execRunner struct{}

type pathListResponse struct {
	Items []map[string]any `json:"items"`
}

type sessionEndpoint struct {
	protocol string
	path     string
}

func New(baseURL string, options ...Option) *Client {
	client := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		logger:           slog.Default(),
		configPath:       "/etc/mediamtx/mediamtx.yml",
		destinationsPath: "/etc/bitriver/mediamtx-destinations.json",
		serviceName:      "mediamtx.service",
		runner:           execRunner{},
		state: adapters.MediaMTXSnapshot{
			ConfigPath: "/etc/mediamtx/mediamtx.yml",
			Detail:     "MediaMTX has not been configured yet",
		},
	}

	for _, option := range options {
		option(client)
	}
	client.state.ConfigPath = client.configPath
	return client
}

func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		if logger != nil {
			client.logger = logger
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithConfigPath(path string) Option {
	return func(client *Client) {
		if path != "" {
			client.configPath = path
		}
	}
}

func WithDestinationsPath(path string) Option {
	return func(client *Client) {
		if path != "" {
			client.destinationsPath = path
		}
	}
}

func WithServiceName(name string) Option {
	return func(client *Client) {
		if name != "" {
			client.serviceName = name
		}
	}
}

func WithCommandRunner(runner CommandRunner) Option {
	return func(client *Client) {
		if runner != nil {
			client.runner = runner
		}
	}
}

func (client *Client) Snapshot(ctx context.Context) (adapters.MediaMTXSnapshot, error) {
	paths, err := client.inspectPaths(ctx)
	if err != nil {
		return adapters.MediaMTXSnapshot{}, err
	}

	sessions, err := client.inspectSessions(ctx)
	if err != nil {
		client.logger.Warn("mediamtx session inspection failed", "error", err)
	}

	state := client.snapshotState()
	state.Ready = true
	state.Healthy = true
	state.Paths = mergePathState(state.Paths, paths, state.LocalIngestPath, state.LocalPreviewPath, state.WebRTCPreviewEnabled)
	state.Sessions = sessions
	state.Destinations = markDestinationState(state.Destinations, pathReady(state.Paths, state.LocalIngestPath))
	state.Detail = snapshotDetail(state)
	return state, nil
}

func (client *Client) ApplyConfig(ctx context.Context, generation string, config model.ControlConfig) (adapters.MediaApplyResult, error) {
	rendered, err := renderManagedConfig(generation, config, client.configPath, client.destinationsPath)
	if err != nil {
		return adapters.MediaApplyResult{}, err
	}

	previousConfig, hadPreviousConfig, err := readOptionalFile(client.configPath)
	if err != nil {
		return adapters.MediaApplyResult{}, fmt.Errorf("read existing mediamtx config: %w", err)
	}
	previousDestinations, hadPreviousDestinations, err := readOptionalFile(client.destinationsPath)
	if err != nil {
		return adapters.MediaApplyResult{}, fmt.Errorf("read existing mediamtx destinations sidecar: %w", err)
	}

	if err := writeFileAtomically(client.configPath, rendered.configYAML, 0o640); err != nil {
		return adapters.MediaApplyResult{}, fmt.Errorf("write mediamtx config: %w", err)
	}
	if err := writeFileAtomically(client.destinationsPath, rendered.destinationsJSON, 0o640); err != nil {
		_ = restoreFile(client.configPath, previousConfig, hadPreviousConfig, 0o640)
		return adapters.MediaApplyResult{}, fmt.Errorf("write mediamtx destinations sidecar: %w", err)
	}

	reloadMode, err := client.reloadOrRestart(ctx)
	if err != nil {
		restoreErr := errors.Join(
			restoreFile(client.configPath, previousConfig, hadPreviousConfig, 0o640),
			restoreFile(client.destinationsPath, previousDestinations, hadPreviousDestinations, 0o640),
		)
		if restoreErr != nil {
			client.logger.Error("mediamtx restore failed", "error", restoreErr)
		}
		if _, restartErr := client.runner.Run(ctx, "systemctl", "restart", client.serviceName); restartErr != nil {
			err = fmt.Errorf("%w; recovery restart failed: %v", err, restartErr)
		}
		return adapters.MediaApplyResult{}, err
	}

	now := time.Now().UTC()
	rendered.snapshot.LastAppliedAt = &now
	rendered.snapshot.LastReloadedAt = &now
	rendered.snapshot.Detail = "Managed MediaMTX configuration is active"
	rendered.snapshot.Healthy = true
	rendered.snapshot.Destinations = markDestinationState(rendered.snapshot.Destinations, false)
	client.storeState(rendered.snapshot)

	return adapters.MediaApplyResult{
		ConfigGeneration: generation,
		ConfigPath:       client.configPath,
		ReloadMode:       reloadMode,
		AppliedAt:        now,
	}, nil
}

func (client *Client) inspectPaths(ctx context.Context) ([]adapters.MediaPath, error) {
	items, err := client.fetchItems(ctx, "/v3/paths/list")
	if err != nil {
		return nil, err
	}

	paths := make([]adapters.MediaPath, 0, len(items))
	for _, item := range items {
		paths = append(paths, adapters.MediaPath{
			Name:        stringValue(item["name"]),
			Source:      sourceValue(item["source"]),
			Readers:     collectionLen(item["readers"]),
			Publisher:   boolValue(item["sourceReady"]),
			Ready:       boolValue(item["sourceReady"]),
			BitrateKbps: bitrateKbps(item["bytesReceived"]),
			Detail:      runtimePathDetail(boolValue(item["sourceReady"]), stringValue(item["name"])),
		})
	}

	return paths, nil
}

func (client *Client) inspectSessions(ctx context.Context) ([]adapters.MediaSession, error) {
	endpoints := []sessionEndpoint{
		{protocol: "rtmp", path: "/v3/rtmpconns/list"},
		{protocol: "srt", path: "/v3/srtconns/list"},
		{protocol: "rtsp", path: "/v3/rtspsessions/list"},
		{protocol: "webrtc", path: "/v3/webrtcsessions/list"},
	}

	sessions := make([]adapters.MediaSession, 0)
	for _, endpoint := range endpoints {
		items, err := client.fetchItems(ctx, endpoint.path)
		if err != nil {
			if errors.Is(err, errEndpointUnsupported) {
				continue
			}
			return nil, err
		}
		for _, item := range items {
			sessions = append(sessions, adapters.MediaSession{
				ID:         stringValue(item["id"]),
				Protocol:   endpoint.protocol,
				Path:       coalesce(stringValue(item["path"]), stringValue(item["pathName"])),
				RemoteAddr: coalesce(stringValue(item["remoteAddr"]), stringValue(item["remoteAddress"])),
				State:      coalesce(stringValue(item["state"]), "active"),
				StartedAt:  timeValue(item["created"]),
			})
		}
	}

	return sessions, nil
}

var errEndpointUnsupported = errors.New("mediamtx endpoint unsupported")

func (client *Client) fetchItems(ctx context.Context, requestPath string) ([]map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+requestPath, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return nil, errEndpointUnsupported
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("mediamtx returned %s for %s", response.Status, requestPath)
	}

	var payload pathListResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", requestPath, err)
	}
	return payload.Items, nil
}

func (client *Client) reloadOrRestart(ctx context.Context) (string, error) {
	if _, err := client.runner.Run(ctx, "systemctl", "reload", client.serviceName); err == nil {
		return "reload", nil
	} else {
		client.logger.Warn("mediamtx reload failed, falling back to restart", "service", client.serviceName, "error", err)
		if _, restartErr := client.runner.Run(ctx, "systemctl", "restart", client.serviceName); restartErr != nil {
			return "", fmt.Errorf("reload failed: %w; restart failed: %v", err, restartErr)
		}
	}
	return "restart", nil
}

func (client *Client) storeState(snapshot adapters.MediaMTXSnapshot) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.state = cloneSnapshot(snapshot)
}

func (client *Client) snapshotState() adapters.MediaMTXSnapshot {
	client.mu.RLock()
	defer client.mu.RUnlock()
	return cloneSnapshot(client.state)
}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	return command.CombinedOutput()
}

func cloneSnapshot(in adapters.MediaMTXSnapshot) adapters.MediaMTXSnapshot {
	out := in
	out.Paths = append([]adapters.MediaPath(nil), in.Paths...)
	out.Destinations = append([]adapters.MediaDestination(nil), in.Destinations...)
	out.Sessions = append([]adapters.MediaSession(nil), in.Sessions...)
	if in.LastAppliedAt != nil {
		timestamp := *in.LastAppliedAt
		out.LastAppliedAt = &timestamp
	}
	if in.LastReloadedAt != nil {
		timestamp := *in.LastReloadedAt
		out.LastReloadedAt = &timestamp
	}
	if in.LastConfigValidatedAt != nil {
		timestamp := *in.LastConfigValidatedAt
		out.LastConfigValidatedAt = &timestamp
	}
	return out
}

func writeFileAtomically(path string, payload []byte, permissions os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".bitriver-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(payload); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Chmod(permissions); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}

func readOptionalFile(path string) ([]byte, bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return payload, true, nil
}

func restoreFile(path string, payload []byte, exists bool, permissions os.FileMode) error {
	if !exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeFileAtomically(path, payload, permissions)
}

func mergePathState(previous []adapters.MediaPath, current []adapters.MediaPath, ingestPath string, previewPath string, webrtcPreview bool) []adapters.MediaPath {
	byName := map[string]adapters.MediaPath{}
	for _, item := range previous {
		byName[item.Name] = item
	}

	merged := make([]adapters.MediaPath, 0, len(current))
	for _, item := range current {
		if existing, ok := byName[item.Name]; ok {
			if item.PublishURL == "" {
				item.PublishURL = existing.PublishURL
			}
		}
		if item.Name == ingestPath && item.PublishURL == "" {
			item.PublishURL = "rtmp://127.0.0.1:1935/" + ingestPath
		}
		if item.Name == previewPath {
			item.Detail = previewDetail(webrtcPreview, item.Ready)
		}
		merged = append(merged, item)
	}
	return merged
}

func markDestinationState(destinations []adapters.MediaDestination, ingestReady bool) []adapters.MediaDestination {
	updated := make([]adapters.MediaDestination, 0, len(destinations))
	for _, destination := range destinations {
		destination.Connected = destination.Enabled && ingestReady
		destination.Detail = destinationDetail(destination.Enabled, destination.Connected)
		updated = append(updated, destination)
	}
	return updated
}

func pathReady(paths []adapters.MediaPath, name string) bool {
	for _, item := range paths {
		if item.Name == name {
			return item.Ready
		}
	}
	return false
}

func snapshotDetail(snapshot adapters.MediaMTXSnapshot) string {
	if pathReady(snapshot.Paths, snapshot.LocalIngestPath) {
		return "MediaMTX program ingest is ready"
	}
	if snapshot.LocalIngestPath == "" {
		return "MediaMTX is reachable"
	}
	return "MediaMTX is reachable and waiting for program ingest"
}

func runtimePathDetail(ready bool, pathName string) string {
	if ready {
		return fmt.Sprintf("Path %s has an active publisher", pathName)
	}
	return fmt.Sprintf("Path %s is idle", pathName)
}

func sourceValue(value any) string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "publisher"
		}
		return typed
	case map[string]any:
		typeName := stringValue(typed["type"])
		if typeName != "" {
			return typeName
		}
	}
	return "publisher"
}

func collectionLen(value any) int {
	items, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func bitrateKbps(value any) int {
	bytesReceived := intValue(value)
	if bytesReceived <= 0 {
		return 0
	}
	return bytesReceived * 8 / 1000
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(typed)
		return number
	default:
		return 0
	}
}

func timeValue(value any) *time.Time {
	stringTime := stringValue(value)
	if stringTime == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, stringTime)
	if err != nil {
		return nil
	}
	return &parsed
}

func coalesce(primary string, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
