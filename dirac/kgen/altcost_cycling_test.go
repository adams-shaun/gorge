package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCyclingAltCostLowers: New Perspectives' "As long as you have seven or
// more cards in hand, you may pay {0} rather than pay cycling costs"
// (S:Mode$ AlternativeCost | ValidSA$ Activated.Cycling) lowers as a
// kop.StatCyclingAlt static gated on the hand count, and every cycling
// ability of the same table gains one latent-priced twin (CycAltOf) whose
// cost is the discard alone (gorge's static Cost$ Discard<1/CARDNAME>
// replaces the printed cost, CR 118.9).
func TestCyclingAltCostLowers(t *testing.T) {
	all, bs := keclBuild(t, []string{"New Perspectives", "Street Wraith", "Lonely Sandbar", "Jhessian Zombies"})
	byName := map[string]int{}
	for j, c := range all {
		byName[c.Name] = j
	}
	np, ok := byName["New Perspectives"]
	if !ok {
		t.Fatal("New Perspectives: not lowered at all")
	}
	if r := lowerRefusals(all[np]); len(r) != 0 {
		for _, x := range r {
			if strings.Contains(x.Reason, "AlternativeCost shape") {
				t.Fatalf("New Perspectives still refused as an AlternativeCost shape: %v", r)
			}
		}
		t.Fatalf("New Perspectives refused: %v", r)
	}
	found := false
	for _, a := range bs[np].def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCyclingAlt && a.Static.Cond2 != 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("New Perspectives: no hand-count gated StatCyclingAlt static")
	}
	twins := 0
	for _, name := range []string{"Street Wraith", "Lonely Sandbar", "Jhessian Zombies"} {
		j, ok := byName[name]
		if !ok {
			continue
		}
		abs := bs[j].def.Abilities
		for i := range abs {
			a := &abs[i]
			if a.CycAltOf == 0 {
				continue
			}
			twins++
			src := &abs[a.CycAltOf-1]
			if src.AFlags&kop.AfCycling == 0 || a.AFlags&kop.AfCycling == 0 || a.ActZone != kop.ZoneHand {
				t.Errorf("%s: twin %d of %d is not a hand cycling ability", name, i, a.CycAltOf-1)
			}
			if a.Cost.Flags != kop.CostDiscardSelf || a.Cost.Mana != (kop.ManaCost{}) || a.Cost.N != 0 {
				t.Errorf("%s: twin cost = %+v, want the discard alone", name, a.Cost)
			}
		}
	}
	if twins != 4 { // Street Wraith 1, Lonely Sandbar 1, Jhessian Zombies 2
		t.Fatalf("cycling twins built beside New Perspectives = %d, want 4", twins)
	}
	// a pool without the static builds no twin
	all2, bs2 := keclBuild(t, []string{"Street Wraith"})
	for _, a := range bs2[0].def.Abilities {
		if a.CycAltOf != 0 {
			t.Fatalf("%s: a twin without the static in the pool", all2[0].Name)
		}
	}
}
