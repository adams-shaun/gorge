package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// inZone reports whether id is in player p's zone z.
func inZone(g *state.Game, z state.Zone, p state.PlayerID, id state.ObjID) bool {
	for _, v := range g.Zone(z, p) {
		if v == id {
			return true
		}
	}
	return false
}

// discardBoard builds a 2-seat game: seat 0 is the caster (holding the source
// spell), seat 1 is the target, whose hand is set to exactly the given
// objects. Returns an asking host, a Ctx targeting seat 1, and the hand IDs.
// The chooser (caster) is seat 0; the discarder (target) is seat 1.
func discardBoard(t *testing.T, hand ...*cards.Card) (*askHost, *Ctx, []state.ObjID) {
	t.Helper()
	ah := &askHost{}
	ah.g = state.NewGame(names(2))
	src := ah.g.AddObject(mkCard(t, "Name:Thoughtseize\nTypes:Sorcery\nOracle:x\n"), 0)
	ids := make([]state.ObjID, 0, len(hand))
	for _, c := range hand {
		o := ah.g.AddObject(c, 1)
		o.Zone = state.ZHand
		ids = append(ids, o.ID)
	}
	ah.g.SetZone(state.ZHand, 1, ids)
	ctx := &Ctx{Source: src.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	return ah, ctx, ids
}

func creature(t *testing.T, name string) *cards.Card {
	return mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:1/1\nOracle:x\n")
}

func land(t *testing.T, name string) *cards.Card {
	return mkCard(t, "Name:"+name+"\nTypes:Land\nOracle:x\n")
}

// TestDiscardRevealYouChooseFallsBackToFrontCardWhenHostCannotAsk is R-9's
// no-engine contract: a host whose Ask returns false must still resolve
// deterministically — the old front-of-hand stand-in — rather than dropping
// the answer and discarding nothing. The Note records why the richer path did
// not run.
func TestDiscardRevealYouChooseFallsBackToFrontCardWhenHostCannotAsk(t *testing.T) {
	h, c, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"))
	// Plain fakeHost: Ask returns false.
	plain := &fakeHost{g: h.g, log: h.log}
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ RevealYouChoose | DiscardValid$ Card | NumCards$ 1")

	effDiscard(plain, c, s)

	if !inZone(plain.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("no-ask host did not discard the front card stand-in")
	}
	if !inZone(plain.g, state.ZHand, 1, ids[1]) {
		t.Fatal("no-ask host discarded a non-front card")
	}
	foundNote := false
	for _, ev := range plain.log {
		if ev.Kind == events.Note && ev.Text == "discards its first card (no engine host to ask)" {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatal("no-ask fallback did not record the stand-in Note")
	}
}

// TestDiscardRevealYouChooseFilterNeverOffersALand pins DiscardValid$
// Card.nonLand: the option list the caster is offered contains exactly the
// non-land cards — a land sitting in the target's hand is never a choice.
func TestDiscardRevealYouChooseFilterNeverOffersALand(t *testing.T) {
	ah, _, ids := discardBoard(t, creature(t, "Frog"), land(t, "Islet"), creature(t, "Bird"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ RevealYouChoose | DiscardValid$ Card.nonLand | NumCards$ 1")

	effDiscard(ah, &Ctx{Source: 1, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, s)

	if ah.asked == nil {
		t.Fatal("RevealYouChoose posed no decision")
	}
	if len(ah.asked.Options) != 2 {
		t.Fatalf("options = %d, want the 2 non-land cards only", len(ah.asked.Options))
	}
	for _, o := range ah.asked.Options {
		if o.Obj == ids[1] {
			t.Fatal("a land was offered as a discard choice")
		}
	}
}

// TestDiscardRevealDiscardAllAsksNothingAndDiscardsEveryMatch pins the Cabal
// Therapy shape: Mode$ RevealDiscardAll is a FILTER, not a choice — Ask is
// never called, and every matching (non-land) card in the target's hand is
// discarded, whether or not it is the front card. NumCards$ is irrelevant.
func TestDiscardRevealDiscardAllAsksNothingAndDiscardsEveryMatch(t *testing.T) {
	ah, _, ids := discardBoard(t, creature(t, "Frog"), land(t, "Islet"), creature(t, "Bird"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ RevealDiscardAll | DiscardValid$ Card.nonLand | NumCards$ 1")

	effDiscard(ah, &Ctx{Source: 1, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, s)

	if ah.asked != nil {
		t.Fatal("RevealDiscardAll asked a decision")
	}
	if !inZone(ah.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("frog (matching) was not discarded")
	}
	if !inZone(ah.g, state.ZGraveyard, 1, ids[2]) {
		t.Fatal("bird (matching) was not discarded")
	}
	if inZone(ah.g, state.ZGraveyard, 1, ids[1]) {
		t.Fatal("a land was discarded despite DiscardValid$ Card.nonLand")
	}
	if !inZone(ah.g, state.ZHand, 1, ids[1]) {
		t.Fatal("the land should still be in hand")
	}
}

// TestDiscardNoModeStaysFrontOfHand guards the deterministic path: a Discard
// with no Mode$ (the cleanup-step and Delve-style cost shape) still takes the
// front card NumCards times and never asks — the change must not have turned
// every discard into a question.
func TestDiscardNoModeStaysFrontOfHand(t *testing.T) {
	ah, _, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | NumCards$ 1")

	effDiscard(ah, &Ctx{Source: 1, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, s)

	if ah.asked != nil {
		t.Fatal("a no-Mode discard asked a decision")
	}
	if !inZone(ah.g, state.ZGraveyard, 1, ids[0]) {
		t.Fatal("front card was not discarded")
	}
	if !inZone(ah.g, state.ZHand, 1, ids[1]) {
		t.Fatal("a non-front card was discarded")
	}
}
