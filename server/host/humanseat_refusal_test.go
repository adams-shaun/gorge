package host

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// mustAttackDeck is 40 copies of a free 2/2 with an unconditional
// `Mode$ MustAttack` static, so the engine's KAttackers decision always has a
// Required option once the creature is untapped: the empty declaration is
// wire-legal but engine-refused (CR 508.1d's required quota).
func mustAttackDeck(t *testing.T) []*cards.Card {
	t.Helper()
	c := stallCard(t, "Name:Charger\nManaCost:0\nTypes:Creature Charger\nPT:2/2\n"+
		"S:Mode$ MustAttack | ValidCreature$ Card.Self | Description$ attacks each combat if able.\n"+
		"Oracle:x\n")
	out := make([]*cards.Card, 40)
	for i := range out {
		out[i] = c
	}
	return out
}

// requiredAttackIntent declares every required attacker against its first
// offered defender. It is the minimal legal declaration for a KAttackers
// decision the engine will otherwise refuse for the required quota.
func requiredAttackIntent(d *decision.Decision) decision.Intent {
	seen := map[state.ObjID]bool{}
	var choices []int
	for _, o := range d.Options {
		if !o.Required || seen[o.Obj] {
			continue
		}
		seen[o.Obj] = true
		choices = append(choices, o.Index)
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
}

// TestRefusedHumanIntentReparksAndReturns409 pins D1b: a HumanSeat answer the
// wire accepts but the engine refuses (an empty declaration with a MustAttack
// creature on the board) must not crash the match. The seat is asked again for
// the same decision, the submit gets the engine's refusal as its error, and a
// legal answer then plays the match to its normal end.
func TestRefusedHumanIntentReparksAndReturns409(t *testing.T) {
	o := testOptions(t)
	o.Seats = twoSeatHumans
	deck := mustAttackDeck(t)
	o.LoadDeck = func(name string) (Deck, error) {
		switch name {
		case "a", "b":
			return Deck{Name: name, Cards: deck}, nil
		}
		return Deck{}, ErrNotFound
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "refusal", Seats: 2, Decks: []string{"a", "b"},
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
				t.Fatalf("match crashed after a refused intent: %s (%d events)", mi.Head, mi.Events)
			}
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchFinished {
				if !refused {
					t.Fatal("match finished without ever exercising a refused intent")
				}
				return
			}
		}
		d, err := r.Pending("t1", 1, 0)
		if err != nil {
			time.Sleep(time.Millisecond)
			continue
		}
		if !refused && d.Kind == decision.KAttackers && hasRequired(d) {
			empty := decision.Intent{Seq: d.Seq, Player: d.Player}
			errc := make(chan error, 1)
			go func() { errc <- r.SubmitIntent("t1", 1, 0, empty) }()
			var err error
			select {
			case err = <-errc:
			case <-time.After(10 * time.Second):
				t.Fatal("refused human intent hung: the submit never received the engine's verdict")
			}
			if err == nil {
				t.Fatalf("empty declaration seq %d was accepted; engine should have refused the required quota", d.Seq)
			} else if !strings.Contains(err.Error(), "must attack with as many required creatures as possible") {
				t.Fatalf("refusal error = %q, want the required-quota reason", err)
			}
			// Refusal preserves the pending decision and re-parks the seat:
			// the same Seq is answerable again, and the match is still alive.
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

func hasRequired(d *decision.Decision) bool {
	for _, o := range d.Options {
		if o.Required {
			return true
		}
	}
	return false
}

// TestTwoDefenderAttackFromHumanSeatIsRefusedAndStaysPending pins the D1
// host contract after D1a: one creature declared against two defenders is
// refused at the seat's wire fence before the match goroutine ever sees it,
// and the same decision stays pending for a legal answer instead of crashing
// the match on Submit.
func TestTwoDefenderAttackFromHumanSeatIsRefusedAndStaysPending(t *testing.T) {
	s := NewHumanSeat()
	d := decision.Decision{Seq: 12, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 38, Player: 2},
			{Index: 1, Kind: "attacker", Obj: 38, Player: 3},
			{Index: 2, Kind: "attacker", Obj: 39, Player: 2},
		}}
	p := s.park(context.Background(), view.View{}, d, nil)
	bad := decision.Intent{Seq: 12, Player: 0, Choices: []int{0, 1}}
	// submitAdmitted waits for the engine's verdict, so if the one-defender
	// wire rule is ever reverted this call would block on a verdict nothing
	// will deliver. Run it with a deadline so the regression is a named
	// failure rather than a package timeout.
	errc := make(chan error, 1)
	go func() { errc <- s.submitAdmitted(bad, nil, false) }()
	var err error
	select {
	case err = <-errc:
	case <-time.After(10 * time.Second):
		t.Fatal("two-defender submit hung: the seat did not reject it at the wire fence")
	}
	if err == nil {
		t.Fatal("one attacker declared against two defenders was accepted by the seat")
	} else if !strings.Contains(err.Error(), "more than one defender") {
		t.Fatalf("seat error = %q, want the multi-defender reason", err)
	}
	if ok, got := s.pending(); !ok || got.Seq != 12 {
		t.Fatalf("after refusal pending = %v seq %d, want the same seq 12", ok, got.Seq)
	}
	if err := s.submit(decision.Intent{Seq: 12, Player: 0, Choices: []int{0, 2}}); err != nil {
		t.Fatalf("legal answer after refusal: %v", err)
	}
	got, err := p.await()
	if err != nil || got.Seq != 12 || len(got.Choices) != 2 {
		t.Fatalf("await = %+v, %v; want the legal seq 12 answer", got, err)
	}
}
