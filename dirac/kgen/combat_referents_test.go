package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// combatReferentCards are the five supported cards whose combat trigger names
// the attacked, attacking or defending player: the two-seat lowering read that
// seat as the one opponent (TestTwoSeatReasons). Each now lowers seat-exact.
var combatReferentCards = []string{
	"Preacher of the Schism",
	"Undercover Butler",
	"Cloak of Confusion",
	"Labyrinth Raptor",
	"Contested Game Ball",
}

// TestCombatReferentsAnySeats checks that the five combat-referent cards lower
// at any seat count: no TwoSeat marker, no refusal, and each referent names
// the event's defending seat (Attacked$ Player.withMostLife,
// TriggeredDefendingPlayer) or the damage source's controller
// (TriggeredAttackingPlayer), never the one opponent.
func TestCombatReferentsAnySeats(t *testing.T) {
	cardsDir := ratchetCards(t)
	p, err := loadCardNames(combatReferentCards, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("build: %v", err)
	}
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, name := range combatReferentCards {
		c := byName[name]
		if c == nil {
			t.Fatalf("%s: not lowered", name)
		}
		if len(c.Refusals) > 0 {
			t.Errorf("%s: refused: %v", name, c.Refusals)
		}
		if c.TwoSeat || len(c.TwoSeatWhy) > 0 {
			t.Errorf("%s: two-seat only (TwoSeat=%v reasons=%v), want any seat count", name, c.TwoSeat, c.TwoSeatWhy)
		}
	}
	for _, name := range []string{"Preacher of the Schism", "Undercover Butler"} {
		c := byName[name]
		ab := combatReferentAbility(c, func(tr *lTrigger) bool { return tr.Event == "Attacks" && tr.Attacked&atkOpp != 0 })
		if ab == nil {
			t.Fatalf("%s: no Attacks/Attacked$ Opponent trigger:\n%s", name, cardLIR(c))
		}
		assertMostLifeGate(t, name, ab.Trigger.Cond)
		if name != "Preacher of the Schism" {
			continue
		}
		// The second Preacher trigger (CheckDefinedPlayer$ You.withMostLife)
		// reads the same event's every living opponent, not the next seat.
		second := combatReferentAbility(c, func(tr *lTrigger) bool {
			return tr.Event == "Attacks" && tr.Attacked == 0 && tr.Cond.Kind != cNone
		})
		if second == nil {
			t.Fatalf("%s: no CheckDefinedPlayer$ You.withMostLife trigger:\n%s", name, cardLIR(c))
		}
		cond := second.Trigger.Cond
		if cond.Kind != cValue || cond.A.Kind != vLifeOf || cond.A.Ref.Kind != rYou || cond.Op != opGE ||
			cond.B.Kind != vOppExtreme || cond.B.N != kop.OppExtLife {
			t.Errorf("%s: CheckDefinedPlayer$ You.withMostLife gate is %s, want You >= every living opponent", name, cond)
		}
	}
	// Cloak of Confusion: the defending player of the enchanted creature's
	// attack discards, not another opponent.
	cloak := byName["Cloak of Confusion"]
	in := combatReferentInstr(cloak, "Discard")
	if in == nil {
		t.Fatalf("Cloak of Confusion: no Discard instruction:\n%s", cardLIR(cloak))
	}
	if in.Obj.Kind != rDefending {
		t.Errorf("Cloak of Confusion: discard names %s, want the defending player (Defending)", in.Obj)
	}
	// Labyrinth Raptor: the defending player of the blocked menace creature's
	// attack sacrifices, not another opponent.
	raptor := byName["Labyrinth Raptor"]
	in = combatReferentInstr(raptor, "Sacrifice")
	if in == nil {
		t.Fatalf("Labyrinth Raptor: no Sacrifice instruction:\n%s", cardLIR(raptor))
	}
	if in.Obj.Kind != rDefending {
		t.Errorf("Labyrinth Raptor: sacrifice names %s, want the defending player (Defending)", in.Obj)
	}
	// Contested Game Ball: the attacking player is the damage source's
	// controller, which the DamageDone queue carries as the trigger object
	// (rTriggeredTgtCtrl); rTriggeredPlayer is the damaged player there.
	ball := byName["Contested Game Ball"]
	in = combatReferentInstr(ball, "GainControl")
	if in == nil {
		t.Fatalf("Contested Game Ball: no GainControl instruction:\n%s", cardLIR(ball))
	}
	if got := int(in.C >> 8); got != int(rTriggeredTgtCtrl) {
		t.Errorf("Contested Game Ball: new controller names ref %d, want the damage source's controller (TriggeredTgtCtrl=%d)", got, int(rTriggeredTgtCtrl))
	}
}

// assertMostLifeGate checks dethroneCond's shape: the defending player's life
// is not less than the controller's and not less than every living opponent's
// (ties included), evaluated against the defended seat.
func assertMostLifeGate(t *testing.T, name string, c lCond) {
	t.Helper()
	if c.Kind != cAnd || len(c.Sub) != 2 {
		t.Errorf("%s: most-life gate is %s, want the defended-seat comparison (And of two Values)", name, c)
		return
	}
	you, extreme := false, false
	for _, s := range c.Sub {
		if s.Kind != cValue || s.A.Kind != vLifeOf || s.A.Ref.Kind != rDefending || s.Op != opGE {
			t.Errorf("%s: most-life clause %s, want Defending >= ...", name, s)
			continue
		}
		switch {
		case s.B.Kind == vLifeOf && s.B.Ref.Kind == rYou:
			you = true
		case s.B.Kind == vOppExtreme && s.B.N == kop.OppExtLife:
			extreme = true
		default:
			t.Errorf("%s: most-life clause compares the defending player with %s, want You or the living opponents", name, s.B)
		}
	}
	if !you || !extreme {
		t.Errorf("%s: most-life gate %s, want Defending >= You and Defending >= every living opponent", name, c)
	}
}

// combatReferentAbility returns the first ability whose trigger matches ok.
func combatReferentAbility(c *lCard, ok func(*lTrigger) bool) *lAbility {
	if c == nil {
		return nil
	}
	for _, ab := range c.Abilities {
		if ab.Trigger != nil && ok(ab.Trigger) {
			return ab
		}
	}
	return nil
}

// combatReferentInstr returns the first instruction with op in any ability's
// program.
func combatReferentInstr(c *lCard, op string) *lInstr {
	if c == nil {
		return nil
	}
	for _, ab := range c.Abilities {
		for i := range ab.Prog {
			if ab.Prog[i].Op == op {
				return &ab.Prog[i]
			}
		}
	}
	return nil
}

func cardLIR(c *lCard) string {
	if c == nil {
		return "<nil>"
	}
	var b strings.Builder
	dumpLIR(&b, c)
	return b.String()
}
