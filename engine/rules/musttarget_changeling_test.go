package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 702.73a: a changeling is every creature type, so it is a Flagbearer and
// an opponent's targeting may choose it as the Flagbearer the rule demands
// (Coalition Honor Guard beside a Changeling Outcast).
func TestFlagbearerRuleCountsChangeling(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	guard := lookup(t, reg, "Coalition Honor Guard")
	outcast := lookup(t, reg, "Changeling Outcast")
	bears := lookup(t, reg, "Grizzly Bears")
	bolt := lookup(t, reg, "Lightning Bolt")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{guard, outcast, bears}, []*cards.Card{bolt})
	g := moveByName(t, e, 0, "Coalition Honor Guard", state.ZBattlefield)
	c := moveByName(t, e, 0, "Changeling Outcast", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 1, "Lightning Bolt", state.ZHand)
	addMana(t, e, 1, "R")
	e.askPriority(1)
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target decision, got %+v", d)
	}
	if indexOfObjOption(d, g) < 0 || indexOfObjOption(d, c) < 0 || indexOfObjOption(d, bear) >= 0 {
		t.Fatalf("Bolt must offer the Flagbearer and the changeling, not the Bear: %+v", d.Options)
	}
}

// TestHasTypeCountsChangelingCreatureTypes pins the read itself: the rules
// package's hasType (used by tapPowerValueFor's host qualifier among others)
// answers true for any creature subtype on a battlefield Changeling, false
// for a word no Changeling grants, and false for a non-Changeling.
func TestHasTypeCountsChangelingCreatureTypes(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	avian := lookup(t, reg, "Avian Changeling")
	bears := lookup(t, reg, "Grizzly Bears")

	e, _ := corpusEngineCfg(t, reg, []*cards.Card{avian, bears}, nil)
	av := e.G.Obj(moveByName(t, e, 0, "Avian Changeling", state.ZBattlefield))
	bear := e.G.Obj(moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield))
	if av == nil || bear == nil {
		t.Fatal("setup: objects missing")
	}
	if !e.hasType(av, "Flagbearer") {
		t.Errorf("Changeling must read as a Flagbearer (CR 702.73a)")
	}
	if !e.hasType(av, "Creature") {
		t.Errorf("Changeling must still read as a Creature")
	}
	if e.hasType(av, "Aura") {
		t.Errorf("Changeling grants creature types only: Aura must read false")
	}
	if e.hasType(bear, "Flagbearer") {
		t.Errorf("a non-Changeling must not read as a Flagbearer")
	}
}
