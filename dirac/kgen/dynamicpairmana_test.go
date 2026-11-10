package kgen

import "testing"

// TestDynamicLargerSameColourMana: a plain {T}: Add {G} then a plain {T}: Add
// {G} for each creature you control (Itlimoc, Cradle of the Sun) is the pair
// the kernel's per-count view plays (mana_dynpair.go). Each negative differs
// from the accepted card in exactly one field and stays refused beside the
// first ability: another colour, an amount that is not a plain count of
// battlefield permanents or reads tapped state, a first ability of two mana,
// a restricted evaluated ability, a third mana ability, and the evaluated
// ability printed first.
func TestDynamicLargerSameColourMana(t *testing.T) {
	plain := func(col string, n int) *lAbility {
		return &lAbility{Kind: akMana, Cost: lCost{Tap: true}, Mana: &lMana{Colors: col, Amount: lValue{Kind: vConst, N: n}}}
	}
	count := func(col string, mod func(*lValue)) *lAbility {
		a := plain(col, 0)
		a.Mana.Amount = lValue{Kind: vCount, F: &lFilter{Clauses: []lClause{{Types: 1 << 1, Ctrl: whoYou}}}}
		if mod != nil {
			mod(&a.Mana.Amount)
		}
		return a
	}
	cases := []struct {
		name string
		ms   []*lAbility
		want bool
	}{
		{"Itlimoc", []*lAbility{plain("G", 1), count("G", nil)}, true},
		{"another colour", []*lAbility{plain("G", 1), count("U", nil)}, false},
		{"aggregate amount", []*lAbility{plain("G", 1), count("G", func(v *lValue) { v.Kind = vValidAgg })}, false},
		{"graveyard count", []*lAbility{plain("G", 1), count("G", func(v *lValue) { v.Kind = vCountGraveyard })}, false},
		{"counted tapped creatures", []*lAbility{plain("G", 1), count("G", func(v *lValue) { v.F.Clauses[0].Flags |= fTapped })}, false},
		{"counted untapped creatures", []*lAbility{plain("G", 1), count("G", func(v *lValue) { v.F.Clauses[0].Flags |= fUntapped })}, false},
		{"scaled count", []*lAbility{plain("G", 1), count("G", func(v *lValue) { v.Mul = 2 })}, false},
		{"first ability of two", []*lAbility{plain("G", 2), count("G", nil)}, false},
		{"colourless", []*lAbility{plain("C", 1), count("C", nil)}, false},
		{"restricted evaluated ability", []*lAbility{plain("G", 1), func() *lAbility {
			a := count("G", nil)
			a.Mana.Restrict = &lFilter{Clauses: []lClause{{Types: 1}}}
			return a
		}()}, false},
		{"evaluated ability first", []*lAbility{count("G", nil), plain("G", 1)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dynamicLargerSameColour(c.ms[0], c.ms[1]); got != c.want {
				t.Errorf("dynamicLargerSameColour = %v, want %v", got, c.want)
			}
			if got := manaGapModelled(c.ms[0], c.ms[1]); c.want && !got {
				t.Errorf("manaGapModelled = false for the accepted shape")
			}
		})
	}
	// a third mana ability beside the pair is no pair: refuseMultiMana's
	// len(ms)==2 gate
	lc := &lowerCtx{out: &lCard{Name: "Synthetic", Abilities: []*lAbility{plain("G", 1), count("G", nil), plain("R", 1)}}}
	lc.refuseMultiMana()
	if len(lc.out.Refusals) == 0 {
		t.Errorf("three mana abilities (plain, count, another colour) lowered clean")
	}
	lc = &lowerCtx{out: &lCard{Name: "Synthetic", Abilities: []*lAbility{plain("G", 1), count("G", nil)}}}
	lc.refuseMultiMana()
	if len(lc.out.Refusals) != 0 {
		t.Errorf("the pair was refused: %v", lc.out.Refusals)
	}
}
