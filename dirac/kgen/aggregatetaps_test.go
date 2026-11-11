package kgen

import (
	"strings"
	"testing"
)

// TestAggregateTaps: Crew and Saddle lower from gorge's
// expansion (one AB$ whose tapXType<Any/...+withTotalPowerGE<N>> cost is
// TapPowN / TapPowF), the Saddle body is OpSaddle, Card.Self+IsSaddled is
// the F2Saddled flag, and Exhaust$ True is an activate-only-once limit. The
// refused shapes: a Crewed trigger, the tapXType X form. A card with two or
// three activate-only-once abilities lowers with ordinals
// (TestOnceAbilityOrdinals).
func TestAggregateTaps(t *testing.T) {
	floors := map[string]int{"Sleek Schooner": 1, "Smuggler's Copter": 1, "Bridled Bighorn": 2, "Seraphic Steed": 4}
	exhaust := map[string]bool{"Skystreak Engineer": true, "Rocketeer Boostbuggy": true}
	refuse := map[string]string{
		"Speedway Fanatic":  "trigger mode Crewed",
		"Burn at the Stake": "tapXType<X/Creature>",
	}
	var names []string
	for _, m := range []map[string]int{floors} {
		for n := range m {
			names = append(names, n)
		}
	}
	for n := range exhaust {
		names = append(names, n)
	}
	for n := range refuse {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		if want, ok := refuse[c.Name]; ok {
			seen++
			hit := false
			for _, r := range c.Refusals {
				hit = hit || strings.Contains(r.Node+" "+r.Reason, want)
			}
			if !hit {
				t.Errorf("%s: no %q refusal (%v)", c.Name, want, c.Refusals)
			}
			continue
		}
		seen++
		if len(c.Refusals) > 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
			continue
		}
		if n, ok := floors[c.Name]; ok {
			got := 0
			saddle := false
			for _, a := range c.Abilities {
				if a.Cost.TapPowN != 0 {
					got = a.Cost.TapPowN
					if a.Cost.TapPowF == nil || len(a.Cost.TapPowF.Clauses) != 1 || a.Cost.TapPowF.Clauses[0].Flags&(fOther|fUntapped) != fOther|fUntapped {
						t.Errorf("%s: tap filter %v", c.Name, a.Cost.TapPowF)
					}
				}
				for _, in := range a.Prog {
					saddle = saddle || in.Op == "Saddle"
				}
			}
			if got != n {
				t.Errorf("%s: floor %d, want %d", c.Name, got, n)
			}
			if strings.Contains(c.Name, "Bighorn") != saddle && strings.Contains(c.Name, "Steed") != saddle {
				t.Errorf("%s: OpSaddle %v", c.Name, saddle)
			}
		}
		if exhaust[c.Name] {
			once := 0
			for _, a := range c.Abilities {
				if a.Limit == 1 {
					once++
				}
			}
			if once != 1 {
				t.Errorf("%s: %d activate-only-once abilities, want 1", c.Name, once)
			}
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d cards", seen, len(names))
	}
}

// TestTapXTypeSubtypeAnyPermanent: a subtype-only tapXType spec is any
// permanent of the subtype (gorge TapCostCandidates), not only a creature:
// Spire Mechcycle taps another Mount or Vehicle.
func TestTapXTypeSubtypeAnyPermanent(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Spire Mechcycle", "Bridled Bighorn", "Sleek Schooner"}) {
		if c.Name != "Spire Mechcycle" {
			continue
		}
		if len(c.Refusals) > 0 {
			t.Fatalf("refused: %v", c.Refusals)
		}
		for _, a := range c.Abilities {
			if a.Cost.TapN == 0 {
				continue
			}
			for _, cl := range a.Cost.TapF.Clauses {
				if cl.Types != 0 {
					t.Errorf("clause %v: types %#x, want none (a subtype only)", cl, cl.Types)
				}
			}
			return
		}
		t.Fatal("no tapXType cost")
	}
}
