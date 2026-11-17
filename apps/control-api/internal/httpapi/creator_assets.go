package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxCreatorCatalogBytes = 8 << 20

var creatorAssetPathPattern = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

type creatorAsset struct {
	Path            string   `json:"path"`
	Title           string   `json:"title"`
	Category        string   `json:"category"`
	Collection      string   `json:"collection"`
	Description     string   `json:"description"`
	UseCase         string   `json:"useCase"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	Preview         string   `json:"preview"`
	Tags            []string `json:"tags"`
}

type creatorAssetCatalog struct {
	Summary json.RawMessage `json:"summary"`
	Assets  []creatorAsset  `json:"assets"`
}

func (handler *Handler) handleCreatorAssetCatalog(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if handler.assetLibraryDir == "" {
		creatorLibraryUnavailable(writer, "Creator asset library is not configured.")
		return
	}

	file, err := os.Open(filepath.Join(handler.assetLibraryDir, "catalog.json"))
	if err != nil {
		creatorLibraryUnavailable(writer, "Creator asset catalog is unavailable. Install OBS-Scene-Assets into the configured library directory.")
		return
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, maxCreatorCatalogBytes+1))
	if err != nil || len(payload) > maxCreatorCatalogBytes {
		creatorLibraryUnavailable(writer, "Creator asset catalog is unreadable or too large.")
		return
	}

	var catalog creatorAssetCatalog
	if err := json.Unmarshal(payload, &catalog); err != nil || catalog.Assets == nil {
		creatorLibraryUnavailable(writer, "Creator asset catalog is invalid.")
		return
	}

	// Catalogs are user-installed content; never generate file URLs for unsafe entries.
	filtered := make([]creatorAsset, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		if asset.Title == "" || !validCreatorAssetPath(asset.Path, "", ".html") ||
			!validCreatorAssetPath(asset.Preview, "previews/", ".png") {
			continue
		}
		filtered = append(filtered, asset)
	}

	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, struct {
		LibraryRoot string          `json:"libraryRoot"`
		Summary     json.RawMessage `json:"summary"`
		Assets      []creatorAsset  `json:"assets"`
	}{
		LibraryRoot: handler.assetLibraryDir,
		Summary:     catalog.Summary,
		Assets:      filtered,
	})
}

func (handler *Handler) handleCreatorAssetPreview(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	if handler.assetLibraryDir == "" {
		notFound(writer)
		return
	}
	const prefix = "/api/v1/creator-assets/preview/"
	previewPath := strings.TrimPrefix(request.URL.Path, prefix)
	if !validCreatorAssetPath(previewPath, "previews/", ".png") {
		notFound(writer)
		return
	}

	// Resolve symlinks to prevent a malicious local asset pack from escaping the
	// preview directory. Never serve HTML on the control API's origin.
	root, err := filepath.EvalSymlinks(filepath.Join(handler.assetLibraryDir, "previews"))
	if err != nil {
		notFound(writer)
		return
	}
	target, err := filepath.EvalSymlinks(filepath.Join(handler.assetLibraryDir, filepath.FromSlash(previewPath)))
	if err != nil {
		notFound(writer)
		return
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		notFound(writer)
		return
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() {
		notFound(writer)
		return
	}
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Type", "image/png")
	writer.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(writer, request, target)
}

func validCreatorAssetPath(path, prefix, extension string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") ||
		!creatorAssetPathPattern.MatchString(path) ||
		filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) != path {
		return false
	}
	return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, extension)
}

func creatorLibraryUnavailable(writer http.ResponseWriter, message string) {
	writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": message})
}
