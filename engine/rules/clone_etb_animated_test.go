package rules

// An as-enters copy of "a creature" ranges over what is a creature now, not over
// the printed types: with Opalescence out, a Glorious Anthem is a creature, so Synth
// Infiltrator is offered it as a template, and the copy happens when the election
// is applied (CR 707.2, 613.1d).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAsEntersCopyOffersAndCopiesAPermanentThatBecameACreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "Synth Infiltrator", "Opalescence", "Glorious Anthem")
	opal := searchMoveByName(t, e, "Opalescence", state.ZBattlefield)
	anthem := searchMoveByName(t, e, "Glorious Anthem", state.ZBattlefield)
	synth := searchMoveByName(t, e, "Synth Infiltrator", state.ZHand)
	if !e.IsCreature(anthem) || e.IsCreature(opal) {
		t.Fatalf("precondition: Opalescence makes the other enchantment a creature (anthem %v, opalescence %v)",
			e.IsCreature(anthem), e.IsCreature(opal))
	}

	addMana(t, e, 0, "CCCUU")
	castCardByName(t, e, synth)
	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" || d.Player != 0 {
		t.Fatalf("Synth's entry did not pose its etb election: %+v", d)
	}
	at := -1
	for _, o := range d.Options {
		if o.Kind != "clone" {
			continue
		}
		if o.Obj == opal {
			t.Fatalf("Opalescence is not a creature and was offered: %+v", d.Options)
		}
		if o.Obj == anthem {
			at = o.Index
		}
	}
	if at < 0 {
		t.Fatalf("the creature Glorious Anthem was not offered: %+v", d.Options)
	}
	submitChoices(t, e, at)
	passUntilStackEmpty(t, e, 40)

	copied := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.ClonePermanent && ev.Obj == synth && len(ev.IDs) > 0 && ev.IDs[0] == anthem {
			copied = true
		}
	}
	if !copied {
		t.Fatalf("Synth Infiltrator did not become a copy of the chosen Glorious Anthem")
	}
}
