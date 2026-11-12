package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// The Optional$ read (Liar's Pendulum's "you may reveal your hand" is no forced
// reveal), driven through the REAL engine: the
// may-reveal ask suspends the resolution, the decline reveals nothing, the
// acceptance emits the ordinary public reveal Note. The effects-level halves
// of this contract are pinned in engine/effects/look_test.go; this file is the
// engine-level end to end (suspend via e.resume, resume via reveal_optional).

// mayRevealEngine builds a seat-0-start engine whose seat 0 holds one
// may-reveal spell (ManaCost 0, no targets) and one Bear to reveal.
func mayRevealEngine(t *testing.T, line string) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	c := card(t, "Name:MayReveal\nManaCost:0\nTypes:Sorcery\nA:"+line+"\nOracle:x\n")
	o := e.G.AddObject(c, state.PlayerID(0))
	o.Zone = state.ZHand
	e.G.Clock++
	o.Timestamp = e.G.Clock
	bearCard := card(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	bear := e.G.AddObject(bearCard, state.PlayerID(0))
	bear.Zone = state.ZHand
	e.G.Clock++
	bear.Timestamp = e.G.Clock
	e.G.SetZone(state.ZHand, 0, []state.ObjID{o.ID, bear.ID})
	return e, o.ID
}

// TestMayRevealHandAskSuspendsAndDeclineRevealsNothing drives the decline
// arm: the ask suspends mid-resolution, "no" reveals nothing, and the
// resolution finishes (the game continues past it).
func TestMayRevealHandAskSuspendsAndDeclineRevealsNothing(t *testing.T) {
	t.Parallel()
	e, id := mayRevealEngine(t, "SP$ RevealHand | Defined$ You | Optional$ True")
	d := castAndReachRevealAsk(t, e, id)
	if d.Player != 0 || len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("ask = %+v, want a yes/no for the hand's owner", d)
	}
	if len(revealNotes(e)) != 0 {
		t.Fatalf("notes before the answer: %+v", revealNotes(e))
	}
	submitChoices(t, e, 1) // "no"
	if notes := revealNotes(e); len(notes) != 0 {
		t.Fatalf("a declined may-reveal left reveal Notes: %+v", notes)
	}
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("spell zone = %v, want the graveyard after a declined reveal", e.G.Obj(id).Zone)
	}
}

// TestMayRevealHandAcceptEmitsThePublicReveal drives the accept arm: "yes"
// emits exactly one public reveal Note naming the hand, and the
// project-attached decision reaches the deciding seat alone while it is
// pending (the delver peek's privacy property, on the hand shape).
func TestMayRevealHandAcceptEmitsThePublicReveal(t *testing.T) {
	t.Parallel()
	e, id := mayRevealEngine(t, "SP$ RevealHand | Defined$ You | Optional$ True")
	d := castAndReachRevealAsk(t, e, id)

	// While pending, only the deciding seat's view carries the decision.
	if v := view.Project(e.G, e, 0, d); v.Decision == nil {
		t.Fatal("the deciding seat's view carries no decision")
	}
	if v := view.Project(e.G, e, 1, d); v.Decision != nil {
		t.Fatal("the other seat's view carries the may-reveal ask")
	}

	bear := e.G.Zone(state.ZHand, 0)
	submitChoices(t, e, 0) // "yes"
	notes := revealNotes(e)
	if len(notes) != 1 || len(notes[0].IDs) != len(bear) {
		t.Fatalf("reveal Notes = %+v, want exactly one naming the hand %v", notes, bear)
	}
	if notes[0].Secret {
		t.Fatal("an accepted hand reveal must be the public Note")
	}
	for i, want := range bear {
		if notes[0].IDs[i] != want {
			t.Fatalf("reveal ids = %v, want %v", notes[0].IDs, bear)
		}
	}
}
