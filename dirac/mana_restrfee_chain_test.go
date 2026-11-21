package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// restrFeeChainScen deals the mixed board (pool tail-73): p0's battlefield is
// an untapped A-Base Camp (its {T}: Add {C}), an untapped Automated Artificer
// ("{T}: Add {C}. Spend this mana only to activate an ability or cast an
// artifact spell") and an untapped Shadowblood Ridge ("{1}, {T}: Add {B}{R}");
// Bold Impaler sits on the battlefield, its "{2}{R}: this gets +2/+0" the
// play whose plan needs the Artificer's restricted {C} together with the
// Ridge's fee chain: the Camp's {C} pays the Ridge's fee, the Ridge's {B}
// and {R} and the Artificer's {C} pay the {2} and the Ridge's {R} the pip.
func restrFeeChainScen(t *testing.T, withRidge, withArtificer bool) (*scen, Obj, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, "tail-73")
	for _, n := range []string{"Bold Impaler", "Arcbound Tracker", "Automated Artificer", "A-Base Camp", "Shadowblood Ridge", "Mountain"} {
		if _, ok := set.Lookup(n); !ok {
			t.Fatalf("%s not in the tail-73 pool", n)
		}
	}
	deck := []cardset.Entry{{Name: "Bold Impaler", Count: 2}, {Name: "Arcbound Tracker", Count: 2},
		{Name: "Automated Artificer", Count: 1}, {Name: "A-Base Camp", Count: 1},
		{Name: "Shadowblood Ridge", Count: 1}, {Name: "Mountain", Count: 3}}
	tbl, da, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Forest", Count: 20}})
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	sc := newScenTableLib(t, tbl, id["Mountain"], cfg)
	sc.perm(id["A-Base Camp"], 0)
	if withArtificer {
		sc.untappedPerm(id["Automated Artificer"], 0)
	}
	if withRidge {
		sc.perm(id["Shadowblood Ridge"], 0)
	}
	impaler := sc.perm(id["Bold Impaler"], 0)
	return sc, impaler, id
}

// activationOffered reports whether an activation of o's printed ability ai
// is among p0's priority options.
func activationOffered(sc *scen, o Obj, ai int) bool {
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	want := MakeAction(AkActivate, TCode(o), 0, uint8(ai))
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == want {
			return true
		}
	}
	return false
}

// TestRestrFeeChainComposedOffer probes the mixed board the ticket predicts
// unoffered: a restricted planner unit whose restriction admits the play
// (the Artificer, "{T}: Add {C}. Spend this mana only to activate an ability
// or cast an artifact spell") beside a fee filter (the Ridge). The kernel
// composes both plays: the restricted arm (restrSrcAffords, through
// castGateAffordable's restrFallback) prices the play over the relaxed avail
// with the filter's fee chain (availFilt), and the filter search
// (filtScript) proves it — the Camp funds the fee, the Ridge's {B} and {R}
// and the Artificer's {C} the {2}, the Ridge's {R} the pip. gorge's planner
// census counts the Artificer per payment (PaymentPlanProductions) and its
// one prefix search prices the chain, so both engines offer.
func TestRestrFeeChainComposedOffer(t *testing.T) {
	sc, impaler, _ := restrFeeChainScen(t, true, true)
	if !sc.s.filtOnly(sc.k, 0) {
		t.Logf("board not filter-only: gapOthers %v", sc.s.gapOthers(sc.k, 0))
	}
	if !activationOffered(sc, impaler, 0) {
		t.Errorf("the {2}{R} activation not offered: the restricted unit's mana and the Ridge's fee chain do not compose (gorge's census and search price both)")
	}
	// the cast: the Artificer's restriction admits artifact spells, so the
	// cast of the Tracker ({2}{R}) is the same composition (the fee from the
	// Camp, the {R} from the Ridge, the {2} from the Ridge's {B} and the
	// Artificer's {C})
	sc4, _, id4 := restrFeeChainScen(t, true, true)
	ogre := sc4.hand(id4["Arcbound Tracker"], 0)
	var dc Decision
	sc4.s.priorityOptions(sc4.k, &dc)
	if !castOffered(&dc, ogre) {
		t.Errorf("the {2}{R} artifact cast not offered: the restricted unit's mana and the Ridge's fee chain do not compose (gorge's census and search price both)")
	}
	// without the Artificer the plan is a unit short (three generic units of
	// the four the fee and the {2} need)
	sc2, impaler2, _ := restrFeeChainScen(t, true, false)
	if activationOffered(sc2, impaler2, 0) {
		t.Errorf("no Artificer: the activation offered although the board funds three units only")
	}
	// without the Ridge the {R} is unpayable: the restricted arm prices the
	// census avail plus the restricted {C}, no coloured pip among them
	sc3, impaler3, _ := restrFeeChainScen(t, false, true)
	if activationOffered(sc3, impaler3, 0) {
		t.Errorf("no Ridge: the activation offered although no source makes {R}")
	}
}

