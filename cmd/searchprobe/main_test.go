package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectInvalidBudgetsBeforeLoadingCorpus(t *testing.T) {
	for _, args := range [][]string{{"-games", "0"}, {"-workers", "0"}, {"-worlds", "3"}, {"-attempts", "0"}, {"-max-submits", "0"}} {
		output := filepath.Join(t.TempDir(), "must-not-exist.json")
		args = append(args, "-out", output, "-cards", "missing-corpus")
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "require games/workers/attempts") {
			t.Fatalf("did not reject invalid budget before loading corpus: %v: %v", args, err)
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("invalid budget created output: %v", err)
		}
	}
}
