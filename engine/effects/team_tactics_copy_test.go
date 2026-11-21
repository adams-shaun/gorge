package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const teamTacticsText = "Name:Team Tactics\nManaCost:1 R\nTypes:Instant\nK:Teamwork:1\n" +
	"A:SP$ Pump | ValidTgts$ Creature | KW$ Double Strike\n" +
	"Oracle:x\n"

func teamTacticsOnStack(t *testing.T, h *fakeHost, paid bool) *state.Object {
	t.Helper()
	c := mkCard(t, teamTacticsText)
	o := h.g.AddObject(c, 0)
	events.Move(h.g, o.ID, state.ZLibrary, state.ZStack)
	o = h.g.Obj(o.ID)
	o.TeamworkPaid = paid // the durable field events.Apply folds from the pay-time CastInfo
	return o
}

// CR 702.194b: "cast using teamwork" is whether the player who cast the spell
// declared the intention to pay the teamwork cost as they cast that spell. CR
// 707.10: a copy of a spell is never cast. So a copy made while Team Tactics is
// still on the stack is not "cast using teamwork": its Trample rider is off
// (dirac's copySpell clears the paid flag the same way).
func TestTeamTacticsCopyOnStackWasNotCastUsingTeamwork(t *testing.T) {
	h := newHost(t, 2)
	o := teamTacticsOnStack(t, h, true)
	sc := SpecContext{You: 0, Source: o.ID}
	if !matchesObjectText(h.g, "Card.Self+Teamwork", o, sc) {
		t.Fatal("precondition: the original, cast using teamwork, must match Card.Self+Teamwork")
	}
	Resolve(h, &Ctx{Source: o.ID, Controller: 0}, sa(t, "SP$ CopySpellAbility | Defined$ Parent"))
	if got := copyEvents(h); got != 1 {
		t.Fatalf("precondition: %d StackCopy events, want 1", got)
	}
	var cp *state.Object
	for i := range h.g.Objs {
		if h.g.Objs[i].IsCopy {
			cp = &h.g.Objs[i]
		}
	}
	if cp == nil || cp.Zone != state.ZStack {
		t.Fatalf("precondition: no copy on the stack: %+v", cp)
	}
	csc := SpecContext{You: 0, Source: cp.ID}
	if matchesObjectText(h.g, "Card.Self+Teamwork", cp, csc) {
		t.Fatal("the copy matched Card.Self+Teamwork: a copy is not cast using teamwork (CR 702.194b, 707.10)")
	}
	if !o.TeamworkPaid {
		t.Fatal("copying must not clear the original's paid status")
	}
}

// CR 608.2b, 701.6a, 707.10: a spell that was already countered has left the
// stack and cannot be copied, so no copy (paid or otherwise) is made.
func TestTeamTacticsCopyOfCounteredSpellMakesNothing(t *testing.T) {
	h := newHost(t, 2)
	o := teamTacticsOnStack(t, h, true)
	events.Move(h.g, o.ID, state.ZStack, state.ZGraveyard)
	if g := h.g.Obj(o.ID); g.Zone != state.ZGraveyard {
		t.Fatalf("precondition: countered spell is in %s, want graveyard", g.Zone)
	}
	Resolve(h, &Ctx{Source: o.ID, Controller: 0}, sa(t, "SP$ CopySpellAbility | Defined$ Parent"))
	for i := range h.g.Objs {
		if h.g.Objs[i].IsCopy {
			t.Fatalf("a copy of the already-countered spell exists: %+v", h.g.Objs[i])
		}
	}
}
