package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var operationCounter uint64

type StateStore struct {
	mu        sync.RWMutex
	startedAt time.Time
	version   string
	system    SystemSummary
	show      ShowState
	sources   []SourceSummary
	audio     AudioState
	storage   StorageSummary
	active    ConfigEnvelope
	draft     ConfigEnvelope
}

func NewStateStore(startedAt time.Time, version string) *StateStore {
	config := DefaultConfig()
	active := newEnvelope("gen-bootstrap", startedAt, config)

	return &StateStore{
		startedAt: startedAt.UTC(),
		version:   version,
		system: SystemSummary{
			ApplianceID: "studio-appliance-dev",
			Name:        config.System.Name,
			Version:     version,
			BootMode:    "development",
			StartedAt:   startedAt.UTC(),
			Health: HealthSummary{
				Overall: HealthHealthy,
				Checks: []HealthCheck{
					{Key: "control_api", State: HealthHealthy, Message: "control API responding"},
					{Key: "obs", State: HealthDegraded, Message: "OBS adapter not wired in scaffold"},
					{Key: "mediamtx", State: HealthDegraded, Message: "MediaMTX adapter not wired in scaffold"},
				},
			},
			Network: NetworkSummary{
				LANAddress: "127.0.0.1",
				Hostname:   "bitriver.local",
			},
		},
		show:    showFromConfig(config),
		sources: sourcesFromConfig(config),
		audio:   audioFromConfig(config),
		storage: StorageSummary{
			RecordingPath: config.Recording.Path,
			TotalGB:       500,
			UsedGB:        84,
			FreeGB:        416,
		},
		active: active,
		draft:  newEnvelope("draft-next", startedAt, config),
	}
}

func (store *StateStore) System() SystemSummary {
	store.mu.RLock()
	defer store.mu.RUnlock()

	snapshot := store.system
	snapshot.UptimeSeconds = int64(time.Since(store.startedAt).Seconds())
	return snapshot
}

func (store *StateStore) Show() ShowState {
	store.mu.RLock()
	defer store.mu.RUnlock()

	snapshot := store.show
	snapshot.Scenes = cloneScenes(store.show.Scenes)
	return snapshot
}

func (store *StateStore) Sources() []SourceSummary {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return cloneSources(store.sources)
}

func (store *StateStore) Audio() AudioState {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return AudioState{Channels: cloneAudioChannels(store.audio.Channels)}
}

func (store *StateStore) Storage() StorageSummary {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage
}

func (store *StateStore) ActiveConfig() ConfigEnvelope {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return newEnvelope(store.active.Generation, store.active.UpdatedAt, store.active.Config)
}

func (store *StateStore) DraftConfig() ConfigEnvelope {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return newEnvelope(store.draft.Generation, store.draft.UpdatedAt, store.draft.Config)
}

func (store *StateStore) UpdateDraft(config ControlConfig) ConfigEnvelope {
	store.mu.Lock()
	defer store.mu.Unlock()

	now := time.Now().UTC()
	store.draft = newEnvelope("draft-"+now.Format("20060102-150405"), now, config)
	return newEnvelope(store.draft.Generation, store.draft.UpdatedAt, store.draft.Config)
}

func (store *StateStore) ValidateDraft() ValidateConfigResponse {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return ValidateConfig(store.draft.Config)
}

func (store *StateStore) ApplyDraft() (OperationResponse, *ValidateConfigResponse) {
	store.mu.Lock()
	defer store.mu.Unlock()

	validation := ValidateConfig(store.draft.Config)
	if !validation.Valid {
		return OperationResponse{}, &validation
	}

	now := time.Now().UTC()
	generation := "gen-" + now.Format("20060102-150405")
	store.active = newEnvelope(generation, now, store.draft.Config)
	store.draft = newEnvelope("draft-next", now, store.active.Config)
	store.show = reconcileShow(store.show, store.active.Config)
	store.sources = sourcesFromConfig(store.active.Config)
	store.audio = audioFromConfig(store.active.Config)
	store.storage.RecordingPath = store.active.Config.Recording.Path
	store.system.Name = store.active.Config.System.Name

	return OperationResponse{
		OperationID:       nextOperationID("apply"),
		Status:            OperationCompleted,
		Message:           "draft promoted to active generation",
		AppliedGeneration: generation,
	}, nil
}

func (store *StateStore) ActivateScene(sceneID string) (OperationResponse, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	for _, scene := range store.show.Scenes {
		if scene.ID == sceneID {
			store.show.ActiveSceneID = sceneID
			return completedOperation("scene_activate", "scene activated"), nil
		}
	}

	return OperationResponse{}, errors.New("scene not found")
}

