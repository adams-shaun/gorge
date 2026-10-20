package host

import (
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// threeSeatHuman is an Options.Seats hook: a HumanSeat at slot 0 and a real
// bot at each of the other two seats, so a creature's attackers decision
// offers the same attacker against two different defenders.
func threeSeatHuman(names []string, seed uint64) []seat.Seat {
	return []seat.Seat{NewHumanSeat(), heuristic.NewBot(seed ^ 1), heuristic.NewBot(seed ^ 2)}
}

// twoDefenderChoices returns two option indices that name the same attacker
// Obj against different defenders, or ok=false when this decision offers no
// such pair.
func twoDefenderChoices(d *decision.Decision) (a, b int, ok bool) {
	for i := range d.Options {
		for j := i + 1; j < len(d.Options); j++ {
			if d.Options[i].Obj == d.Options[j].Obj && d.Options[i].Player != d.Options[j].Player {
				return d.Options[i].Index, d.Options[j].Index, true
			}
		}
	}
	return 0, 0, false
}

// TestTwoDefenderAttackFromHumanSeatContinuesToNormalEnd is Done-means (1)
// end to end: in a three-seat table the human is offered one attacker against
// two defenders, the seat refuses the two-defender intent with the engine's
// reason, the same decision stays pending, and the match then plays to its
// normal finish instead of crashing.
func TestTwoDefenderAttackFromHumanSeatContinuesToNormalEnd(t *testing.T) {
	o := testOptions(t)
	o.Seats = threeSeatHuman
	attack := mustAttackDeck(t)
	rock := rockDeck(t)
	o.LoadDeck = func(name string) (Deck, error) {
		// Match 1 rotates decks: seat i plays Decks[(i+1)%3], so the human
		// at seat 0 gets "b"; "a" and "c" belong to the two bot seats.
		switch name {
		case "b":
			return Deck{Name: name, Cards: attack}, nil
		case "a", "c":
			return Deck{Name: name, Cards: rock}, nil
		}
		return Deck{}, ErrNotFound
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "two-defender", Seats: 3, Decks: []string{"a", "b", "c"},
		Seed: 42, Pace: 0, Spectator: view.Omniscient}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(60 * time.Second)
	refused := false
	for {
		if time.Now().After(deadline) {
			t.Fatal("match did not reach its end within deadline")
		}
		ms, merr := r.Matches("t1")
		if merr != nil {
			t.Fatalf("Matches: %v", merr)
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchCrashed {
				t.Fatalf("match crashed after a two-defender intent: %s (%d events)", mi.Head, mi.Events)
			}
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchFinished {
				if !refused {
					t.Fatal("match finished without ever exercising a two-defender intent")
				}
				return
			}
		}
		d, err := r.Pending("t1", 1, 0)
		if err != nil {
			time.Sleep(time.Millisecond)
			continue
		}
		if !refused && d.Kind == decision.KAttackers {
			if a, b, ok := twoDefenderChoices(d); ok {
				bad := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{a, b}}
				errc := make(chan error, 1)
				go func() { errc <- r.SubmitIntent("t1", 1, 0, bad) }()
				var serr error
				select {
				case serr = <-errc:
				case <-time.After(10 * time.Second):
					t.Fatal("two-defender human intent hung: the submit never returned")
				}
				if serr == nil {
					t.Fatalf("two-defender declaration seq %d was accepted; want the one-defender refusal", d.Seq)
				} else if !strings.Contains(serr.Error(), "more than one defender") {
					t.Fatalf("refusal error = %q, want the multi-defender reason", serr)
				}
				again, err := r.Pending("t1", 1, 0)
				if err != nil {
					t.Fatalf("pending after refusal: %v", err)
				}
				if again.Seq != d.Seq || again.Kind != d.Kind {
					t.Fatalf("after refusal pending = seq %d kind %s, want seq %d kind %s", again.Seq, again.Kind, d.Seq, d.Kind)
				}
				refused = true
				d = again
			}
		}
		var in decision.Intent
		if d.Kind == decision.KAttackers {
			in = requiredAttackIntent(d)
		} else {
			in = legalIntent(d)
		}
		if err := r.SubmitIntent("t1", 1, 0, in); err != nil {
			t.Fatalf("SubmitIntent(seq %d kind %s): %v", d.Seq, d.Kind, err)
		}
	}
}
