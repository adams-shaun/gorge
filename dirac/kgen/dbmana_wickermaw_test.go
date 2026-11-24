package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDbManaWickermawLowers: Foraging Wickermaw's "{1}: Add one mana of any
// color. This creature becomes that color until end of turn. Activate only
// once each turn." is a CR 605.1a mana ability gorge resolves off the stack
// (cards.IsManaAbilitySA: the head ChooseColor's chain reaches a
// Produced$ Chosen Mana). The kernel models it as a planner-deferred rider
// (gorge's PaymentPlanAbilityTier source:last_resort): the {1} fee is paid
// from the pool, its use this turn is the activation limit, and its Prog is
// the chosen-colour Animate (five WUBRG records, one Override op). The
// compiled cost is the head's {1}, charged once; the Mana node's own Cost$ 1
// is the same part, not a second charge.
func TestDbManaWickermawLowers(t *testing.T) {
	cards := lowerNamed(t, []string{"Foraging Wickermaw"})
	if len(cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cards))
	}
	c := cards[0]
	if len(c.Refusals) != 0 {
		t.Fatalf("Foraging Wickermaw refused: %v", c.Refusals)
	}
	var mana *lAbility
	for _, a := range c.Abilities {
		if a.Kind == akMana {
			if mana != nil {
				t.Fatalf("two mana abilities: %+v", c.Abilities)
			}
			mana = a
		}
	}
	if mana == nil {
		t.Fatalf("no mana ability among %+v", c.Abilities)
	}
	// The head's {1}, once each turn, no {T}: the planner-deferred rider route.
	if mana.Cost.Tap || mana.Cost.Mana.Generic != 1 || !mana.TurnOnce {
		t.Fatalf("cost %+v turnOnce %v, want a {1} once-each-turn ability", mana.Cost, mana.TurnOnce)
	}
	if mana.Cost.Mana.Pips != [5]int{} || mana.Cost.Mana.C != 0 || mana.Cost.Mana.X {
		t.Fatalf("cost mana %+v, want the head's {1} only", mana.Cost.Mana)
	}
	if mana.Mana == nil || mana.Mana.Colors != "WUBRG" || !mana.Mana.Amount.isConst() || mana.Mana.Amount.N != 1 {
		t.Fatalf("production %+v, want one mana of any colour", mana.Mana)
	}
	if mana.Mana.Rider != riderChosenAnimate {
		t.Fatalf("rider kind %d, want riderChosenAnimate (%d)", mana.Mana.Rider, riderChosenAnimate)
	}
	// The rider program is the chosen-colour Animate step.
	if len(mana.Prog) != 1 || mana.Prog[0].Op != "Override" || !strings.Contains(mana.Prog[0].CS, "chosenColor") {
		t.Fatalf("rider program %+v, want one chosen-colour Override", mana.Prog)
	}
	// Precondition: five override records W, U, B, R, G from the op's C.
	first := mana.Prog[0].C
	if first < 0 || first+5 > len(c.overrides) {
		t.Fatalf("override start %d of %d records", first, len(c.overrides))
	}
	for i, col := range "WUBRG" {
		o := c.overrides[first+i]
		if !o.HasSetColors || o.SetColors != string(col) {
			t.Fatalf("override %d is %+v, want SetColors %q", first+i, o, string(col))
		}
	}
}

// TestDbManaWickermawCostChargedOnce: the head's {1} and the Mana node's
// Cost$ 1 are the same cost (Forge charges the head's only). The compiled
// ability carries one {1}, so a future lowering cannot charge it twice.
func TestDbManaWickermawCostChargedOnce(t *testing.T) {
	cards := lowerNamed(t, []string{"Foraging Wickermaw"})
	if len(cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cards))
	}
	for _, a := range cards[0].Abilities {
		if a.Kind != akMana {
			continue
		}
		if got := a.Cost.Mana.Generic; got != 1 {
			t.Fatalf("generic cost %d, want 1 (charged once)", got)
		}
		if a.Cost.PayLife != 0 || a.Cost.SacSelf || a.Cost.Sac != nil || a.Cost.DiscardN != 0 {
			t.Fatalf("cost %+v carries a second charge", a.Cost)
		}
		return
	}
	t.Fatal("no mana ability")
}

// TestDbManaWickermawRiderIndexRefused: the once-each-turn record is the
// kernel's uint8 perm.actUsed, so an ability at index 8 or later cannot carry
// the bit. The shape is not tied to Wickermaw's name; the lowering refuses
// any such ability rather than admitting one whose limit never blocks.
func TestDbManaWickermawRiderIndexRefused(t *testing.T) {
	lc := &lowerCtx{}
	sa := &cards.SA{Kind: "AB", API: "ChooseColor"}
	err := lc.lowerChooseColorMana(sa, newParams(nil), &lAbility{Index: 8})
	if err == nil || !strings.Contains(err.Error(), "index 8") {
		t.Fatalf("lowerChooseColorMana at index 8 = %v, want the uint8-index refusal", err)
	}
}
