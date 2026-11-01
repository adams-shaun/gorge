package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// blindChars is stubChars plus the rules.Engine capability that reports the CR
// 103.1 play/draw choice still pending (rules.Engine.StartingChoicePending),
// and the incremental keying surface (BoardReadKey/CharacteristicsReusable/
// Characteristics) so the test exercises the incremental refill as well as
// the scratch one. The botpolicy test binary runs with boardIncVerify on, so
// every incremental refill is checked against a scratch fill of the same
// state: a blind honoured on only one path fails there.
type blindChars struct {
	stubChars
	log *events.Log
}

func (blindChars) StartingChoicePending() bool { return true }

func (c blindChars) BoardReadKey() (*events.Log, uint64, int, int, bool) {
	return c.log, uint64(len(c.log.Events)), 0, 0, true
}

func (blindChars) CharacteristicsReusable(state.ObjID) bool { return false }

func (c blindChars) Characteristics(id state.ObjID) (int32, int32, []string) {
	return c.Power(id), c.Toughness(id), c.Keywords(id)
}

// TestStartingChoiceBoardHidesHand is the botpolicy half of CR 103.5: while
// the CR 103.1 choice is pending, the deciding seat's own hand cards must not
// reach the Board a bot decision is answered from (the count stays in
// HandSize). The own-library fold is hidden with them: it folds the seat's
// own hand (bots/botpolicy/ownlibrary.go), so an OwnLibrary filled while the
// choice is pending would be the complement of the hidden hand and reveal it
// (hand = decklist − OwnLibrary). It asserts the hand card is absent and
// OwnLibrary zero while a pending-capable Chars is handed in, and present --
// with a nonzero (no-manifest) composition -- for the same game without that
// capability (the no-ask control), through both the scratch fill and the
// incremental refill of one reused Board.
func TestStartingChoiceBoardHidesHand(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	hand := g.AddObject(cardFace(t, "Ambusher", "Creature Rogue", 3, 9), 0)
	grave := g.AddObject(cardFace(t, "Ash", "Sorcery", 0, 0), 0)
	field := g.AddObject(cardFace(t, "Whelp", "Creature Whelp", 2, 2), 0)
	command := g.AddObject(cardFace(t, "General", "Legendary Creature Soldier", 4, 4), 0)
	g.SetZone(state.ZHand, 0, []state.ObjID{hand.ID})
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{grave.ID})
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{field.ID})
	g.SetZone(state.ZCommand, 0, []state.ObjID{command.ID})

	blind := blindChars{log: events.NewLog(1)}
	// Scratch path: a fresh Board's first refill.
	scratch := BoardFromGame(g, blind, 0)
	assertHandRedacted(t, "scratch", scratch, hand.ID, grave.ID, field.ID, command.ID)

	// Incremental path: the second refill of one reused Board.
	var b Board
	BoardFromGameInto(g, blind, 0, &b) // arms the cache (scratch fill)
	inc := BoardFromGameInto(g, blind, 0, &b)
	assertHandRedacted(t, "incremental", inc, hand.ID, grave.ID, field.ID, command.ID)

	// Control: the same game with a Chars that carries no pending capability
	// (the no-ask host) keeps the hand card, so the redaction is the
	// capability's doing, not a blanket drop. The control's OwnLibrary is
	// nonzero (the no-manifest unknown) -- the precondition that proves the
	// fold actually ran and the field is reachable, i.e. the blind's zero is
	// the skip's doing and not a stub that never fills it.
	open := BoardFromGame(g, stubChars{}, 0)
	if _, ok := open.Cards.Lookup(hand.ID); !ok {
		t.Errorf("control: hand card %d absent without the pending capability", hand.ID)
	}
	if open.OwnLibrary.Unknown != deck.UnknownNoManifest {
		t.Errorf("control: OwnLibrary = %+v, want the no-manifest unknown (the fold ran)", open.OwnLibrary)
	}
}

func assertHandRedacted(t *testing.T, path string, b Board, hand state.ObjID, present ...state.ObjID) {
	t.Helper()
	if _, ok := b.Cards.Lookup(hand); ok {
		t.Errorf("%s fill: hand card %d present in Board.Cards while the starting choice is pending", path, hand)
	}
	for _, id := range present {
		if _, ok := b.Cards.Lookup(id); !ok {
			t.Errorf("%s fill: non-hand card %d missing from Board.Cards", path, id)
		}
	}
	if b.HandSize != 1 {
		t.Errorf("%s fill: HandSize = %d, want 1 (the count is public)", path, b.HandSize)
	}
	// The library composition is the hand's complement (hand = decklist −
	// OwnLibrary), so it must be zero while the choice is pending.
	if b.OwnLibrary.Known || b.OwnLibrary.Unknown != 0 || b.OwnLibrary.Size != 0 ||
		b.OwnLibrary.Lands != 0 || b.OwnLibrary.Nonlands != 0 || len(b.OwnLibrary.Counts) != 0 {
		t.Errorf("%s fill: OwnLibrary = %+v, want zero while the starting choice is pending", path, b.OwnLibrary)
	}
}
