package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7, 603.10a: Super Shredder dies together with another permanent. It
// looks back and triggers, but the card in the graveyard is a new object the
// ability cannot find, so "put a +1/+1 counter on this creature" does nothing.
func TestDepartedSourceTriggerDoesNotCounterTheGraveyardCard(t *testing.T) {
	for _, tc := range []struct {
		name        string
		sourceFirst bool
	}{{"source first", true}, {"source second", false}} {
		t.Run(tc.name, func(t *testing.T) {
			reg := searchTestRegistry(t)
			e, cfg := searchEngine(t, reg, "Day of Judgment", "Super Shredder")
			var src state.ObjID
			if tc.sourceFirst {
				src = searchMoveByName(t, e, "Super Shredder", state.ZBattlefield)
				searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
			} else {
				searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
				src = searchMoveByName(t, e, "Super Shredder", state.ZBattlefield)
			}
			d := castSpellNamed(t, e, reg, "Day of Judgment", "WWWW")
			cr603LookBackDrive(t, e, d)
			o := e.G.Obj(src)
			if o == nil || o.Zone != state.ZGraveyard {
				t.Fatalf("Super Shredder did not die")
			}
			if got := o.Counter("P1P1"); got != 0 {
				t.Errorf("the graveyard card has %d +1/+1 counters, want 0", got)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// The same for a copy: leaving the battlefield it reverts to the printed card,
// but the ability it created is still the copied creature's (CR 113.7a).
func TestDepartedCopySourceTriggerDoesNotCounterTheGraveyardCard(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Day of Judgment", "Voracious Vermin", "Clone")
	src := searchMoveByName(t, e, "Voracious Vermin", state.ZBattlefield)
	passUntilResolved(t, e, 40)
	castSpellNamed(t, e, reg, "Clone", "UCCC")
	d := passUntilResolved(t, e, 40)
	var clone state.ObjID
	for _, o := range d.Options {
		if o.Obj == src {
			clone = d.Source
			submitChoices(t, e, o.Index)
			break
		}
	}
	if clone == 0 {
		t.Fatalf("Clone's election does not offer Voracious Vermin: %s", d.Prompt)
	}
	for d := passUntilResolved(t, e, 40); d.Kind != decision.KPriority; d = passUntilResolved(t, e, 40) {
		submitChoices(t, e, d.Options[0].Index)
	}
	cr603LookBackDrive(t, e, castSpellNamed(t, e, reg, "Day of Judgment", "WWWW"))
	for _, id := range []state.ObjID{src, clone} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("object %d did not die", id)
		}
		if got := o.Counter("P1P1"); got != 0 {
			t.Errorf("graveyard card %s has %d +1/+1 counters, want 0", o.Face().Name, got)
		}
	}
	replayCheck(t, e, cfg)
}
