package host

// CV-8: the default object cap on hosted tables. The match config arms
// rules.DefaultMaxObjs whenever the per-turn guard is on (the same volume
// bound the shared drive loop arms), the Disabled opt-out is propagated
// unchanged, and a game that crosses the cap mid-resolution ends through the
// ordinary crash path recorded with the stall wording — the table halts
// with a reason an operator reads as a stall, never as a "panic:" bug
// report.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// stormToken is the token the storm ability mints (the stem the TokenScript$
// names must match the path base name the registry keys tokens by).
const stormTokenStem = "t_1_1_goblin_stormtest"

// stormDeck is a 40-card deck of one creature with a free, repeatable
// activated ability that mints 256 Goblins in one resolution: the smallest
// object-arena driver the engine offers, so a live table crosses
// rules.DefaultMaxObjs inside a handful of activations (no corpus needed —
// both card scripts parse inline, the token card is served through
// Options.Tokens under its stem).
func stormDeck(t *testing.T) []*cards.Card {
	t.Helper()
	c := stallCard(t, "Name:Storm-Tokenist\nTypes:Creature Goblin Wizard\nPT:1/1\n"+
		"A:AB$ Token | Cost$ T | TokenAmount$ 512 | TokenScript$ "+stormTokenStem+" | TokenOwner$ You | SpellDescription$ Tap: create 512 1/1 Goblins\nOracle:x")
	out := make([]*cards.Card, 40)
	for i := range out {
		out[i] = c
	}
	return out
}

// rockDeck is 40 lands: nothing to do, nothing to attack with, nobody can
// die — the game's only progress is the storm seat's activations.
func rockDeck(t *testing.T) []*cards.Card {
	t.Helper()
	c := stallCard(t, "Name:Rock\nTypes:Land\nOracle:x")
	out := make([]*cards.Card, 40)
	for i := range out {
		out[i] = c
	}
	return out
}

func stormLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	decks := map[string][]*cards.Card{
		"storm": stormDeck(t),
		"rock":  rockDeck(t),
	}
	return func(name string) (Deck, error) {
		cs, ok := decks[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: cs}, nil
	}
}

// stormTokenCard is the card behind the TokenScript$ stem: a 1/1 Goblin.
func stormTokenCard(t *testing.T) *cards.Card {
	t.Helper()
	c := stallCard(t, "Name:Goblin\nTypes:Creature Goblin\nPT:1/1\nOracle:x")
	c.Path = stormTokenStem + ".txt"
	return c
}

// stormSeat casts the free creature whenever a cast is offered (only a
// battlefield creature offers the ability), then taps for the token ability
// whenever it is offered, and passes otherwise; combat and other asks get
// empty choices, so nothing ever attacks and nobody can lose. Every
// activation resolves (both seats pass) and mints 512 goblins, so the arena
// grows 512 at a time until the armed cap trips mid-resolution.
type stormSeat struct{}

func (stormSeat) Decide(_ context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	switch d.Kind {
	case decision.KAttackers, decision.KBlockers:
		return decision.Intent{Seq: d.Seq, Player: d.Player}, nil
	}
	for _, want := range []string{"cast", "ability"} {
		for i, o := range d.Options {
			if o.Kind == want {
				return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}, nil
			}
		}
	}
	for i, o := range d.Options {
		if o.Kind == "pass" {
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}, nil
		}
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}, nil
}

func stormSeats(names []string, seed uint64) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range out {
		out[i] = stormSeat{}
	}
	return out
}

