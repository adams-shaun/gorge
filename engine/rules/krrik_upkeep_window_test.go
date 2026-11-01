// The krrik-upkeep-window regression: K'rrik, Son of Yawgmoth's
// PayLifeInsteadOf:B grant made paymentManaAskClass (engine/rules/cumulative.go)
// skip the activate/done payment window for BOTH upkeep paths (cumulative
// upkeep enters it directly, Echo through paymentManaAsk). The gate priced
// "the pool already pays" through the grant-derived costPayableClass, so an
// empty pool plus a {B} pip counted as paid, the payer never saw the window,
// and the payment spent 2 life with an untapped Swamp on the battlefield.
// The gate now asks costPayableClassLife with the grant suspended (the pool
// alone must pay); the payment itself and every offer gate keep the grant, so
// answering "done" still spends the 2 life.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	krrikCumulativeFixture = "Name:Test Krrik Cumulative\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nK:Cumulative upkeep:B\nOracle:test\n"
	krrikEchoFixture       = "Name:Test Krrik Echo\nManaCost:2\nTypes:Creature Bear\nPT:2/2\nK:Echo:B\nOracle:test\n"
)

// krrikUpkeepWindowEngine seats seat 0 with the fixture, K'rrik and a Swamp,
// moves all three onto the battlefield with logged MoveZone events (the echo
// gate reads the control-acquisition tuple) and leaves the engine in main
// phase 1 of turn 1 with an EMPTY pool.
func krrikUpkeepWindowEngine(t *testing.T, fixture string) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	fixtureCard := card(t, fixture)
	krrik := corpusAlternativeCard(t, "K'rrik, Son of Yawgmoth")
	var deck []*cards.Card
	deck = append(deck, fixtureCard, krrik, card(t, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"))
	for len(deck) < 40 {
		deck = append(deck, card(t, "Name:Filler\nTypes:Basic Land\nOracle:x\n"))
	}
	cfg := seatZeroStart(Config{Seed: 7317, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	id := searchMoveByName(t, e, fixtureCard.Faces[0].Name, state.ZBattlefield)
	krrikID := searchMoveByName(t, e, "K'rrik, Son of Yawgmoth", state.ZBattlefield)
	if krrikID == 0 {
		t.Fatal("precondition: K'rrik was not moved onto the battlefield")
	}
	swampID := searchMoveByName(t, e, "Swamp", state.ZBattlefield)
	return e, cfg, id, swampID
}

// krrikUpkeepPreconditions asserts everything the regression depends on: the
// permanent and the Swamp are where the rules read them, K'rrik's grant is
// live for the payer, and the pool is empty.
func krrikUpkeepPreconditions(t *testing.T, e *Engine, id, swamp state.ObjID, keyword string) {
	t.Helper()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the fixture permanent is not on the battlefield: %+v", o)
	}
	if !e.G.Obj(id).Face().HasKeyword(keyword) {
		t.Fatal("precondition: the fixture does not print the upkeep keyword")
	}
	if o := e.G.Obj(swamp); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: the Swamp must be an untapped battlefield source: %+v", o)
	}
	if !asEval(e).PayLifeInsteadOfB(0) {
		t.Fatal("precondition: K'rrik's PayLifeInsteadOf:B grant is not active for seat 0")
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool must be empty, got %+v", pool)
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", life)
	}
}

// krrikFindWindowOption returns the index of the option with the given Kind
// (and, for activate options, the given object) in the pending decision.
func krrikFindWindowOption(t *testing.T, d *decision.Decision, kind string, obj state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind != kind {
			continue
		}
		if kind == "activate" && o.Obj != obj {
			continue
		}
		return o.Index
	}
	t.Fatalf("option %q (obj %v) missing from %+v", kind, obj, d.Options)
	return -1
}

// krrikElectManual answers the pay-first election (respay.go) -- posed
// before any mana window, with the plain pay (the planner's route), the
// sacrifice and the manual route in that order -- with the manual route, and
// returns the manual mana window it opens.
func krrikElectManual(t *testing.T, e *Engine, d *decision.Decision, kw string) *decision.Decision {
	t.Helper()
	if len(d.Options) != 3 || d.Options[0].Kind != kw+"_pay" || d.Options[1].Kind != kw+"_sac" || d.Options[2].Kind != kw+"_pay_manual" {
		t.Fatalf("%s election = %+v, want pay / sacrifice / manual pay", kw, d.Options)
	}
	submitChoices(t, e, 2)
	w := e.Pending()
	if w == nil || w.Kind != decision.KChoose {
		t.Fatalf("the manual route posed %+v, want the mana window", w)
	}
	return w
}

