package mock

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

type Suite struct {
	mu       sync.RWMutex
	obs      adapters.OBSSnapshot
	media    adapters.MediaMTXSnapshot
	owncast  adapters.OwncastStatus
	services []adapters.UnitStatus
}

func NewSuite(now time.Time) *Suite {
	lastChanged := now.UTC()
	return &Suite{
		obs: adapters.OBSSnapshot{
			Version: adapters.OBSVersion{
				OBSVersion:             "30.2.0",
				WebSocketVersion:       "5.4.0",
				Platform:               "linux",
				PlatformDescription:    "BitRiver mock OBS",
				RPCVersion:             1,
				AuthenticationRequired: true,
				AvailableRequests: []string{
					"GetVersion",
					"GetSceneList",
					"GetCurrentProgramScene",
					"SetCurrentProgramScene",
					"GetInputList",
					"GetInputMute",
					"SetInputMute",
					"ToggleInputMute",
					"GetInputVolume",
					"SetInputVolume",
					"GetStreamStatus",
					"StartStream",
					"StopStream",
					"GetRecordStatus",
					"StartRecord",
					"StopRecord",
				},
				AvailableEvents: []string{
					"CurrentProgramSceneChanged",
					"InputMuteStateChanged",
					"InputVolumeChanged",
					"StreamStateChanged",
					"RecordStateChanged",
				},
			},
			Scenes: []adapters.OBSScene{
				{Name: "Starting Soon", Index: 0, Active: true},
				{Name: "Host Camera", Index: 1},
				{Name: "Host + Guest", Index: 2},
			},
			ActiveScene: "Starting Soon",
			StreamStatus: adapters.OBSStreamStatus{
				Active: false,
			},
			RecordStatus: adapters.OBSRecordStatus{
				Active: false,
				Path:   "/data/recordings",
			},
			StreamActive:    false,
			RecordingActive: false,
			RecordingPath:   "/data/recordings",
			Inputs: []adapters.OBSInput{
				{Name: "Host Mic", Kind: "pulse_input_capture", HasAudio: true, Muted: false, GainDB: 0, PeakDB: -9.0, Available: true},
				{Name: "Game Mix", Kind: "pulse_input_capture", HasAudio: true, Muted: false, GainDB: -2.5, PeakDB: -12.0, Available: true},
			},
			LastChangedAt: &lastChanged,
		},
		media: adapters.MediaMTXSnapshot{
			Ready:                true,
			Healthy:              true,
			Detail:               "MediaMTX mock adapter is healthy",
			ConfigPath:           "/etc/mediamtx/mediamtx.yml",
			ConfigGeneration:     "gen-bootstrap",
			LocalIngestPath:      "program-live",
			LocalPreviewPath:     "program-preview",
			RecordingPath:        "/data/recordings/program",
			WebRTCPreviewEnabled: false,
			Destinations: []adapters.MediaDestination{
				{
					ID:        "primary-rtmp",
					Name:      "Twitch",
					Protocol:  "rtmp",
					TargetURL: "rtmp://live.twitch.tv/app",
					Enabled:   true,
					Connected: false,
					Detail:    "Program router is idle",
				},
			},
			Paths: []adapters.MediaPath{
				{
					Name:        "program-live",
					Source:      "publisher",
					Readers:     1,
					Publisher:   false,
					Ready:       false,
					BitrateKbps: 0,
					PublishURL:  "rtmp://127.0.0.1:1935/program-live",
					Detail:      "Awaiting OBS publish",
				},
				{
					Name:        "guest-1",
					Source:      "publisher",
					Readers:     0,
					Publisher:   false,
					Ready:       false,
					BitrateKbps: 0,
					PublishURL:  "srt://bitriver.local:8890?streamid=publish:guest-1",
					Detail:      "Awaiting contributor publish",
				},
			},
		},
		owncast: adapters.OwncastStatus{
			Connected: false,
			Detail:    "Owncast disabled or not in use",
		},
		services: []adapters.UnitStatus{
			{Name: "bitriver-control-api.service", Active: true, State: "active", Detail: "running"},
			{Name: "mediamtx.service", Active: true, State: "active", Detail: "running"},
			{Name: "bitriver-update-agent.service", Active: true, State: "active", Detail: "running"},
		},
	}
}

type OBSAdapter struct {
	suite *Suite
}

type MediaMTXAdapter struct {
	suite *Suite
}

type OwncastAdapter struct {
	suite *Suite
}

type ServicesAdapter struct {
	suite *Suite
}

func (suite *Suite) OBS() *OBSAdapter {
	return &OBSAdapter{suite: suite}
}

