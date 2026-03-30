package obsws

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

func (client *Client) runSession(ctx context.Context) error {
	sessionCtx, cancel := context.WithTimeout(ctx, client.connectTimeout)
	connection, hello, err := client.connect(sessionCtx, subscriptionGeneral|subscriptionScenes|subscriptionInputs|subscriptionOutputs)
	cancel()
	if err != nil {
		return err
	}
	defer connection.close()
	go func() {
		<-ctx.Done()
		connection.close()
	}()

	if _, err := client.refreshSnapshot(ctx); err != nil {
		client.logger.Warn("initial OBS cache refresh failed", "error", err)
	}

	client.mu.Lock()
	client.connected = true
	client.lastError = nil
	client.version.AuthenticationRequired = hello.AuthenticationRequired
	client.mu.Unlock()

	client.publishConnectionEvent("obs.connection.connected", nil)
	client.logger.Info("obs-websocket connected", "endpoint", client.endpoint)

	for {
		envelope, err := connection.readEnvelope()
		if err != nil {
			return err
		}

		if envelope.Op != 5 {
			continue
		}

		if err := client.handleEvent(ctx, envelope.Data.EventType, envelope.Data.EventData); err != nil {
			client.logger.Warn("obs event handling failed", "eventType", envelope.Data.EventType, "error", err)
		}
	}
}

