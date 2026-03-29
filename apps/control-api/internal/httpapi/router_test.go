package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/core"
)

func TestSystemEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(NewHandler(core.NewStateStore(time.Unix(0, 0), "test-version"), ""))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/system")
	if err != nil {
		t.Fatalf("get system: %v", err)
	}
	defer response.Body.Close()

	var payload core.SystemSummary
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode system: %v", err)
	}

	if payload.Name != "studio-a" {
		t.Fatalf("unexpected system name: %s", payload.Name)
	}

	if payload.Health.Overall != core.HealthHealthy {
		t.Fatalf("unexpected health: %s", payload.Health.Overall)
	}
}

func TestValidateRejectsDuplicateSourceIDs(t *testing.T) {
	t.Parallel()

	store := core.NewStateStore(time.Unix(0, 0), "test-version")
	config := core.DefaultConfig()
	config.Sources = append(config.Sources, config.Sources[0])
	store.UpdateDraft(config)

	server := httptest.NewServer(NewHandler(store, ""))
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/config/validate", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
	defer response.Body.Close()

	var payload core.ValidateConfigResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode validation: %v", err)
	}

	if payload.Valid {
		t.Fatal("expected validation failure for duplicate source ids")
	}
}

func TestApplyPromotesDraftToActive(t *testing.T) {
	t.Parallel()

	store := core.NewStateStore(time.Unix(0, 0), "test-version")
	server := httptest.NewServer(NewHandler(store, ""))
	defer server.Close()

	config := core.DefaultConfig()
	config.System.Name = "studio-b"

	body, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	updateRequest, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/config/draft", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new draft request: %v", err)
	}
	updateRequest.Header.Set("Content-Type", "application/json")

	updateResponse, err := http.DefaultClient.Do(updateRequest)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	updateResponse.Body.Close()

	applyRequest, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/config/apply", nil)
	if err != nil {
		t.Fatalf("new apply request: %v", err)
	}

	applyResponse, err := http.DefaultClient.Do(applyRequest)
	if err != nil {
		t.Fatalf("apply request: %v", err)
	}
	defer applyResponse.Body.Close()

	if applyResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected apply status: %d", applyResponse.StatusCode)
	}

	activeResponse, err := http.Get(server.URL + "/api/v1/config/active")
	if err != nil {
		t.Fatalf("get active config: %v", err)
	}
	defer activeResponse.Body.Close()

	var active core.ConfigEnvelope
	if err := json.NewDecoder(activeResponse.Body).Decode(&active); err != nil {
		t.Fatalf("decode active config: %v", err)
	}

	if active.Config.System.Name != "studio-b" {
		t.Fatalf("active config not updated: %s", active.Config.System.Name)
	}
}
