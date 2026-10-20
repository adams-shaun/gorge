package mapping_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

func TestAutoPayCastCommitsTheOfferedNativeWitness(t *testing.T) {
	g := untilPending(t, "Burn", 1, func(d *decision.Decision, _ *rules.Engine) bool {
		for _, a := range d.PaymentActions {
			if len(a.Plans) > 0 && len(a.Plans[0].Activations) > 0 {
				return true
			}
		}
		return false
	})
	env := envFor(t, g)
	env.AutoPay = true
	d := g.E.Pending()
	tx, p := mustPose(t, env, d)
	for i, c := range p.Candidates {
		if c.Op.Op != "payment" {
			continue
		}
		if c.Sem.Kind != "cast_spell" || c.Sem.Fields["method"] != "normal" {
			t.Fatal("payment is not a normal cast")
		}
		commit, done, err := tx.Answer(i)
		if err != nil || !done || len(commit) != 1 || !reflect.DeepEqual(commit[0].Payment, c.Op.Payment) {
			t.Fatalf("payment changed: %v %v %v", commit, done, err)
		}
		if _, err := g.Probe(commit...); err != nil {
			t.Fatalf("offered cast rejected: %v", err)
		}
		return
	}
	t.Fatal("no atomic cast candidate")
}

func TestPoolOnlyPaymentAndManualCastHaveTheSameEffect(t *testing.T) {
	var action decision.PaymentAction
	g := untilPending(t, "Burn", 1, func(d *decision.Decision, _ *rules.Engine) bool {
		for _, a := range d.PaymentActions {
			if a.BaseOptionIndex != nil && len(a.Plans) > 0 && len(a.Plans[0].Activations) == 0 {
				action = a
				return true
			}
		}
		return false
	})
	d := g.E.Pending()
	paid, err := g.Probe(decision.Intent{Seq: d.Seq, Player: d.Player, Payment: &decision.PaymentSelection{ActionID: action.ID, Plan: action.Plans[0]}})
	if err != nil {
		t.Fatal(err)
	}
	manual, err := g.Probe(mapping.Intent(d, *action.BaseOptionIndex))
	if err != nil {
		t.Fatal(err)
	}
	bots := [2]seat.Seat{heuristic.NewBot(11), heuristic.NewBot(12)}
	for n := 0; n < 16; n++ {
		if !reflect.DeepEqual(view.Project(paid.G, paid, d.Player, nil), view.Project(manual.G, manual, d.Player, nil)) {
			t.Fatal("pool-only payment changed the player's view")
		}
		next, other := paid.Pending(), manual.Pending()
		if next == nil || next.Kind == decision.KPriority {
			if (next == nil) != (other == nil) {
				t.Fatal("cast completion differs")
			}
			return
		}
		if other == nil || next.Kind != other.Kind {
			t.Fatal("cast follow-ups differ")
		}
		in, err := bots[next.Player].Decide(context.Background(), view.Project(paid.G, paid, next.Player, next), *next)
		if err != nil {
			t.Fatal(err)
		}
		if err := paid.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
		in.Seq, in.Player = other.Seq, other.Player
		if err := manual.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("cast did not complete within its bounded follow-ups")
}

func TestAutoPayHidesPriorityManaActivations(t *testing.T) {
	g := untilPending(t, "Burn", 1, func(d *decision.Decision, _ *rules.Engine) bool {
		if d.Kind != decision.KPriority {
			return false
		}
		for _, o := range d.Options {
			if o.Kind == "activate" {
				return true
			}
		}
		return false
	})
	d := g.E.Pending()
	env := envFor(t, g)
	env.AutoPay = true
	_, p := mustPose(t, env, d)
	for _, c := range p.Candidates {
		if c.Sem.Kind == "activate_mana_ability" {
			t.Fatal("auto-pay offered a mana tap")
		}
	}
	env.AutoPay = false
	_, manual := mustPose(t, env, d)
	found := false
	for _, c := range manual.Candidates {
		found = found || c.Sem.Kind == "activate_mana_ability"
	}
	if !found {
		t.Fatal("manual mode lost its tap candidate")
	}
}
