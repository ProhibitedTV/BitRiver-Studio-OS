package httpapi

import (
	"encoding/json"
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/events"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
	"github.com/bitriver/studio-os/apps/control-api/internal/openapi"
	"github.com/bitriver/studio-os/apps/control-api/internal/service"
)

type Handler struct {
	logger     *slog.Logger
	controller *service.Controller
	uiDir      string
	assetLibraryDir string
	operatorAuth *OperatorAuth
}

func NewHandler(logger *slog.Logger, controller *service.Controller, uiDir string, assetLibraryDirs ...string) http.Handler {
	assetLibraryDir := ""
	if len(assetLibraryDirs) > 0 {
		assetLibraryDir = assetLibraryDirs[0]
	}
	return newHandler(logger, controller, uiDir, assetLibraryDir, nil)
}

// NewSecureHandler enables mandatory operator authentication on appliance APIs.
func NewSecureHandler(logger *slog.Logger, controller *service.Controller, uiDir, assetLibraryDir string, auth *OperatorAuth) http.Handler {
	if auth == nil {
		panic("secure handler requires operator authentication")
	}
	return newHandler(logger, controller, uiDir, assetLibraryDir, auth)
}

func newHandler(logger *slog.Logger, controller *service.Controller, uiDir, assetLibraryDir string, auth *OperatorAuth) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if assetLibraryDir != "" {
		assetLibraryDir = filepath.Clean(assetLibraryDir)
	}
	api := &Handler{
		logger:          logger,
		controller:      controller,
		uiDir:           uiDir,
		assetLibraryDir: assetLibraryDir,
		operatorAuth:    auth,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.handleHealthz)
	mux.HandleFunc("/api/v1/auth/login", api.handleOperatorLogin)
	mux.HandleFunc("/api/v1/auth/session", api.handleOperatorSession)
	mux.HandleFunc("/api/v1/auth/logout", api.handleOperatorLogout)
	mux.HandleFunc("/api/v1/system", api.handleSystem)
	mux.HandleFunc("/api/v1/health", api.handleHealth)
	mux.HandleFunc("/api/v1/setup/status", api.handleSetupStatus)
	mux.HandleFunc("/api/v1/stream", api.handleStream)
	mux.HandleFunc("/api/v1/stream/start", api.handleStreamStart)
	mux.HandleFunc("/api/v1/stream/stop", api.handleStreamStop)
	mux.HandleFunc("/api/v1/outputs", api.handleOutputs)
	mux.HandleFunc("/api/v1/recordings", api.handleRecordings)
	mux.HandleFunc("/api/v1/media-router", api.handleMediaRouter)
	mux.HandleFunc("/api/v1/creator-assets/catalog", api.handleCreatorAssetCatalog)
	mux.HandleFunc("/api/v1/creator-assets/install", api.handleCreatorAssetInstall)
	mux.HandleFunc("/api/v1/creator-assets/preview/", api.handleCreatorAssetPreview)
	mux.HandleFunc("/api/v1/recordings/start", api.handleRecordingStart)
	mux.HandleFunc("/api/v1/recordings/stop", api.handleRecordingStop)
	mux.HandleFunc("/api/v1/scenes", api.handleScenes)
	mux.HandleFunc("/api/v1/scenes/", api.handleSceneActions)
	mux.HandleFunc("/api/v1/sources", api.handleSources)
	mux.HandleFunc("/api/v1/audio", api.handleAudio)
	mux.HandleFunc("/api/v1/audio/", api.handleAudioActions)
	mux.HandleFunc("/api/v1/audio/routes", api.handleAudio)
	mux.HandleFunc("/api/v1/audio/routes/", api.handleAudioActions)
	mux.HandleFunc("/api/v1/settings/active", api.handleActiveSettings)
	mux.HandleFunc("/api/v1/settings/draft", api.handleDraftSettings)
	mux.HandleFunc("/api/v1/settings/validate", api.handleValidateSettings)
	mux.HandleFunc("/api/v1/settings/apply", api.handleApplySettings)
	mux.HandleFunc("/api/v1/update", api.handleUpdate)
	mux.HandleFunc("/api/v1/show", api.handleShow)
	mux.HandleFunc("/api/v1/show/scene/", api.handleLegacySceneActions)
	mux.HandleFunc("/api/v1/show/stream/start", api.handleStreamStart)
	mux.HandleFunc("/api/v1/show/stream/stop", api.handleStreamStop)
	mux.HandleFunc("/api/v1/show/recording/start", api.handleRecordingStart)
	mux.HandleFunc("/api/v1/show/recording/stop", api.handleRecordingStop)
	mux.HandleFunc("/api/v1/storage", api.handleRecordings)
	mux.HandleFunc("/api/v1/config/active", api.handleActiveSettings)
	mux.HandleFunc("/api/v1/config/draft", api.handleDraftSettings)
	mux.HandleFunc("/api/v1/config/validate", api.handleValidateSettings)
	mux.HandleFunc("/api/v1/config/apply", api.handleApplySettings)
	mux.HandleFunc("/api/v1/openapi.yaml", api.handleOpenAPI)
	mux.HandleFunc("/api/v1/ws", api.handleWebsocket)
	mux.HandleFunc("/", api.handleRoot)

	if auth != nil {
		return api.loggingMiddleware(auth.gate(mux))
	}
	return api.loggingMiddleware(mux)
}

