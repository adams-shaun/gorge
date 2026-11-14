package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// livingWeaponGame seats the named corpus cards in seat 0's library of a
// two-seat game (with the corpus token scripts the Germ needs), advanced to a
// priority.
func livingWeaponGame(t *testing.T, seed uint64, names ...string) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := make([]*cards.Card, 0, 40)
	for _, name := range names {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("missing corpus card %s", name)
		}
		deck = append(deck, c)
	}
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// germsOf returns the Germ tokens seat 0 controls.
func germsOf(e *Engine) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Phyrexian Germ Token" {
			out = append(out, id)
		}
	}
	return out
}

// TestEkthiGrantsLivingWeaponToEnteringEquipment: Ekthi, Contaminator
// Priest's "each Equipment you control has living weapon" (CR 702.92a,
// 613.1f). An Equipment entering while Ekthi is out makes a Germ and attaches
// to it; one already on the battlefield when Ekthi arrives makes none, and
// with no Ekthi the same Equipment makes none.
func TestEkthiGrantsLivingWeaponToEnteringEquipment(t *testing.T) {
	t.Parallel()
	const ekthi, sword = "Ekthi, Contaminator Priest", "Short Sword"
	e, cfg := livingWeaponGame(t, 91, ekthi, sword, sword)
	first := enterFromLibrary(t, e, 0, sword)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if got := len(germsOf(e)); got != 0 {
		t.Fatalf("precondition: %d Germs before Ekthi, want 0", got)
	}
	enterFromLibrary(t, e, 0, ekthi)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if got := len(germsOf(e)); got != 0 {
		t.Fatalf("an Equipment already in play made %d Germs when Ekthi arrived, want 0", got)
	}
	second := enterFromLibrary(t, e, 0, sword)
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)
	germs := germsOf(e)
	if len(germs) != 1 {
		t.Fatalf("an Equipment entered under Ekthi: %d Germs, want 1", len(germs))
	}
	if got := e.G.Obj(second).AttachedTo; got != germs[0] {
		t.Fatalf("the entering Equipment is attached to %d, want the Germ %d", got, germs[0])
	}
	if got := e.G.Obj(first).AttachedTo; got != 0 {
		t.Fatalf("the earlier Equipment is attached to %d, want nothing", got)
	}
	replayCheck(t, e, cfg)
}
