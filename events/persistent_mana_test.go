// PersistentMana$ True — the ManaAdd " pm" Text suffix's fold (task
// persistentmana): the pool tally, the ordinary-first spend rule, ManaClear's
// partial clear and the TurnChange expiry. The rules-side end-to-end pins on
// the real corpus carrier (Rousing Refrain) live in rules/persistent_mana_test.go;
// these cover the encoding corners the spell-level flow does not reach: the
// composition with a RestrictValid$ batch (Klauth's
// `PersistentMana$ True | RestrictValid$ Spell`), the tagged (snow/typed)
// tally drain, and the batch survival split.
package events

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func twoSeatGame(t *testing.T) *state.Game {
	t.Helper()
	return state.NewGame([]string{"Ann", "Bob"})
}

// TestPersistentManaFold pins the fold corners the rules-level spell flow
// cannot reach.
func TestPersistentManaFold(t *testing.T) {
	t.Run("add raises the tally, clear keeps the share", func(t *testing.T) {
		g := twoSeatGame(t)
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 2})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText("")})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 3 || per != 1 {
			t.Fatalf("pool=%d persistent=%d, want 3/1", got, per)
		}
		Apply(g, Event{Kind: ManaClear, Player: 0})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 1 || per != 1 {
			t.Fatalf("after clear pool=%d persistent=%d, want 1/1", got, per)
		}
	})

	t.Run("spend consumes ordinary units before persistent ones", func(t *testing.T) {
		g := twoSeatGame(t)
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 2})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText("")})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -2})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 1 || per != 1 {
			t.Fatalf("after a 2-unit spend pool=%d persistent=%d, want 1/1", got, per)
		}
		// Spending past the ordinary share eats the persistent tally.
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -1})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 0 || per != 0 {
			t.Fatalf("after the last spend pool=%d persistent=%d, want 0/0", got, per)
		}
	})

	t.Run("snow tally drains with the cleared share", func(t *testing.T) {
		g := twoSeatGame(t)
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "SG", Amount: 2})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "G", Amount: 1,
			Text: ManaPersistentText("")})
		if got, snow := g.Players[0].Pool[state.MG], g.Players[0].Snow[state.MG]; got != 3 || snow != 2 {
			t.Fatalf("precondition pool=%d snow=%d, want 3/2", got, snow)
		}
		Apply(g, Event{Kind: ManaClear, Player: 0})
		p := &g.Players[0]
		if got, per, snow := p.Pool[state.MG], p.PersistentMana[state.MG], p.Snow[state.MG]; got != 1 || per != 1 || snow != 0 {
			t.Fatalf("after clear pool=%d persistent=%d snow=%d, want 1/1/0 (the cleared share drained the snow tally)", got, per, snow)
		}
	})

	t.Run("persistent restriction batch survives, ordinary does not", func(t *testing.T) {
		g := twoSeatGame(t)
		// Klauth's shape: PersistentMana$ True riding a RestrictValid$ Spell
		// batch, beside an ordinary restricted batch in another slot.
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText(ManaRestrictionText("Spell", 7))})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "U", Amount: 1,
			Text: ManaRestrictionText("Spell", 7)})
		if len(g.Players[0].RestrictedMana) != 2 {
			t.Fatalf("precondition: %d batches, want 2", len(g.Players[0].RestrictedMana))
		}
		Apply(g, Event{Kind: ManaClear, Player: 0})
		p := &g.Players[0]
		if got, per := p.Pool[state.MR], p.PersistentMana[state.MR]; got != 1 || per != 1 {
			t.Fatalf("persistent R after clear pool=%d persistent=%d, want 1/1", got, per)
		}
		if got := p.Pool[state.MU]; got != 0 {
			t.Fatalf("ordinary U after clear pool=%d, want 0", got)
		}
		if len(p.RestrictedMana) != 1 || p.RestrictedMana[0].Color != "R" || !p.RestrictedMana[0].Persistent {
			t.Fatalf("batches after clear: %+v, want only the persistent R batch", p.RestrictedMana)
		}
	})

	t.Run("turn change expires the tally", func(t *testing.T) {
		g := twoSeatGame(t)
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText("")})
		Apply(g, Event{Kind: TurnChange, Player: 1, Amount: 2})
		if per := g.Players[0].PersistentMana[state.MR]; per != 0 {
			t.Fatalf("persistent tally after the turn = %d, want 0", per)
		}
		// The units became ordinary: the next boundary empties them.
		Apply(g, Event{Kind: ManaClear, Player: 0})
		if got := g.Players[0].Pool[state.MR]; got != 0 {
			t.Fatalf("pool after the next boundary = %d, want 0", got)
		}
	})
}
