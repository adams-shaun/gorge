package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// reverberateOnBolt has seat 0 Bolt seat 1 and seat 1 answer with Reverberate,
// passing priority until the Reverberate has begun to resolve.
func reverberateOnBolt(t *testing.T, eng *Engine) (bolt, rev state.ObjID) {
	t.Helper()
	addMana(t, eng, 0, "R")
	addMana(t, eng, 1, "RR")
	bolt = miscHandObj(t, eng, 0, "Lightning Bolt")
	submitChoices(t, eng, miscCastOption(t, eng, bolt))
	for _, o := range eng.Pending().Options {
		if o.Kind == "player" && o.Player == 1 {
			submitChoices(t, eng, o.Index)
			break
		}
	}
	submitPass(t, eng)
	rev = miscHandObj(t, eng, 1, "Reverberate")
	submitChoices(t, eng, miscCastOption(t, eng, rev))
	for _, o := range eng.Pending().Options {
		if o.Obj == bolt {
			submitChoices(t, eng, o.Index)
			break
		}
	}
	passUntilNonPriority(t, eng, 10)
	return bolt, rev
}

// TestCopyElectionPrecedesPriority: CR 707.10c, "once the player has decided
// what the copy's targets will be, the copy is put onto the stack with those
// targets". Nobody receives priority over a copy whose targets are undecided,
// so the election comes inside the resolution of the spell that copies.
func TestCopyElectionPrecedesPriority(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	eng, cfg := miscHandsEngine(t, reg, []string{"Lightning Bolt"}, []string{"Reverberate"}, nil, nil)
	_, rev := reverberateOnBolt(t, eng)
	d := eng.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		t.Fatalf("expected the copy-target ask, got %+v", d)
	}
	if got := eng.G.Obj(rev).Zone; got != state.ZStack {
		t.Fatalf("Reverberate in %s at the election, want still on the stack", got)
	}
	submitChoices(t, eng, d.Options[0].Index)
	replayCheck(t, eng, cfg)
}

// TestCopyElectionTriggersWardAboveTheCopy: a copy's new target becomes the
// target as the copy goes on the stack, so ward goes on the stack above it and
// can counter it.
func TestCopyElectionTriggersWardAboveTheCopy(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	eng, cfg := miscHandsEngine(t, reg, []string{"Lightning Bolt"}, []string{"Reverberate"}, []string{"Waterfall Aerialist"}, nil)
	warded := miscBoardObj(t, eng, 0, "Waterfall Aerialist")
	reverberateOnBolt(t, eng)
	d := eng.Pending()
	if d == nil || d.ResumeKind != "copy_targets" {
		t.Fatalf("expected the copy-target ask, got %+v", d)
	}
	copyID := d.Source
	for _, o := range d.Options {
		if o.Obj == warded {
			submitChoices(t, eng, o.Index)
			break
		}
	}
	n := len(eng.G.Stack)
	if n < 2 || eng.G.Stack[n-2] != copyID || eng.G.Obj(eng.G.Stack[n-1]).Ability == nil {
		t.Fatalf("stack %v: want ward above the copy %d", eng.G.Stack, copyID)
	}
	replayCheck(t, eng, cfg)
}
