package mediamtx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func TestApplyConfigWritesManagedFilesAndFallsBackToRestart(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mediamtx.yml")
	destinationsPath := filepath.Join(tempDir, "mediamtx-destinations.json")
	runner := &fakeRunner{
		results: []fakeResult{
			{err: errors.New("reload unsupported")},
			{},
		},
	}

	client := New("http://127.0.0.1:9997",
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithConfigPath(configPath),
		WithDestinationsPath(destinationsPath),
		WithCommandRunner(runner),
	)

	config := model.DefaultConfig()
	config.MediaRouter.WebRTCPreviewEnabled = true
	config.MediaRouter.PublishDestinations = append(config.MediaRouter.PublishDestinations, model.ConfigMediaDestination{
		ID:        "backup-srt",
		Name:      "Backup Relay",
		Protocol:  "srt",
		TargetURL: "srt://relay.example.com:9000?streamid=publish:backup",
		Enabled:   true,
	})

	result, err := client.ApplyConfig(context.Background(), "gen-router", config)
	if err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if result.ReloadMode != "restart" {
		t.Fatalf("expected restart fallback, got %s", result.ReloadMode)
	}

	managedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read managed config: %v", err)
	}
	configText := string(managedConfig)
	for _, expected := range []string{
		"program-live:",
		"program-preview:",
		"webrtc: yes",
		"runOnReady:",
		"guest-1:",
	} {
		if !strings.Contains(configText, expected) {
			t.Fatalf("managed config missing %q", expected)
		}
	}

	sidecarPayload, err := os.ReadFile(destinationsPath)
	if err != nil {
		t.Fatalf("read destinations sidecar: %v", err)
	}
	var sidecar destinationsDocument
	if err := json.Unmarshal(sidecarPayload, &sidecar); err != nil {
		t.Fatalf("decode destinations sidecar: %v", err)
	}
	if sidecar.ConfigGeneration != "gen-router" {
		t.Fatalf("unexpected sidecar generation: %s", sidecar.ConfigGeneration)
	}
	if len(sidecar.PublishDestinations) != 2 {
		t.Fatalf("unexpected destination count: %d", len(sidecar.PublishDestinations))
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 runner calls, got %d", len(runner.calls))
	}
	if runner.calls[0] != "systemctl reload mediamtx.service" {
		t.Fatalf("unexpected first runner call: %s", runner.calls[0])
	}
	if runner.calls[1] != "systemctl restart mediamtx.service" {
		t.Fatalf("unexpected second runner call: %s", runner.calls[1])
	}
}

func TestApplyConfigRestoresPreviousFilesWhenServiceControlFails(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mediamtx.yml")
	destinationsPath := filepath.Join(tempDir, "mediamtx-destinations.json")
	if err := os.WriteFile(configPath, []byte("old-config\n"), 0o640); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := os.WriteFile(destinationsPath, []byte("{\"old\":true}\n"), 0o640); err != nil {
		t.Fatalf("seed destinations: %v", err)
	}

	runner := &fakeRunner{
		results: []fakeResult{
			{err: errors.New("reload unsupported")},
			{err: errors.New("restart failed")},
			{},
		},
	}

	client := New("http://127.0.0.1:9997",
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithConfigPath(configPath),
		WithDestinationsPath(destinationsPath),
		WithCommandRunner(runner),
	)

	_, err := client.ApplyConfig(context.Background(), "gen-fail", model.DefaultConfig())
	if err == nil {
		t.Fatal("expected apply config to fail")
	}

	configPayload, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(configPayload) != "old-config\n" {
		t.Fatalf("managed config was not restored: %q", string(configPayload))
	}

	destinationsPayload, err := os.ReadFile(destinationsPath)
	if err != nil {
		t.Fatalf("read restored destinations: %v", err)
	}
	if string(destinationsPayload) != "{\"old\":true}\n" {
		t.Fatalf("destinations sidecar was not restored: %q", string(destinationsPayload))
	}
}

func TestSnapshotNormalizesPathsSessionsAndDestinationState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v3/paths/list":
			writeJSON(t, writer, map[string]any{
				"items": []map[string]any{
					{
						"name":          "program-live",
						"source":        "publisher",
						"sourceReady":   true,
						"bytesReceived": 250000.0,
						"readers":       []any{map[string]any{"id": "reader-1"}},
					},
					{
						"name":          "program-preview",
						"source":        "rtsp://127.0.0.1:8554/program-live",
						"sourceReady":   true,
						"bytesReceived": 0.0,
						"readers":       []any{map[string]any{"id": "reader-2"}, map[string]any{"id": "reader-3"}},
					},
				},
			})
		case "/v3/rtmpconns/list":
			writeJSON(t, writer, map[string]any{
				"items": []map[string]any{
					{
						"id":         "rtmp-1",
						"path":       "program-live",
						"remoteAddr": "192.168.50.20:50512",
						"state":      "publish",
						"created":    "2026-03-29T12:34:56Z",
					},
				},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	runner := &fakeRunner{}
	client := New(server.URL,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithHTTPClient(server.Client()),
		WithConfigPath(filepath.Join(tempDir, "mediamtx.yml")),
		WithDestinationsPath(filepath.Join(tempDir, "mediamtx-destinations.json")),
		WithCommandRunner(runner),
	)

	if _, err := client.ApplyConfig(context.Background(), "gen-snapshot", model.DefaultConfig()); err != nil {
		t.Fatalf("apply config: %v", err)
	}

	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snapshot.Healthy {
		t.Fatal("expected snapshot to be healthy")
	}
	if len(snapshot.Paths) != 2 {
		t.Fatalf("unexpected path count: %d", len(snapshot.Paths))
	}
	if !snapshot.Destinations[0].Connected {
		t.Fatal("expected primary destination to be marked connected when ingest is ready")
	}
	if len(snapshot.Sessions) != 1 {
		t.Fatalf("unexpected session count: %d", len(snapshot.Sessions))
	}
	if snapshot.Sessions[0].Protocol != "rtmp" {
		t.Fatalf("unexpected session protocol: %s", snapshot.Sessions[0].Protocol)
	}
}

type fakeResult struct {
	output []byte
	err    error
}

type fakeRunner struct {
	calls   []string
	results []fakeResult
}

func (runner *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, strings.Join(append([]string{name}, args...), " "))
	if len(runner.results) == 0 {
		return nil, nil
	}
	result := runner.results[0]
	runner.results = runner.results[1:]
	return result.output, result.err
}

func writeJSON(t *testing.T, writer http.ResponseWriter, payload any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}
