package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Room cast as one half and later unlocked as the other resolves the other
// half's "when you unlock this door" trigger against that half's own script
// table (CR 709.5, 113.7a): its X counts the cards in hand, where the cast
// half has no such variable.
func TestRoomUnlockOtherHalfTriggerReadsItsOwnVariables(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	room := card(t, "Name:Front Door\nManaCost:0\nTypes:Enchantment Room\nOracle:x\n"+
		"ALTERNATE\nName:Back Door\nManaCost:2 U\nTypes:Enchantment Room\n"+
		"T:Mode$ UnlockDoor | ValidPlayer$ You | ValidCard$ Card.Self | ThisDoor$ True | Execute$ TrigGain | TriggerDescription$ x\n"+
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ HandSize\n"+
		"SVar:HandSize:Count$CardsInYourHand\n"+
		"Oracle:y\nAlternateMode:Split\n")
	room.Faces[0].ManaCost = "W"
	filler := card(t, "Name:Filler Card\nManaCost:0\nTypes:Sorcery\nOracle:z\n")
	e := corpusEngine(t, reg, []*cards.Card{room, filler, filler, filler}, nil)
	id := moveByName(t, e, 0, "Front Door", state.ZBattlefield)
	designateRoomCastFace(e, id)
	o := e.G.Obj(id)
	if o == nil || o.FaceIdx != 0 {
		t.Fatalf("precondition: Room is not live as its front half: %+v", o)
	}
	for i := 0; i < 3; i++ {
		moveByName(t, e, 0, "Filler Card", state.ZHand)
	}
	hand := int32(len(e.G.Zone(state.ZHand, 0)))
	if hand == 0 {
		t.Fatal("precondition: seat 0 holds no cards")
	}
	before := e.G.Players[0].Life
	e.emit(events.Event{Kind: events.DoorUnlock, Obj: id})
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Fatalf("unlock trigger not on the stack: %d items", len(e.G.Stack))
	}
	resolveStack(t, e)
	if got := e.G.Players[0].Life - before; got != hand {
		t.Fatalf("unlocked back door gained %d life, want the %d cards in hand", got, hand)
	}
}
