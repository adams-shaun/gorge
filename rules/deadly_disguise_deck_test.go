// deadly_disguise_deck_test.go — the Deadly Disguise (MKC) Commander precon
// import. The deck is a face-down/morph-family list, and the family's two
// remaining shapes are unimplemented: a LAND carrying the keyword cannot be
// cast face down (legal.go's playableFromHand walk handles f.IsLand() and
// continues before the face-down offer), and a non-mana turn-face-up cost
// (Reveal/Sac/{X}) is parsed but never paid. So kw:Morph / kw:Megamorph /
// kw:Disguise stay OUT of effects.Supported() and the deck's 24 carriers are
// named in knownUnsupported as measured gaps (see rules/morph_turnup.go's
// file comment). These tests pin that honest state: the deck is seated by the
// ratchet and every carrier measures against its exact table entry.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// morphHeads are the three family heads the deck's carriers print.
var morphHeads = []string{"kw:Morph", "kw:Megamorph", "kw:Disguise"}

// morphHeadOn returns the family head a face prints, or "".
func morphHeadOn(f interface {
	KeywordParam(string) (string, bool)
}) string {
	for _, h := range []string{"Morph", "Megamorph", "Disguise"} {
		if _, ok := f.KeywordParam(h); ok {
			return "kw:" + h
		}
	}
	return ""
}

// TestDeadlyDisguiseDeckIsSeatedByTheRatchet asserts the imported precon is
// a repo deck the acceptance and param-census ratchets seat: RepoDeckNames
// must name it, its embedded file must declare the commander, and resolving
// it against the corpus must yield 100 cards. A missing file, a bad commander
// or an unresolvable card name fails here before either ratchet's own
// message can be misread as a card-behaviour gap.
func TestDeadlyDisguiseDeckIsSeatedByTheRatchet(t *testing.T) {
	reg := testutil.CorpusRegistry(t)

	found := false
	for _, name := range testutil.RepoDeckNames() {
		if name == "deadly-disguise" {
			found = true
		}
	}
	if !found {
		t.Fatalf("RepoDeckNames() does not include %q: %v", "deadly-disguise", testutil.RepoDeckNames())
	}

	f := testutil.RepoDeckFile(t, "deadly-disguise")
	if f.Commander != "Kaust, Eyes of the Glade" {
		t.Fatalf("deadly-disguise commander = %q, want Kaust, Eyes of the Glade", f.Commander)
	}
	if f.Format != "commander" {
		t.Fatalf("deadly-disguise format = %q, want commander", f.Format)
	}

	// PRECONDITION: the deck is a real 100-card Commander list, not a stub.
	// Count the resolved slice (count-many-times repeated) rather than the
	// JSON entries, so a wrong count is caught too.
	cs := testutil.RepoDeck(t, reg, "deadly-disguise")
	if len(cs) != 100 {
		t.Fatalf("deadly-disguise resolved to %d cards, want 100", len(cs))
	}
	if err := f.ValidateCommander(reg); err != nil {
		t.Fatalf("deadly-disguise is not a legal Commander deck: %v", err)
	}
}

// TestDeadlyDisguiseMorphCarriersAreMeasuredGaps pins the honest measurement
// the registration revert restores: none of the three heads is in
// effects.Supported() while the land face-down cast and the non-mana turn-up
// cost are unimplemented, and every morph-family carrier in the deck measures
// exactly the head its face prints, matching its knownUnsupported entry.
//
// The count is asserted non-zero first so the per-card loop cannot pass
// vacuously over an empty set. If the heads later become fully supported the
// ratchet itself fails the stale entries; this test fails too, at the
// Supported() assertion, which is the reminder to delete both.
func TestDeadlyDisguiseMorphCarriersAreMeasuredGaps(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	supported := effects.Supported()

	for _, head := range morphHeads {
		if supported[head] {
			t.Fatalf("effects.Supported() claims %q while a land face-down cast and a non-mana turn-up cost are unimplemented", head)
		}
	}

	// PRECONDITION: the deck genuinely carries morph-family cards. Without
	// this, the per-card loop below would pass vacuously over an empty set.
	carriers := 0
	for _, c := range testutil.RepoDeck(t, reg, "deadly-disguise") {
		head := morphHeadOn(c.Faces[0])
		if head == "" {
			continue
		}
		carriers++
		got := reg.Unsupported(c, supported)
		want, ok := knownUnsupported[c.Faces[0].Name]
		if !ok {
			t.Errorf("%s needs %v, not in knownUnsupported — the deck's gap must be in the ratchet table", c.Faces[0].Name, got)
			continue
		}
		if !sameSet(want, got) {
			t.Errorf("%s: measured %v, knownUnsupported says %v", c.Faces[0].Name, got, want)
		}
	}
	if carriers == 0 {
		t.Fatal("no Deadly Disguise card carries Morph/Megamorph/Disguise — the deck or the keyword read is wrong")
	}
}
