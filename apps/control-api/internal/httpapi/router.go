package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitriver/studio-os/apps/control-api/internal/core"
)

type handler struct {
	store *core.StateStore
	uiDir string
}

func NewHandler(store *core.StateStore, uiDir string) http.Handler {
	api := &handler{store: store, uiDir: uiDir}
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", api.handleHealth)
	mux.HandleFunc("/api/v1/system", api.handleSystem)
	mux.HandleFunc("/api/v1/show", api.handleShow)
	mux.HandleFunc("/api/v1/show/scene/", api.handleSceneActions)
	mux.HandleFunc("/api/v1/show/stream/start", api.handleStreamStart)
	mux.HandleFunc("/api/v1/show/stream/stop", api.handleStreamStop)
	mux.HandleFunc("/api/v1/show/recording/start", api.handleRecordingStart)
	mux.HandleFunc("/api/v1/show/recording/stop", api.handleRecordingStop)
	mux.HandleFunc("/api/v1/sources", api.handleSources)
	mux.HandleFunc("/api/v1/audio", api.handleAudio)
	mux.HandleFunc("/api/v1/audio/", api.handleAudioActions)
	mux.HandleFunc("/api/v1/storage", api.handleStorage)
	mux.HandleFunc("/api/v1/config/active", api.handleActiveConfig)
	mux.HandleFunc("/api/v1/config/draft", api.handleDraftConfig)
	mux.HandleFunc("/api/v1/config/validate", api.handleValidateConfig)
	mux.HandleFunc("/api/v1/config/apply", api.handleApplyConfig)
	mux.HandleFunc("/", api.handleRoot)

	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Printf("%s %s", request.Method, request.URL.Path)
		next.ServeHTTP(writer, request)
	})
}

func (api *handler) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte("ok"))
}

func (api *handler) handleSystem(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.System())
}

func (api *handler) handleShow(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.Show())
}

func (api *handler) handleSceneActions(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	sceneID, action, err := splitActionPath(request.URL.Path, "/api/v1/show/scene/")
	if err != nil || action != "activate" {
		notFound(writer)
		return
	}

	response, actionErr := api.store.ActivateScene(sceneID)
	if actionErr != nil {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": actionErr.Error()})
		return
	}

	writeJSON(writer, http.StatusOK, response)
}

func (api *handler) handleStreamStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.StartStream())
}

func (api *handler) handleStreamStop(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.StopStream())
}

func (api *handler) handleRecordingStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.StartRecording())
}

func (api *handler) handleRecordingStop(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.StopRecording())
}

func (api *handler) handleSources(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.Sources())
}

func (api *handler) handleAudio(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.Audio())
}

func (api *handler) handleAudioActions(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	channelID, action, err := splitActionPath(request.URL.Path, "/api/v1/audio/")
	if err != nil {
		notFound(writer)
		return
	}

	switch action {
	case "mute":
		response, actionErr := api.store.SetMute(channelID, true)
		if actionErr != nil {
			writeJSON(writer, http.StatusNotFound, map[string]string{"error": actionErr.Error()})
			return
		}
		writeJSON(writer, http.StatusOK, response)
	case "unmute":
		response, actionErr := api.store.SetMute(channelID, false)
		if actionErr != nil {
			writeJSON(writer, http.StatusNotFound, map[string]string{"error": actionErr.Error()})
			return
		}
		writeJSON(writer, http.StatusOK, response)
	case "gain":
		var requestBody core.GainRequest
		if err := decodeJSON(request, &requestBody); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		response, actionErr := api.store.SetGain(channelID, requestBody.GainDB)
		if actionErr != nil {
			writeJSON(writer, http.StatusNotFound, map[string]string{"error": actionErr.Error()})
			return
		}
		writeJSON(writer, http.StatusOK, response)
	default:
		notFound(writer)
	}
}

func (api *handler) handleStorage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.Storage())
}

func (api *handler) handleActiveConfig(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.ActiveConfig())
}

func (api *handler) handleDraftConfig(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(writer, http.StatusOK, api.store.DraftConfig())
	case http.MethodPut:
		var config core.ControlConfig
		if err := decodeJSON(request, &config); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(writer, http.StatusOK, api.store.UpdateDraft(config))
	default:
		methodNotAllowed(writer)
	}
}

func (api *handler) handleValidateConfig(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	writeJSON(writer, http.StatusOK, api.store.ValidateDraft())
}

func (api *handler) handleApplyConfig(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	response, validation := api.store.ApplyDraft()
	if validation != nil {
		writeJSON(writer, http.StatusUnprocessableEntity, validation)
		return
	}

	writeJSON(writer, http.StatusOK, response)
}

func (api *handler) handleRoot(writer http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, "/api/") {
		notFound(writer)
		return
	}

	if api.uiDir == "" {
		writeJSON(writer, http.StatusOK, map[string]string{
			"service": "bitriverd",
			"status":  "ui assets not configured; run the Vite dev server for local UI work",
		})
		return
	}

	uiPath := filepath.Clean(api.uiDir)
	if _, err := os.Stat(uiPath); err != nil {
		writeJSON(writer, http.StatusOK, map[string]string{
			"service": "bitriverd",
			"status":  "configured UI directory is unavailable",
		})
		return
	}

	requestPath := strings.TrimPrefix(request.URL.Path, "/")
	if requestPath == "" {
		requestPath = "index.html"
	}

	fullPath := filepath.Join(uiPath, requestPath)
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		http.ServeFile(writer, request, fullPath)
		return
	}

	http.ServeFile(writer, request, filepath.Join(uiPath, "index.html"))
}

func splitActionPath(path string, prefix string) (string, string, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid action path")
	}
	return parts[0], parts[1], nil
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func methodNotAllowed(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func notFound(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusNotFound, map[string]string{"error": "not found"})
}
