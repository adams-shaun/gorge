package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRevealXPreAnnouncedZeroRevealsNothing pins the pre-announced-X settle of
// Reveal<X/Spec>: a printed {X} on the same cost makes xAsk announce X before
// the reveal election (CR 601.2b), so the election does NOT announce it. When
// that announcement is 0 the reveal must reveal nothing -- never the payer's
// whole matching hand, which is the opposite of "reveal X = 0".
//
// The probe is synthetic (no corpus card carries a printed {X} beside a
// Reveal<X/Spec> today); the shape is reachable through any announce-bearing
// part composed with Reveal<X/Spec>.
func TestRevealXPreAnnouncedZeroRevealsNothing(t *testing.T) {
	t.Parallel()
	const probe = "Name:Reveal X Probe\nManaCost:X\nTypes:Sorcery\n" +
		"A:SP$ GainLife | Cost$ Reveal<X/Card.White> | LifeAmount$ X | SpellDescription$ Gain X life.\nOracle:x\n"
	const white = "Name:White Scout\nManaCost:W\nTypes:Creature Human Scout\nPT:1/1\nOracle:x\n"

	e := handEngine(t, card(t, probe), card(t, white), card(t, white), card(t, white))
	probeID := e.G.Zone(state.ZHand, 0)[0]
	whites := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)[1:]...)

	// Precondition: the three white cards really are in hand, so the reveal
	// filter has candidates to (wrongly) offer. Without them the assertion
	// below would be vacuous.
	if len(whites) != 3 {
		t.Fatalf("precondition: %d white cards in hand, want 3", len(whites))
	}
	for _, id := range whites {
		if o := e.G.Obj(id); o.Zone != state.ZHand || !strings.Contains(o.Face().Name, "White") {
			t.Fatalf("precondition: candidate %d is %q in %s", id, o.Face().Name, o.Zone)
		}
	}

	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("precondition: no priority decision")
	}
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == probeID {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("precondition: probe not castable at X = 0: %+v", d.Options)
	}
	submitChoices(t, e, castIdx)

	// The printed {X} announces first. Announce 0.
	d = e.Pending()
	if d == nil || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("precondition: want the X announce after choosing cast, got %+v", d)
	}
	xIdx := -1
	for _, o := range d.Options {
		if o.Label == "X = 0" {
			xIdx = o.Index
		}
	}
	if xIdx < 0 {
		t.Fatalf("precondition: no X = 0 option: %+v", d)
	}
	submitChoices(t, e, xIdx)

	// X = 0 has nothing to reveal: posing a reveal pick (even an empty
	// 0..0 one) or auto-revealing the matching hand is the bug. The reveal
	// ask must not appear; anything else pending is the ordinary
	// payment/priority flow.
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "revealcost" {
				t.Fatalf("X = 0 posed a reveal pick: %+v", d)
			}
		}
	}

	passUntilStackEmpty(t, e, 20)

	var revealed []string
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "revealed") && strings.Contains(ev.Text, "as a cost") {
			revealed = append(revealed, ev.Text)
		}
	}
	if len(revealed) != 0 {
		t.Fatalf("X = 0 revealed cards as a cost: %q", revealed)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("life = %d, want 20 (the ability's X stayed 0)", got)
	}
	for _, id := range whites {
		if z := e.G.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("white card %d moved to %s; a reveal leaves the hand", id, z)
		}
	}
	if z := e.G.Obj(probeID).Zone; z != state.ZGraveyard {
		t.Fatalf("probe in %s after resolution, want graveyard", z)
	}
}
