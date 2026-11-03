package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTokenCopyOfDoubleFacedPermanentKeepsItsExceptionsOnBothFaces: Saheeli,
// the Sun's Brilliance copies Khenra Spellspear "except it's an artifact in
// addition to its other types". The token is double-faced (CR 707.8a) and the
// exception is part of each face's copiable values (CR 707.9b), so once the
// token transforms it is Gitaxian Spellstalker, an artifact creature, and
// turned back it is the Spellspear's front face again.
func TestTokenCopyOfDoubleFacedPermanentKeepsItsExceptionsOnBothFaces(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	board := []string{"Saheeli, the Sun's Brilliance", "Khenra Spellspear"}
	var deck []*cards.Card
	for _, n := range board {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	var saheeli, spear state.ObjID
	for i, n := range board {
		id := findAndMoveToHand(t, e, 0, n)
		moveToBattlefield(t, e, id)
		if i == 0 {
			saheeli = id
		} else {
			spear = id
		}
	}
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)

	addMana(t, e, 0, "UR")
	e.priorityRound()
	opt := abilityOption(t, e, saheeli, 0)
	submitChoices(t, e, opt.Index)
	answerKTarget(t, e, spear)
	passUntilStackEmpty(t, e, 20)

	cid := findTokenCopyOf(t, e, e.G.Obj(spear).Card, spear)
	if d := e.Derived(cid); !copyHasTypeWord(d.Types, "Artifact") || !copyHasTypeWord(d.Types, "Warrior") {
		t.Fatalf("front face types = %v, want an artifact Jackal Warrior", d.Types)
	}
	e.emit(events.Event{Kind: events.FlipFace, Obj: cid, Amount: 1, Text: "Transformed"})
	d := e.Derived(cid)
	if !copyHasTypeWord(d.Types, "Phyrexian") || !copyHasTypeWord(d.Types, "Artifact") {
		t.Fatalf("transformed token types = %v, want an artifact Phyrexian Jackal (the exception rides the back face)", d.Types)
	}
	e.emit(events.Event{Kind: events.FlipFace, Obj: cid, Amount: 0, Text: "Transformed"})
	if d := e.Derived(cid); !copyHasTypeWord(d.Types, "Warrior") || !copyHasTypeWord(d.Types, "Artifact") {
		t.Fatalf("token turned back has types %v, want its front face with the exception", d.Types)
	}
	replayCheck(t, e, cfg)
}
