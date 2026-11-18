package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/configstore"
	"github.com/bitriver/studio-os/apps/control-api/internal/events"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

type RuntimePaths struct {
	SetupStatusPath  string
	HealthStatusPath string
	UpdateStatusPath string
}

type Dependencies struct {
	Logger       *slog.Logger
	ConfigStore  *configstore.Store
	OBS          adapters.OBS
	MediaMTX     adapters.MediaMTX
	Owncast      adapters.Owncast
	ServiceUnits adapters.Services
	EventHub     *events.Hub
	StartedAt    time.Time
	Version      string
	Paths        RuntimePaths
	PollInterval time.Duration
}

type Controller struct {
	logger       *slog.Logger
	configStore  *configstore.Store
	obs          adapters.OBS
	mediaMTX     adapters.MediaMTX
	owncast      adapters.Owncast
	serviceUnits adapters.Services
	eventHub     *events.Hub
	startedAt    time.Time
	version      string
	paths        RuntimePaths
	pollInterval time.Duration
}

func NewController(deps Dependencies) *Controller {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if deps.EventHub == nil {
		deps.EventHub = events.NewHub()
	}
	if deps.PollInterval <= 0 {
		deps.PollInterval = 3 * time.Second
	}

	return &Controller{
		logger:       logger,
		configStore:  deps.ConfigStore,
		obs:          deps.OBS,
		mediaMTX:     deps.MediaMTX,
		owncast:      deps.Owncast,
		serviceUnits: deps.ServiceUnits,
		eventHub:     deps.EventHub,
		startedAt:    deps.StartedAt.UTC(),
		version:      deps.Version,
		paths:        deps.Paths,
		pollInterval: deps.PollInterval,
	}
}

func (controller *Controller) EventHub() *events.Hub {
	return controller.eventHub
}

func (controller *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(controller.pollInterval)
	defer ticker.Stop()

	controller.publishSnapshotEvent(ctx, "snapshot.initial")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			controller.publishSnapshotEvent(ctx, "snapshot.tick")
		}
	}
}

func (controller *Controller) System(ctx context.Context) model.SystemSummary {
	config := controller.configStore.Active().Config
	setup := controller.SetupStatus(ctx)
	services := controller.services(ctx)
	health := controller.Health(ctx)
	cpu, memory, network := collectHostMetrics(config.MediaRouter.RecordingPath, setup)
	disk := collectDiskStatus(config.MediaRouter.RecordingPath)

	return model.SystemSummary{
		ApplianceID:   fallbackString(setup.ApplianceID, "studio-appliance-dev"),
		Name:          config.System.Name,
		Version:       controller.version,
		BootMode:      fallbackString(setup.DisplayMode, "headless"),
		StartedAt:     controller.startedAt,
		UptimeSeconds: int64(time.Since(controller.startedAt).Seconds()),
		Health:        health,
		Network:       network,
		CPU:           cpu,
		Memory:        memory,
		Disk:          disk,
		Services:      services,
	}
}

func (controller *Controller) Health(ctx context.Context) model.HealthSummary {
	if summary, ok := loadJSONFile[model.HealthSummary](controller.paths.HealthStatusPath); ok {
		return summary
	}

	setup := controller.SetupStatus(ctx)
	statuses := controller.services(ctx)
	checks := make([]model.HealthCheck, 0, len(statuses)+1)
	overall := model.HealthHealthy

	for _, status := range statuses {
		state := model.HealthHealthy
		message := status.Detail
		if !status.Active {
			state = model.HealthDegraded
			if status.Name == "bitriver-control-api.service" || status.Name == "mediamtx.service" {
				state = model.HealthError
			}
			message = fallbackString(status.Detail, "service is not active")
		}
		checks = append(checks, model.HealthCheck{
			Key:     status.Name,
			State:   state,
			Message: message,
		})
		overall = maxHealth(overall, state)
	}

	if len(setup.Issues) > 0 {
		checks = append(checks, model.HealthCheck{
			Key:     "setup",
			State:   model.HealthDegraded,
			Message: setup.Issues[0],
		})
		overall = maxHealth(overall, model.HealthDegraded)
	}

	mediaRouter := controller.MediaRouter(ctx)
	mediaState := model.HealthHealthy
	if !mediaRouter.Healthy {
		mediaState = model.HealthError
	}
	checks = append(checks, model.HealthCheck{
		Key:     "media-router",
		State:   mediaState,
		Message: fallbackString(mediaRouter.Detail, "MediaMTX is unavailable"),
	})
	overall = maxHealth(overall, mediaState)

	return model.HealthSummary{
		Overall:   overall,
		Checks:    checks,
		UpdatedAt: time.Now().UTC(),
	}
}

