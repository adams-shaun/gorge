package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadDiracSupportedTakesAPodRowName pins that an evidence row's seats
// column (Name\t<seats>, what a pod run's landing records) is not part of the
// card name: dirac's Commander coverage would otherwise lose every pod-only
// card.
func TestReadDiracSupportedTakesAPodRowName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence", "pod-cmd5.txt"),
		[]byte("Sol Ring\t4\nLightning Bolt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readDiracSupported(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got["sol ring"] || !got["lightning bolt"] {
		t.Errorf("readDiracSupported = %v, want both names without the seats column", got)
	}
	if len(got) != 2 {
		t.Errorf("readDiracSupported = %v, want 2 names", got)
	}
}
