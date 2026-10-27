// altcost_copy_provenance_test.go pins the alternative-cost half of CR 707.10:
// a copy of a spell copied while it is on the stack keeps the alternative cost
// that was paid for it (evoke, dash, warp), so the token the copy becomes gets
// the same battlefield-entry hook. It is the counterpart of
// TestMayFlashSacCopiedSpellIsNotSacrificed, which covers the provenance bit a
// copy loses because its rider is conditioned on the spell having been CAST.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// copyStackSpell copies the spell id on the stack and returns the minted
// copy's id. The preconditions are asserted here so a test that stops
// exercising the provenance reads fails instead of passing vacuously: the
// original is a stack spell carrying bit, and the copy is a fresh stack object
// carrying the same bit.
func copyStackSpell(t *testing.T, e *Engine, id state.ObjID, bit uint64) state.ObjID {
	t.Helper()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack || o.CastFlags&bit == 0 {
		t.Fatalf("setup: cast spell %d is not a stack spell carrying bit %#x", id, bit)
	}
	before := len(e.G.Objs)
	e.emit(events.Event{Kind: events.StackCopy, Obj: id, Player: 0})
	copyID := state.ObjID(len(e.G.Objs))
	if copyID != state.ObjID(before+1) {
		t.Fatalf("setup: StackCopy minted %d objects, want 1", copyID-state.ObjID(before))
	}
	if o := e.G.Obj(copyID); o == nil || !o.IsCopy || o.Zone != state.ZStack {
		t.Fatalf("setup: minted copy %+v, want a stack copy", e.G.Obj(copyID))
	}
	if o := e.G.Obj(copyID); o.CastFlags&bit == 0 {
		t.Fatalf("setup: the copy did not inherit cast bit %#x", bit)
	}
	return copyID
}

// orderDelayedTriggers answers an ordering ask for the end-step delayed
// triggers both the cast permanent and its copy token registered. Both are the
// same rider, so the offered order is taken as-is.
func orderDelayedTriggers(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		return
	}
	choices := make([]int, 0, len(d.Options))
	for _, o := range d.Options {
		choices = append(choices, o.Index)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit trigger order: %v", err)
	}
}

// TestCopyOfEvokedSpellIsSacrificed: the evoke sacrifice (CR 702.74a) keys on
// the alternative cost having been paid, so a copy of an evoked spell enters
// with the same sacrifice trigger even though the copy itself was never cast.
func TestCopyOfEvokedSpellIsSacrificed(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 951, []string{"Nulldrifter"}, nil, nil)
	id := findCardObj(t, e, 0, "Nulldrifter", state.ZHand)
	addMana(t, e, 0, "CCU")
	submitChoices(t, e, castModeOption(t, e, id, "evoked"))
	copyID := copyStackSpell(t, e, id, state.FlagEvoked)

	passUntilStackEmpty(t, e, 60)

	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("cast Nulldrifter zone %s, want graveyard (evoke sacrifices it)", o.Zone)
	}
	if o := e.G.Obj(copyID); o.Zone != state.ZCeased {
		t.Fatalf("copy token zone %s, want ceased: the inherited evoke trigger sacrificed it", o.Zone)
	}
	sacrificed := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield &&
			ev.To == state.ZGraveyard && ev.Text == "sacrificed" &&
			(ev.Obj == id || ev.Obj == copyID) {
			sacrificed++
		}
	}
	if sacrificed != 2 {
		t.Fatalf("evoke sacrifice MoveZones %d, want 2 (the cast permanent and the copy token)", sacrificed)
	}
	replayCheck(t, e, cfg)
}

// TestCopyOfDashedSpellReturnsAtEndStep: dash (CR 702.109a) grants haste and
// returns the permanent at the beginning of the next end step. A copy of a
// dashed spell keeps that cost-conditioned hook, so the token it becomes is
// hasty and leaves at the end step too.
func TestCopyOfDashedSpellReturnsAtEndStep(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 952, []string{"Mardu Scout"}, nil, nil)
	id := findCardObj(t, e, 0, "Mardu Scout", state.ZHand)
	addMana(t, e, 0, "CR")
	submitChoices(t, e, castModeOption(t, e, id, "dashed"))
	copyID := copyStackSpell(t, e, id, state.FlagDashed)

	passUntilStackEmpty(t, e, 60)

	if o := e.G.Obj(copyID); o.Zone != state.ZBattlefield || !o.IsToken {
		t.Fatalf("copy zone %s isToken=%v, want a battlefield token", o.Zone, o.IsToken)
	}
	if !e.HasKeyword(copyID, "Haste") {
		t.Fatal("the dashed copy token has no haste")
	}

	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	orderDelayedTriggers(t, e)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(id); o.Zone != state.ZHand {
		t.Fatalf("dashed Mardu Scout zone %s after the end step, want hand", o.Zone)
	}
	if o := e.G.Obj(copyID); o.Zone != state.ZCeased {
		t.Fatalf("dashed copy token zone %s after the end step, want ceased", o.Zone)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("dash registrations not consumed: %d", len(e.G.Delayed))
	}
	replayCheck(t, e, cfg)
}

// TestCopyOfWarpedSpellIsExiledAtEndStep: warp (CR 702.185a) exiles the
// permanent the spell becomes if the warp cost was paid. A copy of a warped
// spell keeps that hook, so the token it becomes is exiled at the same end
// step while the cast card also goes to exile.
func TestCopyOfWarpedSpellIsExiledAtEndStep(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 953, []string{"Timeline Culler"}, nil, nil)
	id := findCardObj(t, e, 0, "Timeline Culler", state.ZGraveyard)
	addMana(t, e, 0, "B")
	submitChoices(t, e, castModeOption(t, e, id, "warped"))
	copyID := copyStackSpell(t, e, id, state.FlagWarped)

	passUntilStackEmpty(t, e, 60)

	if o := e.G.Obj(copyID); o.Zone != state.ZBattlefield || !o.IsToken {
		t.Fatalf("copy zone %s isToken=%v, want a battlefield token", o.Zone, o.IsToken)
	}
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("cast Timeline Culler zone %s, want battlefield", o.Zone)
	}

	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	orderDelayedTriggers(t, e)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(id); o.Zone != state.ZExile {
		t.Fatalf("warped Timeline Culler zone %s after the end step, want exile", o.Zone)
	}
	if o := e.G.Obj(copyID); o.Zone != state.ZCeased {
		t.Fatalf("warped copy token zone %s after the end step, want ceased", o.Zone)
	}
	replayCheck(t, e, cfg)
}
