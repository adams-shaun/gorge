package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredForageCanBePaid (CR 701.61a, cr-g77): a triggered "you may
// forage" (Bushy Bodyguard, Treetop Sentries, Corpseberry Cultivator, Curious
// Forager, Euru Acorn Scrounger) is payable, not decline-only. Before the fix
// the trigger-cost window kept the decline-only ask, so the payer could never
// forage. The corpus cards come from the shared corpus registry
// (searchTestRegistry); the helpers are search_library_test.go's
// searchEngine/searchMoveByName, cast_test.go's submitChoices/replayCheck,
// resolution_test.go's passUntilNonPriority and delayed_test.go's
// submitTarget.

// forageCostAsk drives seat 0's turn forward until the trigger-cost
// pay/decline ask of Curious Forager's enter trigger is pending, answering
// empty attack/block declarations on the way, and returns it.
func forageCostAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 200 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		switch d.Kind {
		case decision.KPriority:
			passUntilNonPriority(t, e, 200)
			continue
		case decision.KAttackers, decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err != nil {
				t.Fatalf("submit %s: %v", d.Kind, err)
			}
			continue
		}
		for _, o := range d.Options {
			if o.Kind == "trigger_cost_decline" {
				return d
			}
		}
		if len(d.Options) > 0 && d.Options[0].Kind == "yes" {
			submitChoices(t, e, 0)
			continue
		}
		t.Fatalf("unexpected decision while driving to the forage cost ask: %+v", d)
	}
	t.Fatal("never reached the forage cost ask")
	return nil
}

func foragePayIndex(d *decision.Decision) int {
	for _, o := range d.Options {
		if o.Kind == "trigger_cost_pay" {
			return o.Index
		}
	}
	return -1
}

func forageOption(d *decision.Decision, kind string) *decision.Option {
	for i := range d.Options {
		if d.Options[i].Kind == kind {
			return &d.Options[i]
		}
	}
	return nil
}

// forageDrainReturn passes priority until the returned card reaches the
// hand: the body's ChangeZone sits on the stack after its target answer and
// resolves on the next priority pass.
func forageDrainReturn(t *testing.T, e *Engine, returned state.ObjID) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if o := e.G.Obj(returned); o != nil && o.Zone == state.ZHand {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while waiting for the return (card zone %+v)", e.G.Obj(returned))
		}
		if d.Kind == decision.KPriority {
			passUntilNonPriority(t, e, 200)
			continue
		}
		t.Fatalf("unexpected decision while waiting for the return: %+v", d)
	}
	t.Fatal("the returned card never reached the hand")
}

// forageFixture builds the corpus engine with n bears in seat 0's graveyard
// (the deck's filler, moved in deck order), optionally the Food on the
// battlefield, and Curious Forager entering last, so its ETB trigger fires
// with the resources in place, and returns the engine, the bear ids and the
// Food id (zero without one).
func forageFixture(t *testing.T, bears int, withFood bool) (*Engine, Config, []state.ObjID, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	fixtures := []string{"Curious Forager"}
	if withFood {
		fixtures = append(fixtures, "Bagel and Schmear")
	}
	e, cfg := searchEngine(t, reg, fixtures...)
	var ids []state.ObjID
	for i := 0; i < bears; i++ {
		ids = append(ids, searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard))
	}
	var food state.ObjID
	if withFood {
		food = searchMoveByName(t, e, "Bagel and Schmear", state.ZBattlefield)
	}
	forager := searchMoveByName(t, e, "Curious Forager", state.ZBattlefield)
	if e.G.Obj(forager) == nil || e.G.Obj(forager).Zone != state.ZBattlefield {
		t.Fatalf("Curious Forager not on the battlefield (fixture)")
	}
	return e, cfg, ids, food
}

func forageActionCount(evs []events.Event, at int) int {
	n := 0
	for _, ev := range evs[at:] {
		if ev.Kind == events.ForageAction {
			n++
		}
	}
	return n
}

// forageWindow drives to the forage cost ask and returns the window,
// asserting the precondition that it is the Forage cost.
func forageWindow(t *testing.T, e *Engine) *triggeredEffectCost {
	t.Helper()
	d := forageCostAsk(t, e)
	tc := e.triggerCost
	if tc == nil || !tc.amount.Forage {
		t.Fatalf("precondition: pending window is not the Forage cost (ask %+v)", d)
	}
	return tc
}

