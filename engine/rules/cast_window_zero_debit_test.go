// No replayCheck here or in the sibling test below: the payer's life is set
// out-of-band (no event carries a life set), so a log-only replay cannot
// reproduce it -- the same boundary mana_negative_life_test.go accepts.
// The zero-debit life guard: a window alt with Life == 0 is unaffected by
// the payer's life total, even when the payer has been carried below 0 (the
// loser's remaining priority turns, CR 704.3 -- state-based actions are not
// checked until a player would receive priority). A guard that reads
// lifeLeft directly (`a.Life > lifeLeft`) skips every zero-debit alt of a
// dead payer and strands casts the window can pay: the 20000-game census
// measured ~450 extra "cast aborted: no legal target" notes without it.
// The guard lives in TWO search sites that must answer identically:
// castWindowReachable (engine/rules/cast_window.go) and the shared
// pay.UnlessManaReachable plan-shape search (engine/rules/pay/planshape.go).
//
// No Forge .txt is committed: the Rebuke and probe fixtures are authored
// inline (the same shapes cast_target_menu_reprice_test.go and
// mana_negative_life_test.go pin).

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const zdPhyProbeSrc = "Name:ZD PhyProbe\nManaCost:2 UP\nTypes:Instant\n" +
	"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"

// TestZeroDebitCastWindowSurvivesNegativeLife drives the target-menu filter
// (the castWindowReachable site) with the payer at -1 life: the tapped
// target's repriced {1}{W} is reachable from the {W}{W} zero-debit window,
// so the menu offers it and the cast pays completely. Without the
// `a.Life > 0` conjunct every zero-debit alt is skipped at lifeLeft < 0,
// the menu empties and the proposal aborts.
func TestZeroDebitCastWindowSurvivesNegativeLife(t *testing.T) {
	t.Parallel()
	e, spellID, untapped, tapped := rebukeMenuGame(t)
	// The loser's remaining priority turns: the payer was carried below 0
	// by its own damage before this cast was proposed.
	e.G.Players[0].Life = -1

	pc := &pendingCast{player: 0, card: spellID, from: state.ZHand, ability: -1,
		cost: e.parseCost("4 W")}
	units := e.castWindowUnits(pc)
	if len(units) == 0 {
		t.Fatal("precondition: the window census is empty; the two Plains are not on the board")
	}
	for _, u := range units {
		for _, a := range u.Alts {
			if a.Life != 0 {
				t.Fatalf("precondition: window alt carries life %d, want 0 (zero-debit): %+v", a.Life, a)
			}
		}
	}
	// Direct guard-site read at life -1: the window reaches the tapped
	// target's repriced {1}{W} from an empty pool, and still refuses {4}{W}.
	if !e.castWindowReachable(0, e.parseCost("1 W"), state.Mana{}, state.Mana{},
		[7]state.Mana{}, -1, nil, pipRider{}, units) {
		t.Fatal("castWindowReachable refused {1}{W} from the zero-debit {W}{W} window at life -1")
	}
	if e.castWindowReachable(0, e.parseCost("4 W"), state.Mana{}, state.Mana{},
		[7]state.Mana{}, -1, nil, pipRider{}, units) {
		t.Fatal("precondition: {4}{W} priced reachable from the {W}{W} window; the assertion is vacuous")
	}

	e.cast = &pendingCast{player: 0, card: spellID, from: state.ZHand, ability: -1,
		cost: e.parseCost("4 W")}
	e.continueCast()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want the target decision at life -1, got %+v", d)
	}
	offered := map[state.ObjID]bool{}
	for _, o := range d.Options {
		if o.Obj == untapped {
			t.Errorf("untapped creature offered: its repriced {4}{W} is beyond the {W}{W} window (CR 601.2c): %+v", d.Options)
		}
		offered[o.Obj] = true
	}
	if !offered[tapped] {
		t.Fatalf("tapped creature not offered at life -1; menu = %+v", d.Options)
	}

	// The offered cast completes at the tapped target's {1}{W}. The payer is
	// at -1, so the closing SBA of a submit ends the game (CR 704.3, the
	// same boundary mana_negative_life_test.go pins) -- but the payment
	// settles BEFORE the loss is recorded: pool drained to 0 from the two
	// {W} productions, the spell put on the stack, and never moved off it
	// before the loss.
	targetObject(t, e, tapped)
	for i := 0; i < 60; i++ {
		dd := e.Pending()
		if dd == nil || dd.Kind == decision.KPriority {
			break
		}
		if len(dd.Options) == 0 {
			t.Fatalf("empty decision mid-payment: %+v", dd)
		}
		submitChoices(t, e, dd.Options[0].Index)
	}
	if hasNote(e, "no longer payable") || hasNote(e, "cast aborted") {
		t.Fatal("payment abort Note: the window could pay {1}{W} at life -1")
	}
	if !nlPutOnStack(e, spellID) {
		t.Fatal("the spell never reached the stack; the payment did not complete")
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the cast = %d, want 0", got)
	}
	zdPaidBeforeLost(t, e, spellID)
}

