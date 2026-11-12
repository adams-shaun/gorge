package kgen

import (
	"strings"
	"testing"
)

// The static Condition$ NotPlayerTurn class (gorge alternativeCostScopeOK's
// costConditionHolds read and combatBoard.StaticGateHolds'
// continuousGateHolds): the Force cycle's exile-a-card alternative cost (and
// Mine Collapse's PlayerTurn twin), and the three CantAttack/CantBlock
// self-restrictions with an UnlessDefender$ hasFewer<Type>sInPlayThanYou
// count (Monstrous Hound, Mogg Toady, Goblin Goon).

// TestForceCycleConditionAltCost pins the Force cycle's exile-only
// alternative cost with its turn condition: the Cost$ lowers as the bare
// hand-exile pick (ExileFromHand<1/Spec> alone, Misdirection the same shape
// with no condition) and the Condition$ NotPlayerTurn rides the static as
// Cond cYourTurn Neg (Mine Collapse: PlayerTurn, not negated). Force of
// Virtue's shape lowers but its card keeps refusing (an enchantment, where
// the kernel offers no alternative cast), and Commandeer's two-card exile
// keeps refusing, by name.
func TestForceCycleConditionAltCost(t *testing.T) {
	lw := lowerNamedPool(t, []string{
		"Force of Negation", "Force of Vigor", "Force of Despair",
		"Force of Rage", "Force of Virtue", "Misdirection", "Mine Collapse", "Commandeer",
	})
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	cs := lw.cards
	want := map[string]string{
		"Force of Negation": "",
		"Force of Vigor":    "",
		"Force of Despair":  "",
		// the cost and its condition lower; these two are refused by
		// their own spell bodies or the card the kernel offers no cast
		// for, not by the alternative
		"Force of Rage":   `SP$ Token: Token AtEOT "YourSacrificeUpkeep"`,
		"Misdirection":    "",
		"Force of Virtue": "alternative cost with an exile-from-hand part on a card the kernel offers no alternative cast for",
		"Mine Collapse":   "",
		"Commandeer":      `exile cost "ExileFromHand<2/Card.Blue+Other>"`,
	}
	seen := 0
	for _, c := range cs {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		got := strings.Join(why, "; ")
		if w == "" && got != "" {
			t.Errorf("%s refused: %s", c.Name, got)
		}
		if w != "" && !strings.Contains(got, w) {
			t.Errorf("%s refusal = %q, want it to contain %q", c.Name, got, w)
		}
		if w != "" {
			continue
		}
		// The turn condition rides the alternative-cost static: NotPlayerTurn
		// cYourTurn Neg (Mine Collapse's PlayerTurn not negated; Misdirection
		// has no condition at all).
		var alt *lStatic
		for i := range c.Statics {
			if c.Statics[i].Kind == "AlternativeCost" {
				alt = &c.Statics[i]
			}
		}
		if alt == nil {
			t.Fatalf("%s: no lowered AlternativeCost static", c.Name)
		}
		if c.Name == "Misdirection" {
			if alt.Cond.Kind != cNone {
				t.Errorf("Misdirection AlternativeCost Cond = %+v, want none", alt.Cond)
			}
			continue
		}
		wantNeg := c.Name != "Mine Collapse"
		if alt.Cond.Kind != cYourTurn || alt.Cond.Neg != wantNeg {
			t.Errorf("%s AlternativeCost Cond = %+v, want cYourTurn (Neg %v)", c.Name, alt.Cond, wantNeg)
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}

// TestFewerInPlayCantAttackBlock pins the three combat self-restrictions:
// the CantAttack line's UnlessDefender$ hasFewer<Type>sInPlayThanYou lowers
// to a value-vs-value battlefield count (the defender's -- read as the
// static controller's opponent -- at least the controller's), and the
// CantBlock line's NotPlayerTurn + CheckSVar$ Y SVarCompare$ GEX lowers to
// cAnd of cYourTurn Neg and the active player's count at least the
// controller's.
func TestFewerInPlayCantAttackBlock(t *testing.T) {
	cs := lowerNamed(t, []string{"Monstrous Hound", "Mogg Toady", "Goblin Goon"})
	typeCounts := map[string]string{"Monstrous Hound": "Land", "Mogg Toady": "Creature", "Goblin Goon": "Creature"}
	seen := 0
	for _, c := range cs {
		typ, ok := typeCounts[c.Name]
		if !ok {
			continue
		}
		seen++
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		if got := strings.Join(why, "; "); got != "" {
			t.Errorf("%s refused: %s", c.Name, got)
		}
		countCond := func(v lValue, op uint8, other lValue, aSpec string) bool {
			return v.Kind == vCount && v.F != nil && filterIs(v.F, aSpec) &&
				op == opGE && other.Kind == vCount && other.F != nil && filterIs(other.F, typ+".YouCtrl")
		}
		var atk, blk *lStatic
		for i := range c.Statics {
			switch c.Statics[i].Kind {
			case "CantAttack":
				atk = &c.Statics[i]
			case "CantBlock":
				blk = &c.Statics[i]
			}
		}
		if atk == nil || blk == nil {
			t.Fatalf("%s: missing the CantAttack/CantBlock statics", c.Name)
		}
		// CantAttack: the defender's count >= the controller's.
		if atk.Cond.Kind != cValue || !countCond(atk.Cond.A, atk.Cond.Op, atk.Cond.B, typ+".OppCtrl") {
			t.Errorf("%s CantAttack Cond = %+v, want vCount(%s.OppCtrl) >= vCount(%s.YouCtrl)",
				c.Name, atk.Cond, typ, typ)
		}
		// CantBlock: NotPlayerTurn AND the active player's count >= the
		// controller's (CheckSVar$ Y SVarCompare$ GEX).
		if blk.Cond.Kind != cAnd || len(blk.Cond.Sub) != 2 {
			t.Fatalf("%s CantBlock Cond = %+v, want a two-operand conjunction", c.Name, blk.Cond)
		}
		turn, cmpv := blk.Cond.Sub[0], blk.Cond.Sub[1]
		if turn.Kind != cYourTurn || !turn.Neg {
			t.Errorf("%s CantBlock first operand = %+v, want cYourTurn Neg", c.Name, turn)
		}
		if cmpv.Kind != cValue || !countCond(cmpv.A, cmpv.Op, cmpv.B, typ+".ActivePlayerCtrl") {
			t.Errorf("%s CantBlock second operand = %+v, want vCount(%s.ActivePlayerCtrl) >= vCount(%s.YouCtrl)",
				c.Name, cmpv, typ, typ)
		}
	}
	if seen != len(typeCounts) {
		t.Fatalf("lowered %d of %d cards", seen, len(typeCounts))
	}
}

// filterIs reports whether f is the single clause over the exact spec
// "<Type>.<Ctrl>": one type word, one controller selector, nothing else. An
// ActivePlayerCtrl clause carries cmpActiveCtrl instead of a controller
// selector word.
func filterIs(f *lFilter, expr string) bool {
	if f == nil || len(f.Clauses) != 1 {
		return false
	}
	head, ctrl, ok := strings.Cut(expr, ".")
	if !ok {
		return false
	}
	c := f.Clauses[0]
	switch ctrl {
	case "YouCtrl":
		if c.Ctrl != whoYou {
			return false
		}
	case "OppCtrl":
		if c.Ctrl != whoOpp {
			return false
		}
	case "ActivePlayerCtrl":
		if c.CmpStat != cmpActiveCtrl {
			return false
		}
	default:
		return false
	}
	if c.Subtype != "" || c.Name != "" || c.Flags != 0 || c.NotTypes != 0 || c.Colors != "" || c.Owner != whoAny {
		return false
	}
	bit, ok := typeBits[head]
	return ok && c.Types == bit
}
