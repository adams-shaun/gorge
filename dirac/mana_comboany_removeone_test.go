package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestComboAnyPickSparesOneColourEntry: removeCA takes only the group units
// availCA added, never a one[] entry. A waterbend or convoke pick on the Combo
// Any source taps it for its amount of any-colour mana, and availXWindow counts
// that tap in the same 0x1f group a manaOne source's census rides ("{T}: Add
// two mana of any one color": addUnits(0x1f, n) plus addOneColour(a, n) at the
// same amount, mana.go availFor). removeCA removing through removeWide ran
// dropOneColour(0x1f, n), which matches the manaOne source's entry and deletes
// it: oneColourPays loses the source and the X ask prices the board against
// gorge. The gap's own census adds no one[] entry, so the removal is
// group-only (takeWide).
func TestComboAnyPickSparesOneColourEntry(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	ca := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Combo Any", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, AFlags: kop.AfManaChoice, ManaColors: kop.ColAny, ManaN: 2, Cost: kop.Cost{Flags: kop.CostTap}},
	}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.gen.anyComboAny || !k.isComboAny(ca) {
		t.Fatal("precondition: the probe's two-mana any-combination ability is not the Combo Any census gap")
	}
	sl := sc.slot(sc.perm(ca, 0))
	s.perms[sl].flags &^= pfTapped
	if !s.caGaps(k, 0).has(sl) {
		t.Fatal("precondition: the Combo Any source is not an activatable gap")
	}

	// The X ask's census: availXWindow counts the gap's two units
	// (availCA), and a manaOne source's census contribution rides the same
	// group at the same amount (mana.go availFor: addUnits(0x1f, n) plus
	// addOneColour(a, n)).
	var av avail
	s.availXWindow(k, 0, &av)
	av.addUnits(uint8(kop.ColAny), 2)
	s.addOneColour(&av, 2)
	if av.nOne != 1 || av.one[0] != 2 {
		t.Fatalf("precondition: the census holds %d one-colour entries (first amount %d), want one of two", av.nOne, av.one[0])
	}
	n0 := av.total

	s.removeCA(k, &av, sl)
	if av.nOne != 1 || av.one[0] != 2 {
		t.Fatalf("removeCA deleted the manaOne source's one-colour entry: %d entries left (first %d)", av.nOne, av.one[0])
	}
	if av.total != n0-2 {
		t.Fatalf("removeCA took %d units off the census, want the gap's two", n0-av.total)
	}

	// The manaOne source still pays as one colour: its two units cannot cover
	// two different pips.
	var cost kop.ManaCost
	cost.Pips[kop.U], cost.Pips[kop.R] = 1, 1
	if av.affords(&cost, pipMaskOf(&cost), 0) {
		t.Error("after the pick the manaOne source's two units read independent and paid {U}{R}")
	}
}
