package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func TestDraftAndApplyRoutesRejectConcurrentOperatorChanges(t *testing.T) {
	t.Parallel()
	server := newTestServer(t)
	defer server.Close()
	getDraft := func() model.ConfigEnvelope {
		t.Helper()
		response, err := http.Get(server.URL + "/api/v1/settings/draft")
		if err != nil { t.Fatal(err) }
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK { t.Fatalf("GET draft: HTTP %d", response.StatusCode) }
		var value model.ConfigEnvelope
		if err := json.NewDecoder(response.Body).Decode(&value); err != nil { t.Fatal(err) }
		return value
	}
	putDraft := func(config model.ControlConfig, etag string) (int, model.ConfigEnvelope) {
		t.Helper()
		body, err := json.Marshal(config)
		if err != nil { t.Fatal(err) }
		request, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/settings/draft", bytes.NewReader(body))
		if err != nil { t.Fatal(err) }
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("If-Match", etag)
		response, err := http.DefaultClient.Do(request)
		if err != nil { t.Fatal(err) }
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK { io.Copy(io.Discard, response.Body); return response.StatusCode, model.ConfigEnvelope{} }
		var result model.ConfigEnvelope
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil { t.Fatal(err) }
		return response.StatusCode, result
	}
	apply := func(etag string) int {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/settings/apply", nil)
		if err != nil { t.Fatal(err) }
		request.Header.Set("If-Match", etag)
		response, err := http.DefaultClient.Do(request)
		if err != nil { t.Fatal(err) }
		defer response.Body.Close()
		io.Copy(io.Discard, response.Body)
		return response.StatusCode
	}

	first := getDraft()
	aConfig := first.Config
	aConfig.System.Name = "operator-a"
	status, a := putDraft(aConfig, `"`+first.Generation+`"`)
	if status != 200 { t.Fatalf("first write: HTTP %d", status) }

	bConfig := first.Config
	bConfig.System.Name = "operator-b"
	status, _ = putDraft(bConfig, `"`+first.Generation+`"`)
	if status != 409 { t.Fatalf("stale update: HTTP %d; wanted 409", status) }
	if getDraft().Config.System.Name != "operator-a" { t.Fatal("stale write replaced operator A") }

	status, _ = putDraft(bConfig, `*`)
	if status != 400 { t.Fatalf("malformed If-Match: HTTP %d; wanted 400", status) }

	status, b := putDraft(bConfig, `"`+a.Generation+`"`)
	if status != 200 { t.Fatalf("fresh write: HTTP %d", status) }
	if a.Generation == b.Generation { t.Fatal("two updates reused the same generation") }
	if status := apply(`"`+a.Generation+`"`); status != 409 {
		t.Fatalf("stale apply: HTTP %d; wanted 409", status)
	}
	if getDraft().Config.System.Name != "operator-b" { t.Fatal("stale apply changed the draft") }
	if status := apply(`"`+b.Generation+`"`); status != 200 {
		t.Fatalf("fresh apply: HTTP %d", status)
	}
	response, err := http.Get(server.URL + "/api/v1/settings/active")
	if err != nil { t.Fatal(err) }
	defer response.Body.Close()
	var active model.ConfigEnvelope
	if err := json.NewDecoder(response.Body).Decode(&active); err != nil { t.Fatal(err) }
	if active.Config.System.Name != "operator-b" { t.Fatal("wrong settings promoted to active") }
}