func (suite *Suite) MediaMTX() *MediaMTXAdapter {
	return &MediaMTXAdapter{suite: suite}
}

func (suite *Suite) Owncast() *OwncastAdapter {
	return &OwncastAdapter{suite: suite}
}

func (suite *Suite) Services() *ServicesAdapter {
	return &ServicesAdapter{suite: suite}
}

func (adapter *OBSAdapter) Snapshot(context.Context) (adapters.OBSSnapshot, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return cloneOBS(suite.obs), nil
}

func (adapter *OBSAdapter) Version(context.Context) (adapters.OBSVersion, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return cloneVersion(suite.obs.Version), nil
}

func (adapter *OBSAdapter) Scenes(context.Context) ([]adapters.OBSScene, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return append([]adapters.OBSScene(nil), suite.obs.Scenes...), nil
}

func (adapter *OBSAdapter) CurrentProgramScene(context.Context) (string, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return suite.obs.ActiveScene, nil
}

func (adapter *OBSAdapter) Inputs(context.Context) ([]adapters.OBSInput, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return append([]adapters.OBSInput(nil), suite.obs.Inputs...), nil
}

func (adapter *OBSAdapter) StreamStatus(context.Context) (adapters.OBSStreamStatus, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return cloneStreamStatus(suite.obs.StreamStatus), nil
}

func (adapter *OBSAdapter) RecordStatus(context.Context) (adapters.OBSRecordStatus, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return cloneRecordStatus(suite.obs.RecordStatus), nil
}

func (adapter *OBSAdapter) StartStream(context.Context) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	if suite.obs.StreamActive {
		return errors.New("stream is already active")
	}
	suite.obs.StreamActive = true
	suite.obs.StreamStatus.Active = true
	suite.touchLocked()
	return nil
}

func (adapter *OBSAdapter) StopStream(context.Context) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	if !suite.obs.StreamActive {
		return errors.New("stream is already stopped")
	}
	suite.obs.StreamActive = false
	suite.obs.StreamStatus.Active = false
	suite.touchLocked()
	return nil
}

func (adapter *OBSAdapter) StartRecording(context.Context) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	if suite.obs.RecordingActive {
		return errors.New("recording is already active")
	}
	filename := "bitriver-session-" + time.Now().UTC().Format("20060102-150405") + ".mkv"
	suite.obs.RecordingActive = true
	suite.obs.RecordingFile = filename
	suite.obs.RecordStatus.Active = true
	suite.obs.RecordStatus.Filename = filename
	suite.touchLocked()
	return nil
}

func (adapter *OBSAdapter) StopRecording(context.Context) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	if !suite.obs.RecordingActive {
		return errors.New("recording is already stopped")
	}
	suite.obs.RecordingActive = false
	suite.obs.RecordStatus.Active = false
	suite.touchLocked()
	return nil
}

func (adapter *OBSAdapter) SetCurrentScene(_ context.Context, name string) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	found := false
	for index := range suite.obs.Scenes {
		suite.obs.Scenes[index].Active = suite.obs.Scenes[index].Name == name
		if suite.obs.Scenes[index].Active {
			found = true
		}
	}
	if !found {
		return errors.New("scene not found")
	}
	suite.obs.ActiveScene = name
	suite.touchLocked()
	return nil
}

func (adapter *OBSAdapter) SetInputMuted(_ context.Context, name string, muted bool) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	for index := range suite.obs.Inputs {
		if suite.obs.Inputs[index].Name == name {
			suite.obs.Inputs[index].Muted = muted
			suite.touchLocked()
			return nil
		}
	}
	return errors.New("input not found")
}

func (adapter *OBSAdapter) ToggleInputMuted(_ context.Context, name string) (bool, error) {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	for index := range suite.obs.Inputs {
		if suite.obs.Inputs[index].Name == name {
			suite.obs.Inputs[index].Muted = !suite.obs.Inputs[index].Muted
			suite.touchLocked()
			return suite.obs.Inputs[index].Muted, nil
		}
	}
	return false, errors.New("input not found")
}

func (adapter *OBSAdapter) SetInputVolume(_ context.Context, name string, gainDB float64) error {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()
	for index := range suite.obs.Inputs {
		if suite.obs.Inputs[index].Name == name {
			suite.obs.Inputs[index].GainDB = gainDB
			suite.obs.Inputs[index].PeakDB = gainDB - 3.0
			suite.touchLocked()
			return nil
		}
	}
	return errors.New("input not found")
}

func (adapter *MediaMTXAdapter) Snapshot(context.Context) (adapters.MediaMTXSnapshot, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return cloneMedia(suite.media), nil
}