// TestRestrFeeChainAnyROffer probes the ticket's second shape: A-Base Camp's
// party-restricted any-colour unit ("Add one mana of any color. Spend this
// mana only to cast a Cleric, Rogue, Warrior, or Wizard spell or to activate
// an ability of a Cleric, Rogue, Warrior, or Wizard") beside the Ridge — a
// restricted planner unit (restrSrcAb: its widened alternative view holds
// the whole any-colour production) whose payment admits the play. The kernel
// composes both the activation (the Dunecaster's "{1}{W}, {T}: tap target
// creature without flying": the Camp's unit the {W} pip, the Mountain's {R}
// the fee, the Ridge's {B} the {1}) and the cast of Cunning Geysermage (a
// Human Wizard, {2}{U}: the Camp's unit the {U} pip, the Ridge's {B} and {R}
// the {2}). The printed census without the Ridge funds two units only, and
// without the Camp the coloured pip is unpayable.
func TestRestrFeeChainAnyROffer(t *testing.T) {
	scen := func(t *testing.T, withCamp, withRidge bool) (*scen, map[string]CardID, Obj) {
		t.Helper()
		set := cardsettest.Synth(t, "tail-73")
		for _, n := range []string{"Cunning Geysermage", "A-Base Camp", "Shadowblood Ridge", "Mountain", "Dromoka Dunecaster"} {
			if _, ok := set.Lookup(n); !ok {
				t.Fatalf("%s not in the tail-73 pool", n)
			}
		}
		deck := []cardset.Entry{{Name: "Cunning Geysermage", Count: 2},
			{Name: "A-Base Camp", Count: 1}, {Name: "Shadowblood Ridge", Count: 1},
			{Name: "Mountain", Count: 3}, {Name: "Bold Impaler", Count: 1},
			{Name: "Dromoka Dunecaster", Count: 1}}
		tbl, da, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Forest", Count: 20}})
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		cfg := LockstepConfig(SurfaceFull)
		cfg.NoElide = true
		sc := newScenTableLib(t, tbl, id["Mountain"], cfg)
		sc.perm(id["Mountain"], 0)     // the printed {R} unit: the Ridge's fee's funder
		sc.perm(id["Bold Impaler"], 0) // the Dunecaster's activation's target
		if withCamp {
			sc.untappedPerm(id["A-Base Camp"], 0)
		}
		if withRidge {
			sc.perm(id["Shadowblood Ridge"], 0)
		}
		dune := sc.perm(id["Dromoka Dunecaster"], 0)
		return sc, id, dune
	}
	sc, id, dune := scen(t, true, true)
	// the activation: the Dunecaster's "{1}{W}, {T}: tap target creature
	// without flying" — the Camp's unit is the {W} pip, the Mountain's {R}
	// the fee, the Ridge's {B} the {1}
	if !activationOffered(sc, dune, 0) {
		t.Errorf("the {1}{W} Wizard activation not offered: the party-restricted any-colour unit and the Ridge's fee chain do not compose (gorge's census and search price both)")
	}
	mage := sc.hand(id["Cunning Geysermage"], 0)
	var dc Decision
	sc.s.priorityOptions(sc.k, &dc)
	if !castOffered(&dc, mage) {
		t.Errorf("the {2}{U} Wizard cast not offered: the party-restricted any-colour unit and the Ridge's fee chain do not compose (gorge's census and search price both)")
	}
	sc2, id2, dune2 := scen(t, false, true)
	if activationOffered(sc2, dune2, 0) {
		t.Errorf("no Camp: the activation offered although the {W} pip is unpayable")
	}
	mage2 := sc2.hand(id2["Cunning Geysermage"], 0)
	var dc2 Decision
	sc2.s.priorityOptions(sc2.k, &dc2)
	if castOffered(&dc2, mage2) {
		t.Errorf("no Camp: the cast offered although the {U} pip is unpayable")
	}
	sc3, id3, _ := scen(t, true, false)
	mage3 := sc3.hand(id3["Cunning Geysermage"], 0)
	var dc3 Decision
	sc3.s.priorityOptions(sc3.k, &dc3)
	if castOffered(&dc3, mage3) {
		t.Errorf("no Ridge: the cast offered although the board funds two units only")
	}
}