func (controller *Controller) SetupStatus(context.Context) model.SetupStatus {
	if setup, ok := loadJSONFile[model.SetupStatus](controller.paths.SetupStatusPath); ok {
		return model.CloneSetupStatus(setup)
	}
	return model.DefaultSetupStatus(controller.startedAt)
}

func (controller *Controller) MediaRouter(ctx context.Context) model.MediaRouterSummary {
	config := controller.configStore.Active().Config
	summary := configuredMediaRouter(config)
	if controller.mediaMTX == nil {
		summary.Detail = "MediaMTX adapter is not configured"
		return summary
	}

	snapshot, err := controller.mediaMTX.Snapshot(ctx)
	if err != nil {
		summary.Detail = err.Error()
		return summary
	}

	return mediaRouterSummaryFromSnapshot(summary, snapshot)
}

func (controller *Controller) Stream(ctx context.Context) (model.StreamSummary, error) {
	obs, err := controller.obsSnapshot(ctx)
	if err != nil {
		return model.StreamSummary{}, err
	}
	mediaRouter := controller.MediaRouter(ctx)

	state := model.StreamStopped
	if obs.StreamActive {
		state = model.StreamLive
	}

	return model.StreamSummary{
		State:         state,
		ActiveSceneID: controller.activeSceneID(controller.configStore.Active().Config, obs.ActiveScene),
		Destination:   firstMediaDestinationLabel(mediaRouter.Destinations),
		StartedAt:     obs.LastChangedAt,
	}, nil
}

func (controller *Controller) Outputs(ctx context.Context) (model.OutputSummary, error) {
	obs, err := controller.obsSnapshot(ctx)
	if err != nil {
		return model.OutputSummary{}, err
	}
	config := controller.configStore.Active().Config
	mediaRouter := controller.MediaRouter(ctx)

	destinations := make([]model.DestinationSummary, 0, len(mediaRouter.Destinations)+1)
	for _, destination := range mediaRouter.Destinations {
		destinations = append(destinations, model.DestinationSummary{
			ID:        destination.ID,
			Mode:      "external_" + destination.Protocol,
			Label:     destination.Name,
			Target:    destination.Target,
			Connected: destination.Connected && obs.StreamActive,
			Enabled:   destination.Enabled,
			Detail:    destination.Detail,
		})
	}

	if config.Owncast.Enabled {
		if controller.owncast == nil {
			destinations = append(destinations, model.DestinationSummary{
				ID:        "owncast",
				Mode:      "owncast",
				Label:     "Owncast",
				Target:    config.Owncast.PublicURL,
				Connected: false,
				Enabled:   true,
				Detail:    "Owncast adapter is not configured",
			})
		} else {
			status, err := controller.owncast.Status(ctx)
			if err != nil {
				destinations = append(destinations, model.DestinationSummary{
					ID:        "owncast",
					Mode:      "owncast",
					Label:     "Owncast",
					Target:    config.Owncast.PublicURL,
					Connected: false,
					Enabled:   true,
					Detail:    err.Error(),
				})
			} else {
				destinations = append(destinations, model.DestinationSummary{
					ID:        "owncast",
					Mode:      "owncast",
					Label:     "Owncast",
					Target:    config.Owncast.PublicURL,
					Connected: status.Connected,
					Enabled:   true,
					Detail:    status.Detail,
				})
			}
		}
	}

	streamState := model.StreamStopped
	if obs.StreamActive {
		streamState = model.StreamLive
	}

	return model.OutputSummary{
		StreamState:  streamState,
		Destinations: destinations,
	}, nil
}

