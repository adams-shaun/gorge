package draftlab

import (
	"os"
	"strings"
	"testing"
)

// TestDraftlabImportsStayClean keeps draftlab's non-test files free of
// imports whose reach is a determinism break or a host coupling, exactly
// the way cmd/draftsim's imports test does (a compile-time ban needs a
// Go file; a regular expression is caught by it too, so the file list is
// checked instead).
func TestDraftlabImportsStayClean(t *testing.T) {
	banned := map[string]string{
		"time":             "draftlab is a pure API: no wall clock may reach a result",
		"math/rand":        "use math/rand/v2 and seed every stream (PCG) explicitly",
		"os/exec":          "draftlab never shells out",
		"net":              "draftlab has no network reach",
		"net/http":         "draftlab has no network reach",
		"testkit/testutil": "testutil is test-only; a non-test file importing it breaks `go build ./...`",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			for path, why := range banned {
				switch {
				case strings.HasPrefix(line, "import \"") && strings.HasSuffix(line, path+"\""),
					strings.HasPrefix(line, "import ") && strings.Contains(line, "\""+path+"\""),
					strings.HasPrefix(line, "\""+path+"\""):
					t.Errorf("%s imports %s: %s", e.Name(), path, why)
				}
			}
		}
	}
}
