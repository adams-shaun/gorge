package rules

// CR 602.2b with CR 605.3a and CR 106.12: Absorb Vis's {1}{B} basic
// landcycling is payable from Three Tree City's {2},{T} "add X mana of any
// one color" arm (X is the creatures of the chosen type: one Changeling
// Mistwalker) when the last untapped Island, doubled by the opponent's Mana
// Flare, pays the arm's {2}: the arm's pick plus Flare's reflected extra is
// {B}{B}. The seat's potential-play search prices the cycle on clones that
// pause on the arm's colour ask; the clone once dropped the activation's
// mana-tap mark (Engine.manaTapMark), so Flare's triggered mana ability
// found no mana to reflect, the arm made one {B} and the search offered
// nothing (the dr-mbc-abbb lockstep divergence, decision 171, where the
// kernel offers the cycle).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mirrorBoard2 is mirrorBoard with both seats' hands set: hand0 and hand1 are
// the named corpus cards in that order.
func mirrorBoard2(t *testing.T, bf0, bf1, hand0, hand1 []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range append(append(append(append([]string{}, bf0...), bf1...), hand0...), hand1...) {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.SetZone(state.ZHand, 1, nil)
	setHand := func(p state.PlayerID, names []string) {
		var hand []state.ObjID
		for _, n := range names {
			o := e.G.AddObject(lookup(t, reg, n), p)
			o.Zone = state.ZHand
			hand = append(hand, o.ID)
		}
		e.G.SetZone(state.ZHand, p, hand)
	}
	setHand(0, hand0)
	setHand(1, hand1)
	place := func(n string, p state.PlayerID) {
		o := e.G.AddObject(lookup(t, reg, n), p)
		o.Zone = state.ZBattlefield
		o.SummonSick = false
		e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
	}
	for _, n := range bf0 {
		place(n, 0)
	}
	for _, n := range bf1 {
		place(n, 1)
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// The decision-171 board: TTC chosen Aenar with one Changeling Mistwalker
// counting, three Islands of which two paid the Mistwalker pumps, Wizard
// Class, the opponent's Mana Flare, an empty pool.
func TestAbsorbVisCycleDrivenPumpsOffered(t *testing.T) {
	t.Parallel()
	e := mirrorBoard2(t,
		[]string{"Three Tree City", "Mistwalker", "Island", "Island", "Island", "Wizard Class"},
		[]string{"Mana Flare"},
		[]string{"Absorb Vis"},
		nil)
	ttcO := e.G.Zone(state.ZBattlefield, 0)[0]
	e.G.Obj(ttcO).ChosenType = "Aenar"
	e.Advance()
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	e.G.Players[0].Pool = state.Mana{}
	pump := func() {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			d = e.Pending()
			if d == nil {
				t.Fatal("no pending decision")
			}
		}
		var opt *decision.Option
		for i := range d.Options {
			if o := &d.Options[i]; o.Kind == "ability" {
				if opt == nil {
					opt = o
				}
			}
		}
		if opt == nil {
			t.Fatalf("no ability option to pump Mistwalker: %+v", d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
			t.Fatal(err)
		}
		for i := 0; ; i++ {
			d := e.Pending()
			if d == nil || i == 20 {
				t.Fatal("the activation did not settle")
			}
			if d.Kind == decision.KPriority {
				break
			}
			t.Fatalf("unexpected %v ask at %q: %+v", d.Kind, d.Prompt, d.Options)
		}
	}
	pump()
	pump()
	ttcTapped := e.G.Obj(ttcO).Tapped
	poolC := e.G.Players[0].Pool[5]
	if ttcTapped || poolC != 0 {
		t.Fatalf("precondition: after the two pumps TTC tapped=%v pool C=%d, want untapped, 0", ttcTapped, poolC)
	}
	plan, found := absorbVisCyclePlan(e)
	if !found {
		t.Fatal("precondition: the potential walk lists no Absorb Vis cycle")
	}
	if plan.Reason != "" || len(plan.Script) == 0 {
		t.Fatalf("the {1}{B} cycle is payable (an Island's {U}{U} funds the arm, the arm's {B} plus Mana Flare's reflected {B} pays it) but the potential search found no prefix: reason %q detail %q script %v", plan.Reason, plan.Detail, plan.Script)
	}
}

// absorbVisCyclePlan is seat 0's potential play for Absorb Vis's own cycling.
func absorbVisCyclePlan(e *Engine) (PotentialPlan, bool) {
	for _, pp := range e.potentialPlans(0, potentialScriptPlays) {
		if pp.Action.Kind == optAbility && pp.Action.Obj == absorbVisHandObj(e) {
			return pp, true
		}
	}
	return PotentialPlan{}, false
}

func absorbVisHandObj(e *Engine) state.ObjID {
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == "Absorb Vis" {
			return id
		}
	}
	return 0
}
