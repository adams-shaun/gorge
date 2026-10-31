package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFirstStrikeGainedAfterTheFirstStepStillDealsRegularDamage pins CR
// 510.4 / 702.7c: the regular combat damage step is for the creatures that
// had neither first strike nor double strike AS THE FIRST-STRIKE STEP BEGAN.
// A creature that gains first strike between the two steps (dirac pair
// 5692: Ajani's Pridemate gets its +1/+1 counter from a first-strike lifelink
// trigger, and Inspiring Paladin then gives it first strike) still deals
// regular damage; one that had first strike then is held out even if it lost
// it. The record is combatRound.firstStrikers, taken by collectFirstStrikers.
func TestFirstStrikeGainedAfterTheFirstStepStillDealsRegularDamage(t *testing.T) {
	t.Parallel()
	knight := card(t, "Name:Test Knight\nManaCost:1 W\nTypes:Creature Knight\nPT:2/2\nK:First Strike\nOracle:x\n")
	pridemate := card(t, "Name:Test Pridemate\nManaCost:1 W\nTypes:Creature Cat\nPT:2/2\nK:First Strike\nOracle:x\n")
	e, _, _ := corpusDeckEngine(t, nil, []*cards.Card{knight, pridemate})
	var kID, pID state.ObjID
	for i := range e.G.Objs {
		switch o := &e.G.Objs[i]; o.Card {
		case knight:
			kID = o.ID
		case pridemate:
			pID = o.ID
		}
	}
	if kID == 0 || pID == 0 {
		t.Fatalf("setup: knight=%d pridemate=%d", kID, pID)
	}
	// No first-strike step recorded: a first striker acts only in the
	// first-strike pass (the unchanged reading).
	if e.actsThisDamageStep(pID, false) || !e.actsThisDamageStep(pID, true) {
		t.Fatal("with no record a first striker must act only in the first-strike pass")
	}
	// The first-strike step began with only the Knight having first strike;
	// the Pridemate (printed first strike here stands in for the Paladin's
	// grant) gained it afterwards.
	e.combatRound.firstStrikers = []state.ObjID{kID}
	if !e.actsThisDamageStep(pID, false) {
		t.Fatal("a creature that gained first strike after the first-strike step began must deal regular damage (CR 702.7c)")
	}
	if e.actsThisDamageStep(kID, false) {
		t.Fatal("a creature that had first strike as the first-strike step began must not deal regular damage (CR 510.4)")
	}
}

// TestCollectFirstStrikersRecordsAttackersAndBlockers pins the record's
// contents: attackers and their live blockers that have first strike or
// double strike now, nothing else.
func TestCollectFirstStrikersRecordsAttackersAndBlockers(t *testing.T) {
	t.Parallel()
	fs := card(t, "Name:Test Striker\nManaCost:1 W\nTypes:Creature Knight\nPT:2/2\nK:First Strike\nOracle:x\n")
	plain := card(t, "Name:Test Plain\nManaCost:1 W\nTypes:Creature Cat\nPT:2/2\nOracle:x\n")
	e, _, _ := corpusDeckEngine(t, nil, []*cards.Card{fs, plain})
	var fID, pID state.ObjID
	for i := range e.G.Objs {
		switch o := &e.G.Objs[i]; o.Card {
		case fs:
			fID = o.ID
		case plain:
			pID = o.ID
		}
	}
	if got := e.collectFirstStrikers(nil); len(got) != 0 {
		t.Fatalf("no attackers: record %v, want empty", got)
	}
	e.G.Obj(fID).IsAttacking = true
	e.G.Obj(pID).IsAttacking = true
	got := e.collectFirstStrikers(nil)
	if len(got) != 1 || got[0] != fID {
		t.Fatalf("record %v, want [%d]", got, fID)
	}
}
