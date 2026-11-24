package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMissHighwaterEffectArmsLossCopy drives the REAL corpus card Miss
// Highwater's contract Effect on a three-seat board, end to end: the
// contract's DB$ Effect resolves, its comma-listed Triggers$
// "LoseGameTrigger,ExileEffect" arm as delayed registrations, and the
// remembered player's later loss (which does not end a three-seat game)
// fires the armed LosesGame trigger, copying the loser's artifacts and
// creatures for the Effect's controller.
//
// The three readers of an Effect's Triggers$ (effects.effEffect,
// effectNamesBecomeMonarchTrigger and effectSelfExileOnCastSpec) used to
// split the list on whitespace only, so the comma list was one unparseable
// name and a Note was all the Effect emitted -- no copy after the loss.
func TestMissHighwaterEffectArmsLossCopy(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mhwCard := mustCorpusCard(t, reg, "Miss Highwater")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	ornithopter := mustCorpusCard(t, reg, "Ornithopter")
	deck := func(front ...*cards.Card) []*cards.Card {
		return append(append([]*cards.Card(nil), front...), mountainDeck(t, 40-len(front))...)
	}
	cfg := seatZeroStart(Config{Seed: 47, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{deck(mhwCard), deck(bears, bears, ornithopter), deck(bears)}})
	e := New(cfg)

	park := func(owner state.PlayerID, name string, n int) []state.ObjID {
		var out []state.ObjID
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if len(out) == n {
				break
			}
			if o.Owner == owner && o.Zone != state.ZBattlefield && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
				out = append(out, o.ID)
			}
		}
		if len(out) != n {
			t.Fatalf("parked %d of %d %q for seat %d", len(out), n, name, owner)
		}
		return out
	}
	mhw := park(0, "Miss Highwater", 1)[0]
	loserBoard := append(append(park(1, "Grizzly Bears", 2), park(1, "Ornithopter", 1)...), park(1, "Mountain", 1)...)
	park(2, "Grizzly Bears", 1)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	e.pending = nil
	e.pendingTriggers = nil

	tokensOf := func(p state.PlayerID) (names map[string]int, total int) {
		names = map[string]int{}
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o.IsToken && o.Face() != nil {
				names[o.Face().Name]++
				total++
			}
		}
		return names, total
	}
	onBattlefield := 0
	for _, id := range loserBoard {
		if o := e.G.Obj(id); o.Zone == state.ZBattlefield && e.controllerOf(id) == 1 {
			onBattlefield++
		}
	}
	if onBattlefield != len(loserBoard) {
		t.Fatalf("precondition: %d of %d loser permanents on the battlefield, want all", onBattlefield, len(loserBoard))
	}
	if _, n := tokensOf(0); n != 0 {
		t.Fatalf("precondition: seat 0 already has %d tokens", n)
	}

	// The contract's chain as the real trigger resolves it: seat 0's Miss
	// Highwater deals combat damage to seat 1, who has no contract counter.
	// The referents are captured from the real Damage event exactly as a
	// fire captures them (rules' triggerReferents); the discard election the
	// printed card poses is outside this test's reach, so the chain is driven
	// from DBPutCounter (the counter placement the paid branch runs) onward.
	face := e.G.Obj(mhw).Face()
	trig := fx20TriggerWithParam(t, e, mhw, "Player.counters_EQ0_Contract")
	e.combatDamaging = true
	e.dmgSrcOverride = mhw
	dmg := events.Event{Kind: events.Damage, Player: 1, Amount: 2}
	if !trigmatch.DamageMatches(boardOf(e), trig, mhw, dmg) {
		t.Fatal("precondition: combat damage to a player with no contract counter must match")
	}
	refs := e.triggerReferents(trig, mhw, dmg, nil)
	n0 := len(e.L.Events)
	ctx := effects.NewCtxPtr(mhw, 0, effects.CtxInit{TriggerContext: refs, SVars: face.SVars})
	effects.Resolve(e, ctx, cards.ResolveSVar(face.SVars, "DBPutCounter"))
	drainStack(t, e, 40)
	if n := e.G.Players[1].Counter("Contract"); n != 1 {
		t.Fatalf("precondition: seat 1 has %d contract counters after the chain, want 1", n)
	}
	for i := n0; i < len(e.L.Events); i++ {
		if ev := &e.L.Events[i]; ev.Kind == events.Note && strings.HasPrefix(ev.Text, "unparseable Effect trigger") {
			t.Fatalf("the Effect's comma-listed Triggers$ left %q: the list must arm, not Note", ev.Text)
		}
	}
	armed := false
	for i := n0; i < len(e.L.Events); i++ {
		if ev := &e.L.Events[i]; ev.Kind == events.DelayedRegister && strings.HasPrefix(ev.Text, "LosesGame:") {
			armed = true
		}
	}
	if !armed {
		t.Fatal("precondition: the contract Effect registered no LosesGame delayed trigger")
	}

	// Seat 1 loses while two seats still stand: in a three-seat game the loss
	// does not end it, and the armed LosesGame trigger must fire, copying the
	// artifacts and creatures seat 1 controlled when it lost.
	if !e.playerLoses(1, loseReasonEffect, "test loss") {
		t.Fatal("precondition: seat 1's loss did not happen")
	}
	e.Advance() // the loss's state-based sweep, then the armed trigger's fire
	drainStack(t, e, 40)
	names, total := tokensOf(0)
	if total != 3 || names["Grizzly Bears"] != 2 || names["Ornithopter"] != 1 {
		t.Fatalf("seat 0 tokens after the loss = %v (total %d), want 2 Grizzly Bears + 1 Ornithopter", names, total)
	}
	if _, n := tokensOf(1); n != 0 {
		t.Fatalf("seat 1 (the loser) controls %d tokens, want 0: the copies belong to the Effect's controller", n)
	}
	if _, n := tokensOf(2); n != 0 {
		t.Fatalf("seat 2 controls %d tokens, want 0", n)
	}
}
