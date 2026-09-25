// deadly_disguise_deck_test.go — the Deadly Disguise (MKC) Commander precon
// import. The deck is a face-down/morph-family list, so the acceptance
// ratchet cannot cover it without the three morph heads (kw:Morph,
// kw:Megamorph, kw:Disguise) being registered: the cast side
// (rules/cast.go's morphDownFamily) and the CR 708.6 turn-face-up special
// action (rules/morph_turnup.go) read them straight off the printed K: line,
// so they register beside bestow.go, mutate.go and mayflashsac.go.
//
// These tests assert the deck is actually seated by the ratchet and that its
// morph-family carriers measure fully supported — the exact two things that
// fail if the deck file or the registration is removed.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
)

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

// TestDeadlyDisguiseMorphFamilyHeadsAreRegistered pins the registration the
// deck's 24 morph-family carriers depend on: all three heads must be in
// effects.Supported(), and every carrier in the deck must then measure with
// no missing primitive. Reverting the RegisterNonAPI call in
// rules/morph_turnup.go makes the first assertion fail and makes 24 entries
// reappear in the ratchet's measured set.
func TestDeadlyDisguiseMorphFamilyHeadsAreRegistered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	supported := effects.Supported()

	for _, head := range []string{"kw:Morph", "kw:Megamorph", "kw:Disguise"} {
		if !supported[head] {
			t.Fatalf("effects.Supported() is missing %q — the Deadly Disguise carriers will measure as gaps", head)
		}
	}

	// PRECONDITION: the deck genuinely carries morph-family cards. Without
	// this, the per-card loop below would pass vacuously over an empty set.
	carriers := 0
	for _, c := range testutil.RepoDeck(t, reg, "deadly-disguise") {
		f := c.Faces[0]
		if _, ok := f.KeywordParam("Morph"); !ok {
			if _, ok := f.KeywordParam("Megamorph"); !ok {
				if _, ok := f.KeywordParam("Disguise"); !ok {
					continue
				}
			}
		}
		carriers++
		if m := reg.Unsupported(c, supported); len(m) != 0 {
			t.Errorf("%s: Unsupported = %v, want none (its morph family is registered)", f.Name, m)
		}
	}
	if carriers == 0 {
		t.Fatal("no Deadly Disguise card carries Morph/Megamorph/Disguise — the deck or the keyword read is wrong")
	}
}
