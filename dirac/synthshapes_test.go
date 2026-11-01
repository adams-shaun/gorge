package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// synthScen is a scenario over a matchup of a synthetic pool whose two decks
// are names, one of each.
func synthScen(t *testing.T, pool string, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, pool)
	var deck []cardset.Entry
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// TestProtectionFromASubtypeStopsTheBlock: K:Protection:Coyote names a subtype
// no other filter reads, and the matchup keeps it (CR 702.16f).
func TestProtectionFromASubtypeStopsTheBlock(t *testing.T) {
	sc, id := synthScen(t, "std-otj", "Resilient Roadrunner", "Cunning Coyote")
	runner := sc.perm(id["Resilient Roadrunner"], 0)
	coyote := sc.perm(id["Cunning Coyote"], 1)
	if sc.s.canBlock(sc.k, sc.slot(coyote), sc.slot(runner)) {
		t.Fatal("a Coyote can block a creature with protection from Coyotes")
	}
}

// TestHostOverrideReadsItsCondition: Burden of Proof's base 1/1 asks more of its
// host than being attached, so the layer walk reads it; a plain "enchanted
// creature" doesn't need the walk.
func TestHostOverrideReadsItsCondition(t *testing.T) {
	sc, id := synthScen(t, "std-mkm", "Burden of Proof", "Evidence Examiner")
	cd := &sc.k.t.Cards[id["Burden of Proof"]]
	found := false
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
			found = true
			if filterHostOnly(sc.k.t, a.Static.Filter) {
				t.Fatal("Burden of Proof's override reads as unconditional")
			}
			if !filterAttachedOnly(sc.k.t, a.Static.Filter) {
				t.Fatal("Burden of Proof's override is not limited to its host")
			}
		}
	}
	if !found {
		t.Fatal("no override static on Burden of Proof")
	}
}

// TestCardInAGraveyardHearsADeath: Furious Forebear's Dies trigger works from the
// graveyard, for a creature its owner controls.
func TestCardInAGraveyardHearsADeath(t *testing.T) {
	sc, id := synthScen(t, "std-tdm", "Furious Forebear", "Tempest Hawk")
	sc.s.place(sc.obj(id["Furious Forebear"], 0), kop.ZoneGraveyard, false)
	mine := sc.perm(id["Tempest Hawk"], 0)
	theirs := sc.perm(id["Tempest Hawk"], 1)
	sc.s.fireGraveDies(sc.k, sc.slot(theirs))
	if sc.s.npend != 0 {
		t.Fatalf("the opponent's creature dying queued %d triggers", sc.s.npend)
	}
	sc.s.fireGraveDies(sc.k, sc.slot(mine))
	if sc.s.npend != 1 {
		t.Fatalf("its owner's creature dying queued %d triggers, want 1", sc.s.npend)
	}
}

// TestTokenCeasingLeavesNoGraveyard: a token ceasing in a graveyard fires no
// graveyard-leave trigger and adds no zone entry (CR 111.8).
func TestTokenCeasingLeavesNoGraveyard(t *testing.T) {
	sc, id := synthScen(t, "std-mkm", "Chalk Outline", "Evidence Examiner")
	sc.perm(id["Chalk Outline"], 0)
	tok := sc.s.newToken(id["Evidence Examiner"], 0)
	sc.s.place(tok, kop.ZoneGraveyard, false)
	n, e := sc.s.npend, sc.s.g.enteredN
	sc.s.ceaseTokensGen()
	if sc.s.objs[tok].zone != kop.ZoneNone || sc.s.pl[0].graveN != 0 {
		t.Fatal("the token did not cease")
	}
	if sc.s.npend != n || sc.s.g.enteredN != e {
		t.Fatalf("ceasing queued %d triggers and %d zone entries", sc.s.npend-n, sc.s.g.enteredN-e)
	}
}