func TestKrrikEchoPosesManaWindowWhenSourceCanPay(t *testing.T) {
	t.Parallel()
	e, cfg, bear, swamp := krrikUpkeepWindowEngine(t, krrikEchoFixture)
	krrikUpkeepPreconditions(t, e, bear, swamp, "Echo")
	if c := ParseCost("B"); c.Colored[state.MB] != 1 || c.Generic != 0 || !c.Priceable() {
		t.Fatalf("precondition: ParseCost(B) = %+v, want one plain {B} pip", c)
	}

	d := driveEchoQuiet(t, e, bear, 3, 0, state.StepDraw)
	if d == nil {
		t.Fatal("the echo payment window must be posed at the first upkeep after entry")
	}
	if d.Kind != decision.KChoose || d.Source != bear {
		t.Fatalf("the echo election must be posed, got %+v", d)
	}
	d = krrikElectManual(t, e, d, "echo")
	swampOpt := krrikFindWindowOption(t, d, "activate", swamp)
	krrikFindWindowOption(t, d, "done", 0)
	submitChoices(t, e, swampOpt) // tap the Swamp: the elected pay settles

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("paid the echo but the permanent is %+v", o)
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("life=%d, want 20 (the pip was paid with the Swamp's mana, not K'rrik's life)", life)
	}
	if !e.G.Obj(swamp).Tapped {
		t.Fatal("the Swamp must be tapped to pay the pip")
	}
	replayCheck(t, e, cfg)
}

func TestKrrikEchoWindowDonePaysLife(t *testing.T) {
	t.Parallel()
	e, cfg, bear, swamp := krrikUpkeepWindowEngine(t, krrikEchoFixture)
	krrikUpkeepPreconditions(t, e, bear, swamp, "Echo")

	d := driveEchoQuiet(t, e, bear, 3, 0, state.StepDraw)
	if d == nil {
		t.Fatal("the echo payment window must be posed at the first upkeep after entry")
	}
	if d.Kind != decision.KChoose || d.Source != bear {
		t.Fatalf("the echo election must be posed, got %+v", d)
	}
	d = krrikElectManual(t, e, d, "echo")
	doneOpt := krrikFindWindowOption(t, d, "done", 0)
	submitChoices(t, e, doneOpt) // the elected pay settles with the granted life

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("paid the echo with life but the permanent is %+v", o)
	}
	if life := e.G.Players[0].Life; life != 18 {
		t.Fatalf("life=%d, want 18 (\"done\" spends the {B} pip's 2 granted life)", life)
	}
	if e.G.Obj(swamp).Tapped {
		t.Fatal("the Swamp must stay untapped when the payer answers \"done\"")
	}
	replayCheck(t, e, cfg)
}

// TestKrrikEchoAutoPayTapsTheSwamp is the auto route under K'rrik's grant:
// the plain pay funds the {B} pip from the untapped Swamp through the
// planner (the pool alone did not pay it, so the grant was suspended for the
// funding gate), and no life is spent.
func TestKrrikEchoAutoPayTapsTheSwamp(t *testing.T) {
	t.Parallel()
	e, cfg, bear, swamp := krrikUpkeepWindowEngine(t, krrikEchoFixture)
	krrikUpkeepPreconditions(t, e, bear, swamp, "Echo")

	d := driveEchoQuiet(t, e, bear, 3, 0, state.StepDraw)
	if d == nil || d.Kind != decision.KChoose || d.Source != bear || len(d.Options) == 0 || d.Options[0].Kind != "echo_pay" {
		t.Fatalf("the echo election must be posed with pay first, got %+v", d)
	}
	submitChoices(t, e, 0)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("paid the echo but the permanent is %+v", o)
	}
	if !e.G.Obj(swamp).Tapped {
		t.Fatal("the auto route must tap the Swamp for the pip")
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("life=%d, want 20 (the planner paid with mana, not K'rrik's life)", life)
	}
	replayCheck(t, e, cfg)
}
