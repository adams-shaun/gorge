package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A card printing a red flashback, with a further flashback instance granted
// at its blue mana cost (CR 702.34a, 113.2c: each instance is its own
// permission, cast for its own cost). Only the surged mode's composed cost is
// read for the search: every other card mode stays unread, because dirac's
// mirror prices those plays from no pips (mana_grantedoffers.go
// playNeedPrinted). A read here reorders gorge's colour asks against the
// kernel's hint and diverges on a supported card (dirac/oracle
// TestFlashbackCastPoolColoursLockstep).
func TestFlashbackCastNeedStaysUnread(t *testing.T) {
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

	ppPriority(t, e)
	a := decision.PotentialAction{Kind: "cast", Obj: card, Mode: "flashback"}
	sr := e.newScriptSearch(0, a, 0, potentialScriptNodes)
	if sr == nil {
		t.Fatal("no search for the flashback cast")
	}
	if sr.costOK || sr.pipOK || sr.need[state.MU] || sr.need[state.MR] {
		t.Fatalf("flashback cast priced a mode cost it must leave unread: costOK=%v pipOK=%v need=%v", sr.costOK, sr.pipOK, sr.need)
	}
	// The descriptor keeps the pre-existing conservative printed base.
	if c := scriptPlayCost(e, e.G.Obj(card), 0, a); c.Generic != 0 || c.Colored[state.MU] != 1 {
		t.Fatalf("descriptor = %+v, want the printed base {U}", c)
	}
}
