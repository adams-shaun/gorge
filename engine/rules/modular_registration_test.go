package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Modular is expanded by cards/kw_modular.go; the corpus reader must also
// see it as supported, or the Arcbound cards never enter a synth pool.
func TestModularCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Arcbound Ravager", "Arcbound Worker", "Arcbound Wanderer"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}

// No corpus card may still report kw:Modular as missing.
func TestNoCardReportsModularUnsupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	supported := effects.Supported()
	for _, c := range reg.AllCards() {
		for _, m := range reg.Unsupported(c, supported) {
			if m == "kw:Modular" {
				t.Errorf("%s still reports kw:Modular", c.Faces[0].Name)
			}
		}
	}
}
