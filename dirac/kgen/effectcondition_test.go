package kgen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEffectConditionGateLowers: a no-target Effect whose own Condition*$
// keys (ConditionPresent$, Condition$ Threshold, ConditionCheckSVar$) lower
// clean: the unread check in effectStaticEOT always drops the Effect's own
// Condition prefix, and the node's condition read (nodeCondition,
// lower_effect.go) runs afterwards, puts a SkipIf over the node alone and
// refuses any key it does not read — as gorge's sub-ability conditions do
// (engine/effects/conditions.go). The three cards this covers are Barrage of
// Boulders and Wild Slash ("Ferocious — …") and Demoralize ("Threshold —
// …"); each of their Effect nodes lowers to one emblem whose node program
// begins with the SkipIf, with the non-Effect nodes (Barrage's DamageAll,
// Demoralize's PumpAll, Wild Slash's DBDmg) outside the guard.
func TestEffectConditionGateLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Barrage of Boulders\nDemoralize\nWild Slash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		byName[c.Name] = c
	}
	statics := map[string]string{"Barrage of Boulders": "CantBlock", "Demoralize": "CantBlock", "Wild Slash": "CantPreventDamage"}
	for _, n := range []string{"Barrage of Boulders", "Demoralize", "Wild Slash"} {
		c := byName[n]
		if c == nil || len(c.Refusals) != 0 {
			t.Fatalf("%s: %+v, want lowered clean (refusals %v)", n, c, refusalsOf(c))
		}
		if len(c.Emblems) != 1 {
			t.Fatalf("%s: %d emblems, want 1", n, len(c.Emblems))
		}
		// The Effect node's program begins with the SkipIf nodeCondition
		// placed: some instruction is a SkipIf whose target is the node's
		// CreateEmblem. A Condition key nodeCondition does not read would
		// refuse before this point.
		if !progGuardsCreateEmblem(c) {
			t.Errorf("%s: no SkipIf in front of the Effect's CreateEmblem", n)
		}
		if e := c.Emblems[0]; len(e.Statics) != 1 || e.Statics[0].Kind != statics[n] {
			t.Errorf("%s: emblem = %+v, want one %s static", n, e, statics[n])
		}
	}
	// Wild Slash's Effect is the root SP$ Effect: its SkipIf covers the node
	// alone, so the SubAbility$ DBDmg node (2 damage to any target) still
	// resolves when the condition fails — the DealDamage instruction is not
	// behind the guard.
	w := byName["Wild Slash"]
	for _, a := range w.Abilities {
		for i, in := range a.Prog {
			if in.Op == "DealDamage" && i > 0 && a.Prog[i-1].Op == "SkipIf" {
				t.Errorf("Wild Slash: DealDamage sits behind a SkipIf guard: %v", a.Prog[max(0, i-3):i+1])
			}
		}
	}
}

// progGuardsCreateEmblem reports whether any ability program places a
// SkipIf instruction immediately in front of a CreateEmblem (the Effect
// node's guard covers the node alone, from its first instruction).
func progGuardsCreateEmblem(c *lCard) bool {
	for _, a := range c.Abilities {
		for i, in := range a.Prog {
			if in.Op == "CreateEmblem" && i > 0 && a.Prog[i-1].Op == "SkipIf" {
				return true
			}
		}
	}
	return false
}

func refusalsOf(c *lCard) []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Refusals))
	for _, rf := range c.Refusals {
		out = append(out, rf.Reason)
	}
	return out
}
