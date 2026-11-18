package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestOffZoneGrantsProject: the off-battlefield projection of the
// StatOffZoneGrant and StatRetraceGrant statics. Ashes of the Fallen gives a
// graveyard card (and only a graveyard card) its chosen subtype; Leyline of
// Transformation gives one in hand the same; Six lists Retrace on a graveyard
// card. The lockstep compares battlefield characteristics only, so this is the
// projection's only guard.
func TestOffZoneGrantsProject(t *testing.T) {
	deck := []cardset.Entry{{Name: "Ashes of the Fallen", Count: 1}, {Name: "Leyline of Transformation", Count: 1},
		{Name: "Six", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Raging Goblin", Count: 1}, {Name: "Forest", Count: 10}}
	tbl, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	goblin := slices.Index(tbl.Subtypes, "Goblin")
	if goblin < 0 || tbl.Cards[id["Grizzly Bears"]].Subtypes.Has(goblin) {
		t.Fatalf("precondition: Goblin %d must be a table subtype Grizzly Bears lacks", goblin)
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	ashes := sc.perm(id["Ashes of the Fallen"], 0)
	sc.s.perms[sc.slot(ashes)].chosen = uint8(goblin)
	sc.s.refreshTy(sc.k)
	grave := sc.grave(id["Grizzly Bears"], 0, 1)[0]
	hand := sc.hand(id["Hill Giant"], 0)
	if _, subs, _, _, ok := sc.s.FeatViewTypes(grave); !ok || !subs.Has(goblin) {
		t.Errorf("Ashes: graveyard Bears subs %v ok %v, want Goblin granted", subs, ok)
	}
	if _, subs, _, _, ok := sc.s.FeatViewTypes(hand); ok && subs.Has(goblin) {
		t.Errorf("Ashes: Hill Giant in hand got Goblin; the grant is graveyard-only")
	}
	ley := sc.perm(id["Leyline of Transformation"], 0)
	sc.s.perms[sc.slot(ley)].chosen = uint8(goblin)
	sc.s.refreshTy(sc.k)
	if _, subs, _, _, ok := sc.s.FeatViewTypes(hand); !ok || !subs.Has(goblin) {
		t.Errorf("Leyline: Hill Giant in hand subs %v ok %v, want Goblin granted", subs, ok)
	}
	if names := sc.s.FeatOffKwNames(grave); slices.Contains(names, "Retrace") {
		t.Errorf("Retrace listed on a graveyard card with no Six: %v", names)
	}
	sc.perm(id["Six"], 0)
	sc.s.refreshTy(sc.k)
	if names := sc.s.FeatOffKwNames(grave); !slices.Contains(names, "Retrace") {
		t.Errorf("Six: graveyard Bears keyword names %v, want Retrace", names)
	}
}