func TestTriggeredForageCanBePaid(t *testing.T) {
	t.Run("ExileArmExilesThreeAndBodyReturns", func(t *testing.T) {
		// Both arms live: the pay arm poses the either-or election, the
		// exile arm exiles the top three graveyard cards, and Curious
		// Forager's body then returns the remaining permanent card.
		e, cfg, bears, food := forageFixture(t, 4, true)
		if len(bears) != 4 {
			t.Fatalf("fixture: %d bears, want 4", len(bears))
		}
		if len(e.G.Zone(state.ZGraveyard, 0)) != 4 {
			t.Fatalf("fixture: graveyard = %d cards, want 4", len(e.G.Zone(state.ZGraveyard, 0)))
		}
		d := forageCostAsk(t, e)
		payIdx := foragePayIndex(d)
		if payIdx < 0 {
			t.Fatalf("Forage cost with 4 graveyard cards and a Food offers no pay: %+v", d.Options)
		}
		submitChoices(t, e, payIdx)
		el := e.Pending()
		if el == nil || el.Kind != decision.KChoose {
			t.Fatalf("after pay: %+v, want the forage election ask", el)
		}
		exile := forageOption(el, "forage_exile")
		foodOpt := forageOption(el, "forage_food")
		if exile == nil || foodOpt == nil {
			t.Fatalf("forage election offered %v, want both arms", el.Options)
		}
		if foodOpt.Obj != food {
			t.Fatalf("forage_food names %d, want the Food %d", foodOpt.Obj, food)
		}
		start := len(e.L.Events)
		submitChoices(t, e, exile.Index)
		var exiled []state.ObjID
		for _, ev := range e.L.Events[start:] {
			if ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.Text == "exiled as a forage cost" {
				exiled = append(exiled, ev.Obj)
			}
		}
		if len(exiled) != 3 || forageActionCount(e.L.Events, start) != 1 {
			t.Fatalf("exile arm settled %v with %d ForageAction, want 3 exiles and 1", exiled, forageActionCount(e.L.Events, start))
		}
		grave := e.G.Zone(state.ZGraveyard, 0)
		if len(grave) != 1 || !slices.Contains(bears, grave[0]) || slices.Contains(exiled, grave[0]) {
			t.Fatalf("graveyard after forage = %v (bears %v, exiled %v), want one unexiled bear", grave, bears, exiled)
		}
		// The body's target ask: return the remaining permanent card.
		returned := grave[0]
		submitTarget(t, e, returned)
		forageDrainReturn(t, e, returned)
		if got := e.G.Obj(returned); got == nil || got.Zone != state.ZHand {
			t.Fatalf("returned card zone = %+v, want hand", got)
		}
		if e.G.Obj(food) == nil || e.G.Obj(food).Zone != state.ZBattlefield {
			t.Fatalf("Food moved by the exile arm: %+v", e.G.Obj(food))
		}
		replayCheck(t, e, cfg)
	})
	t.Run("FoodArmSacrificesWhenGraveyardShort", func(t *testing.T) {
		// One graveyard card: only the Food arm is live, and it is
		// deterministic -- no election ask, the Food is sacrificed.
		e, cfg, bears, food := forageFixture(t, 1, true)
		d := forageCostAsk(t, e)
		payIdx := foragePayIndex(d)
		if payIdx < 0 {
			t.Fatalf("Forage cost with a Food offers no pay: %+v", d.Options)
		}
		start := len(e.L.Events)
		submitChoices(t, e, payIdx)
		if f := e.G.Obj(food); f == nil || f.Zone != state.ZGraveyard {
			t.Fatalf("Food zone after paying = %+v, want sacrificed to the graveyard", f)
		}
		if forageActionCount(e.L.Events, start) != 1 {
			t.Fatalf("ForageAction count = %d, want 1", forageActionCount(e.L.Events, start))
		}
		// The body: return a permanent card from the graveyard (the bear or
		// the sacrificed Food).
		d2 := e.Pending()
		if d2 == nil || d2.Kind != decision.KTarget {
			t.Fatalf("after paying %+v, want the body's target ask", d2)
		}
		returned := bears[0]
		submitTarget(t, e, returned)
		forageDrainReturn(t, e, returned)
		if got := e.G.Obj(returned); got == nil || got.Zone != state.ZHand {
			t.Fatalf("returned card zone = %+v, want hand", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("FoodArmPickedFromBothArms", func(t *testing.T) {
		// Both arms live, and the payer elects the Food: the Food is
		// sacrificed, the graveyard is untouched, one ForageAction marks the
		// forage, and the body still returns a card.
		e, cfg, bears, food := forageFixture(t, 4, true)
		d := forageCostAsk(t, e)
		payIdx := foragePayIndex(d)
		if payIdx < 0 {
			t.Fatalf("Forage cost with 4 graveyard cards and a Food offers no pay: %+v", d.Options)
		}
		submitChoices(t, e, payIdx)
		el := e.Pending()
		if el == nil || el.Kind != decision.KChoose {
			t.Fatalf("after pay: %+v, want the forage election ask", el)
		}
		foodOpt := forageOption(el, "forage_food")
		if foodOpt == nil || forageOption(el, "forage_exile") == nil {
			t.Fatalf("forage election offered %v, want both arms", el.Options)
		}
		start := len(e.L.Events)
		submitChoices(t, e, foodOpt.Index)
		if f := e.G.Obj(food); f == nil || f.Zone != state.ZGraveyard {
			t.Fatalf("Food zone after the Food arm = %+v, want sacrificed to the graveyard", f)
		}
		grave := e.G.Zone(state.ZGraveyard, 0)
		if len(grave) != len(bears)+1 {
			t.Fatalf("graveyard after the Food arm = %d cards, want the %d bears plus the Food", len(grave), len(bears))
		}
		if forageActionCount(e.L.Events, start) != 1 {
			t.Fatalf("ForageAction count = %d, want 1", forageActionCount(e.L.Events, start))
		}
		d2 := e.Pending()
		if d2 == nil || d2.Kind != decision.KTarget {
			t.Fatalf("after paying %+v, want the body's target ask", d2)
		}
		returned := bears[0]
		submitTarget(t, e, returned)
		forageDrainReturn(t, e, returned)
		if got := e.G.Obj(returned); got == nil || got.Zone != state.ZHand {
			t.Fatalf("returned card zone = %+v, want hand", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("PayEnergyBesideForageSettles", func(t *testing.T) {
		// A Forage + PayEnergy<1> cost: the gate prices the forage on the
		// energy-stripped cost, so with one energy in the pool pay is
		// offered and paying spends the energy beside the Food sacrifice
		// (the settle must not re-prove with the energy still in the cost).
		e, cfg, _, food := forageFixture(t, 1, true)
		tc := forageWindow(t, e)
		tc.amount.Energy = []CostPart{{N: 1}}
		if e.triggeredCostPayable(tc) {
			t.Fatal("Forage + PayEnergy<1> with an empty pool gate says payable")
		}
		e.emit(events.Event{Kind: events.PlayerCounterChange, Player: tc.player, Counter: "ENERGY", Amount: 1})
		if !e.triggeredCostPayable(tc) {
			t.Fatal("Forage + PayEnergy<1> with one energy in the pool is not payable")
		}
		if tc.forageElectionAsk(e) {
			t.Fatal("the lone Food arm posed an election")
		}
		start := len(e.L.Events)
		if !tc.forageSettle(e, tc.announcedCost()) {
			t.Fatal("the settle declined a forage the gate offered (its re-proof must read the energy-stripped cost)")
		}
		if got := e.G.Players[tc.player].Counter("ENERGY"); got != 0 {
			t.Fatalf("energy pool after paying = %d, want 0 (the PayEnergy part spent)", got)
		}
		if f := e.G.Obj(food); f == nil || f.Zone != state.ZGraveyard {
			t.Fatalf("Food zone after paying = %+v, want sacrificed to the graveyard", f)
		}
		if forageActionCount(e.L.Events, start) != 1 {
			t.Fatalf("ForageAction count = %d, want 1", forageActionCount(e.L.Events, start))
		}
		replayCheck(t, e, cfg)
	})
	t.Run("ForageBesideEvidenceKeepsDeclineOnly", func(t *testing.T) {
		// A Forage + CollectEvidence<1> cost: Evidence is outside the
		// mana/life half, so the gate keeps the decline-only ask -- the
		// Evidence part must never be silently skipped by the settle.
		e, _, _, _ := forageFixture(t, 3, true)
		tc := forageWindow(t, e)
		d := e.Pending()
		if foragePayIndex(d) < 0 {
			t.Fatalf("precondition: the forage alone offered no pay: %+v", d.Options)
		}
		tc.amount.Evidence = []CostPart{{N: 1}}
		e.pending = nil
		e.triggeredCostPaymentAsk()
		d2 := e.Pending()
		if foragePayIndex(d2) >= 0 || len(d2.Options) != 1 || d2.Options[0].Kind != "trigger_cost_decline" {
			t.Fatalf("Forage + CollectEvidence ask = %+v, want decline-only", d2.Options)
		}
	})
	t.Run("NeitherResourceKeepsDeclineOnly", func(t *testing.T) {
		// No graveyard cards, no Food: the ask is decline-only -- "pay" is
		// not offered, and declining never forages.
		e, _, _, _ := forageFixture(t, 0, false)
		d := forageCostAsk(t, e)
		if foragePayIndex(d) >= 0 || len(d.Options) != 1 || d.Options[0].Kind != "trigger_cost_decline" {
			t.Fatalf("unpayable forage ask = %+v, want decline-only", d.Options)
		}
		start := len(e.L.Events)
		submitChoices(t, e, d.Options[0].Index)
		if forageActionCount(e.L.Events, start) != 0 {
			t.Fatal("decline emitted a ForageAction")
		}
	})
}
