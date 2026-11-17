package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicReplacesReport(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "capabilities.json")
	for _, expected := range [][]byte{[]byte("{\"first\":true}\n"), []byte("{\"second\":true}\n")} {
		if err := writeAtomic(path, expected); err != nil {
			t.Fatalf("write report: %v", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, expected) {
			t.Fatalf("wrong content: %q", content)
		}
	}
	mode, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode.Mode().Perm() != 0o640 {
		t.Fatalf("report should be private to service accounts, got %#o", mode.Mode().Perm())
	}
}
