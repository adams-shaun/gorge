package kop

import "testing"

// TestFeeOnlyManaAbShapes: the fee-only mana ability shapes (Nomadic Elf,
// Orochi Leafcaller, Agent of Stromgald, Bog Initiate, Energy Refractor) are
// admitted, and the neighbours that stay out are not.
func TestFeeOnlyManaAbShapes(t *testing.T) {
	mk := func(generic, pip uint8, pipCol int, mut func(*Ability)) *Ability {
		a := &Ability{Kind: AbMana, ManaColors: ColAny, ManaN: 1}
		a.Cost.Mana.Generic = generic
		if pip != 0 {
			a.Cost.Mana.Pips[pipCol] = pip
		}
		if mut != nil {
			mut(a)
		}
		return a
	}
	accept := map[string]*Ability{
		"{G}->Any":    mk(0, 1, 4, nil),
		"{1}{G}->Any": mk(1, 1, 4, nil),
		"{R}->{B}":    mk(0, 1, 3, func(a *Ability) { a.ManaColors = ColB }),
		"{1}->{B}":    mk(1, 0, 0, func(a *Ability) { a.ManaColors = ColB }),
		"{2}->Any":    mk(2, 0, 0, nil),
		"{1}->Combo R W": mk(1, 0, 0, func(a *Ability) {
			a.AFlags, a.ManaColors = AfManaChoice, ColR|ColW
		}),
	}
	for name, a := range accept {
		if !FeeOnlyManaAb(a) {
			t.Errorf("%s: FeeOnlyManaAb false, want true", name)
		}
		if !FilterManaAb(a) {
			t.Errorf("%s: FilterManaAb false, want true", name)
		}
	}
	reject := map[string]*Ability{
		"{T}+{1}":     mk(1, 0, 0, func(a *Ability) { a.Cost.Flags = CostTap }),
		"{1}{G}{G}":   mk(1, 2, 4, nil),
		"{C} pip":     mk(0, 1, C, nil),
		"{X}":         mk(0, 0, 0, func(a *Ability) { a.Cost.Mana.X = 1 }),
		"hybrid":      mk(0, 0, 0, func(a *Ability) { a.Cost.Mana.Hyb[0] = ColW | ColU }),
		"ManaN 2":     mk(1, 0, 0, func(a *Ability) { a.ManaN = 2 }),
		"restricted":  mk(1, 0, 0, func(a *Ability) { a.ManaRestrict = 1 }),
		"Prog":        mk(1, 0, 0, func(a *Ability) { a.Prog = []Instr{{}} }),
		"free":        mk(0, 0, 0, nil),
		"two colours": mk(1, 0, 0, func(a *Ability) { a.ManaColors = ColR | ColW }),
	}
	for name, a := range reject {
		if FeeOnlyManaAb(a) {
			t.Errorf("%s: FeeOnlyManaAb true, want false", name)
		}
	}
	if !FilterManaAb(reject["{T}+{1}"]) {
		t.Errorf("{T}+{1} stays FilterManaAb's tap arm")
	}
}
