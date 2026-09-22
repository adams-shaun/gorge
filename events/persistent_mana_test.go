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

	t.Run("a marked spend consumes the persistent share, an unmarked one is ordinary", func(t *testing.T) {
		g := twoSeatGame(t)
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 2})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText("")})
		// The payment path splits a spend that spans both shares: the
		// ordinary part rides an unmarked event, the persistent part a marked
		// one (rules/stack.go's payManaForSpent attribution).
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -2})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 1 || per != 1 {
			t.Fatalf("after the ordinary spend pool=%d persistent=%d, want 1/1", got, per)
		}
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -1,
			Text: ManaPersistentText("")})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 0 || per != 0 {
			t.Fatalf("after the marked spend pool=%d persistent=%d, want 0/0", got, per)
		}
	})

	// The two misattributions the r2 review broke the fresh-rule with: the
	// fold must follow the units the PAYMENT consumed, which the events now
	// name (a marked plain spend, or a restricted spend's batch flags), not
	// raw slot arithmetic — whose fresh share disagrees with the payment's
	// visible pool whenever a restricted batch is hidden from the payment or
	// the carve consumed the persistent batch first.
	t.Run("hidden restricted batch does not make the persistent tally survive the wrong unit", func(t *testing.T) {
		g := twoSeatGame(t)
		// 1 persistent plain R + 1 ordinary Spell-restricted R. A non-Spell
		// payment hides the restricted batch and consumes the persistent unit;
		// the emission carries the attribution (marked), and the boundary
		// clears exactly the restricted unit.
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText("")})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaRestrictionText("Spell", 7)})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 2 || per != 1 {
			t.Fatalf("precondition pool=%d persistent=%d, want 2/1", got, per)
		}
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -1,
			Text: ManaPersistentText("")})
		if got, per := g.Players[0].Pool[state.MR], g.Players[0].PersistentMana[state.MR]; got != 1 || per != 0 {
			t.Fatalf("after the payment pool=%d persistent=%d, want 1/0", got, per)
		}
		Apply(g, Event{Kind: ManaClear, Player: 0})
		if got := g.Players[0].Pool[state.MR]; got != 0 {
			t.Fatalf("the ordinary restricted unit survived the boundary: pool=%d, want 0", got)
		}
	})

	t.Run("a restricted spend follows the consumed batch's own flag", func(t *testing.T) {
		g := twoSeatGame(t)
		// Klauth's shape beside an ordinary restricted batch in the same slot.
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText(ManaRestrictionText("Spell", 7))})
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaRestrictionText("Instant", 7)})
		p := &g.Players[0]
		if got, per, n := p.Pool[state.MR], p.PersistentMana[state.MR], len(p.RestrictedMana); got != 2 || per != 1 || n != 2 {
			t.Fatalf("precondition pool=%d persistent=%d batches=%d, want 2/1/2", got, per, n)
		}
		// Consuming the ORDINARY batch leaves the persistent tally.
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -1,
			Text: ManaRestrictionText("Instant", 7)})
		if got, per, n := p.Pool[state.MR], p.PersistentMana[state.MR], len(p.RestrictedMana); got != 1 || per != 1 || n != 1 {
			t.Fatalf("after the ordinary batch's spend pool=%d persistent=%d batches=%d, want 1/1/1", got, per, n)
		}
		// Consuming the PERSISTENT batch drops the tally with it.
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: -1,
			Text: ManaRestrictionText("Spell", 7)})
		if got, per, n := p.Pool[state.MR], p.PersistentMana[state.MR], len(p.RestrictedMana); got != 0 || per != 0 || n != 0 {
			t.Fatalf("after the persistent batch's spend pool=%d persistent=%d batches=%d, want 0/0/0", got, per, n)
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

	// The phantom-batch defect the r2 review found: ManaClear keeps a
	// persistent batch unconditionally, so a batch surviving the turn with
	// the tally zeroed would outlive its units — manaAvailableFor subtracts
	// its Amount from every non-matching payment and hides the seat's REAL
	// mana behind units that no longer exist. The turn change demotes the
	// batch to ordinary (the printed restriction is not time-bounded; only
	// the don't-lose clause is), so the next boundary empties the units WITH
	// the batch.
	t.Run("turn change demotes the persistent batches too", func(t *testing.T) {
		g := twoSeatGame(t)
		Apply(g, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 1,
			Text: ManaPersistentText(ManaRestrictionText("Spell", 7))})
		Apply(g, Event{Kind: TurnChange, Player: 1, Amount: 2})
		p := &g.Players[0]
		if per := p.PersistentMana[state.MR]; per != 0 {
			t.Fatalf("persistent tally after the turn = %d, want 0", per)
		}
		if len(p.RestrictedMana) != 1 || p.RestrictedMana[0].Persistent {
			t.Fatalf("batch after the turn: %+v, want the persistent batch demoted to ordinary", p.RestrictedMana)
		}
		Apply(g, Event{Kind: ManaClear, Player: 0})
		if got, n := p.Pool[state.MR], len(p.RestrictedMana); got != 0 || n != 0 {
			t.Fatalf("after the next boundary pool=%d batches=%d, want 0/0 (no phantom batch)", got, n)
		}
	})
}