func (controller *Controller) Recordings(ctx context.Context) (model.RecordingSummary, error) {
	config := controller.configStore.Active().Config
	obs, err := controller.obsSnapshot(ctx)
	if err != nil {
		return model.RecordingSummary{}, err
	}

	disk := collectDiskStatus(config.MediaRouter.RecordingPath)
	state := model.RecordStopped
	if obs.RecordingActive {
		state = model.RecordRecording
	}

	recordingPath := config.MediaRouter.RecordingPath
	if obs.RecordingPath != "" {
		recordingPath = obs.RecordingPath
	}

	return model.RecordingSummary{
		State:         state,
		RecordingPath: recordingPath,
		CurrentFile:   obs.RecordingFile,
		TotalGb:       disk.TotalGb,
		UsedGb:        disk.UsedGb,
		FreeGb:        disk.FreeGb,
		Warning:       disk.Warning,
	}, nil
}

func (controller *Controller) Scenes(ctx context.Context) (model.ScenesState, error) {
	config := controller.configStore.Active().Config
	obs, err := controller.obsSnapshot(ctx)
	if err != nil {
		return model.ScenesState{}, err
	}

	items := make([]model.SceneSummary, 0, len(config.Scenes))
	activeSceneID := ""
	for _, scene := range config.Scenes {
		active := scene.Name == obs.ActiveScene
		if active {
			activeSceneID = scene.ID
		}
		items = append(items, model.SceneSummary{
			ID:     scene.ID,
			Name:   scene.Name,
			Active: active,
		})
	}

	return model.ScenesState{
		ActiveSceneID: activeSceneID,
		Items:         items,
	}, nil
}

func (controller *Controller) Sources(ctx context.Context) []model.SourceSummary {
	config := controller.configStore.Active().Config
	setup := controller.SetupStatus(ctx)
	mediaErr := errors.New("mediamtx adapter not configured")
	mediaSnapshot := adapters.MediaMTXSnapshot{}
	if controller.mediaMTX != nil {
		mediaSnapshot, mediaErr = controller.mediaMTX.Snapshot(ctx)
	}

	sources := make([]model.SourceSummary, 0, len(config.Sources))
	for _, source := range config.Sources {
		summary := model.SourceSummary{
			ID:          source.ID,
			Name:        source.Name,
			Kind:        source.Kind,
			Status:      model.SourceReady,
			Available:   true,
			Detail:      source.DeviceRef,
			PublishURL:  source.PublishURL,
			BitrateKbps: 0,
		}

		switch source.Kind {
		case "local_video":
			summary.Available = contains(setup.Hardware.VideoDevices, source.DeviceRef) || len(setup.Hardware.VideoDevices) == 0
			if summary.Available {
				summary.Detail = "Local capture device is available"
			} else {
				summary.Status = model.SourceDisconnected
				summary.Detail = "Local capture device is not present"
			}
		case "local_audio":
			summary.Available = contains(setup.Hardware.AudioCaptureDevices, source.DeviceRef) || len(setup.Hardware.AudioCaptureDevices) == 0
			if summary.Available {
				summary.Detail = "Local audio input is available"
			} else {
				summary.Status = model.SourceDisconnected
				summary.Detail = "Local audio input is not present"
			}
		case "browser_source":
			summary.Detail = "Browser source configured for OBS"
		case "network_ingest":
			summary.Available = mediaErr == nil
			summary.Status = model.SourceWarning
			summary.Detail = "Awaiting contributor publish"
			if mediaErr == nil {
				for _, path := range mediaSnapshot.Paths {
					if path.Name == source.ID {
						summary.Available = path.Ready
						summary.BitrateKbps = path.BitrateKbps
						summary.PublishURL = coalesce(path.PublishURL, source.PublishURL)
						if path.Ready {
							summary.Status = model.SourceReady
						}
						summary.Detail = path.Detail
						break
					}
				}
			}
		}

		sources = append(sources, summary)
	}

	return sources
}