func (adapter *MediaMTXAdapter) ApplyConfig(_ context.Context, generation string, config model.ControlConfig) (adapters.MediaApplyResult, error) {
	suite := adapter.suite
	suite.mu.Lock()
	defer suite.mu.Unlock()

	now := time.Now().UTC()
	config = model.NormalizeConfig(config)
	suite.media.ConfigGeneration = generation
	suite.media.LocalIngestPath = config.MediaRouter.LocalIngestPath
	suite.media.LocalPreviewPath = config.MediaRouter.LocalPreviewPath
	suite.media.RecordingPath = config.MediaRouter.RecordingPath
	suite.media.WebRTCPreviewEnabled = config.MediaRouter.WebRTCPreviewEnabled
	suite.media.LastAppliedAt = &now
	suite.media.LastReloadedAt = &now
	suite.media.LastConfigValidatedAt = &now
	suite.media.Detail = "MediaMTX mock configuration applied"
	suite.media.Healthy = true
	suite.media.Destinations = make([]adapters.MediaDestination, 0, len(config.MediaRouter.PublishDestinations))
	for _, destination := range config.MediaRouter.PublishDestinations {
		suite.media.Destinations = append(suite.media.Destinations, adapters.MediaDestination{
			ID:        destination.ID,
			Name:      destination.Name,
			Protocol:  destination.Protocol,
			TargetURL: destination.TargetURL,
			Enabled:   destination.Enabled,
			Connected: false,
			Detail:    "Program router is idle",
		})
	}

	if len(suite.media.Paths) == 0 {
		suite.media.Paths = []adapters.MediaPath{}
	}

	updatedPaths := make([]adapters.MediaPath, 0, len(suite.media.Paths))
	foundIngest := false
	foundPreview := false
	for _, path := range suite.media.Paths {
		switch path.Name {
		case config.MediaRouter.LocalIngestPath:
			path.Source = "publisher"
			path.PublishURL = "rtmp://127.0.0.1:1935/" + config.MediaRouter.LocalIngestPath
			foundIngest = true
		case config.MediaRouter.LocalPreviewPath:
			path.Source = "rtsp://127.0.0.1:8554/" + config.MediaRouter.LocalIngestPath
			foundPreview = true
		}
		updatedPaths = append(updatedPaths, path)
	}
	if !foundIngest {
		updatedPaths = append(updatedPaths, adapters.MediaPath{
			Name:        config.MediaRouter.LocalIngestPath,
			Source:      "publisher",
			Readers:     1,
			Publisher:   false,
			Ready:       false,
			BitrateKbps: 0,
			PublishURL:  "rtmp://127.0.0.1:1935/" + config.MediaRouter.LocalIngestPath,
			Detail:      "Awaiting OBS publish",
		})
	}
	if !foundPreview {
		updatedPaths = append(updatedPaths, adapters.MediaPath{
			Name:        config.MediaRouter.LocalPreviewPath,
			Source:      "rtsp://127.0.0.1:8554/" + config.MediaRouter.LocalIngestPath,
			Readers:     0,
			Publisher:   false,
			Ready:       false,
			BitrateKbps: 0,
			Detail:      "Local preview path is idle",
		})
	}
	suite.media.Paths = updatedPaths

	return adapters.MediaApplyResult{
		ConfigGeneration: generation,
		ConfigPath:       suite.media.ConfigPath,
		ReloadMode:       "mock",
		AppliedAt:        now,
	}, nil
}

func (adapter *OwncastAdapter) Status(context.Context) (adapters.OwncastStatus, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return suite.owncast, nil
}

func (adapter *ServicesAdapter) Status(_ context.Context, _ []string) ([]adapters.UnitStatus, error) {
	suite := adapter.suite
	suite.mu.RLock()
	defer suite.mu.RUnlock()
	return append([]adapters.UnitStatus(nil), suite.services...), nil
}

func (suite *Suite) touchLocked() {
	now := time.Now().UTC()
	suite.obs.LastChangedAt = &now
	suite.obs.StreamStatus.LastChangedAt = &now
	suite.obs.RecordStatus.LastChangedAt = &now
	for index := range suite.obs.Inputs {
		suite.obs.Inputs[index].LastChangedAt = &now
	}
}

func cloneOBS(in adapters.OBSSnapshot) adapters.OBSSnapshot {
	out := in
	out.Version = cloneVersion(in.Version)
	out.Scenes = append([]adapters.OBSScene(nil), in.Scenes...)
	out.Inputs = append([]adapters.OBSInput(nil), in.Inputs...)
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

func cloneMedia(in adapters.MediaMTXSnapshot) adapters.MediaMTXSnapshot {
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
