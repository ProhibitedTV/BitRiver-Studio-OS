package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/core"
	"github.com/bitriver/studio-os/apps/control-api/internal/httpapi"
	"github.com/bitriver/studio-os/apps/control-api/internal/version"
)

func main() {
	httpAddr := flag.String("http-addr", "127.0.0.1:8080", "HTTP listen address")
	uiDir := flag.String("ui-dir", "", "directory containing built control UI assets")
	flag.Parse()

	startedAt := time.Now().UTC()
	store := core.NewStateStore(startedAt, version.String())
	handler := httpapi.NewHandler(store, *uiDir)

	server := &http.Server{
		Addr:              *httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("bitriverd listening on %s", *httpAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