func (client *Client) handleEvent(ctx context.Context, eventType string, payload []byte) error {
	now := time.Now().UTC()

	switch eventType {
	case "CurrentProgramSceneChanged":
		var event struct {
			SceneName string `json:"sceneName"`
		}
		if err := decodeJSON(payload, &event); err != nil {
			return err
		}
		client.mu.Lock()
		client.snapshot.ActiveScene = event.SceneName
		client.snapshot.LastChangedAt = &now
		for index := range client.snapshot.Scenes {
			client.snapshot.Scenes[index].Active = client.snapshot.Scenes[index].Name == event.SceneName
		}
		client.mu.Unlock()

		client.publish("obs.scene.current_program_changed", adapters.OBSEvent{
			Type:      "obs.scene.current_program_changed",
			Timestamp: now,
			SceneName: event.SceneName,
		})
	case "InputMuteStateChanged":
		var event struct {
			InputName  string `json:"inputName"`
			InputMuted bool   `json:"inputMuted"`
		}
		if err := decodeJSON(payload, &event); err != nil {
			return err
		}
		client.mu.Lock()
		for index := range client.snapshot.Inputs {
			if client.snapshot.Inputs[index].Name == event.InputName {
				client.snapshot.Inputs[index].HasAudio = true
				client.snapshot.Inputs[index].Muted = event.InputMuted
				client.snapshot.Inputs[index].Available = true
				client.snapshot.Inputs[index].LastChangedAt = &now
			}
		}
		client.snapshot.LastChangedAt = &now
		client.mu.Unlock()

		muted := event.InputMuted
		client.publish("obs.input.mute_changed", adapters.OBSEvent{
			Type:       "obs.input.mute_changed",
			Timestamp:  now,
			InputName:  event.InputName,
			InputMuted: &muted,
		})
	case "InputVolumeChanged":
		var event struct {
			InputName     string  `json:"inputName"`
			InputVolumeDb float64 `json:"inputVolumeDb"`
		}
		if err := decodeJSON(payload, &event); err != nil {
			return err
		}
		client.mu.Lock()
		for index := range client.snapshot.Inputs {
			if client.snapshot.Inputs[index].Name == event.InputName {
				client.snapshot.Inputs[index].HasAudio = true
				client.snapshot.Inputs[index].GainDB = event.InputVolumeDb
				client.snapshot.Inputs[index].PeakDB = event.InputVolumeDb - 3
				client.snapshot.Inputs[index].Available = true
				client.snapshot.Inputs[index].LastChangedAt = &now
			}
		}
		client.snapshot.LastChangedAt = &now
		client.mu.Unlock()

		gainDB := event.InputVolumeDb
		client.publish("obs.input.volume_changed", adapters.OBSEvent{
			Type:        "obs.input.volume_changed",
			Timestamp:   now,
			InputName:   event.InputName,
			InputGainDB: &gainDB,
		})
	case "StreamStateChanged":
		var event struct {
			OutputActive        bool    `json:"outputActive"`
			OutputReconnecting  bool    `json:"outputReconnecting"`
			OutputTimecode      string  `json:"outputTimecode"`
			OutputDuration      float64 `json:"outputDuration"`
			OutputBytes         int64   `json:"outputBytes"`
			OutputSkippedFrames int64   `json:"outputSkippedFrames"`
			OutputTotalFrames   int64   `json:"outputTotalFrames"`
		}
		if err := decodeJSON(payload, &event); err != nil {
			return err
		}
		status := adapters.OBSStreamStatus{
			Active:          event.OutputActive,
			Reconnecting:    event.OutputReconnecting,
			Timecode:        event.OutputTimecode,
			DurationSeconds: int64(event.OutputDuration / 1000),
			BytesPerSecond:  event.OutputBytes,
			FramesSkipped:   event.OutputSkippedFrames,
			FramesTotal:     event.OutputTotalFrames,
			LastChangedAt:   &now,
		}

		client.mu.Lock()
		client.snapshot.StreamStatus = status
		client.snapshot.StreamActive = status.Active
		client.snapshot.LastChangedAt = &now
		client.mu.Unlock()

		client.publish("obs.stream.state_changed", adapters.OBSEvent{
			Type:      "obs.stream.state_changed",
			Timestamp: now,
			Stream:    &status,
		})
	case "RecordStateChanged":
		var event struct {
			OutputActive   bool    `json:"outputActive"`
			OutputPaused   bool    `json:"outputPaused"`
			OutputPath     string  `json:"outputPath"`
			OutputTimecode string  `json:"outputTimecode"`
			OutputDuration float64 `json:"outputDuration"`
			OutputBytes    int64   `json:"outputBytes"`
		}
		if err := decodeJSON(payload, &event); err != nil {
			return err
		}
		status := adapters.OBSRecordStatus{
			Active:          event.OutputActive,
			Paused:          event.OutputPaused,
			Path:            recordDirectory(event.OutputPath),
			Filename:        recordFilename(event.OutputPath),
			Timecode:        event.OutputTimecode,
			DurationSeconds: int64(event.OutputDuration / 1000),
			Bytes:           event.OutputBytes,
			LastChangedAt:   &now,
		}

		client.mu.Lock()
		client.snapshot.RecordStatus = status
		client.snapshot.RecordingActive = status.Active
		client.snapshot.RecordingPath = status.Path
		client.snapshot.RecordingFile = status.Filename
		client.snapshot.LastChangedAt = &now
		client.mu.Unlock()

		client.publish("obs.record.state_changed", adapters.OBSEvent{
			Type:      "obs.record.state_changed",
			Timestamp: now,
			Record:    &status,
		})
	case "SceneListChanged":
		client.publish("obs.scene.list_changed", adapters.OBSEvent{
			Type:      "obs.scene.list_changed",
			Timestamp: now,
		})
		client.triggerRefresh()
	case "InputCreated", "InputRemoved", "InputNameChanged":
		client.publish("obs.inputs.changed", adapters.OBSEvent{
			Type:      "obs.inputs.changed",
			Timestamp: now,
		})
		client.triggerRefresh()
	case "ExitStarted":
		return errors.New("OBS exit started")
	default:
		client.publish("obs.event", adapters.OBSEvent{
			Type:      "obs.event",
			Timestamp: now,
		})
	}

	return nil
}

func recordDirectory(outputPath string) string {
	if outputPath == "" {
		return ""
	}
	cleaned := filepath.Clean(outputPath)
	if strings.Contains(filepath.Base(cleaned), ".") {
		return filepath.Dir(cleaned)
	}
	return cleaned
}

func recordFilename(outputPath string) string {
	if outputPath == "" {
		return ""
	}
	cleaned := filepath.Clean(outputPath)
	base := filepath.Base(cleaned)
	if strings.Contains(base, ".") {
		return base
	}
	return ""
}
