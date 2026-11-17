package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitriver/studio-os/apps/control-api/internal/operatorcredentials"
)

func main() {
	path := flag.String("credentials-path", "/var/lib/bitriver/bootstrap/admin-credentials.json", "local operator credentials file")
	flag.Parse()
	if flag.NArg() != 1 || flag.Arg(0) != "rotate" {
		fmt.Fprintln(os.Stderr, "usage: bitriver-operator-key [-credentials-path FILE] rotate")
		os.Exit(2)
	}

	key, err := operatorcredentials.Rotate(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "operator key rotation failed:", err)
		os.Exit(1)
	}
	// An earlier first-boot recovery note is now obsolete. Invalidate it to
	// prevent operators attempting to use the previous key later.
	notice := filepath.Join(filepath.Dir(*path), "one-time-admin-password.txt")
	if err := os.Remove(notice); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "warning: delete the obsolete first-boot notice manually:", err)
	}

	fmt.Fprintln(os.Stdout, "New BitRiver operator key (shown once; save it securely):")
	fmt.Fprintln(os.Stdout, key)
	fmt.Fprintln(os.Stdout, "All previous operator keys and active sessions are invalidated.")
}
