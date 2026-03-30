package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/mock"
	"github.com/bitriver/studio-os/apps/control-api/internal/configstore"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
	"github.com/bitriver/studio-os/apps/control-api/internal/service"
)

func TestSystemEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/system")
	if err != nil {
		t.Fatalf("get system: %v", err)
	}
	defer response.Body.Close()

	var payload model.SystemSummary
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode system: %v", err)
	}

	if payload.Name != "studio-a" {
		t.Fatalf("unexpected system name: %s", payload.Name)
	}
	if payload.Health.Overall == "" {
		t.Fatal("expected health state to be present")
	}
}

func TestStreamActionEndpoints(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/stream/start", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}
	response.Body.Close()

	streamResponse, err := http.Get(server.URL + "/api/v1/stream")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer streamResponse.Body.Close()

	var payload model.StreamSummary
	if err := json.NewDecoder(streamResponse.Body).Decode(&payload); err != nil {
		t.Fatalf("decode stream: %v", err)
	}
	if payload.State != model.StreamLive {
		t.Fatalf("expected live stream state, got %s", payload.State)
	}
}

func TestApplySettingsPromotesDraft(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	defer server.Close()

	settings := model.DefaultConfig()
	settings.System.Name = "studio-router-test"

	body, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}

	updateRequest, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/settings/draft", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new draft request: %v", err)
	}
	updateRequest.Header.Set("Content-Type", "application/json")

	updateResponse, err := http.DefaultClient.Do(updateRequest)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	updateResponse.Body.Close()

	applyRequest, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/settings/apply", nil)
	if err != nil {
		t.Fatalf("new apply request: %v", err)
	}

	applyResponse, err := http.DefaultClient.Do(applyRequest)
	if err != nil {
		t.Fatalf("apply draft: %v", err)
	}
	defer applyResponse.Body.Close()

	if applyResponse.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(applyResponse.Body)
		t.Fatalf("unexpected apply status: %d %s", applyResponse.StatusCode, string(payload))
	}

	activeResponse, err := http.Get(server.URL + "/api/v1/settings/active")
	if err != nil {
		t.Fatalf("get active settings: %v", err)
	}
	defer activeResponse.Body.Close()

	var active model.ConfigEnvelope
	if err := json.NewDecoder(activeResponse.Body).Decode(&active); err != nil {
		t.Fatalf("decode active settings: %v", err)
	}

	if active.Config.System.Name != "studio-router-test" {
		t.Fatalf("active settings not updated: %s", active.Config.System.Name)
	}
}

func TestSetupStatusEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/setup/status")
	if err != nil {
		t.Fatalf("get setup status: %v", err)
	}
	defer response.Body.Close()

	var payload model.SetupStatus
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if payload.AdminUsername != "admin" {
		t.Fatalf("unexpected admin username: %s", payload.AdminUsername)
	}
}

func TestMediaRouterEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/media-router")
	if err != nil {
		t.Fatalf("get media router: %v", err)
	}
	defer response.Body.Close()

	var payload model.MediaRouterSummary
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode media router: %v", err)
	}
	if payload.LocalIngestPath != "program-live" {
		t.Fatalf("unexpected local ingest path: %s", payload.LocalIngestPath)
	}
	if len(payload.Destinations) < 1 {
		t.Fatal("expected at least one media destination")
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	controller := newTestController(t)
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), controller, "")
	return httptest.NewServer(handler)
}

func newTestController(t *testing.T) *service.Controller {
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
	return service.NewController(service.Dependencies{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ConfigStore:  store,
		OBS:          suite.OBS(),
		MediaMTX:     suite.MediaMTX(),
		Owncast:      suite.Owncast(),
		ServiceUnits: suite.Services(),
		StartedAt:    now,
		Version:      "test-version",
	})
}

func TestControllerSnapshotIsAvailableForWebsocketBootstrap(t *testing.T) {
	t.Parallel()

	controller := newTestController(t)
	snapshot, err := controller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.Stream.State != model.StreamStopped {
		t.Fatalf("unexpected stream state: %s", snapshot.Stream.State)
	}
}
