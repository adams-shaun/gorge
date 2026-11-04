package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// firstContactFillerSrc is a free-authored instant standing in for "another
// spell": it draws a card, so it never touches the life total under test.
const firstContactFillerSrc = `Name:Test Filler Instant
ManaCost:R
Types:Instant
A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw a card.
Oracle:x
`

// TestFirstContactGorge pins the corpus First Contact's rider through
// Count$YouCastThisGame: the SpellCast static trigger stores the controller's
// game cast count the moment the spell is cast, and the resolving chain gains
// 2 life only when that stored number is 1. The count is fixed at cast: a
// spell cast in response (resolving first) does not take the gain away, and a
// spell cast earlier does.
func TestFirstContactGorge(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	fc := lookup(t, reg, "First Contact")
	filler := card(t, firstContactFillerSrc)
	twincast := lookup(t, reg, "Twincast")
	setup := func(t *testing.T) (*Engine, Config) {
		return corpusEngineCfg(t, reg, []*cards.Card{fc, fc, filler, filler, twincast}, nil)
	}
	hand := func(t *testing.T, e *Engine, name, mana string) {
		t.Helper()
		moveByName(t, e, 0, name, state.ZHand)
		addMana(t, e, 0, mana)
	}
	t.Run("first spell gains 2", func(t *testing.T) {
		e, cfg := setup(t)
		hand(t, e, "First Contact", "1G")
		before := lifeOf(t, e, 0)
		castNamed(t, e, "First Contact")
		if got := e.SpellsCastThisGame(0); got != 1 {
			t.Fatalf("SpellsCastThisGame after one cast = %d, want 1", got)
		}
		drainFirstContact(t, e)
		if got := lifeOf(t, e, 0) - before; got != 2 {
			t.Fatalf("first spell life change = %d, want +2", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("second spell gains nothing", func(t *testing.T) {
		e, _ := setup(t)
		hand(t, e, "Test Filler Instant", "R")
		castNamed(t, e, "Test Filler Instant")
		drainFirstContact(t, e)
		hand(t, e, "First Contact", "1G")
		before := lifeOf(t, e, 0)
		castNamed(t, e, "First Contact")
		if got := e.SpellsCastThisGame(0); got != 2 {
			t.Fatalf("SpellsCastThisGame after two casts = %d, want 2", got)
		}
		drainFirstContact(t, e)
		if got := lifeOf(t, e, 0) - before; got != 0 {
			t.Fatalf("second spell life change = %d, want 0", got)
		}
	})
	t.Run("a response does not change the stored count", func(t *testing.T) {
		e, cfg := setup(t)
		hand(t, e, "First Contact", "1G")
		moveByName(t, e, 0, "Test Filler Instant", state.ZHand)
		addMana(t, e, 0, "R")
		before := lifeOf(t, e, 0)
		castNamed(t, e, "First Contact")
		// First Contact is on the stack; the response is the second spell and
		// resolves first, so by First Contact's resolution two are cast.
		castNamed(t, e, "Test Filler Instant")
		if got := e.SpellsCastThisGame(0); got != 2 {
			t.Fatalf("SpellsCastThisGame with a response = %d, want 2", got)
		}
		drainFirstContact(t, e)
		if got := lifeOf(t, e, 0) - before; got != 2 {
			t.Fatalf("first-cast spell with a response life change = %d, want +2", got)
		}
		replayCheck(t, e, cfg)
	})
	// A copy of the spell resolves against the stored value its original's cast
	// wrote (Forge copies the card's SVars): a copy of a first-cast First
	// Contact gains too, a copy of a later one does not. Twincast is the second
	// spell in both, so the original is first only when Twincast is cast after
	// it.
	copyCase := func(t *testing.T, earlier bool) int32 {
		e, _ := setup(t)
		if earlier {
			hand(t, e, "Test Filler Instant", "R")
			castNamed(t, e, "Test Filler Instant")
			drainFirstContact(t, e)
		}
		hand(t, e, "First Contact", "1G")
		moveByName(t, e, 0, "Twincast", state.ZHand)
		addMana(t, e, 0, "UU")
		fcID := findInHand(e, 0, "First Contact")
		before := lifeOf(t, e, 0)
		castNamed(t, e, "First Contact")
		castNamed(t, e, "Twincast")
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("Twincast asked no target: %+v", d)
		}
		for _, o := range d.Options {
			if o.Obj == fcID {
				chooseIdx(t, e, o.Index)
			}
		}
		drainFirstContact(t, e)
		copies := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.StackCopy })
		if copies != 1 {
			t.Fatalf("StackCopy events = %d, want 1 (the copy resolved)", copies)
		}
		return lifeOf(t, e, 0) - before
	}
	t.Run("a copy of the first spell gains with it", func(t *testing.T) {
		if got := copyCase(t, false); got != 4 {
			t.Fatalf("original and copy life change = %d, want +4", got)
		}
	})
	t.Run("a copy of a later spell gains nothing", func(t *testing.T) {
		if got := copyCase(t, true); got != 0 {
			t.Fatalf("original and copy life change = %d, want 0", got)
		}
	})
	t.Run("the opponent's casts are not yours", func(t *testing.T) {
		e, _ := setup(t)
		hand(t, e, "First Contact", "1G")
		if got := e.SpellsCastThisGame(1); got != 0 {
			t.Fatalf("seat 1 SpellsCastThisGame = %d, want 0", got)
		}
		castNamed(t, e, "First Contact")
		if got := e.SpellsCastThisGame(1); got != 0 {
			t.Fatalf("seat 1 SpellsCastThisGame after seat 0's cast = %d, want 0", got)
		}
	})
}

// drainFirstContact passes priority until the stack is empty, declining the
// optional "move a card" confirmation so the milled cards stay put.
func drainFirstContact(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 80 && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KChoose {
			idx := d.Options[0].Index
			for _, o := range d.Options {
				if o.Kind == "no" {
					idx = o.Index
				}
			}
			chooseIdx(t, e, idx)
			continue
		}
		submitPass(t, e)
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("stack still holds %d objects", len(e.G.Stack))
	}
}

func chooseIdx(t *testing.T, e *Engine, idx int) {
	t.Helper()
	d := e.Pending()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit choice %d: %v", idx, err)
	}
}
