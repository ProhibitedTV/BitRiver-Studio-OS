package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/mock"
	"github.com/bitriver/studio-os/apps/control-api/internal/configstore"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func TestControllerSceneActivationUsesConfiguredSceneIDs(t *testing.T) {
	t.Parallel()

	controller := newTestController(t)
	if _, err := controller.ActivateScene(context.Background(), "host-cam"); err != nil {
		t.Fatalf("activate scene: %v", err)
	}

	scenes, err := controller.Scenes(context.Background())
	if err != nil {
		t.Fatalf("scenes: %v", err)
	}
	if scenes.ActiveSceneID != "host-cam" {
		t.Fatalf("unexpected active scene: %s", scenes.ActiveSceneID)
	}
}

func TestControllerMuteUsesConfiguredAudioRouteIDs(t *testing.T) {
	t.Parallel()

	controller := newTestController(t)
	if _, err := controller.SetMute(context.Background(), "mic-host", true); err != nil {
		t.Fatalf("set mute: %v", err)
	}

	audio, err := controller.Audio(context.Background())
	if err != nil {
		t.Fatalf("audio: %v", err)
	}
	if !audio.Channels[0].Muted {
		t.Fatal("expected channel to be muted")
	}
}

func TestControllerGainUsesConfiguredAudioRouteIDs(t *testing.T) {
	t.Parallel()

	controller := newTestController(t)
	if _, err := controller.SetGain(context.Background(), "mic-host", -3.5); err != nil {
		t.Fatalf("set gain: %v", err)
	}

	audio, err := controller.Audio(context.Background())
	if err != nil {
		t.Fatalf("audio: %v", err)
	}
	if audio.Channels[0].GainDB != -3.5 {
		t.Fatalf("unexpected channel gain: %f", audio.Channels[0].GainDB)
	}
}

func TestControllerOutputsIncludeOptionalOwncastWhenEnabled(t *testing.T) {
	t.Parallel()

	controller := newTestController(t)
	config := controller.ActiveConfig().Config
	config.Owncast.Enabled = true
	if _, err := controller.UpdateDraft(config); err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if _, _, err := controller.ApplyDraft(context.Background()); err != nil {
		t.Fatalf("apply draft: %v", err)
	}

	outputs, err := controller.Outputs(context.Background())
	if err != nil {
		t.Fatalf("outputs: %v", err)
	}
	if len(outputs.Destinations) != 2 {
		t.Fatalf("expected 2 destinations, got %d", len(outputs.Destinations))
	}
}

func TestControllerMediaRouterReflectsAppliedGeneration(t *testing.T) {
	t.Parallel()

	controller := newTestController(t)
	config := controller.ActiveConfig().Config
	config.MediaRouter.LocalIngestPath = "program-main"
	if _, err := controller.UpdateDraft(config); err != nil {
		t.Fatalf("update draft: %v", err)
	}
	response, validation, err := controller.ApplyDraft(context.Background())
	if err != nil {
		t.Fatalf("apply draft: %v", err)
	}
	if validation != nil {
		t.Fatalf("unexpected validation response: %+v", validation)
	}

	mediaRouter := controller.MediaRouter(context.Background())
	if mediaRouter.LocalIngestPath != "program-main" {
		t.Fatalf("unexpected media router ingest path: %s", mediaRouter.LocalIngestPath)
	}
	if mediaRouter.ConfigGeneration != response.AppliedGeneration {
		t.Fatalf("unexpected media router generation: %s", mediaRouter.ConfigGeneration)
	}
}

func newTestController(t *testing.T) *Controller {
	t.Helper()

	now := time.Unix(0, 0).UTC()
	tempDir := t.TempDir()
	store, err := configstore.New(configstore.Paths{
		ActivePath: filepath.Join(tempDir, "active.json"),
		DraftPath:  filepath.Join(tempDir, "draft.json"),
	}, now)
	if err != nil {
		t.Fatalf("new config store: %v", err)
	}

	suite := mock.NewSuite(now)
	return NewController(Dependencies{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ConfigStore:  store,
		OBS:          suite.OBS(),
		MediaMTX:     suite.MediaMTX(),
		Owncast:      suite.Owncast(),
		ServiceUnits: suite.Services(),
		StartedAt:    now,
		Version:      "test-version",
		Paths:        RuntimePaths{},
	})
}

func TestDefaultUpdateStatusUsesVersion(t *testing.T) {
	t.Parallel()

	status := model.DefaultUpdateStatus(time.Unix(0, 0).UTC(), "test-version")
	if status.CurrentVersion != "test-version" {
		t.Fatalf("unexpected current version: %s", status.CurrentVersion)
	}
}

func TestApplyDraftRejectsActiveBroadcastAndRecording(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		start func(*Controller, context.Context) (model.OperationResponse, error)
		stop func(*Controller, context.Context) (model.OperationResponse, error)
	}{
		{"streaming", (*Controller).StartStream, (*Controller).StopStream},
		{"recording", (*Controller).StartRecording, (*Controller).StopRecording},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			controller := newTestController(t)
			original := controller.ActiveConfig()
			config := original.Config
			config.System.Name = "new-show-name"
			if _, err := controller.UpdateDraft(config); err != nil {
				t.Fatal(err)
			}
			if _, err := tc.start(controller, ctx); err != nil {
				t.Fatalf("activate OBS: %v", err)
			}
			_, validation, err := controller.ApplyDraft(ctx)
			if validation != nil {
				t.Fatalf("expected conflict, got validation %+v", validation)
			}
			var apiErr *model.APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 409 {
				t.Fatalf("expected HTTP 409 conflict while %s, got %v", tc.name, err)
			}
			if controller.ActiveConfig().Generation != original.Generation {
				t.Fatal("rejected settings apply changed active generation")
			}
			if controller.DraftConfig().Config.System.Name != "new-show-name" {
				t.Fatal("rejected settings apply discarded the draft")
			}
			if _, err := tc.stop(controller, ctx); err != nil {
				t.Fatalf("deactivate OBS: %v", err)
			}
			_, validation, err = controller.ApplyDraft(ctx)
			if err != nil || validation != nil {
				t.Fatalf("expected apply after OBS stops: validation=%+v err=%v", validation, err)
			}
			if controller.ActiveConfig().Config.System.Name != "new-show-name" {
				t.Fatal("off-air apply did not promote draft")
			}
		})
	}
}

func TestApplyDraftFailsClosedWhenOBSUnavailable(t *testing.T) {
	t.Parallel()
	controller := newTestController(t)
	initial := controller.ActiveConfig().Generation
	controller.obs = nil // an unreachable OBS adapter must not permit a router restart
	_, validation, err := controller.ApplyDraft(context.Background())
	if validation != nil {
		t.Fatalf("unexpected config validation error: %+v", validation)
	}
	var apiErr *model.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 503 {
		t.Fatalf("expected OBS-unavailable 503, got %v", err)
	}
	if controller.ActiveConfig().Generation != initial {
		t.Fatal("OBS-unavailable apply altered active settings")
	}
}
