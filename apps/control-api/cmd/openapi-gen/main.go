package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitriver/studio-os/apps/control-api/internal/openapi"
)

func main() {
	outputPath := flag.String("out", filepath.Join("..", "..", "packages", "shared", "openapi", "bitriver-control-api.yaml"), "output path for generated OpenAPI spec")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outputPath, openapi.YAML(), 0o640); err != nil {
		fmt.Fprintf(os.Stderr, "write spec: %v\n", err)
		os.Exit(1)
	}
}
