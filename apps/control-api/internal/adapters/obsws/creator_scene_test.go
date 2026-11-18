package obsws

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

func TestInstallBrowserSceneCreatesInactiveSourceWithoutPublishing(t *testing.T) {
	t.Parallel()
	server := newFakeOBSServer(t)
	defer server.Close()
	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithTimeouts(500*time.Millisecond, 500*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.InstallBrowserScene(ctx, "BitRiver - Be Right Back", "BitRiver overlay - be-right-back",
		"file:///usr/local/share/bitriver/creator-assets/assets/scenes/be-right-back.html?title=My+Show", 1920, 1080)
	if err != nil { t.Fatalf("install: %v", err) }

	server.mu.Lock()
	created := len(server.scenes) == 4 && server.scenes[3] == "BitRiver - Be Right Back"
	sourceFound := false
	for _, input := range server.inputs {
		if input.Name == "BitRiver overlay - be-right-back" && input.Kind == "browser_source" { sourceFound = true }
	}
	gotURL := server.lastBrowserURL
	program := server.currentProgramScene
	live := server.streamActive
	server.mu.Unlock()

	if !created || !sourceFound || program != "Starting Soon" || live {
		t.Fatalf("installer changed broadcast state: created=%v source=%v program=%s live=%v", created, sourceFound, program, live)
	}
	if !strings.Contains(gotURL, "title=My+Show") { t.Fatalf("custom title lost in URL: %s", gotURL) }
	if err := client.InstallBrowserScene(ctx, "BitRiver - Be Right Back", "BitRiver overlay - be-right-back", gotURL, 1920, 1080); !errors.Is(err, adapters.ErrSceneInstallCollision) {
		t.Fatalf("duplicate must not overwrite OBS scene: %v", err)
	}
	if err := client.InstallBrowserScene(ctx, "Unsafe", "browser", "https://host.example/remote.html", 1920, 1080); err == nil {
		t.Fatal("remote URLs must never be accepted for asset installation")
	}
}

func TestInstallBrowserSceneBlocksActiveStreamAndRecording(t *testing.T) {
	t.Parallel()
	server := newFakeOBSServer(t)
	defer server.Close()
	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithTimeouts(500*time.Millisecond, 500*time.Millisecond))

	for _, recording := range []bool{false, true} {
		server.mu.Lock()
		server.streamActive = !recording
		server.recordActive = recording
		server.mu.Unlock()
		err := client.InstallBrowserScene(context.Background(), "BitRiver - Test", "BitRiver overlay - test", "file:///tmp/test.html", 1920, 1080)
		if !errors.Is(err, adapters.ErrSceneInstallBusy) { t.Fatalf("expected busy OBS rejection (recording=%v): %v", recording, err) }
		server.mu.Lock()
		size := len(server.scenes)
		server.mu.Unlock()
		if size != 3 { t.Fatalf("scene count changed while on-air: %d", size) }
	}
}

func TestInstallBrowserSceneRollsBackOnInputFailure(t *testing.T) {
	t.Parallel()
	server := newFakeOBSServer(t)
	server.mu.Lock()
	server.failCreateInput = true
	server.mu.Unlock()
	defer server.Close()
	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithTimeouts(500*time.Millisecond, 500*time.Millisecond))

	err := client.InstallBrowserScene(context.Background(), "BitRiver - Test", "BitRiver overlay - test", "file:///tmp/test.html", 1920, 1080)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("expected clear input failure and rollback, got %v", err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.scenes) != 3 || server.currentProgramScene != "Starting Soon" {
		t.Fatalf("failed input creation left a scene behind: %#v", server.scenes)
	}
}

func TestInstallBrowserSceneRejectsMissingPluginWithoutMutation(t *testing.T) {
	t.Parallel()
	server := newFakeOBSServer(t)
	server.mu.Lock()
	server.browserPluginMissing = true
	server.mu.Unlock()
	defer server.Close()
	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithTimeouts(500*time.Millisecond, 500*time.Millisecond))

	err := client.InstallBrowserScene(context.Background(), "BitRiver - Test",
		"BitRiver overlay - test", "file:///tmp/test.html", 1920, 1080)
	if !errors.Is(err, adapters.ErrBrowserSourceUnavailable) {
		t.Fatalf("missing browser plugin must fail explicitly: %v", err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.scenes) != 3 || server.currentProgramScene != "Starting Soon" {
		t.Fatalf("plugin failure mutated OBS scene state: %#v", server.scenes)
	}
}
