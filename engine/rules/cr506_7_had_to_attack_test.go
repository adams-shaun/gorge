package rules

// CR 506.7 (cr-g56): "A creature had to attack if one or more effects were
// requiring that creature to attack at the time attackers were declared in
// that combat." Firkraag, Cunning Instigator's second trigger reads it as
// `ValidSource$ Creature.hadToAttackThisCombat`: "Whenever a creature deals
// combat damage to one of your opponents, if that creature had to attack
// this combat, you put a +1/+1 counter on Firkraag and you draw a card."

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// hadToAttackBruteSrc attacks each combat if able (the corpus's 58-line
// `S:Mode$ MustAttack | ValidCreature$ Card.Self` shape); hadToAttackFreeSrc
// is the same body with no requirement. Both have haste so they attack the
// turn they are seeded.
const (
	hadToAttackBruteSrc = "Name:Brute Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nK:Haste\n" +
		"S:Mode$ MustAttack | ValidCreature$ Card.Self | Description$ CARDNAME attacks each combat if able.\nOracle:x\n"
	hadToAttackFreeSrc = "Name:Free Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nK:Haste\nOracle:x\n"
)

// hadToAttackCombat seeds Firkraag and the named attacker on seat 0's
// battlefield, declares that attacker at seat 1, and passes through combat
// damage to the end-of-combat step. It returns the engine, Firkraag's id and
// seat 0's hand size before the attack.
func hadToAttackCombat(t *testing.T, attackerSrc string) (*Engine, state.ObjID, state.ObjID, int) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, 5067, corpusCardText(t, "f/firkraag_cunning_instigator.txt"), attackerSrc)
	firk := moveSeeded(t, e, 0, corpusCardText(t, "f/firkraag_cunning_instigator.txt"), state.ZBattlefield)
	atk := putCreature(t, e, 0, attackerSrc)
	e.priorityRound()
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	hand := len(e.G.Zone(state.ZHand, 0))
	submitAttackersOnly(t, e, atk)
	for i := 0; i < 60 && !e.G.Over && e.G.Step != state.StepEndCombat && e.G.Step != state.StepMain2; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KBlockers:
			submitChoices(t, e)
		default:
			t.Fatalf("unexpected decision while driving combat: %+v", d)
		}
	}
	if o := e.G.Obj(atk); o == nil || !o.IsAttacking && e.G.Step == state.StepDeclareAttackers {
		t.Fatalf("precondition: %d did not attack", atk)
	}
	return e, firk, atk, hand
}

// TestCR506_7HadToAttackPredicateIsRegistered: the filter word is known.
func TestCR506_7HadToAttackPredicateIsRegistered(t *testing.T) {
	t.Parallel()
	if un := effects.UnknownPredicates("Creature.hadToAttackThisCombat"); len(un) != 0 {
		t.Fatalf("UnknownPredicates(Creature.hadToAttackThisCombat) = %v, want none (CR 506.7)", un)
	}
}

// TestCR506_7FirkraagRewardsARequiredAttacker: Brute Bear had to attack and
// dealt combat damage to seat 1, so Firkraag gets a +1/+1 counter and seat 0
// draws a card.
func TestCR506_7FirkraagRewardsARequiredAttacker(t *testing.T) {
	t.Parallel()
	e, firk, _, hand := hadToAttackCombat(t, hadToAttackBruteSrc)
	if n := e.G.Obj(firk).Counter("P1P1"); n != 1 {
		t.Fatalf("CR 506.7: Firkraag has %d +1/+1 counters after a creature that had to attack dealt combat damage, want 1", n)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("Firkraag's draw: hand %d -> %d, want +1", hand, got)
	}
}

// TestCR506_7FirkraagIgnoresAFreeAttacker guards the negative: a creature no
// effect required to attack does not trigger Firkraag.
func TestCR506_7FirkraagIgnoresAFreeAttacker(t *testing.T) {
	t.Parallel()
	e, firk, _, hand := hadToAttackCombat(t, hadToAttackFreeSrc)
	if n := e.G.Obj(firk).Counter("P1P1"); n != 0 {
		t.Fatalf("Firkraag has %d +1/+1 counters after a free attacker's damage, want 0", n)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand {
		t.Fatalf("hand %d -> %d after a free attacker's damage, want unchanged", hand, got)
	}
}
