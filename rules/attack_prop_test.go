// CantAttackUnless (the attack-prop static, rules/attack_cost.go): Ghostly
// Prison's "creatures can't attack you unless their controller pays {2} for
// each creature they control that's attacking you" and its SVar-priced
// sibling Sphere of Safety, pinned end to end through the declare-attackers
// decision, the pool payment and the tap-payment window.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// attackPropSeat builds a three-seat table with the named corpus prop card on
// seat 0 and one ready attacker on seat 1, active on seat 1 in the
// declare-attackers step. nLands puts n untapped Mountains on seat 1 (the
// payment window's sources); the seats' own Mountain decks stay in their
// libraries, so every battlefield permanent is the fixture's.
func attackPropSeat(t *testing.T, propName string, nLands int) (*Engine, state.ObjID) {
	t.Helper()
	cfg := Config{Seed: 716, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	onBoardCard(t, e, 0, mshCorpusCard(t, propName))
	bear := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
	for i := 0; i < nLands; i++ {
		onBoardCard(t, e, 1, card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"))
	}
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	return e, bear
}

// floatMana floats the named symbols on seat p through the log (the addMana
// shape, without the turn movement the shared helper does).
func floatMana(t *testing.T, e *Engine, p state.PlayerID, symbols string) {
	t.Helper()
	for _, r := range symbols {
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: string(r), Amount: 1})
	}
}

// TestGhostlyPrisonChargesPerAttackerFromThePool pins the pool-payment half:
// with {2} floating the pair is offered, the declaration pays it, and the
// attack commits; with less floating the pair is not offered at all (the
// attack is illegal, CR 508.1 -- the decision never tempts the seat with a
// declaration the engine would reject).
func TestGhostlyPrisonChargesPerAttackerFromThePool(t *testing.T) {
	e, bear := attackPropSeat(t, "Ghostly Prison", 0)

	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Player == 0 {
			t.Fatalf("unaffordable pair offered with an empty pool: %+v", o)
		}
	}

	// {2} floating: the pair is offered, priced, and paying it empties the
	// pool and commits the attack.
	floatMana(t, e, 1, "GG")
	e.askAttackers()
	d = e.Pending()
	var opt *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Player == 0 && o.Obj == bear {
			opt = o
		}
	}
	if opt == nil {
		t.Fatalf("affordable pair not offered: %+v", d.Options)
	}
	if opt.Label != "Attack with Runeclaw Bear at a (pay {2} per creature)" {
		t.Fatalf("price label %q", opt.Label)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit charged attack: %v", err)
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying the {2} attack cost = %d, want 0", got)
	}
	if o := e.G.Obj(bear); !o.IsAttacking {
		t.Fatal("charged attacker never declared")
	}
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("no payment window expected when the pool covers the charge, got %+v", d)
	}
	drainCombatPriority(t, e)
}

// TestGhostlyPrisonTapsManaSourcesToPay pins the payment window: with an
// empty pool and two untapped Mountains the seat is asked to tap, one source
// at a time, and the declaration commits exactly when the pool covers the
// {2} -- both sources tapped, no over-tap, pool back to zero.
func TestGhostlyPrisonTapsManaSourcesToPay(t *testing.T) {
	e, bear := attackPropSeat(t, "Ghostly Prison", 2)
	var mountains []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if e.G.Obj(id).Face().Name == "Mountain" {
			mountains = append(mountains, id)
		}
	}
	if len(mountains) != 2 {
		t.Fatalf("fixture needs two Mountains, has %d", len(mountains))
	}

	e.askAttackers()
	d := e.Pending()
	var opt *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Player == 0 && o.Obj == bear {
			opt = o
		}
	}
	if opt == nil {
		t.Fatalf("pair not offered although two Mountains can pay: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit attack: %v", err)
	}

	for round := 0; round < 2; round++ {
		pay := e.Pending()
		if pay == nil || pay.Kind != decision.KChoose || len(pay.Options) == 0 ||
			pay.Options[0].Kind != "attack_mana" {
			t.Fatalf("round %d: expected the attack payment window, got %+v", round, pay)
		}
		if pay.Player != 1 {
			t.Fatalf("payment window posed to seat %d, want the declaring seat 1", pay.Player)
		}
		if err := e.Submit(decision.Intent{Seq: pay.Seq, Player: pay.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("tap source: %v", err)
		}
	}
	if pay := e.Pending(); pay != nil && pay.Kind != decision.KPriority {
		t.Fatalf("window should have completed at exactly {2}, got %+v", pay)
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying = %d, want 0", got)
	}
	for _, id := range mountains {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("Mountain %d not tapped as the payment", id)
		}
	}
	if o := e.G.Obj(bear); !o.IsAttacking {
		t.Fatal("paid attacker never declared")
	}
	drainCombatPriority(t, e)
}

