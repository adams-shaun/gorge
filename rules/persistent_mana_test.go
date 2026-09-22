// PersistentMana$ True — "you don't lose this mana as steps and phases end"
// (CR 500.4 with the producing card's exception, bounded by the cards'
// "until end of turn"). The brief's census carrier is Rousing Refrain
// (`SP$ Mana | Produced$ R | Amount$ Z | PersistentMana$ True | Defined$ You`,
// Z = the target opponent's hand size); the corpus population is 23 files /
// 24 raw lines, every occurrence the literal True. The pool tally is
// Player.PersistentMana (state/game.go), raised by the ManaAdd event's
// " pm" Text suffix, consumed ordinary-first by the fresh-rule in
// events.Apply, kept by ManaClear's partial clear, and expired by the
// TurnChange fold.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestRousingRefrainManaSurvivesTheEndStep is the end-to-end pin on the real
// corpus card: the produced R is persistent (survives the end step and every
// later step boundary of the turn), is spendable like any mana, and empties
// normally once the turn ends.
func TestRousingRefrainManaSurvivesTheEndStep(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Rousing Refrain"))
	// The spell's Z is the target opponent's hand size: give seat 1 exactly
	// 3 cards and assert the precondition the count rides on.
	for i := 0; i < 3; i++ {
		o := e.G.AddObject(card(t, "Name:Filler "+string(rune('A'+i))+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), o.ID))
	}
	if n := len(e.G.Zone(state.ZHand, 1)); n != 3 {
		t.Fatalf("test precondition: opponent hand = %d cards, want 3", n)
	}
	rr := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 5 // 3RR paid entirely from R

	castMode(t, e, rr, "")
	d := e.Pending()
	if d == nil || d.Kind != "target" {
		t.Fatalf("Rousing Refrain did not ask for its target: %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no opponent target offered: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	if o := e.G.Obj(rr); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: spell did not reach the stack paid: %+v", o)
	}
	e.resolveTop()
	e.pending = nil

	// Z = 3 cards in the opponent's hand -> 3 persistent R. The pool is the
	// 3 R and every one of them is persistent.
	if got := e.G.Players[0].Pool[state.MR]; got != 3 {
		t.Fatalf("after resolve pool R = %d, want 3 (Z = the opponent's 3-card hand)", got)
	}
	if got := e.G.Players[0].PersistentMana[state.MR]; got != 3 {
		t.Fatalf("persistent tally R = %d, want 3", got)
	}

	// The mana survives the end step (CR 500.4's ordinary emptying does not
	// touch it): Main1 -> End emits the boundary ManaClear, and the pool is
	// still the 3 R.
	e.setStep(state.StepEnd)
	if got := e.G.Players[0].Pool[state.MR]; got != 3 {
		t.Fatalf("pool R after the end-step boundary = %d, want 3 (persistent mana survives)", got)
	}
	if got := e.G.Players[0].PersistentMana[state.MR]; got != 3 {
		t.Fatalf("persistent tally after the boundary = %d, want 3", got)
	}

	// It spends like any mana: a vanilla {R} spell consumes one unit (the
	// slot holds only persistent units, so the spend eats into the
	// persistent tally) and the rest still survives the next boundary.
	vanilla := e.G.AddObject(card(t, "Name:Red Pump\nTypes:Sorcery\nManaCost:R\nOracle:x\n"), 0)
	vanilla.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), vanilla.ID))
	castMode(t, e, vanilla.ID, "")
	if o := e.G.Obj(vanilla.ID); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: vanilla spell did not reach the stack paid: %+v", o)
	}
	e.resolveTop()
	e.pending = nil
	if got := e.G.Players[0].Pool[state.MR]; got != 2 || e.G.Players[0].PersistentMana[state.MR] != 2 {
		t.Fatalf("after spending one unit pool=%d persistent=%d, want 2/2", e.G.Players[0].Pool[state.MR], e.G.Players[0].PersistentMana[state.MR])
	}
	e.setStep(state.StepMain2)
	if got := e.G.Players[0].Pool[state.MR]; got != 2 {
		t.Fatalf("pool R after the main2 boundary = %d, want 2", got)
	}

	// "Until end of turn": the TurnChange fold expires the tally, the units
	// become ordinary again, and the next turn's first boundary ManaClear
	// empties them.
	e.beginTurn(1)
	if got := e.G.Players[0].Pool[state.MR]; got != 0 {
		t.Fatalf("pool R after the turn ended = %d, want 0 (emptied normally after the turn)", got)
	}
	if got := e.G.Players[0].PersistentMana[state.MR]; got != 0 {
		t.Fatalf("persistent tally after the turn ended = %d, want 0", got)
	}
}

// TestPersistentManaOrdinarySpentFirst pins the spend ordering through a real
// payment: within a slot the ordinary units are consumed before the
// persistent ones, so spending one of {2 ordinary R, 1 persistent R} leaves
// the persistent unit to survive the boundary and takes an ordinary one.
func TestPersistentManaOrdinarySpentFirst(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	// 2 ordinary R, then 1 persistent R (the encoding the effect path emits).
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1,
		Text: events.ManaPersistentText("")})
	if got, per := e.G.Players[0].Pool[state.MR], e.G.Players[0].PersistentMana[state.MR]; got != 3 || per != 1 {
		t.Fatalf("test precondition: pool=%d persistent=%d, want 3/1", got, per)
	}
	vanilla := e.G.AddObject(card(t, "Name:Red Pump\nTypes:Sorcery\nManaCost:R\nOracle:x\n"), 0)
	vanilla.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), vanilla.ID))
	castMode(t, e, vanilla.ID, "")
	if o := e.G.Obj(vanilla.ID); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: spell did not reach the stack paid: %+v", o)
	}
	e.resolveTop()
	e.pending = nil
	if got, per := e.G.Players[0].Pool[state.MR], e.G.Players[0].PersistentMana[state.MR]; got != 2 || per != 1 {
		t.Fatalf("after the spend pool=%d persistent=%d, want 2/1 (the ordinary unit was spent first)", got, per)
	}
	e.setStep(state.StepEnd)
	if got, per := e.G.Players[0].Pool[state.MR], e.G.Players[0].PersistentMana[state.MR]; got != 1 || per != 1 {
		t.Fatalf("after the boundary pool=%d persistent=%d, want 1/1 (the persistent unit survives)", got, per)
	}
}

// TestPersistentManaReplaysIdentically proves the tally is derived, not
// engine scratch: a clone driven through the same emits and boundaries holds
// exactly the same pool and persistence state.
func TestPersistentManaReplaysIdentically(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	clone := e.Clone()
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1,
		Text: events.ManaPersistentText("")})
	clone.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	clone.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1,
		Text: events.ManaPersistentText("")})
	e.setStep(state.StepEnd)
	clone.setStep(state.StepEnd)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: -1})
	clone.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: -1})
	e.beginTurn(1)
	clone.beginTurn(1)
	if diff := diffGames(e.G, clone.G); diff != "" {
		t.Fatalf("persistent-mana replay diverged:\n%s", diff)
	}
	if got, per := e.G.Players[0].Pool[state.MR], e.G.Players[0].PersistentMana[state.MR]; got != 0 || per != 0 {
		t.Fatalf("pool=%d persistent=%d after the spend and the turn, want 0/0", got, per)
	}
}
