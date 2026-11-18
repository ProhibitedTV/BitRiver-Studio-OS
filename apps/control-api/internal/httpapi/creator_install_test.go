package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func creatorInstallServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scenes"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, "scenes", "starting-soon.html"), []byte("<!doctype html><title>Offline</title>"), 0o644); err != nil { t.Fatal(err) }
	catalog := map[string]any{
		"assets": []map[string]any{{
			"path": "scenes/starting-soon.html",
			"title": "Starting Soon",
			"width": 1920,
			"height": 1080,
			"preview": "previews/scenes/starting-soon.png",
		}},
	}
	data, err := json.Marshal(catalog)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), data, 0o644); err != nil { t.Fatal(err) }
	server := httptest.NewServer(NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), newTestController(t), "", dir))
	return server, dir
}

func postCreatorInstall(t *testing.T, server *httptest.Server, body string) *http.Response {
	t.Helper()
	response, err := http.Post(server.URL+"/api/v1/creator-assets/install", "application/json", strings.NewReader(body))
	if err != nil { t.Fatal(err) }
	return response
}

func TestInstallCreatorSceneIsOptInAndNoOverwrite(t *testing.T) {
	t.Parallel()
	server, _ := creatorInstallServer(t)
	defer server.Close()
	body := `{"path":"scenes/starting-soon.html","title":"My Show","subtitle":"Episode 1"}`

	res := postCreatorInstall(t, server, body)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated { raw, _ := io.ReadAll(res.Body); t.Fatalf("install status %d: %s", res.StatusCode, raw) }
	var installed struct { SceneName string `json:"sceneName"`; InputName string `json:"inputName"` }
	if err := json.NewDecoder(res.Body).Decode(&installed); err != nil { t.Fatal(err) }
	if installed.SceneName != "BitRiver - Starting Soon" || installed.InputName != "BitRiver overlay - starting-soon" {
		t.Fatalf("unexpected scene created: %#v", installed)
	}

	// Creating a scene may never change the on-air/previewed program.
	state, err := http.Get(server.URL+"/api/v1/stream")
	if err != nil { t.Fatal(err) }
	defer state.Body.Close()
	var stream struct { ActiveSceneID string `json:"activeSceneId"`; State string `json:"state"` }
	if err := json.NewDecoder(state.Body).Decode(&stream); err != nil { t.Fatal(err) }
	if stream.State != "stopped" {
		t.Fatalf("installation unexpectedly started streaming: %#v", stream)
	}
	// The newly installed scene must be discoverable and switchable through
	// the exact API used by the dashboard, without editing settings JSON.
	showResponse, err := http.Get(server.URL + "/api/v1/show")
	if err != nil { t.Fatal(err) }
	var show struct {
		ActiveSceneID string `json:"activeSceneId"`
		Scenes []struct { ID string `json:"id"`; Name string `json:"name"`; Active bool `json:"active"` } `json:"scenes"`
	}
	if err := json.NewDecoder(showResponse.Body).Decode(&show); err != nil { t.Fatal(err) }
	showResponse.Body.Close()
	if show.ActiveSceneID != "starting-soon" || len(show.Scenes) != 4 {
		t.Fatalf("install changed program or failed to expose scene in show: %#v", show)
	}
	newID := show.Scenes[3].ID
	if show.Scenes[3].Name != installed.SceneName || !strings.HasPrefix(newID, "obs-") {
		t.Fatalf("installed scene has no stable ID in show: %#v", show.Scenes[3])
	}
	switchResponse, err := http.Post(server.URL + "/api/v1/show/scene/" + newID + "/activate", "application/json", nil)
	if err != nil { t.Fatal(err) }
	switchResponse.Body.Close()
	if switchResponse.StatusCode != http.StatusOK {
		t.Fatalf("scene switch failed: %d", switchResponse.StatusCode)
	}
	afterResponse, err := http.Get(server.URL + "/api/v1/show")
	if err != nil { t.Fatal(err) }
	defer afterResponse.Body.Close()
	if err := json.NewDecoder(afterResponse.Body).Decode(&show); err != nil { t.Fatal(err) }
	if show.ActiveSceneID != newID {
		t.Fatalf("new scene not active after explicit operator switch: %#v", show)
	}
	duplicate := postCreatorInstall(t, server, body)
	defer duplicate.Body.Close()
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate should refuse to overwrite (409), got %d", duplicate.StatusCode)
	}
}

func TestInstallCreatorSceneRejectsUnsafeSelectionsAndOnAirMutation(t *testing.T) {
	t.Parallel()
	server, dir := creatorInstallServer(t)
	defer server.Close()
	for _, body := range []string{
		`{"path":"../../etc/shadow"}`,
		`{"path":"scenes/unknown.html"}`,
		`{"path":"scenes/starting-soon.html","title":"bad\nname"}`,
		`{"path":"scenes/starting-soon.html","surprise":1}`,
	} {
		res := postCreatorInstall(t, server, body)
		res.Body.Close()
		if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusNotFound {
			t.Errorf("expected bad selection rejection, got %d for %s", res.StatusCode, body)
		}
	}

	// Installed paths cannot escape the approved root even through symlinks.
	outside := filepath.Join(t.TempDir(), "evil.html")
	if err := os.WriteFile(outside, []byte("bad"), 0o644); err != nil { t.Fatal(err) }
	if err := os.Remove(filepath.Join(dir, "scenes", "starting-soon.html")); err != nil { t.Fatal(err) }
	if err := os.Symlink(outside, filepath.Join(dir, "scenes", "starting-soon.html")); err != nil { t.Fatal(err) }
	symlink := postCreatorInstall(t, server, `{"path":"scenes/starting-soon.html"}`)
	symlink.Body.Close()
	if symlink.StatusCode != http.StatusBadRequest { t.Fatalf("symlink escape returned %d", symlink.StatusCode) }

	// Restore valid file and start a stream; installing must be blocked.
	if err := os.Remove(filepath.Join(dir, "scenes", "starting-soon.html")); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, "scenes", "starting-soon.html"), []byte("safe"), 0o644); err != nil { t.Fatal(err) }
	live, err := http.Post(server.URL+"/api/v1/stream/start", "application/json", bytes.NewReader(nil))
	if err != nil { t.Fatal(err) }
	live.Body.Close()
	if live.StatusCode != http.StatusOK { t.Fatalf("start stream returned %d", live.StatusCode) }
	blocked := postCreatorInstall(t, server, `{"path":"scenes/starting-soon.html"}`)
	blocked.Body.Close()
	if blocked.StatusCode != http.StatusConflict { t.Fatalf("active broadcast must block installs; got %d", blocked.StatusCode) }
}

func TestInstallCreatorSceneNoCatalog(t *testing.T) {
	t.Parallel()
	server := newTestServer(t)
	defer server.Close()
	res := postCreatorInstall(t, server, `{"path":"scenes/starting-soon.html"}`)
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable { t.Fatalf("expected 503, got %d", res.StatusCode) }
}