// TestGhostlyPrisonBlocksWhenTheWindowCannotReach: one Mountain produces 1,
// the {2} charge is out of reach, and the pair is never offered -- the
// conservative direction for a prop (the attack is refused, never wedged).
func TestGhostlyPrisonBlocksWhenTheWindowCannotReach(t *testing.T) {
	e, bear := attackPropSeat(t, "Ghostly Prison", 1)
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Player == 0 {
			t.Fatalf("pair offered although one Mountain cannot pay {2}: %+v", o)
		}
	}
	if o := e.G.Obj(bear); o.IsAttacking {
		t.Fatal("attacker declared without being submitted")
	}
}

// TestSphereOfSafetyPricesItsEnchantmentCount pins the SVar-priced shape:
// Sphere of Safety's Cost$ X with `SVar:X:Count$Valid Enchantment.YouCtrl`
// reads X off the static's own source (the Sphere plus the Ghostly Prison =
// 2 enchantments), so the price is {2} per attacker, paid from the pool.
func TestSphereOfSafetyPricesItsEnchantmentCount(t *testing.T) {
	e, bear := attackPropSeat(t, "Sphere of Safety", 0)
	onBoardCard(t, e, 0, mshCorpusCard(t, "Ghostly Prison"))

	e.askAttackers()
	d := e.Pending()
	for _, o := range d.Options {
		if o.Player == 0 {
			t.Fatalf("pair offered with an empty pool: %+v", o)
		}
	}
	floatMana(t, e, 1, "BBBB")
	e.askAttackers()
	d = e.Pending()
	var opt *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Player == 0 && o.Obj == bear {
			opt = o
		}
	}
	if opt == nil {
		t.Fatalf("pair not offered at {4} floating: %+v", d.Options)
	}
	if opt.Label != "Attack with Runeclaw Bear at a (pay {4} per creature)" {
		t.Fatalf("Sphere of Safety price label %q", opt.Label)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit charged attack: %v", err)
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying the {4} attack cost = %d, want 0", got)
	}
	drainCombatPriority(t, e)
}

// TestWindbornMusePropIsTheSameShape is the census's other named carrier: a
// {2}-per-attacker prop whose pair pricing and payment are identical to
// Ghostly Prison's.
func TestWindbornMusePropIsTheSameShape(t *testing.T) {
	e, bear := attackPropSeat(t, "Windborn Muse", 2)
	e.askAttackers()
	d := e.Pending()
	var opt *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Player == 0 && o.Obj == bear {
			opt = o
		}
	}
	if opt == nil {
		t.Fatalf("pair not offered although the window can pay: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit attack: %v", err)
	}
	for i := 0; i < 2; i++ {
		pay := e.Pending()
		if pay == nil || pay.Kind != decision.KChoose || len(pay.Options) == 0 ||
			pay.Options[0].Kind != "attack_mana" {
			t.Fatalf("tap %d: expected the payment window, got %+v", i, pay)
		}
		if err := e.Submit(decision.Intent{Seq: pay.Seq, Player: pay.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("tap source: %v", err)
		}
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying = %d, want 0", got)
	}
	if o := e.G.Obj(bear); !o.IsAttacking {
		t.Fatal("paid attacker never declared")
	}
	drainCombatPriority(t, e)
}
