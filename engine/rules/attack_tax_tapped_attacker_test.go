// The attack payment window (engine/rules/attack_cost.go askNextAttackPay)
// and the declared attackers CR 508.1f taps. The declare-attackers sequence
// taps the chosen creatures (508.1f) before the player activates mana
// abilities to pay the attack costs (508.1h), so a declared attacker without
// vigilance is tapped by then and cannot pay its own (or another's) tax. The
// window used to list every untapped source, the declared attackers
// included, because gorge taps them only after the payment
// (finishAttackers). A declared attacker with vigilance is not tapped
// (702.20b) and may still pay.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	taxedElfScript   = "Name:Taxed Elf\nTypes:Creature Elf\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ G | Oracle:x\nOracle:x\n"
	taxedVigilScript = "Name:Taxed Sentry\nTypes:Creature Elf\nPT:1/1\nK:Vigilance\nA:AB$ Mana | Cost$ T | Produced$ G | Oracle:x\nOracle:x\n"
	taxedForest      = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
)

// taxedAttackerSeat stages Ghostly Prison on seat 0 and, on the active seat
// 1, the mana creature from script (on the battlefield first, so it leads
// the window's battlefield order) and two Forests: a {2} tax the creature
// attacking seat 0 can pay only with the Forests once it is tapped.
func taxedAttackerSeat(t *testing.T, script string) (*Engine, state.ObjID) {
	t.Helper()
	cfg := Config{Seed: 718, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	onBoardCard(t, e, 0, corpusAlternativeCard(t, "Ghostly Prison"))
	elf := onBoardReady(t, e, 1, script)
	for i := 0; i < 2; i++ {
		onBoardCard(t, e, 1, card(t, taxedForest))
	}
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	return e, elf
}

// declareTaxed declares id at seat 0 and returns the payment window it opens.
func declareTaxed(t *testing.T, e *Engine, id state.ObjID) *decision.Decision {
	t.Helper()
	e.askAttackers()
	d := e.Pending()
	opt := findAttackOption(d, id, 0)
	if opt == nil {
		t.Fatalf("the taxed pair is not offered: %+v", d)
	}
	if opt.Value != 3 || d.MaxSum != 3 {
		t.Fatalf("pair Value %d under MaxSum %d, want 3 under 3 (the {2} tax plus the creature's own unit)", opt.Value, d.MaxSum)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit the taxed attack: %v", err)
	}
	pay := e.Pending()
	if pay == nil || pay.Kind != decision.KChoose || len(pay.Options) == 0 || pay.Options[0].Kind != "attack_mana" {
		t.Fatalf("expected the attack payment window, got %+v", pay)
	}
	return pay
}

// TestAttackWindowWithholdsTappedAttacker: the declared attacker without
// vigilance is not a source of the window that pays its tax, and the two
// Forests pay it.
func TestAttackWindowWithholdsTappedAttacker(t *testing.T) {
	t.Parallel()
	e, elf := taxedAttackerSeat(t, taxedElfScript)
	// PRECONDITION: the creature is an untapped source the window's
	// membership walk lists first, so only the CR 508.1f rule keeps it out.
	if srcs := e.attackManaSources(1); len(srcs) != 3 || srcs[0].id != elf {
		t.Fatalf("precondition: attackManaSources(1) = %+v, want the elf first of three", srcs)
	}
	pay := declareTaxed(t, e, elf)
	if offersObjOption(pay, elf) {
		t.Fatalf("the window lists the declared attacker CR 508.1f taps: %+v", pay.Options)
	}
	if len(pay.Options) != 2 {
		t.Fatalf("window offered %d sources, want the 2 Forests", len(pay.Options))
	}
	for i := 0; i < 2; i++ {
		pay = e.Pending()
		if pay == nil || pay.Kind != decision.KChoose || pay.Options[0].Kind != "attack_mana" {
			t.Fatalf("tap %d: expected the window, got %+v", i, pay)
		}
		if err := e.Submit(decision.Intent{Seq: pay.Seq, Player: pay.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("tap %d: %v", i, err)
		}
	}
	if o := e.G.Obj(elf); !o.IsAttacking || !o.Tapped {
		t.Fatalf("the elf attacks tapped after the Forests paid: attacking %v tapped %v", o.IsAttacking, o.Tapped)
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying {2} = %d, want 0", got)
	}
	drainCombatPriority(t, e)
}

// TestAttackWindowKeepsVigilantAttacker: a declared attacker with vigilance
// stays untapped (CR 702.20b), so the window still lists it.
func TestAttackWindowKeepsVigilantAttacker(t *testing.T) {
	t.Parallel()
	e, sentry := taxedAttackerSeat(t, taxedVigilScript)
	pay := declareTaxed(t, e, sentry)
	if !offersObjOption(pay, sentry) || len(pay.Options) != 3 {
		t.Fatalf("the vigilant attacker is not among the window's 3 sources: %+v", pay.Options)
	}
}
