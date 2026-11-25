package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A card printing a red flashback, with a further flashback instance granted
// at its blue mana cost (CR 702.34a, 113.2c: each instance is its own
// permission, cast for its own cost). The exact-Option search must price the
// instance it searches (Option.Cost "U"), not the printed one ("3 R"); a
// caller holding only the PotentialAction projection cannot name the
// instance, so the need reads must fail open rather than price the printed
// red pips beside the blue play.
func TestFlashbackInstanceNeedPricesSearchedInstance(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const probe = "Name:Flashback Need Probe\nManaCost:U\nTypes:Instant\nK:Flashback:3 R\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	e, _, card := newFixtureDeck(t, 9706, probe)
	toMain1(t, e)
	onBoardCard(t, e, 0, tlaCorpusCard(t, reg, "Iroh, Grand Lotus"))
	onBoardReady(t, e, 0, ggcGuide)
	toGraveyard(t, e, card)
	instances := e.extraFlashbackCosts(card)
	if len(instances) != 1 {
		t.Fatalf("precondition: extraFlashbackCosts = %v, want the granted mana-cost instance", instances)
	}
	granted := instances[0]
	if granted != "U" {
		t.Fatalf("precondition: granted instance = %q, want the mana cost {U}", granted)
	}

	opt := decision.Option{Kind: "cast", Obj: card, Mode: "flashback", Cost: granted}
	ppPriority(t, e)
	action := decision.PotentialAction{Kind: opt.Kind, Obj: opt.Obj, Mode: opt.Mode}
	sr := e.newScriptSearchFor(0, action, opt, true, 0, potentialScriptNodes)
	if sr == nil {
		t.Fatal("no search for the granted flashback instance")
	}
	if !sr.need[state.MU] || sr.need[state.MR] {
		t.Fatalf("need = %v, want U (the searched instance's pip), not R (the printed instance's)", sr.need)
	}
	if !sr.costOK || sr.costCol[state.MU] != 1 || sr.costCol[state.MR] != 0 {
		t.Fatalf("costCol = %v ok=%v, want {U:1} for the searched instance", sr.costCol, sr.costOK)
	}
	if !sr.pipOK || sr.pipCol[state.MU] != 1 || sr.pipCol[state.MR] != 0 {
		t.Fatalf("pipCol = %v ok=%v, want {U:1} for the searched instance", sr.pipCol, sr.pipOK)
	}
	// The descriptor prices the same instance.
	if c := scriptPlayCost(e, e.G.Obj(card), 0, opt); c.Generic != 0 || c.Colored[state.MU] != 1 {
		t.Fatalf("descriptor = %+v, want {U}", c)
	}

	// A caller holding only the projection cannot tell the instances apart:
	// the need reads must fail open, not price the printed {3}{R} beside the
	// searched blue instance. The bargained mode's may-play permission
	// (Option.MayPlayPerm) is the same class of omitted field.
	if _, exact := scriptNeedOption(action); exact {
		t.Fatal("scriptNeedOption reports the flashback projection as exact")
	}
	if _, exact := scriptNeedOption(decision.PotentialAction{Kind: "cast", Mode: "bargained"}); exact {
		t.Fatal("scriptNeedOption reports the bargained projection as exact")
	}
	shadow := e.newScriptSearch(0, action, 0, potentialScriptNodes)
	if shadow == nil {
		t.Fatal("no projection search for the granted flashback instance")
	}
	if shadow.costOK || shadow.pipOK || shadow.need[state.MR] {
		t.Fatalf("projection search priced a cost it cannot name: costOK=%v pipOK=%v need=%v", shadow.costOK, shadow.pipOK, shadow.need)
	}
}