func (handler *Handler) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		handler.logger.Info("http request",
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
			"durationMs", time.Since(startedAt).Milliseconds(),
			"remoteAddr", request.RemoteAddr,
		)
	})
}

func (handler *Handler) handleOperatorLogin(writer http.ResponseWriter, request *http.Request) {
	if handler.operatorAuth == nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "authentication disabled in development mode"})
		return
	}
	handler.operatorAuth.handleLogin(writer, request)
}

func (handler *Handler) handleOperatorSession(writer http.ResponseWriter, request *http.Request) {
	if handler.operatorAuth == nil {
		writeJSON(writer, http.StatusOK, map[string]bool{"enabled": false, "authenticated": true})
		return
	}
	handler.operatorAuth.handleSession(writer, request)
}

func (handler *Handler) handleOperatorLogout(writer http.ResponseWriter, request *http.Request) {
	if handler.operatorAuth == nil {
		writeJSON(writer, http.StatusOK, map[string]bool{"authenticated": false})
		return
	}
	handler.operatorAuth.handleLogout(writer, request)
}

func (handler *Handler) handleHealthz(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte("ok"))
}

func (handler *Handler) handleSystem(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.System(request.Context()))
}

func (handler *Handler) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.Health(request.Context()))
}

func (handler *Handler) handleSetupStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.SetupStatus(request.Context()))
}

