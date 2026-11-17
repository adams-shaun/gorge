package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// Scratch probe for the lockstep Lashwrithe equip divergence: the kernel
// offers activate(Lashwrithe) at p0's priority and gorge does not. Deleted
// before the fix's commit.

const lashProbeSrc = "Name:Lashwrithe\nManaCost:4\nTypes:Artifact Equipment\nK:Equip:BP BP\n" +
	"S:Mode$ Continuous | AffectedDefined$ Equipped | AddPower$ X | AddToughness$ X\nSVar:X:Count$Valid Swamp.YouCtrl\nOracle:x\n"

const lashPlainSrc = "Name:Lashwrithe Plain\nManaCost:4\nTypes:Artifact Equipment\nK:Equip:3\nOracle:x\n"

const knightProbeSrc = "Name:Probe Knight\nManaCost:2 W\nTypes:Creature Human Knight\nPT:2/2\nOracle:x\n"
const secondKnightSrc = "Name:Probe Squire\nManaCost:W\nTypes:Creature Human Knight\nPT:1/1\nOracle:x\n"

func lashProbeSetupSrc(t *testing.T, src, extra string, attach bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, 91, src, knightProbeSrc, secondKnightSrc)
	if extra != "" {
		moveSeeded(t, e, 0, extra, state.ZBattlefield)
	}
	lash := moveSeeded(t, e, 0, src, state.ZBattlefield)
	knight := moveSeeded(t, e, 0, knightProbeSrc, state.ZBattlefield)
	// settle every trigger the moves queued and drain the stack: the offer
	// walk prices equips at sorcery speed, which an unresolved trigger denies
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			break
		}
		e.resolveTop()
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("setup: stack did not settle (%d entries)", len(e.G.Stack))
	}
	toMain1(t, e)
	if attach {
		e.emit(events.Event{Kind: events.Attach, Obj: lash, IDs: []state.ObjID{knight}})
	}
	return e, lash, knight
}

func lashEquipOffered(e *Engine, lash state.ObjID) bool {
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == lash {
			return true
		}
	}
	return false
}

func TestZZLashProbePlainEquipB(t *testing.T) {
	e, lash, _ := lashProbeSetupSrc(t, lashPlainSrc, secondKnightSrc, true)
	addMana(t, e, 0, "CCC")
	if !lashEquipOffered(e, lash) {
		t.Fatal("BUG: plain {3} equip option withheld with {C}{C}{C} floating")
	}
}

func TestZZLashProbePlainUnattached(t *testing.T) {
	e, lash, knight := lashProbeSetupSrc(t, lashPlainSrc, secondKnightSrc, false)
	addMana(t, e, 0, "CCC")
	for _, o := range e.legalActions(0) {
		t.Logf("opt kind=%s label=%q obj=%d mode=%q", o.Kind, o.Label, o.Obj, o.Mode)
	}
	t.Logf("step=%v active=%v", e.G.Step, e.G.Active)
	t.Logf("knight ctrl=%v tapped=%v", e.G.Obj(knight).Controller, e.G.Obj(knight).Tapped)
	if !lashEquipOffered(e, lash) {
		t.Fatal("BUG: plain {3} equip option withheld with lash UNEQUIPPED and {C}{C}{C} floating")
	}
}

func TestZZLashProbePhyrexianEquip(t *testing.T) {
	e, lash, _ := lashProbeSetupSrc(t, lashProbeSrc, secondKnightSrc, true)
	sa := e.G.Obj(lash).Face().Abilities[0]
	t.Logf("SA kind=%s api=%s cost=%q params=%v", sa.Kind, sa.API, sa.Params["Cost"], sa.Params)
	c := ParseCost(sa.Params["Cost"])
	t.Logf("parsed: generic=%d colored=%v phyrexian=%v hybridphyrexian=%v life=%d", c.Generic, c.Colored, c.Phyrexian, c.HybridPhyrexian, c.Life)
	cc := e.abilityCompiledCost(sa).Cost
	t.Logf("compiled: generic=%d colored=%v phyrexian=%v life=%d", cc.Generic, cc.Colored, cc.Phyrexian, cc.Life)
	pool := e.G.Players[0].Pool
	d := pay.DescriptorFor(lash, true, cc)
	ok := e.manaFeasibleDescriptor(0, d, cc, costMods{}, 0, 0, pipRider{})
	t.Logf("manaFeasible(%v pool=%v) = %v", cc, pool, ok)
	addMana(t, e, 0, "BB")
	if !lashEquipOffered(e, lash) {
		t.Fatal("BUG: {B/P}{B/P} equip option withheld with {B}{B} floating")
	}
}

func TestZZLashProbePhyrexianNoMana(t *testing.T) {
	e, lash, _ := lashProbeSetupSrc(t, lashProbeSrc, secondKnightSrc, true)
	if !lashEquipOffered(e, lash) {
		t.Fatal("BUG: {B/P}{B/P} equip option withheld with no mana (both pips by 2 life, life 20)")
	}
}

func TestZZLashProbePhyrexianGeneric(t *testing.T) {
	e, lash, _ := lashProbeSetupSrc(t, lashProbeSrc, secondKnightSrc, true)
	addMana(t, e, 0, "CC")
	sa := e.G.Obj(lash).Face().Abilities[0]
	cc := e.abilityCompiledCost(sa).Cost
	pool := e.G.Players[0].Pool
	d := pay.DescriptorFor(lash, true, cc)
	ok := e.manaFeasibleDescriptor(0, d, cc, costMods{}, 0, 0, pipRider{})
	cs := costStaticSource{e: e}
	statics := cs.get()
	scope := abilityScope(sa)
	var lc Cost = cc
	ok2 := e.offerCastableUsing(statics, 0, lash, &lc, scope, true, nil)
	t.Logf("generic: pool=%v life=%d step=%v manaFeasible=%v offerCastableUsing=%v", pool, e.G.Players[0].Life, e.G.Step, ok, ok2)
	for _, o := range e.legalActions(0) {
		if o.Obj == lash {
			t.Logf("opt kind=%s label=%q mode=%q", o.Kind, o.Label, o.Mode)
		}
	}
	if !lashEquipOffered(e, lash) {
		wc := newWindowCollector(0)
		e.legalActionsWithWindow(0, wc)
		for _, r := range wc.finish(e.legalActions(0)) {
			t.Logf("window: obj=%d kind=%s reason=%s", r.Obj, r.Kind, r.Reason)
		}
		t.Fatal("BUG: {B/P}{B/P} equip option withheld with {C}{C} floating (one pip by life)")
	}
}