func (store *StateStore) StartStream() OperationResponse {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.show.StreamState = StreamLive
	store.show.Destination.Connected = true
	return completedOperation("stream_start", "stream started")
}

func (store *StateStore) StopStream() OperationResponse {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.show.StreamState = StreamStopped
	store.show.Destination.Connected = false
	return completedOperation("stream_stop", "stream stopped")
}

func (store *StateStore) StartRecording() OperationResponse {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.show.RecordState = RecordRecording
	return completedOperation("recording_start", "recording started")
}

func (store *StateStore) StopRecording() OperationResponse {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.show.RecordState = RecordStopped
	return completedOperation("recording_stop", "recording stopped")
}

func (store *StateStore) SetMute(channelID string, muted bool) (OperationResponse, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	for index := range store.audio.Channels {
		if store.audio.Channels[index].ID == channelID {
			store.audio.Channels[index].Muted = muted
			message := "channel muted"
			if !muted {
				message = "channel unmuted"
			}
			return completedOperation("audio_mute", message), nil
		}
	}

	return OperationResponse{}, errors.New("audio channel not found")
}

func (store *StateStore) SetGain(channelID string, gainDB float64) (OperationResponse, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	for index := range store.audio.Channels {
		if store.audio.Channels[index].ID == channelID {
			store.audio.Channels[index].GainDB = gainDB
			store.audio.Channels[index].PeakDB = gainDB - 3.0
			return completedOperation("audio_gain", fmt.Sprintf("gain set to %.1f dB", gainDB)), nil
		}
	}

	return OperationResponse{}, errors.New("audio channel not found")
}

func nextOperationID(prefix string) string {
	sequence := atomic.AddUint64(&operationCounter, 1)
	return fmt.Sprintf("%s-%06d", prefix, sequence)
}

func completedOperation(prefix string, message string) OperationResponse {
	return OperationResponse{
		OperationID: nextOperationID(prefix),
		Status:      OperationCompleted,
		Message:     message,
	}
}

func showFromConfig(config ControlConfig) ShowState {
	scenes := make([]SceneSummary, 0, len(config.Scenes))
	for _, scene := range config.Scenes {
		scenes = append(scenes, SceneSummary{
			ID:   scene.ID,
			Name: scene.Name,
		})
	}

	activeSceneID := ""
	if len(scenes) > 0 {
		activeSceneID = scenes[0].ID
	}

	return ShowState{
		ActiveSceneID: activeSceneID,
		StreamState:   StreamStopped,
		RecordState:   RecordStopped,
		Destination: DestinationSummary{
			Mode:      config.Destination.Mode,
			Label:     config.Destination.TargetLabel,
			Target:    config.Destination.TargetURL,
			Connected: false,
		},
		Scenes: scenes,
	}
}

func reconcileShow(current ShowState, config ControlConfig) ShowState {
	next := showFromConfig(config)
	next.StreamState = current.StreamState
	next.RecordState = current.RecordState
	next.Destination.Connected = current.Destination.Connected

	for _, scene := range next.Scenes {
		if scene.ID == current.ActiveSceneID {
			next.ActiveSceneID = current.ActiveSceneID
			return next
		}
	}

	return next
}

func sourcesFromConfig(config ControlConfig) []SourceSummary {
	sources := make([]SourceSummary, 0, len(config.Sources))
	for _, source := range config.Sources {
		detail := source.DeviceRef
		status := SourceReady
		bitrate := 0

		switch source.Kind {
		case "local_video":
			detail = "Local capture ready"
		case "local_audio":
			detail = "Local audio input ready"
		case "browser_source":
			detail = "Browser overlay source configured"
		case "network_ingest":
			detail = "Awaiting contributor publish"
			status = SourceWarning
		}

		sources = append(sources, SourceSummary{
			ID:          source.ID,
			Name:        source.Name,
			Kind:        source.Kind,
			Status:      status,
			Detail:      detail,
			BitrateKbps: bitrate,
			PublishURL:  source.PublishURL,
		})
	}

	return sources
}

func audioFromConfig(config ControlConfig) AudioState {
	channels := make([]AudioChannelSummary, 0, len(config.Audio.Channels))
	for index, channel := range config.Audio.Channels {
		channels = append(channels, AudioChannelSummary{
			ID:     channel.ID,
			Name:   channel.Name,
			Muted:  false,
			GainDB: 0,
			PeakDB: float64(-6 - index),
			Source: channel.DeviceRef,
		})
	}

	return AudioState{Channels: channels}
}
