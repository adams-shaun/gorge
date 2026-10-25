package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// probeAttackerSrc is an ordinary attacker whose only ability is the singular
// "Whenever CARDNAME attacks, you gain 1 life" trigger. GainLife is the
// cheapest observable: the attack doubling shows up as a folded life total.
func probeAttackerSrc(name string) string {
	return "Name:" + name + "\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ ProbeLife | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME attacks, you gain 1 life.\n" +
		"SVar:ProbeLife:DB$ GainLife | LifeAmount$ 1\n"
}

// assertPanharmoniconAttackProbe pins the board shape each echo test depends
// on: the Panharmonicon carrier is on seat 0's battlefield with the expected
// ValidCause$, the probes are on seat 0's battlefield (so Creature.YouCtrl
// resolves relative to that controller), and their triggers are Mode$ Attacks.
func assertPanharmoniconAttackProbe(t *testing.T, e *Engine, carrier state.ObjID, wantCause string, probes ...state.ObjID) {
	t.Helper()
	o := e.G.Obj(carrier)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: carrier %d is not on seat 0's battlefield (%+v)", carrier, o)
	}
	found := false
	for _, st := range o.Face().Statics {
		if st.Mode != "Panharmonicon" {
			continue
		}
		if got := st.ParamStr(cards.PKValidCause); got == wantCause {
			found = true
		}
	}
	if !found {
		t.Fatalf("precondition: carrier has no Panharmonicon static with ValidCause$ %q", wantCause)
	}
	for _, id := range probes {
		p := e.G.Obj(id)
		if p == nil || p.Zone != state.ZBattlefield || p.Controller != 0 {
			t.Fatalf("precondition: probe %d is not on seat 0's battlefield (%+v)", id, p)
		}
		if len(p.Face().Triggers) != 1 || p.Face().Triggers[0].Mode != "Attacks" {
			t.Fatalf("precondition: probe %d does not carry one Mode$ Attacks trigger: %+v", id, p.Face().Triggers)
		}
	}
}

// TestPanharmoniconAttackEchoPerAttacker pins Wulfgar's ValidCause$
// Creature.YouCtrl on the per-attacker queue: a Mode$ Attacks trigger split
// per attacker must be doubled once for EACH matching attacker. Before the
// fix the echo's ValidCause$ arm saw a two-ID DeclareAttackers event, found
// no singular cause, and doubled neither instance.
func TestPanharmoniconAttackEchoPerAttacker(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	wulfgar := onBoardCard(t, e, 0, corpusCard(t, "Wulfgar of Icewind Dale"))
	a := onBoardReady(t, e, 0, probeAttackerSrc("Probe A"))
	b := onBoardReady(t, e, 0, probeAttackerSrc("Probe B"))
	assertPanharmoniconAttackProbe(t, e, wulfgar, "Creature.YouCtrl", a, b)
	if e.G.Players[0].Life != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", e.G.Players[0].Life)
	}

	one := layerEngine(t)
	w1 := onBoardCard(t, one, 0, corpusCard(t, "Wulfgar of Icewind Dale"))
	a1 := onBoardReady(t, one, 0, probeAttackerSrc("Probe A"))
	assertPanharmoniconAttackProbe(t, one, w1, "Creature.YouCtrl", a1)
	one.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{a1}})
	if got := len(one.pendingTriggers); got != 2 {
		t.Fatalf("one attacker queued %d triggers, want 2 (original + echo)", got)
	}
	drainTriggers(t, one)
	if got := one.G.Players[0].Life; got != 22 {
		t.Fatalf("one attacker gained to life %d, want 22", got)
	}

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{a, b}})
	if got := len(e.pendingTriggers); got != 4 {
		t.Fatalf("two attackers queued %d triggers, want 4 (two originals + two echoes)", got)
	}
	drainTriggers(t, e)
	if got := e.G.Players[0].Life; got != 24 {
		t.Fatalf("two attackers gained to life %d, want 24", got)
	}
}

// TestIsshinAttackEchoPerAttacker repeats the per-attacker pin with Isshin's
// broader ValidCause$ Creature, so the fix cannot depend on the YouCtrl
// relative predicate.
func TestIsshinAttackEchoPerAttacker(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	isshin := onBoardCard(t, e, 0, corpusCard(t, "Isshin, Two Heavens as One"))
	a := onBoardReady(t, e, 0, probeAttackerSrc("Probe A"))
	b := onBoardReady(t, e, 0, probeAttackerSrc("Probe B"))
	assertPanharmoniconAttackProbe(t, e, isshin, "Creature", a, b)
	if e.G.Players[0].Life != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", e.G.Players[0].Life)
	}

	one := layerEngine(t)
	i1 := onBoardCard(t, one, 0, corpusCard(t, "Isshin, Two Heavens as One"))
	a1 := onBoardReady(t, one, 0, probeAttackerSrc("Probe A"))
	assertPanharmoniconAttackProbe(t, one, i1, "Creature", a1)
	one.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{a1}})
	drainTriggers(t, one)
	if got := one.G.Players[0].Life; got != 22 {
		t.Fatalf("one attacker gained to life %d, want 22", got)
	}

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{a, b}})
	drainTriggers(t, e)
	if got := e.G.Players[0].Life; got != 24 {
		t.Fatalf("two attackers gained to life %d, want 24", got)
	}
}

