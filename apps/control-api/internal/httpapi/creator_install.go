package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

// InstallBrowserSceneRequest identifies catalog content, never an arbitrary URL.
// A logged-in operator explicitly selects an installed local asset to use.
type installBrowserSceneRequest struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

func (handler *Handler) handleCreatorAssetInstall(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	if handler.assetLibraryDir == "" {
		creatorLibraryUnavailable(writer, "Creator asset library is not configured.")
		return
	}

	var input installBrowserSceneRequest
	if err := decodeJSON(request, &input); err != nil ||
		!validCreatorAssetPath(input.Path, "", ".html") ||
		!validOverlayCopy(input.Title) || !validOverlayCopy(input.Subtitle) {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid creator asset selection or text"})
		return
	}

	asset, err := findInstallableCreatorAsset(handler.assetLibraryDir, input.Path)
	if err != nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "creator asset catalog unavailable"})
		return
	}
	if asset == nil {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "selected creator asset is not in the reviewed local catalog"})
		return
	}
	if asset.Width < 1 || asset.Height < 1 || asset.Width > 7680 || asset.Height > 4320 ||
		utf8.RuneCountInString(asset.Title) > 64 || !safeSceneLabel(asset.Title) {
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]string{"error": "selected asset has invalid OBS dimensions or title"})
		return
	}

	root, err := filepath.EvalSymlinks(handler.assetLibraryDir)
	if err != nil {
		creatorLibraryUnavailable(writer, "Creator asset directory is not accessible.")
		return
	}
	target, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(asset.Path)))
	if err != nil {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "selected asset file is missing"})
		return
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "selected file escapes the asset library"})
		return
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "selected asset is not a regular file"})
		return
	}

	params := url.Values{}
	if title := strings.TrimSpace(input.Title); title != "" {
		params.Set("title", title)
	}
	if subtitle := strings.TrimSpace(input.Subtitle); subtitle != "" {
		params.Set("subtitle", subtitle)
	}
	sourceURL := (&url.URL{Scheme: "file", Path: target, RawQuery: params.Encode()}).String()
	sceneName := "BitRiver - " + asset.Title
	inputName := "BitRiver overlay - " + strings.TrimSuffix(filepath.Base(asset.Path), ".html")

	if err := handler.controller.InstallBrowserScene(request.Context(), sceneName, inputName, sourceURL, asset.Width, asset.Height); err != nil {
		switch {
		case errors.Is(err, adapters.ErrSceneInstallBusy):
			writeJSON(writer, http.StatusConflict, map[string]string{"error": "Stop OBS streaming and recording before installing a scene."})
		case errors.Is(err, adapters.ErrSceneInstallCollision):
			writeJSON(writer, http.StatusConflict, map[string]string{"error": "The BitRiver scene or browser input already exists in OBS. Nothing was overwritten."})
		default:
			handler.logger.Warn("OBS creator scene installation failed", "error", err)
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "OBS could not install the scene. Check the OBS connection and inspect scenes before retrying."})
		}
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, map[string]string{
		"sceneName": sceneName,
		"inputName": inputName,
		"message": fmt.Sprintf("Installed %s in OBS. Preview it and select it manually when ready.", sceneName),
	})
}

func validOverlayCopy(value string) bool {
	if utf8.RuneCountInString(value) > 120 || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func safeSceneLabel(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func findInstallableCreatorAsset(dir, path string) (*creatorAsset, error) {
	file, err := os.Open(filepath.Join(dir, "catalog.json"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, maxCreatorCatalogBytes+1))
	if err != nil || len(payload) > maxCreatorCatalogBytes {
		return nil, errors.New("creator asset catalog unreadable or oversized")
	}
	var catalog creatorAssetCatalog
	if err := json.Unmarshal(payload, &catalog); err != nil || catalog.Assets == nil {
		return nil, errors.New("invalid creator asset catalog")
	}
	for _, asset := range catalog.Assets {
		if asset.Path == path && validCreatorAssetPath(asset.Path, "", ".html") &&
			validCreatorAssetPath(asset.Preview, "previews/", ".png") {
			return &asset, nil
		}
	}
	return nil, nil
}
