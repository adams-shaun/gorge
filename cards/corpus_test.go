package cards

import (
	"os"
	"testing"
)

// corpusDir is the fetched corpus. Tests that need it skip when it is absent
// so a clean checkout still passes `go test ./...` without a network fetch.
func corpusDir(t *testing.T) string {
	t.Helper()
	dir := "../.cards/cardsfolder"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("corpus not fetched; run `make fetch-cards`")
	}
	return dir
}

// TestWholeCorpusCompiles is the M0 acceptance gate. The diagnostic budget is
// deliberately tight: a jump means either a parser regression or an upstream
// data change, and both are worth a human look.
func TestWholeCorpusCompiles(t *testing.T) {
	r, diags, err := CompileDir(corpusDir(t))
	if err != nil {
		t.Fatalf("CompileDir: %v", err)
	}
	if len(r.Cards) < 30000 {
		t.Fatalf("compiled %d cards, expected >30000", len(r.Cards))
	}
	const budget = 20
	if len(diags) > budget {
		for i, d := range diags {
			if i == 25 {
				t.Logf("... and %d more", len(diags)-25)
				break
			}
			t.Logf("%s: %s", d.Path, d.Msg)
		}
		t.Fatalf("%d diagnostics, budget is %d", len(diags), budget)
	}
	t.Logf("compiled %d cards with %d diagnostics", len(r.Cards), len(diags))
}

func TestCorpusPrimitiveSurface(t *testing.T) {
	r, _, err := CompileDir(corpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]bool{}
	for _, c := range r.Cards {
		for _, p := range c.Primitives() {
			all[p] = true
		}
	}
	// The spec measured 694 primitives plus api:Mana from intrinsics. Upstream
	// adds a few a year; the bound catches an accidental explosion, not growth.
	if len(all) < 600 || len(all) > 900 {
		t.Fatalf("primitive surface = %d, expected 600..900", len(all))
	}
	t.Logf("primitive surface: %d", len(all))
}
