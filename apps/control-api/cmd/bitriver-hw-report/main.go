package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/hardware"
)

func main() {
	output := flag.String("output", "-", "output JSON file, or - for stdout")
	root := flag.String("root", "/", "filesystem root (for fixture testing)")
	flag.Parse()

	report := hardware.Probe(*root, time.Now().UTC())
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	payload = append(payload, '\n')

	if *output == "-" {
		if _, err := os.Stdout.Write(payload); err != nil {
			fatal(err)
		}
		return
	}
	if err := writeAtomic(*output, payload); err != nil {
		fatal(err)
	}
}

func writeAtomic(path string, payload []byte) (err error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, ".bitriver-hw-report-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	if err := temp.Chmod(0o640); err != nil {
		return err
	}
	if _, err := temp.Write(payload); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("commit hardware report: %w", err)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "bitriver-hw-report:", err)
	os.Exit(1)
}