func (controller *Controller) Audio(ctx context.Context) (model.AudioState, error) {
	config := controller.configStore.Active().Config
	obs, err := controller.obsSnapshot(ctx)
	if err != nil {
		return model.AudioState{}, err
	}

	channels := make([]model.AudioChannelSummary, 0, len(config.Audio.Channels))
	for _, channel := range config.Audio.Channels {
		audio := model.AudioChannelSummary{
			ID:        channel.ID,
			Name:      channel.Name,
			Source:    channel.DeviceRef,
			Available: false,
			GainDB:    0,
			PeakDB:    -60,
		}
		for _, input := range obs.Inputs {
			if input.Name == channel.Name {
				audio.Muted = input.Muted
				audio.GainDB = input.GainDB
				audio.PeakDB = input.PeakDB
				audio.Available = input.Available
				break
			}
		}
		channels = append(channels, audio)
	}

	return model.AudioState{Channels: channels}, nil
}

func (controller *Controller) ActiveConfig() model.ConfigEnvelope {
	return controller.configStore.Active()
}

func (controller *Controller) DraftConfig() model.ConfigEnvelope {
	return controller.configStore.Draft()
}

func (controller *Controller) UpdateDraft(config model.ControlConfig) (model.ConfigEnvelope, error) {
	return controller.configStore.UpdateDraft(config)
}

func (controller *Controller) ValidateDraft() model.ValidateConfigResponse {
	return controller.configStore.ValidateDraft()
}

func (controller *Controller) ApplyDraft(ctx context.Context) (model.OperationResponse, *model.ValidateConfigResponse, error) {
	response, validation, err := controller.configStore.ApplyDraft(func(document model.ConfigDocument) error {
		// Applying MediaMTX settings can restart the router, dropping program
		// output or recordings. Re-check actual OBS state on the server, even
		// if a client hid the Apply button. Fail closed if OBS is unavailable.
		obs, err := controller.obsSnapshot(ctx)
		if err != nil {
			return model.NewUnavailableError("OBS", err)
		}
		if obs.StreamActive || obs.RecordingActive {
			return model.NewConflictError("stop streaming and recording before applying settings")
		}
		if controller.mediaMTX == nil {
			return nil
		}
		// MediaMTX may have contributors or viewers even when OBS itself is
		// stopped. Applying the router configuration can disconnect those
		// sessions, so also check the live router state before restarting it.
		media, err := controller.mediaMTX.Snapshot(ctx)
		if err != nil {
			return model.NewUnavailableError("MediaMTX", err)
		}
		if mediaRouterHasActiveSessions(media) {
			return model.NewConflictError("stop active MediaMTX publishers and readers before applying settings")
		}
		_, err = controller.mediaMTX.ApplyConfig(ctx, document.Generation, document.ControlConfig)
		return err
	})
	if err == nil && validation == nil {
		controller.publishSnapshotEvent(ctx, "settings.applied")
	}
	return response, validation, err
}

// mediaRouterHasActiveSessions errs on the side of not tearing down an
// active contributor or viewer. MediaMTX paths may exist when nobody is
// broadcasting, so empty/unready paths do not block configuration recovery.
func mediaRouterHasActiveSessions(snapshot adapters.MediaMTXSnapshot) bool {
	if len(snapshot.Sessions) > 0 {
		return true
	}
	for _, path := range snapshot.Paths {
		if path.Ready && (path.Publisher || path.Readers > 0) {
			return true
		}
	}
	return false
}