// TestTheDefaultObjectCapIsArmedOnEveryTableWithThePerTurnGuard pins the
// arming half: a match config built by the host with the per-turn guard on
// carries LoopGuard.MaxObjs == rules.DefaultMaxObjs on the live match.
func TestTheDefaultObjectCapIsArmedOnEveryTableWithThePerTurnGuard(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.LoadDeck = stormLoader(t)
	o.Tokens = map[string]*cards.Card{stormTokenStem: stormTokenCard(t)}
	o.MaxDecisionsPerTurn = 400 // the guard is ON: the arming condition
	o.Seats = stormSeats
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := TableConfig{ID: "t1", Name: "arm", Seats: 2, Decks: []string{"storm", "rock"},
		Seed: 5, Pace: 0, Spectator: view.Omniscient, Mulligans: 0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for liveIntents(t, r, "t1") <= 0 {
		if time.Now().After(deadline) {
			t.Fatal("the match never answered a decision")
		}
		time.Sleep(time.Millisecond)
	}
	tb := tableRef(t, r, "t1")
	tb.mu.RLock()
	m := tb.cur
	m.mu.RLock()
	g := m.rulesConfig().LoopGuard
	m.mu.RUnlock()
	tb.mu.RUnlock()
	r.Close()
	r.Wait("t1")
	if g == nil || g.MaxObjs != rules.DefaultMaxObjs || g.Disabled {
		t.Fatalf("armed guard %+v, want MaxObjs %d, not disabled", g, rules.DefaultMaxObjs)
	}
}

// TestTheObjectCapOptOutIsPropagatedUnchanged pins that the
// supervised-infinite-loop opt-out (MaxDecisionsPerTurn == 0) still propagates
// the Disabled guard with NO object cap: an embedder that took over
// supervision must not have the volume bound resurrected underneath it.
func TestTheObjectCapOptOutIsPropagatedUnchanged(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.LoadDeck = stallLoader(t) // churning on free activations, never a token
	o.MaxDecisionsPerTurn = 0   // explicit opt-out
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	deadline := time.Now().Add(30 * time.Second)
	for liveIntents(t, r, "t1") < 50 {
		if time.Now().After(deadline) {
			t.Fatal("the match never answered 50 decisions")
		}
		time.Sleep(time.Millisecond)
	}
	tb := tableRef(t, r, "t1")
	tb.mu.RLock()
	m := tb.cur
	m.mu.RLock()
	g := m.rulesConfig().LoopGuard
	m.mu.RUnlock()
	tb.mu.RUnlock()
	r.Close()
	r.Wait("t1")
	if g == nil || !g.Disabled || g.MaxObjs != 0 {
		t.Fatalf("opted-out guard %+v, want Disabled, MaxObjs 0", g)
	}
}

// TestAnObjectCapTripEndsAHostedTableAsARecordedStall is the end-to-end
// contract: a live table whose storm seat grows the arena past the default
// cap ends as an ordinary MatchCrashed with the stall wording (never a
// "panic:"), the table halts, and the run ends on its own.
func TestAnObjectCapTripEndsAHostedTableAsARecordedStall(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.LoadDeck = stormLoader(t)
	o.Tokens = map[string]*cards.Card{stormTokenStem: stormTokenCard(t)}
	// The per-turn guard at the production default stays silent — a few
	// hundred storm decisions over a couple of turns never approach 25000 —
	// so the ONLY thing that can end this match is the object cap.
	o.MaxDecisionsPerTurn = DefaultMaxDecisionsPerTurn
	o.Seats = stormSeats
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := TableConfig{ID: "t1", Name: "storm", Seats: 2, Decks: []string{"storm", "rock"},
		Seed: 5, Pace: 0, Spectator: view.Omniscient, Mulligans: 0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	if got := r.Tables()[0].State; got != protocol.TableHalted {
		t.Fatalf("table state %s, want TableHalted", got)
	}
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("matches %+v, want one MatchCrashed", ms)
	}
	reason := tableReason(t, r, "t1")
	for _, want := range []string{"stalled", "object cap", "cap 10000"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("halt reason %q lacks %q", reason, want)
		}
	}
	if strings.Contains(reason, "panic:") {
		t.Fatalf("halt reason %q still reads as a panic bug report", reason)
	}
	// The recorded match reason (the crash report) carries the same wording.
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	fm := tb.history[0]
	tb.mu.RUnlock()
	fm.mu.RLock()
	cr := fm.reason
	fm.mu.RUnlock()
	if !strings.Contains(cr, "object cap") {
		t.Fatalf("match reason %q does not name the object cap", cr)
	}
}
