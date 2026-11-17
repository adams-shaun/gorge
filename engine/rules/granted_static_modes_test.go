package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedStaticModesReachReaders pins a granted static reaching its
// mode's reader: Urban Burgeoning grants the enchanted land "untap this land
// during each other player's untap step" (AddStaticAbility$ naming a
// Mode$ UntapOtherPlayer body), so a tapped enchanted land of seat 0 untaps
// in seat 1's untap step (CR 502.3).
func TestGrantedStaticModesReachReaders(t *testing.T) {
	reg := searchTestRegistry(t)
	deck := append([]*cards.Card{searchCorpusCard(t, reg, "Urban Burgeoning")}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 9451, Names: []string{"burgeon", "opp"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	land := searchMoveByName(t, e, "Mountain", state.ZBattlefield)
	aura := searchMoveByName(t, e, "Urban Burgeoning", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{land}})
	driveToStep(t, e, 1, 0, state.StepEnd)
	e.emit(events.Event{Kind: events.Tap, Obj: land})
	if !e.G.Obj(land).Tapped {
		t.Fatal("precondition: the enchanted land is not tapped")
	}
	driveToStep(t, e, 2, 1, state.StepDraw)
	if e.G.Obj(land).Tapped {
		t.Fatal("Urban Burgeoning's enchanted land did not untap in the other player's untap step")
	}
	replayCheck(t, e, cfg)
}