func (controller *Controller) UpdateStatus(context.Context) model.UpdateStatus {
	if status, ok := loadJSONFile[model.UpdateStatus](controller.paths.UpdateStatusPath); ok {
		return status
	}
	return model.DefaultUpdateStatus(time.Now().UTC(), controller.version)
}

func (controller *Controller) Snapshot(ctx context.Context) (model.Snapshot, error) {
	stream, err := controller.Stream(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}
	outputs, err := controller.Outputs(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}
	recordings, err := controller.Recordings(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}
	scenes, err := controller.Scenes(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}
	audio, err := controller.Audio(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}

	return model.Snapshot{
		System:      controller.System(ctx),
		Health:      controller.Health(ctx),
		Stream:      stream,
		Outputs:     outputs,
		Recordings:  recordings,
		MediaRouter: controller.MediaRouter(ctx),
		Scenes:      scenes,
		Sources:     controller.Sources(ctx),
		Audio:       audio,
		Settings:    controller.ActiveConfig(),
		Update:      controller.UpdateStatus(ctx),
		Setup:       controller.SetupStatus(ctx),
	}, nil
}

func (controller *Controller) StartStream(ctx context.Context) (model.OperationResponse, error) {
	if err := controller.obs.StartStream(ctx); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "stream.started")
	return completedOperation("stream_start", "stream started"), nil
}

func (controller *Controller) StopStream(ctx context.Context) (model.OperationResponse, error) {
	if err := controller.obs.StopStream(ctx); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "stream.stopped")
	return completedOperation("stream_stop", "stream stopped"), nil
}

func (controller *Controller) StartRecording(ctx context.Context) (model.OperationResponse, error) {
	if err := controller.obs.StartRecording(ctx); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "recording.started")
	return completedOperation("recording_start", "recording started"), nil
}

func (controller *Controller) StopRecording(ctx context.Context) (model.OperationResponse, error) {
	if err := controller.obs.StopRecording(ctx); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "recording.stopped")
	return completedOperation("recording_stop", "recording stopped"), nil
}

func (controller *Controller) ActivateScene(ctx context.Context, sceneID string) (model.OperationResponse, error) {
	config := controller.configStore.Active().Config
	sceneName := ""
	for _, scene := range config.Scenes {
		if scene.ID == sceneID {
			sceneName = scene.Name
			break
		}
	}
	if sceneName == "" {
		return model.OperationResponse{}, model.NewNotFoundError("scene", sceneID)
	}
	if err := controller.obs.SetCurrentScene(ctx, sceneName); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "scene.activated")
	return completedOperation("scene_activate", "scene activated"), nil
}

func (controller *Controller) SetMute(ctx context.Context, channelID string, muted bool) (model.OperationResponse, error) {
	config := controller.configStore.Active().Config
	channelName := ""
	for _, channel := range config.Audio.Channels {
		if channel.ID == channelID {
			channelName = channel.Name
			break
		}
	}
	if channelName == "" {
		return model.OperationResponse{}, model.NewNotFoundError("audio route", channelID)
	}
	if err := controller.obs.SetInputMuted(ctx, channelName, muted); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "audio.mute")
	message := "channel muted"
	if !muted {
		message = "channel unmuted"
	}
	return completedOperation("audio_mute", message), nil
}

func (controller *Controller) SetGain(ctx context.Context, channelID string, gainDB float64) (model.OperationResponse, error) {
	config := controller.configStore.Active().Config
	channelName := ""
	for _, channel := range config.Audio.Channels {
		if channel.ID == channelID {
			channelName = channel.Name
			break
		}
	}
	if channelName == "" {
		return model.OperationResponse{}, model.NewNotFoundError("audio route", channelID)
	}
	if err := controller.obs.SetInputVolume(ctx, channelName, gainDB); err != nil {
		return model.OperationResponse{}, wrapUnavailable("OBS", err)
	}
	controller.publishSnapshotEvent(ctx, "audio.gain")
	return completedOperation("audio_gain", fmt.Sprintf("gain set to %.1f dB", gainDB)), nil
}

