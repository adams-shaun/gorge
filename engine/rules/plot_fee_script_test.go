package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// plotFeeScriptBoard builds the pair-1397 divergence's board: seat 0 has a
// Terror of the Peaks, seat 1 an untapped Forest and Conduit Pylons and a
// Demonic Ruckus in hand, nothing floated, at seat 1's first main phase with
// priority pending. Both boards and the hand card arrive through logged
// MoveZone events (the findAndMoveToHand pattern); Conduit Pylons' surveil-1
// enter trigger is answered like feeFilterBoard answers the Devotee's scry.
func plotFeeScriptBoard(t *testing.T) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck0, deck1 []*cards.Card
	for _, n := range []string{"Terror of the Peaks"} {
		deck0 = append(deck0, lookup(t, reg, n))
	}
	for _, n := range []string{"Forest", "Conduit Pylons", "Demonic Ruckus"} {
		deck1 = append(deck1, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck0, mountainDeck(t, 40-len(deck0))...), append(deck1, mountainDeck(t, 40-len(deck1))...)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range []string{"Terror of the Peaks"} {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	for _, n := range []string{"Forest", "Conduit Pylons"} {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 1, n))
	}
	findAndMoveToHand(t, e, 1, "Demonic Ruckus")
	e.pending = nil
	e.Advance()
	// Drive to seat 1's first main of turn 2, answering Conduit Pylons'
	// surveil-1 enter ask (it resolves after the first priority pass, like
	// feeFilterBoard's scry) and every cleanup discard on the way.
	for i := 0; i < 4000; i++ {
		if e.G.Turn == 2 && e.G.Active == 1 && e.G.Step == state.StepMain1 {
			break
		}
		if e.G.Over {
			t.Fatalf("game ended before turn 2 seat 1 main1")
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending while driving to turn 2 seat 1 main1")
		}
		switch d.Kind {
		case decision.KArrange:
			ch := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				ch = append(ch, o.Index)
			}
			if d.Max >= 0 && int(d.Max) < len(ch) {
				ch = ch[:d.Max]
			}
			submitChoices(t, e, ch...)
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected %s decision while driving to turn 2 seat 1 main1: %+v", d.Kind, d)
		}
	}
	if e.G.Turn != 2 || e.G.Active != 1 || e.G.Step != state.StepMain1 {
		t.Fatalf("did not reach turn 2 seat 1 main1 (at turn %d seat %d step %s)", e.G.Turn, e.G.Active, e.G.Step)
	}
	return e
}

// TestPlotFeeScriptSurvivesTargetPricedSkip pins the CR 701.34a exemption in
// potentialTargetPriced: plotting poses no target ask -- the cast from exile
// announces the enchant target later, at its own price -- so a ValidTarget$
// cost static (Terror of the Peaks' RaiseCost) cannot price the plot action,
// and the plot stays script-searchable. The pool is empty, so the plot's {R}
// is payable only by Conduit Pylons' "{1}, {T}: Add one mana of any color"
// (CR 601.2g funds the {1} from the Forest): the planner census prices the
// Pylons' paid ability as a gap and only the prefix script search can reach
// it -- the search the target-priced skip used to withhold, dropping the
// plot from the priority vocabulary.
func TestPlotFeeScriptSurvivesTargetPricedSkip(t *testing.T) {
	e := plotFeeScriptBoard(t)

	// Preconditions: the Ruckus is in seat 1's hand, Terror of the Peaks is
	// on seat 0's board with a RaiseCost static that prices a cast of the
	// Ruckus (so the old, mode-blind skip fired), both mana sources are
	// untapped, and nothing is floated -- the plot's {R} is payable only
	// through a gap-funded activation.
	var ruckus state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 1) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Demonic Ruckus" {
			ruckus = id
		}
	}
	if ruckus == 0 {
		t.Fatal("precondition failed: no Demonic Ruckus in seat 1's hand")
	}
	findBattlefieldCard(t, e, "Terror of the Peaks")
	if !e.paymentPlanTargetDependentFor(e.collectCostStatics(), 1, ruckus, spellScope("")) {
		t.Fatal("precondition failed: Terror of the Peaks' RaiseCost does not price a cast of Demonic Ruckus; the old skip would not have fired")
	}
	for _, n := range []string{"Forest", "Conduit Pylons"} {
		id := findBattlefieldCardByName(t, e, 1, n)
		if o := e.G.Obj(id); o.Tapped {
			t.Fatalf("precondition failed: %s is tapped", n)
		}
	}
	if e.G.Players[1].Pool != (state.Mana{}) {
		t.Fatalf("precondition failed: seat 1's pool is %v, want empty", e.G.Players[1].Pool)
	}

	plans := e.PotentialPaymentPlans(1)
	var plot *PotentialPlan
	for i := range plans {
		if pp := &plans[i]; pp.Action.Kind == "cast" && pp.Action.Mode == "plot" && pp.Action.Obj == ruckus {
			plot = pp
			break
		}
	}
	if plot == nil {
		t.Fatal("the payment planner priced no plot potential action for Demonic Ruckus")
	}
	if plot.Reason != "" {
		t.Fatalf("the plot's planner verdict is %q (%s), want payable -- a legal plot (CR 701.34a) must not be proven unpayable", plot.Reason, plot.Detail)
	}
	if len(plot.Script) == 0 {
		t.Fatal("the plot has no prefix script: the fee-funded Pylons activation the census prices as a gap was not searched")
	}
}

// findBattlefieldCardByName is findBattlefieldCard for either seat.
func findBattlefieldCardByName(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s is not on seat %d's battlefield", name, p)
	return 0
}
