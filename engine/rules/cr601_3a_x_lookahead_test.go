package rules

// CR 601.3a: "If an effect prohibits a player from casting a spell with
// certain qualities, that player may consider any choices to be made during
// that spell's proposal that may cause those qualities to change. If any
// such choices could cause that effect to no longer prohibit that player
// from casting that spell, the player may begin to cast the spell, ignoring
// the effect." CR 601.2e then rechecks the proposal with the announced X
// (CR 202.3e), so only an X the prohibition no longer covers is legal.
// Real corpus cards read as script text; the board is built eventlessly and
// funded through the pool.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR601_3aXSpellEscapesManaValueProhibition: the opponent controls Void
// Winnower ("Your opponents can't cast spells with even mana values"). Mind
// Spring ({X}{U}{U}) has mana value 2 in hand, but announcing X = 1 makes it
// 3, so it is offered (CR 601.3a). With {U}{U}{U}{U} floating, announcing
// X = 1 completes the cast onto the stack; announcing X = 0 (mana value 2)
// is reversed to hand by the CR 601.2e recheck.
func TestCR601_3aXSpellEscapesManaValueProhibition(t *testing.T) {
	t.Parallel()
	for _, x := range []int{0, 1} {
		t.Run(map[int]string{0: "X0_even_reversed", 1: "X1_odd_cast"}[x], func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, corpusCardText(t, "m/mind_spring.txt")))
			spring := e.G.Zone(state.ZHand, 0)[0]
			battlePerm(t, e, 1, corpusCardText(t, "v/void_winnower.txt"))
			e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 4})
			e.askPriority(0)
			if !hasCastOptionFor(e, spring) {
				t.Fatalf("CR 601.3a: Mind Spring is withheld under Void Winnower although X = 1 gives it an odd mana value")
			}
			cr601Choose(t, e, "cast Mind Spring", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == spring })
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
				t.Fatalf("want the X announcement, got %+v", d)
			}
			cr601Choose(t, e, "announce X", func(o decision.Option) bool { return o.Kind == "x" && o.Amount == x })
			want := state.ZStack
			if x == 0 {
				want = state.ZHand
			}
			if z := e.G.Obj(spring).Zone; z != want {
				t.Fatalf("Mind Spring is in %s after announcing X = %d, want %s", z, x, want)
			}
		})
	}
}

// TestCR601_3aFixedManaValueStaysProhibited guards the unchanged direction:
// a spell with no X (Grizzly Bears, mana value 2) has no choice that changes
// its mana value, so Void Winnower keeps it from being offered.
func TestCR601_3aFixedManaValueStaysProhibited(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "g/grizzly_bears.txt")))
	bears := e.G.Zone(state.ZHand, 0)[0]
	battlePerm(t, e, 1, corpusCardText(t, "v/void_winnower.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "G", Amount: 4})
	if hasCastOption(e.legalActions(0), bears) {
		t.Fatalf("Void Winnower must keep Grizzly Bears (mana value 2) from being cast")
	}
}

// TestCR601_3aFreeCastHasNoXChoice: a Play effect that casts Mind Spring
// without paying its mana cost announces no X (CR 107.3b: X is 0), so no
// choice can change its mana value and Void Winnower still refuses the
// cast; the card stays in hand with the play's restriction Note.
func TestCR601_3aFreeCastHasNoXChoice(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "m/mind_spring.txt")))
	spring := e.G.Zone(state.ZHand, 0)[0]
	battlePerm(t, e, 1, corpusCardText(t, "v/void_winnower.txt"))
	base := len(e.L.Events)
	e.beginPlay(0, spring, true, "", false, false)
	if z := e.G.Obj(spring).Zone; z != state.ZHand {
		t.Fatalf("a free Play cast of Mind Spring (X = 0, mana value 2) under Void Winnower left it in %s, want hand", z)
	}
	noted := false
	for _, ev := range e.L.Events[base:] {
		if ev.Kind == events.Note && ev.Text == "the play cannot cast a restricted card" {
			noted = true
		}
	}
	if !noted {
		t.Fatalf("want the play's restriction Note for a free X cast with no X choice")
	}
}