func (controller *Controller) publishSnapshotEvent(ctx context.Context, eventType string) {
	snapshot, err := controller.Snapshot(ctx)
	if err != nil {
		controller.logger.Warn("publish snapshot event failed", "type", eventType, "error", err)
		return
	}
	controller.eventHub.Publish(model.EventEnvelope{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   snapshot,
	})
}

func (controller *Controller) obsSnapshot(ctx context.Context) (adapters.OBSSnapshot, error) {
	if controller.obs == nil {
		return adapters.OBSSnapshot{}, model.NewUnavailableError("OBS", errors.New("adapter not configured"))
	}
	snapshot, err := controller.obs.Snapshot(ctx)
	if err != nil {
		return adapters.OBSSnapshot{}, model.NewUnavailableError("OBS", err)
	}
	return snapshot, nil
}

func (controller *Controller) services(ctx context.Context) []model.ServiceStatus {
	names := []string{
		"bitriver-control-api.service",
		"mediamtx.service",
		"bitriver-update-agent.service",
		"bitriver-health-supervisor.service",
		"owncast.service",
		"obs.service",
	}
	if controller.serviceUnits == nil {
		return []model.ServiceStatus{}
	}
	statuses, err := controller.serviceUnits.Status(ctx, names)
	if err != nil {
		controller.logger.Warn("service status probe failed", "error", err)
		return []model.ServiceStatus{}
	}

	result := make([]model.ServiceStatus, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, model.ServiceStatus{
			Name:       status.Name,
			Active:     status.Active,
			State:      status.State,
			Detail:     status.Detail,
			ManagedBy:  "systemd",
			Restarting: status.Restarting,
		})
	}
	return result
}

func (controller *Controller) activeSceneID(config model.ControlConfig, sceneName string) string {
	for _, scene := range config.Scenes {
		if scene.Name == sceneName {
			return scene.ID
		}
	}
	return ""
}

func configuredMediaRouter(config model.ControlConfig) model.MediaRouterSummary {
	destinations := make([]model.MediaDestinationSummary, 0, len(config.MediaRouter.PublishDestinations))
	for _, destination := range config.MediaRouter.PublishDestinations {
		destinations = append(destinations, model.MediaDestinationSummary{
			ID:        destination.ID,
			Name:      destination.Name,
			Protocol:  destination.Protocol,
			Target:    destination.TargetURL,
			Enabled:   destination.Enabled,
			Connected: false,
			Detail:    configuredDestinationDetail(destination.Enabled),
		})
	}

	return model.MediaRouterSummary{
		Healthy:              false,
		Detail:               "MediaMTX runtime data is unavailable",
		LocalIngestPath:      config.MediaRouter.LocalIngestPath,
		LocalPreviewPath:     config.MediaRouter.LocalPreviewPath,
		RecordingPath:        config.MediaRouter.RecordingPath,
		WebRTCPreviewEnabled: config.MediaRouter.WebRTCPreviewEnabled,
		Destinations:         destinations,
		ActivePaths:          []model.MediaPathSummary{},
		ActiveSessions:       []model.MediaSessionSummary{},
	}
}

