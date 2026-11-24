package lockstep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
)

// fallbackCount is the number of fallback files; the register changes only in
// review.
const fallbackCount = 2

func readFallbackRegister(t *testing.T) (string, map[string]gorgebe.Fallback) {
	t.Helper()
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	fb, err := gorgebe.ReadFallbacks(filepath.Join(root, "engine", "cardc", "testdata", "fallback"))
	if err != nil {
		t.Fatal(err)
	}
	return root, fb
}

// TestFallbackRegister: every fallback card is a proof-slice card with no
// ledger file, and no carried fork patch claims to fix it.
func TestFallbackRegister(t *testing.T) {
	root, fb := readFallbackRegister(t)
	names := map[string]bool{}
	for _, s := range []string{"fdn", "legacy"} {
		ns, err := mtgish.SliceNames(root, s)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range ns {
			names[n] = true
			if f, _, ok := strings.Cut(n, " // "); ok {
				names[f] = true
			}
		}
	}
	fork, err := mtgish.ReadForkRegister(filepath.Join(root, "engine", "cardc", "testdata", "mtgish-fork.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	carried := map[string]bool{}
	for _, p := range fork {
		if p.Status == "carried" {
			for _, c := range p.Cards {
				carried[c] = true
			}
		}
	}
	dir, _, _ := ledgerPaths(root)
	for name, f := range fb {
		if !names[name] {
			t.Errorf("%s: fallback card is in neither slice", name)
		}
		if _, err := os.Stat(filepath.Join(dir, gorgebe.Stem(name)+".tsv")); err == nil {
			t.Errorf("%s: fallback card still has a ledger file", name)
		}
		if carried[name] {
			t.Errorf("%s: a carried fork patch fixes it; serve it from cardc", name)
		}
		if f.Class == "mtgish-bug" && f.Issue == 0 {
			t.Logf("%s: mtgish-bug fallback with no upstream issue filed", name)
		}
	}
}

func TestFallbackCount(t *testing.T) {
	_, fb := readFallbackRegister(t)
	if len(fb) != fallbackCount {
		t.Errorf("%d fallback files, want %d (update fallbackCount in review)", len(fb), fallbackCount)
	}
}
