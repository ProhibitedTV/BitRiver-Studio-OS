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
	"testing"
)

func TestCreatorAssetCatalogAndPreview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "previews", "scenes"), 0o755); err != nil {
		t.Fatal(err)
	}

	catalog := map[string]any{
		"summary": map[string]int{"assetCount": 2},
		"assets": []map[string]any{
			{
				"path": "scenes/starting-soon.html", "title": "Starting Soon",
				"category": "scenes", "collection": "core", "width": 1920, "height": 1080,
				"preview": "previews/scenes/starting-soon.png",
			},
			{
				"path": "../private.html", "title": "Unsafe",
				"preview": "previews/scenes/starting-soon.png",
			},
		},
	}
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	preview := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0}
	if err := os.WriteFile(filepath.Join(dir, "previews", "scenes", "starting-soon.png"), preview, 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), newTestController(t), "", dir))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/v1/creator-assets/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("catalog status: %d", response.StatusCode)
	}
	var result struct {
		LibraryRoot string         `json:"libraryRoot"`
		Assets      []creatorAsset `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.LibraryRoot != dir || len(result.Assets) != 1 || result.Assets[0].Title != "Starting Soon" {
		t.Fatalf("unexpected filtered catalog: %#v", result)
	}

	imageResponse, err := http.Get(server.URL + "/api/v1/creator-assets/preview/previews/scenes/starting-soon.png")
	if err != nil {
		t.Fatal(err)
	}
	defer imageResponse.Body.Close()
	if imageResponse.StatusCode != http.StatusOK || imageResponse.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected preview status %d / %s", imageResponse.StatusCode, imageResponse.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(imageResponse.Body)
	if !bytes.Equal(body, preview) {
		t.Fatal("preview content mismatch")
	}

	for _, path := range []string{
		"/api/v1/creator-assets/preview/../../private.png",
		"/api/v1/creator-assets/preview/previews/scenes/starting-soon.html",
		"/api/v1/creator-assets/preview/catalog.json",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), newTestController(t), "", dir).ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("unexpected access granted to %s", path)
		}
	}
}

func TestCreatorAssetsUnavailableAndReadOnly(t *testing.T) {
	t.Parallel()
	server := newTestServer(t)
	defer server.Close()

	res, err := http.Get(server.URL + "/api/v1/creator-assets/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected disabled catalog to return 503, got %d", res.StatusCode)
	}

	post, err := http.Post(server.URL+"/api/v1/creator-assets/catalog", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer post.Body.Close()
	if post.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected read-only catalog to return 405, got %d", post.StatusCode)
	}
}

func TestValidCreatorAssetPathRejectsTraversal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path      string
		prefix    string
		extension string
		want      bool
	}{
		{path: "scenes/starting-soon.html", extension: ".html", want: true},
		{path: "previews/scenes/starting-soon.png", prefix: "previews/", extension: ".png", want: true},
		{path: "../private.html", extension: ".html"},
		{path: "../../secrets.html", extension: ".html"},
		{path: "scenes/../../private.html", extension: ".html"},
		{path: "scenes/../private.html", extension: ".html"},
		{path: "/etc/private.html", extension: ".html"},
		{path: "previews/../../private.png", prefix: "previews/", extension: ".png"},
		{path: "previews/scenes/../private.png", prefix: "previews/", extension: ".png"},
		{path: "previews\\scenes\\starting-soon.png", prefix: "previews/", extension: ".png"},
		{path: "previews/scenes/preview.html", prefix: "previews/", extension: ".png"},
	}
	for _, tc := range cases {
		if got := validCreatorAssetPath(tc.path, tc.prefix, tc.extension); got != tc.want {
			t.Errorf("validCreatorAssetPath(%q, %q, %q)=%t; want %t",
				tc.path, tc.prefix, tc.extension, got, tc.want)
		}
	}
}
