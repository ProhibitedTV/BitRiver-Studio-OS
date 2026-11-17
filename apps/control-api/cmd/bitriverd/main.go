package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/mediamtx"
	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/mock"
	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/obsws"
	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/owncast"
	"github.com/bitriver/studio-os/apps/control-api/internal/adapters/systemd"
	"github.com/bitriver/studio-os/apps/control-api/internal/configstore"
	"github.com/bitriver/studio-os/apps/control-api/internal/events"
	"github.com/bitriver/studio-os/apps/control-api/internal/httpapi"
	"github.com/bitriver/studio-os/apps/control-api/internal/service"
	"github.com/bitriver/studio-os/apps/control-api/internal/version"
)

func main() {
	httpAddr := flag.String("http-addr", "127.0.0.1:8080", "HTTP listen address")
	uiDir := flag.String("ui-dir", "", "directory containing built control UI assets")
	operatorCredentialsPath := flag.String("operator-credentials-path", "", "path to first-boot operator credentials JSON (required in appliance mode)")
	assetLibraryDir := flag.String("asset-library-dir", "", "local OBS-Scene-Assets assets directory (optional)")
	setupStatusPath := flag.String("setup-status-path", "", "path to first-boot setup status JSON")
	initialConfigPath := flag.String("initial-config-path", "", "path to active settings JSON")
	draftConfigPath := flag.String("draft-config-path", "", "path to draft settings JSON")
	healthSummaryPath := flag.String("health-summary-path", "", "path to appliance health summary JSON")
	updateStatusPath := flag.String("update-status-path", "", "path to appliance update status JSON")
	adapterMode := flag.String("adapter-mode", "mock", "adapter mode: mock or appliance")
	obsWebsocketURL := flag.String("obs-websocket-url", "ws://127.0.0.1:4455", "OBS websocket endpoint")
	obsWebsocketPassword := flag.String("obs-websocket-password", "", "OBS websocket password")
	mediamtxURL := flag.String("mediamtx-url", "http://127.0.0.1:9997", "MediaMTX API base URL")
	mediamtxConfigPath := flag.String("mediamtx-config-path", "/etc/mediamtx/mediamtx.yml", "path to the managed MediaMTX configuration file")
	mediamtxDestinationsPath := flag.String("mediamtx-destinations-path", "/etc/bitriver/mediamtx-destinations.json", "path to the managed MediaMTX destinations sidecar")
	mediamtxServiceName := flag.String("mediamtx-service-name", "mediamtx.service", "systemd unit name used to reload or restart MediaMTX")
	owncastURL := flag.String("owncast-url", "http://127.0.0.1:8081", "Owncast base URL")
	logFormat := flag.String("log-format", "json", "log format: json or text")
	pollInterval := flag.Duration("poll-interval", 3*time.Second, "state poll interval for websocket events")
	flag.Parse()

	logger := newLogger(*logFormat)
	if *adapterMode == "appliance" && *operatorCredentialsPath == "" {
		logger.Error("appliance mode requires operator credentials")
		os.Exit(1)
	}
	if !strings.HasPrefix(*httpAddr, "127.0.0.1:") && !strings.HasPrefix(*httpAddr, "localhost:") && !strings.HasPrefix(*httpAddr, "[::1]:") {
		logger.Error("HTTP control API must bind to loopback until HTTPS is configured", "listenAddress", *httpAddr)
		os.Exit(1)
	}
	startedAt := time.Now().UTC()

	activePath := *initialConfigPath
	draftPath := *draftConfigPath
	if draftPath == "" && activePath != "" {
		draftPath = filepath.Join(filepath.Dir(activePath), "draft.json")
	}

	store, err := configstore.New(configstore.Paths{
		ActivePath: activePath,
		DraftPath:  draftPath,
	}, startedAt)
	if err != nil {
		logger.Error("failed to initialize settings store", "error", err)
		os.Exit(1)
	}

	hub := events.NewHub()

	var (
		obsAdapter     adapters.OBS
		mediaAdapter   adapters.MediaMTX
		owncastAdapter adapters.Owncast
		serviceAdapter adapters.Services
	)

	switch *adapterMode {
	case "appliance":
		obsAdapter = obsws.New(
			*obsWebsocketURL,
			*obsWebsocketPassword,
			obsws.WithLogger(logger),
			obsws.WithEventPublisher(hub),
		)
		mediaAdapter = mediamtx.New(
			*mediamtxURL,
			mediamtx.WithLogger(logger),
			mediamtx.WithConfigPath(*mediamtxConfigPath),
			mediamtx.WithDestinationsPath(*mediamtxDestinationsPath),
			mediamtx.WithServiceName(*mediamtxServiceName),
		)
		owncastAdapter = owncast.New(*owncastURL)
		serviceAdapter = systemd.New()
	default:
		mockSuite := mock.NewSuite(startedAt)
		obsAdapter = mockSuite.OBS()
		mediaAdapter = mockSuite.MediaMTX()
		owncastAdapter = mockSuite.Owncast()
		serviceAdapter = mockSuite.Services()
	}

	if *adapterMode == "appliance" && mediaAdapter != nil {
		active := store.Active()
		if _, err := mediaAdapter.ApplyConfig(context.Background(), active.Generation, active.Config); err != nil {
			logger.Warn("initial mediamtx reconcile failed", "error", err)
		}
	}

	controller := service.NewController(service.Dependencies{
		Logger:       logger,
		ConfigStore:  store,
		OBS:          obsAdapter,
		MediaMTX:     mediaAdapter,
		Owncast:      owncastAdapter,
		ServiceUnits: serviceAdapter,
		EventHub:     hub,
		StartedAt:    startedAt,
		Version:      version.String(),
		Paths: service.RuntimePaths{
			SetupStatusPath:  *setupStatusPath,
			HealthStatusPath: *healthSummaryPath,
			UpdateStatusPath: *updateStatusPath,
		},
		PollInterval: *pollInterval,
	})

	var handler http.Handler
	if *operatorCredentialsPath != "" {
		auth, err := httpapi.NewOperatorAuth(*operatorCredentialsPath)
		if err != nil {
			logger.Error("operator authentication unavailable; refusing startup", "error", err)
			os.Exit(1)
		}
		handler = httpapi.NewSecureHandler(logger, controller, *uiDir, *assetLibraryDir, auth)
	} else {
		handler = httpapi.NewHandler(logger, controller, *uiDir, *assetLibraryDir)
	}
	server := &http.Server{
		Addr:              *httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if runner, ok := obsAdapter.(adapters.Runner); ok {
		go runner.Run(ctx)
	}
	go controller.Run(ctx)

	go func() {
		logger.Info("bitriver-control-api listening", "addr", *httpAddr, "adapterMode", *adapterMode)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func newLogger(format string) *slog.Logger {
	options := &slog.HandlerOptions{Level: slog.LevelInfo}
	if format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
