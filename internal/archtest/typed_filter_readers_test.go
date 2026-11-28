package archtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// typedFilterReaderCalls are the spellings of a read of, or a registration
// for, engine/cards' typed filter slots (engine/cards/filter_slot.go).
var typedFilterReaderCalls = []string{".FilterOf(", "ParamSetFilter(", "RegisterFilterBinder("}

// typedFilterOffenders returns, sorted, each file of files (paths relative to
// root) outside engine/cards/ and engine/cardc/ whose text holds a typed
// filter call.
func typedFilterOffenders(t *testing.T, root string, files []string) []string {
	t.Helper()
	var out []string
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "engine/cards/") || strings.HasPrefix(rel, "engine/cardc/") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range typedFilterReaderCalls {
			if strings.Contains(string(src), call) {
				out = append(out, rel+": "+call)
				break
			}
		}
	}
	return out
}

// TestNoEngineReaderOfTypedFilters: until a ticket wires the typed filters
// into play, no engine package outside engine/cards and engine/cardc reads
// (or registers a binder for) them, so the seam is inert.
func TestNoEngineReaderOfTypedFilters(t *testing.T) {
	root := filepath.Join("..", "..")

	// The detector, on a synthetic tree.
	tmp := t.TempDir()
	write := func(rel, body string) string {
		p := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	synthetic := []string{
		write("engine/rules/a.go", "package rules\nvar _, _ = sa.FilterOf(cards.PKValidTgts)\n"),
		write("engine/effects/b.go", "package effects\nfunc init() { cards.RegisterFilterBinder(\"x\", nil) }\n"),
		write("engine/cards/c.go", "package cards\nvar _, _ = ParamSetFilter(nil, nil, 0)\n"),
		write("engine/state/d.go", "package state\n"),
	}
	got := typedFilterOffenders(t, tmp, synthetic)
	if len(got) != 2 || !strings.HasPrefix(got[0], "engine/rules/a.go") || !strings.HasPrefix(got[1], "engine/effects/b.go") {
		t.Fatalf("synthetic tree: want engine/rules/a.go and engine/effects/b.go, got %q", got)
	}

	// No vacuous pass: the walk reaches the seam's own home.
	files := goSourcesUnder(t, filepath.Join(root, "engine"))
	seam := false
	for _, f := range files {
		if filepath.ToSlash(f) == filepath.ToSlash(filepath.Join(root, "engine/cards/filter_slot.go")) {
			seam = true
		}
	}
	if !seam {
		t.Fatal("engine/cards/filter_slot.go not in the walk; the check would pass vacuously")
	}
	for _, off := range typedFilterOffenders(t, root, files) {
		t.Errorf("engine reader of typed filters: %s", off)
	}
}
