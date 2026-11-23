package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestReflAmtBoundCountsTheDruidBesideAnOpenRestrictedSource: Incubation Druid's
// {C} is in gorge's potential bound (the blank Produced$ with an indeterminate
// Amount$ is unbounded in {C}, mana_reflamt.go), so beside Thieving Varmint -- a
// restricted any-colour source that opens the bound for no payment and adds
// nothing to a spell its caster owns -- the bound built for Abzan Kin-Guard's
// payment is two Swamps, a Forest and the Druid's {C}, which pays {3}{G}. The
// kernel's fold was gated on canTapForManaPC, false for any census gap, so the
// bound priced the cast at three mana and restrBoundDrops dropped it (tail-55
// pair 170, decision 163).
func TestReflAmtBoundCountsTheDruidBesideAnOpenRestrictedSource(t *testing.T) {
	offered := func(varmint bool) bool {
		deck := []cardset.Entry{
			{Name: "Abzan Kin-Guard", Count: 1}, {Name: "Incubation Druid", Count: 1}, {Name: "Thieving Varmint", Count: 1},
			{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 1},
		}
		tbl, da, _, err := cardsettest.Synth(t, "tail-55").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		cfg := LockstepConfig(SurfaceFull)
		cfg.NoElide = true
		sc := newScenTableLib(t, tbl, id["Forest"], cfg)
		sc.perm(id["Swamp"], 0)
		sc.perm(id["Swamp"], 0)
		sc.perm(id["Forest"], 0)
		druid := sc.perm(id["Incubation Druid"], 0)
		if !sc.k.isReflAmt(id["Incubation Druid"]) || !sc.s.caGaps(sc.k, 0).has(sc.slot(druid)) {
			t.Fatal("precondition: the Druid is not a live reflected Amount$ census gap")
		}
		if varmint {
			sc.perm(id["Thieving Varmint"], 0)
		}
		return sc.castOffered(0, sc.hand(id["Abzan Kin-Guard"], 0))
	}
	if !offered(false) {
		t.Fatal("precondition: the cast is not offered without the Varmint (two Swamps, a Forest, the Druid)")
	}
	if !offered(true) {
		t.Error("Abzan Kin-Guard is not offered beside Thieving Varmint: the bound dropped the Druid's {C}")
	}
}