func mediaRouterSummaryFromSnapshot(fallback model.MediaRouterSummary, snapshot adapters.MediaMTXSnapshot) model.MediaRouterSummary {
	summary := fallback
	summary.Healthy = snapshot.Healthy
	summary.Detail = fallbackString(snapshot.Detail, fallback.Detail)
	summary.ConfigGeneration = snapshot.ConfigGeneration
	summary.ConfigPath = fallbackString(snapshot.ConfigPath, fallback.ConfigPath)
	summary.LocalIngestPath = fallbackString(snapshot.LocalIngestPath, fallback.LocalIngestPath)
	summary.LocalPreviewPath = fallbackString(snapshot.LocalPreviewPath, fallback.LocalPreviewPath)
	summary.RecordingPath = fallbackString(snapshot.RecordingPath, fallback.RecordingPath)
	summary.WebRTCPreviewEnabled = snapshot.WebRTCPreviewEnabled
	summary.LastAppliedAt = snapshot.LastAppliedAt
	summary.LastReloadedAt = snapshot.LastReloadedAt
	summary.LastConfigValidatedAt = snapshot.LastConfigValidatedAt
	summary.Destinations = mediaDestinationSummaries(snapshot.Destinations)
	summary.ActivePaths = mediaPathSummaries(snapshot.Paths)
	summary.ActiveSessions = mediaSessionSummaries(snapshot.Sessions)
	return summary
}

func mediaDestinationSummaries(destinations []adapters.MediaDestination) []model.MediaDestinationSummary {
	summaries := make([]model.MediaDestinationSummary, 0, len(destinations))
	for _, destination := range destinations {
		summaries = append(summaries, model.MediaDestinationSummary{
			ID:        destination.ID,
			Name:      destination.Name,
			Protocol:  destination.Protocol,
			Target:    destination.TargetURL,
			Enabled:   destination.Enabled,
			Connected: destination.Connected,
			Detail:    destination.Detail,
		})
	}
	return summaries
}

func mediaPathSummaries(paths []adapters.MediaPath) []model.MediaPathSummary {
	summaries := make([]model.MediaPathSummary, 0, len(paths))
	for _, path := range paths {
		summaries = append(summaries, model.MediaPathSummary{
			Name:        path.Name,
			Ready:       path.Ready,
			Source:      path.Source,
			Readers:     path.Readers,
			BitrateKbps: path.BitrateKbps,
			Detail:      path.Detail,
			PublishURL:  path.PublishURL,
		})
	}
	return summaries
}

func mediaSessionSummaries(sessions []adapters.MediaSession) []model.MediaSessionSummary {
	summaries := make([]model.MediaSessionSummary, 0, len(sessions))
	for _, session := range sessions {
		summaries = append(summaries, model.MediaSessionSummary{
			ID:         session.ID,
			Protocol:   session.Protocol,
			Path:       session.Path,
			RemoteAddr: session.RemoteAddr,
			State:      session.State,
			StartedAt:  session.StartedAt,
		})
	}
	return summaries
}

func loadJSONFile[T any](path string) (T, bool) {
	var value T
	if path == "" {
		return value, false
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return value, false
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, false
	}
	return value, true
}

func configuredDestinationDetail(enabled bool) string {
	if enabled {
		return "Waiting for local program ingest"
	}
	return "Destination is disabled"
}

func firstMediaDestinationLabel(destinations []model.MediaDestinationSummary) string {
	for _, destination := range destinations {
		if destination.Enabled {
			return destination.Name
		}
	}
	if len(destinations) > 0 {
		return destinations[0].Name
	}
	return "Local Media Router"
}

func coalesce(primary string, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func fallbackString(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func maxHealth(current model.HealthState, next model.HealthState) model.HealthState {
	rank := map[model.HealthState]int{
		model.HealthHealthy:  0,
		model.HealthDegraded: 1,
		model.HealthError:    2,
	}
	if rank[next] > rank[current] {
		return next
	}
	return current
}

var operationSequence uint64

func completedOperation(prefix string, message string) model.OperationResponse {
	id := atomic.AddUint64(&operationSequence, 1)
	return model.OperationResponse{
		OperationID: fmt.Sprintf("%s-%06d", prefix, id),
		Status:      model.OperationCompleted,
		Message:     message,
	}
}

func wrapUnavailable(component string, err error) error {
	var apiErr *model.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return model.NewUnavailableError(component, err)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

func recordingsDir(path string) string {
	if path == "" {
		return "/data/recordings"
	}
	return filepath.Clean(path)
}