func (handler *Handler) handleStream(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.Stream(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleStreamStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.StartStream(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleStreamStop(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.StopStream(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleOutputs(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.Outputs(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleRecordings(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.Recordings(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleMediaRouter(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.MediaRouter(request.Context()))
}

func (handler *Handler) handleRecordingStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.StartRecording(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleRecordingStop(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.StopRecording(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleScenes(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.Scenes(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleSceneActions(writer http.ResponseWriter, request *http.Request) {
	handler.handleActivateScene(writer, request, "/api/v1/scenes/")
}

func (handler *Handler) handleLegacySceneActions(writer http.ResponseWriter, request *http.Request) {
	handler.handleActivateScene(writer, request, "/api/v1/show/scene/")
}

func (handler *Handler) handleActivateScene(writer http.ResponseWriter, request *http.Request, prefix string) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	sceneID, action, err := splitActionPath(request.URL.Path, prefix)
	if err != nil || action != "activate" {
		notFound(writer)
		return
	}
	payload, err := handler.controller.ActivateScene(request.Context(), sceneID)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleSources(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.Sources(request.Context()))
}

func (handler *Handler) handleAudio(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	payload, err := handler.controller.Audio(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleAudioActions(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}

	channelID, action, err := splitAudioActionPath(request.URL.Path)
	if err != nil {
		notFound(writer)
		return
	}

	switch action {
	case "mute":
		payload, err := handler.controller.SetMute(request.Context(), channelID, true)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, payload)
	case "unmute":
		payload, err := handler.controller.SetMute(request.Context(), channelID, false)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, payload)
	case "gain":
		var requestBody model.GainRequest
		if err := decodeJSON(request, &requestBody); err != nil {
			writeError(writer, model.NewInvalidError("request body must include gainDb", map[string]string{"body": err.Error()}))
			return
		}
		payload, err := handler.controller.SetGain(request.Context(), channelID, requestBody.GainDB)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, payload)
	default:
		notFound(writer)
	}
}

func (handler *Handler) handleActiveSettings(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.ActiveConfig())
}

func (handler *Handler) handleDraftSettings(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(writer, http.StatusOK, handler.controller.DraftConfig())
	case http.MethodPut:
		var config model.ControlConfig
		if err := decodeJSON(request, &config); err != nil {
			writeError(writer, model.NewInvalidError("request body must be a valid settings document", map[string]string{"body": err.Error()}))
			return
		}
		payload, err := handler.controller.UpdateDraft(config)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, payload)
	default:
		methodNotAllowed(writer)
	}
}

func (handler *Handler) handleValidateSettings(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.ValidateDraft())
}

func (handler *Handler) handleApplySettings(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	payload, validation, err := handler.controller.ApplyDraft(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	if validation != nil {
		writeJSON(writer, http.StatusUnprocessableEntity, validation)
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) handleUpdate(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, handler.controller.UpdateStatus(request.Context()))
}

func (handler *Handler) handleShow(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	scenes, err := handler.controller.Scenes(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	stream, err := handler.controller.Stream(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	recordings, err := handler.controller.Recordings(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}
	outputs, err := handler.controller.Outputs(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}

	show := model.ShowState{
		ActiveSceneID: scenes.ActiveSceneID,
		StreamState:   stream.State,
		RecordState:   recordings.State,
		Scenes:        scenes.Items,
	}
	if len(outputs.Destinations) > 0 {
		show.Destination = outputs.Destinations[0]
	}

	writeJSON(writer, http.StatusOK, show)
}

func (handler *Handler) handleOpenAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(openapi.YAML())
}

func (handler *Handler) handleWebsocket(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}

	snapshot, err := handler.controller.Snapshot(request.Context())
	if err != nil {
		writeError(writer, err)
		return
	}

	events.ServeWebsocket(writer, request, handler.controller.EventHub(), model.EventEnvelope{
		Type:      "snapshot.initial",
		Timestamp: time.Now().UTC(),
		Payload:   snapshot,
	})
}

func (handler *Handler) handleRoot(writer http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, "/api/") {
		notFound(writer)
		return
	}

	if handler.uiDir == "" {
		writeJSON(writer, http.StatusOK, map[string]string{
			"service": "bitriver-control-api",
			"status":  "ui assets not configured; run the Vite dev server for local UI work",
		})
		return
	}

	uiPath := filepath.Clean(handler.uiDir)
	if _, err := os.Stat(uiPath); err != nil {
		writeJSON(writer, http.StatusOK, map[string]string{
			"service": "bitriver-control-api",
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

// Preserve HTTP upgrade support through request logging (e.g. OBS/dashboard WS).
func (recorder *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := recorder.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, buffer, err := hijacker.Hijack()
	if err == nil {
		recorder.status = http.StatusSwitchingProtocols
	}
	return conn, buffer, err
}

func splitActionPath(path string, prefix string) (string, string, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid action path")
	}
	return parts[0], parts[1], nil
}

func splitAudioActionPath(path string) (string, string, error) {
	for _, prefix := range []string{"/api/v1/audio/", "/api/v1/audio/routes/"} {
		if strings.HasPrefix(path, prefix) {
			return splitActionPath(path, prefix)
		}
	}
	return "", "", fmt.Errorf("invalid audio action path")
}

func decodeJSON(request *http.Request, target any) error {
	defer request.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("request body must contain a single JSON document")
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func writeError(writer http.ResponseWriter, err error) {
	var apiErr *model.APIError
	if errors.As(err, &apiErr) {
		writeJSON(writer, apiErr.Status, apiErr)
		return
	}

	writeJSON(writer, http.StatusInternalServerError, &model.APIError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: err.Error(),
	})
}

func methodNotAllowed(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusMethodNotAllowed, &model.APIError{
		Status:  http.StatusMethodNotAllowed,
		Code:    "method_not_allowed",
		Message: "method not allowed for this endpoint",
	})
}

func notFound(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusNotFound, &model.APIError{
		Status:  http.StatusNotFound,
		Code:    "not_found",
		Message: "resource not found",
	})
}