// TestPanharmoniconAttackEchoBatchOnce pins the batch AttackersDeclared shape:
// one batch trigger for the declaration, doubled exactly once no matter how
// many attackers it contains. The echo must see the declaration's attackers,
// but it must not fire once per attacker.
func TestPanharmoniconAttackEchoBatchOnce(t *testing.T) {
	t.Parallel()
	e, wulfgar, bard, a, b := panharmoniconBatchSeat(t)
	if len(e.G.Obj(bard).Face().Triggers) == 0 || e.G.Obj(bard).Face().Triggers[0].Mode != "AttackersDeclared" {
		t.Fatalf("precondition: Bard's trigger is not AttackersDeclared: %+v", e.G.Obj(bard).Face().Triggers)
	}
	assertPanharmoniconAttackProbe(t, e, wulfgar, "Creature.YouCtrl")
	for _, id := range []state.ObjID{a, b} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("precondition: attacker %d is not on seat 0's battlefield (%+v)", id, o)
		}
	}

	one, _, _, a1, _ := panharmoniconBatchSeat(t)
	declareOneBatch(t, one, a1)
	handBefore := len(one.G.Zone(state.ZHand, 0))
	drainTriggers(t, one)
	if got := len(one.G.Zone(state.ZHand, 0)) - handBefore; got != 2 {
		t.Fatalf("one-attacker batch declaration drew %d cards, want 2 (trigger + echo)", got)
	}

	declareTwoBatch(t, e, a, b)
	handBefore = len(e.G.Zone(state.ZHand, 0))
	drainTriggers(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)) - handBefore; got != 2 {
		t.Fatalf("two-attacker batch declaration drew %d cards, want 2 (one trigger + one echo)", got)
	}
}

// TestPanharmoniconAttackEchoBatchUsesWholeDeclaration pins the split-attack
// case: finishAttackers emits one DeclareAttackers event per defender, so the
// batch trigger latches on the first event (Fox at seat 1) while its
// ValidCause$ matches the Bear that attacked seat 2. The echo must use the
// whole declaration scratch, not the latched event's own IDs.
func TestPanharmoniconAttackEchoBatchUsesWholeDeclaration(t *testing.T) {
	t.Parallel()
	const bearCauseSrc = "Name:Bear Cause Probe\nManaCost:1 W\nTypes:Enchantment\n" +
		"S:Mode$ Panharmonicon | ValidMode$ AttackersDeclared | ValidCard$ Permanent.YouCtrl | ValidCause$ Creature.Bear | Description$ If a Bear attacking causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time.\n"

	e := threeSeatEngine(t)
	bard := onBoardCard(t, e, 0, corpusCard(t, "Bard, Heir of Girion"))
	probe := onBoard(t, e, 0, bearCauseSrc)
	bear := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	fox := onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	if got := e.G.Obj(probe).Face().Statics[0].ParamStr(cards.PKValidCause); got != "Creature.Bear" {
		t.Fatalf("precondition: custom static ValidCause$ = %q, want Creature.Bear", got)
	}
	if len(e.G.Obj(bard).Face().Triggers) == 0 || e.G.Obj(bard).Face().Triggers[0].Mode != "AttackersDeclared" {
		t.Fatalf("precondition: Bard's trigger is not AttackersDeclared: %+v", e.G.Obj(bard).Face().Triggers)
	}

	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	foxSeat1 := batchAttackerOptionIndex(t, e, fox, 1)
	bearSeat2 := batchAttackerOptionIndex(t, e, bear, 2)
	declareAttackersAt(t, e, [][2]int{{int(fox), foxSeat1}, {int(bear), bearSeat2}})

	var seat1, seat2 []state.ObjID
	for _, ev := range e.L.Events {
		if ev.Kind != events.DeclareAttackers {
			continue
		}
		switch ev.Player {
		case 1:
			seat1 = append(seat1, ev.IDs...)
		case 2:
			seat2 = append(seat2, ev.IDs...)
		}
	}
	if len(seat1) != 1 || seat1[0] != fox || len(seat2) != 1 || seat2[0] != bear {
		t.Fatalf("precondition: split declaration events = seat1 %v, seat2 %v; want only the Fox then only the Bear", seat1, seat2)
	}

	handBefore := len(e.G.Zone(state.ZHand, 0))
	drainTriggers(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)) - handBefore; got != 2 {
		t.Fatalf("split batch declaration drew %d cards, want 2 (the echo saw the Bear in the whole declaration)", got)
	}
}

// panharmoniconBatchSeat builds the three-seat batch board: Wulfgar and Bard
// on seat 0, two ordinary ready attackers, active and in the declare step.
func panharmoniconBatchSeat(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	e := threeSeatEngine(t)
	wulfgar := onBoardCard(t, e, 0, corpusCard(t, "Wulfgar of Icewind Dale"))
	bard := onBoardCard(t, e, 0, corpusCard(t, "Bard, Heir of Girion"))
	a := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e, wulfgar, bard, a, b
}

// panharmoniconBatchEngine is the same board as panharmoniconBatchSeat for the
// one-attacker control, returning [engine, wulfgar, bard, attacker].
func panharmoniconBatchEngine(t *testing.T) [4]interface{} {
	t.Helper()
	e := threeSeatEngine(t)
	wulfgar := onBoardCard(t, e, 0, corpusCard(t, "Wulfgar of Icewind Dale"))
	bard := onBoardCard(t, e, 0, corpusCard(t, "Bard, Heir of Girion"))
	a := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return [4]interface{}{e, wulfgar, bard, a}
}

// declareOneBatch declares the single attacker at seat 1.
func declareOneBatch(t *testing.T, e *Engine, attacker state.ObjID) {
	t.Helper()
	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, attacker, 1)
	declareAttackersAt(t, e, [][2]int{{int(attacker), ai}})
}

// declareTwoBatch declares both attackers at seat 1 in one declaration.
func declareTwoBatch(t *testing.T, e *Engine, a, b state.ObjID) {
	t.Helper()
	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, a, 1)
	bi := batchAttackerOptionIndex(t, e, b, 1)
	declareAttackersAt(t, e, [][2]int{{int(a), ai}, {int(b), bi}})
}
