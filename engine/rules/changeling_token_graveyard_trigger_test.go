package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const spiritWatcherFixture = "Name:Watcher\nManaCost:1 U\nTypes:Creature Spirit\nPT:1/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Graveyard | ValidCard$ Spirit.YouOwn | TriggerZones$ Battlefield | Execute$ TrigCtr | TriggerDescription$ x\n" +
	"SVar:TrigCtr:DB$ PutCounter | CounterType$ P1P1 | CounterNum$ 1\nOracle:x\n"

// CR 702.73a / 603.10a: a changeling token that dies is a Spirit put into a
// graveyard even while a layer-4 effect is registered (which arms the
// departure type snapshot), so a Spirit watcher triggers on it.
func TestChangelingTokenDeathTriggersSpiritWatcherWithLayer4InPool(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	e.G.Tokens = reg.Tokens
	w := onBoard(t, e, 0, spiritWatcherFixture)
	tok := mintToken(t, e, 0, "c_1_1_shapeshifter_changeling")
	// A layer-4 effect on the watcher alone arms layer4InPool.
	e.AddContinuous(state.ContinuousEffect{Source: w, Timestamp: 1, Layer: state.LType, Sub: state.SubModify,
		Affects: "Card.Self", Controller: 0, AddTypes: []string{"Artifact"}})
	e.AddContinuous(state.ContinuousEffect{Source: w, Timestamp: 2, Layer: state.LPT, Sub: state.SubModify,
		Affects: "Creature.token", Controller: 0, AddPower: -1, AddToughness: -1})
	if !e.layer4InPool {
		t.Fatal("precondition: layer-4 effect must be in the pool")
	}
	if e.Toughness(tok) != 0 || !e.Derived(tok).AllCreatureTypes {
		t.Fatalf("precondition: token toughness %d, changeling %v", e.Toughness(tok), e.Derived(tok).AllCreatureTypes)
	}
	e.checkStateBased()
	if o := e.G.Obj(tok); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("precondition: the token must have died")
	}
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Errorf("stack = %d objects after the changeling token died, want 1 Spirit-watcher trigger", len(e.G.Stack))
	}
}