// zdPaidBeforeLost pins the ordering the zero-debit guard exists to protect:
// the payment's mana events precede the payer's loss (the payment ran to
// completion inside the dead payer's own cast), and the spell was never
// reversed off the stack before that loss.
func zdPaidBeforeLost(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	lost := -1
	paid := 0
	for i, ev := range e.L.Events {
		switch {
		case ev.Kind == events.PlayerLost && ev.Player == 0 && lost < 0:
			lost = i
		case ev.Kind == events.ManaAdd && ev.Amount < 0 && lost < 0:
			paid++
		case ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZStack && lost < 0:
			t.Fatalf("event %d moved the spell off the stack before the loss: %+v", i, ev)
		}
	}
	if lost < 0 {
		t.Fatal("the payer was never lost; the SBA boundary did not run")
	}
	if paid == 0 {
		t.Fatalf("no mana payment event before the loss; the payment did not settle (events=%v)", e.L.Events)
	}
}

// TestZeroDebitManaReachableSurvivesNegativeLife runs the search
// announceFeasible reaches (engine/rules/cast_payment.go) at the payer's real
// life of -1: the {U} face of
// {2}{U/P} prices only from the untapped Island's zero-debit window, the
// 2-life face is beyond the payer, so the pip ask offers pay_U and the
// payment completes through the real cast flow.
func TestZeroDebitManaReachableSurvivesNegativeLife(t *testing.T) {
	t.Parallel()
	e, _, probe := newFixtureDeck(t, 883, zdPhyProbeSrc, nlIslandSrc)
	island := putCreature(t, e, 0, nlIslandSrc)
	addMana(t, e, 0, "CC")
	e.G.Players[0].Life = -1
	e.pending = nil
	e.priorityRound()

	// Preconditions: the payer really is at -1 with the Island untapped and
	// the floating {C}{C} on the table, and the pool-only offer gate (which
	// cannot see the window) withheld the cast -- so the flow below reaches
	// the shared search the same way announceFeasible does.
	if got := e.G.Players[0].Life; got != -1 {
		t.Fatalf("precondition: payer life = %d, want -1", got)
	}
	if o := e.G.Obj(island); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Island not untapped on the battlefield: %+v", o)
	}
	if got := e.G.Players[0].Pool.Total(); got != 2 {
		t.Fatalf("precondition: floating pool = %d, want 2", got)
	}
	if castOffered(e, probe) {
		t.Fatal("precondition: the pool-only offer gate offered the {2}{U/P} cast at life -1; the window arm below would be skipped")
	}

	// Direct guard-site read, the exact invocation cast_payment.go makes:
	// pool-only feasibleAny refuses the {U} face, and castWindowReachable must
	// accept it from the Island's zero-debit alt at the payer's life of -1.
	pc := &pendingCast{player: 0, card: probe, from: state.ZHand, ability: -1,
		cost: e.parseCost("2 UP")}
	face := e.parseCost("2 U")
	payment := paymentForCast(pc, face)
	rider := pipRider{}
	if e.manaFeasibleDescriptor(0, payment, face, costMods{}, 0, 0, rider) {
		t.Fatal("precondition: the pool alone pays {2}{U}; the plan-shape search would not be reached")
	}
	av := pay.AvailableFor(asPayer(e), 0, payment)
	units := e.castWindowUnits(pc)
	if len(units) == 0 || len(units[0].Alts) != 1 || units[0].Alts[0].Life != 0 {
		t.Fatalf("precondition: window census = %+v, want one zero-debit Island alt", units)
	}
	conv := asEval(e).Conv(0, payment.ID, payment.Class == paymentActivated)
	if !e.castWindowReachable(0, face, av.Pool, e.G.Players[0].Snow,
		av.Typed, e.G.Players[0].Life, conv, rider, units) {
		t.Fatal("castWindowReachable refused {2}{U} from the zero-debit window at life -1")
	}

	// The real flow: the pip ask prices the {U} face (the guard site) and
	// withholds the 2-life face (CR 119.4), the Island pays the pip, and the
	// {C}{C} pool settles the {2}.
	e.pending = nil
	e.beginCast(0, decision.Option{Kind: "cast", Obj: probe})
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("after beginning the cast = %+v, want the {U/P} pip-face ask", d)
	}
	blue, lifeOpt := -1, -1
	for _, o := range d.Options {
		switch o.Kind {
		case "pay_U":
			blue = o.Index
		case "pay_life":
			lifeOpt = o.Index
		}
	}
	if lifeOpt >= 0 {
		t.Errorf("the 2-life face is offered at life -1 (CR 119.4): %+v", d.Options)
	}
	if blue < 0 {
		t.Fatalf("the {U} face was withheld at life -1 though the untapped Island can produce it: %+v", d.Options)
	}
	submitChoices(t, e, blue)
	if !cwActivateInWindow(t, e, island) {
		t.Fatalf("the mana window did not offer the Island: %+v", e.Pending())
	}
	dd := e.Pending()
	if dd != nil && dd.Kind != decision.KPriority {
		t.Fatalf("after the Island paid the pip = %+v, want the window closed", dd)
	}
	if hasNote(e, "no longer payable") || hasNote(e, "cast aborted") {
		t.Fatal("payment abort Note: the zero-debit window could pay the {U} face")
	}
	if !nlPutOnStack(e, probe) {
		t.Fatal("the probe never reached the stack; the payment did not complete")
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the cast = %d, want 0", got)
	}
	zdPaidBeforeLost(t, e, probe)
}
