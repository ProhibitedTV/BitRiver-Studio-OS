package service

import (
	"context"
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

func TestControllerInstalledOBSScenesCanBeSwitchedByStableIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	controller := newTestController(t)
	if err := controller.InstallBrowserScene(ctx, "BitRiver - Just Chatting", "BitRiver overlay - just-chatting",
		"file:///usr/local/share/bitriver/creator-assets/assets/scenes/just-chatting.html", 1920, 1080); err != nil {
		t.Fatalf("install OBS scene: %v", err)
	}

	before, err := controller.Scenes(ctx)
	if err != nil { t.Fatal(err) }
	if len(before.Items) != 4 {
		t.Fatalf("expected 3 configured + 1 newly installed scene, got %+v", before.Items)
	}
	if before.ActiveSceneID != "starting-soon" {
		t.Fatalf("install must not activate new scene: %+v", before)
	}
	installed := before.Items[3]
	if installed.Name != "BitRiver - Just Chatting" || installed.Active || len(installed.ID) != 68 ||
		installed.ID[:4] != "obs-" {
		t.Fatalf("invalid installed scene metadata: %+v", installed)
	}
	// Mapping is deterministic across calls, so the scene rail can keep the
	// same key even if OBS session restarts.
	again, err := controller.Scenes(ctx)
	if err != nil { t.Fatal(err) }
	if again.Items[3].ID != installed.ID { t.Fatal("OBS scene ID was not stable") }

	if _, err := controller.ActivateScene(ctx, installed.ID); err != nil {
		t.Fatalf("switch to installed scene: %v", err)
	}
	after, err := controller.Scenes(ctx)
	if err != nil { t.Fatal(err) }
	if after.ActiveSceneID != installed.ID || !after.Items[3].Active {
		t.Fatalf("installed scene is not active: %+v", after)
	}
	stream, err := controller.Stream(ctx)
	if err != nil { t.Fatal(err) }
	if stream.ActiveSceneID != installed.ID {
		t.Fatalf("stream summary lost installed scene: %+v", stream)
	}
	if _, err := controller.ActivateScene(ctx, "obs-invalid"); err == nil {
		t.Fatal("arbitrary names/IDs must not activate OBS scenes")
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
