package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// kmana2: a permanent with several plain fixed-colour mana abilities (a
// dual land's intrinsic {T}: Add {B} and {T}: Add {R}) is one deterministic
// unit producing the union of its colours.

func tapMana(col kop.ColorMask) kop.Ability {
	return kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: col, ManaN: 1}
}

// kmana2Table is vanilla plus a dual (B, R), a triome (U, R, G), a gain
// land shaped Combo source (one ability, B or R) and a Nightshade
// Dryad-shaped land ({C} beside an any-colour ability: not folded).
func kmana2Table() (tbl *kop.Table, dual, triome, gain, dryad CardID) {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	add := func(name string, abs ...kop.Ability) CardID {
		t.Cards = append(t.Cards, kop.CardDef{Name: name, Types: kop.TLand, Abilities: abs})
		return CardID(len(t.Cards) - 1)
	}
	dual = add("Test Badlands", tapMana(kop.ColB), tapMana(kop.ColR))
	triome = add("Test Ketria Triome", tapMana(kop.ColU), tapMana(kop.ColR), tapMana(kop.ColG))
	gain = add("Test Gain Land", tapMana(kop.ColB|kop.ColR))
	dryad = add("Test Dryad Land", tapMana(kop.ColC), tapMana(kop.ColAny))
	return &t, dual, triome, gain, dryad
}

func TestMultiManaPrep(t *testing.T) {
	tbl, dual, triome, gain, dryad := kmana2Table()
	k := prepare(tbl)
	for _, c := range []struct {
		id    CardID
		col   kop.ColorMask
		multi bool
	}{{dual, kop.ColB | kop.ColR, true}, {triome, kop.ColU | kop.ColR | kop.ColG, true}, {gain, kop.ColB | kop.ColR, false}, {dryad, kop.ColC, false}} {
		pc := &k.cards[c.id]
		if pc.manaCol != c.col || pc.manaMulti != c.multi || pc.manaAb != 0 {
			t.Errorf("%s: manaCol %#x multi %v ab %d; want %#x %v 0", tbl.Cards[c.id].Name, pc.manaCol, pc.manaMulti, pc.manaAb, c.col, c.multi)
		}
	}
}

// The dual's second ability pays: {R} from a lone Badlands (the first
// ability alone could not), and {U}{R}{G} from three triomes.
func TestMultiManaPays(t *testing.T) {
	tbl, dual, triome, _, _ := kmana2Table()
	sc := newScenTable(t, tbl, Config{})
	b := sc.perm(dual, 0)
	var av avail
	c := cost(0, kop.R)
	sc.s.availFor(sc.k, 0, &av)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		t.Fatal("a Badlands does not afford {R}")
	}
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(b) || sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("Badlands tapped %v pool %v", sc.tapped(b), sc.s.pl[0].pool)
	}
	sc = newScenTable(t, tbl, Config{})
	var ts [3]Obj
	for i := range ts {
		ts[i] = sc.perm(triome, 0)
	}
	c = cost(0, kop.U, kop.R, kop.G)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	for i, o := range ts {
		if !sc.tapped(o) {
			t.Fatalf("triome %d untapped", i)
		}
	}
}

// gorge's census lists a multi-ability source with the fixed-production
// units, in battlefield order, and a choice-shaped one (the gain land)
// after them: the dual is unit 0 though the gain land entered first.
func TestMultiManaUnitOrder(t *testing.T) {
	tbl, dual, _, gain, _ := kmana2Table()
	sc := newScenTable(t, tbl, Config{})
	g := sc.perm(gain, 0)
	d := sc.perm(dual, 0)
	var pk planner
	pk.need[kop.B], pk.needT = 1, 2
	sc.s.plan(sc.k, 0, &pk, NoObj)
	if pk.nu != 2 || pk.units[0] != sc.slot(d) || pk.units[1] != sc.slot(g) {
		t.Fatalf("units %v (dual slot %d, gain slot %d)", pk.units[:pk.nu], sc.slot(d), sc.slot(g))
	}
	if pk.types[pk.utyp[0]].flex != 2 {
		t.Fatalf("dual flex %d, want 2 (gorge Alt.Flex: distinct types over its alternatives)", pk.types[pk.utyp[0]].flex)
	}
}

// The payment window's "choose a mana ability" ask lists what the cost owes first
// (gorge owedAbilitiesFirst; the oracle takes its first option), so a Badlands
// makes the owed {R}; a gain land's flattened colour options do too.
func TestMultiManaWindow(t *testing.T) {
	tbl, dual, _, gain, _ := kmana2Table()
	c := cost(0, kop.R)
	sc := newScenTable(t, tbl, Config{})
	b := sc.perm(dual, 0)
	m := sc.perm(v.Mountain, 0)
	if ok, _ := sc.s.windowTap(sc.k, 0, &c, false); !ok {
		t.Fatal("window did not pay")
	}
	if !sc.tapped(b) || sc.tapped(m) || sc.s.pl[0].pool[kop.R] != 1 {
		t.Fatalf("dual: tapped %v %v pool %v; want the dual alone, {R} floating", sc.tapped(b), sc.tapped(m), sc.s.pl[0].pool)
	}
	sc = newScenTable(t, tbl, Config{})
	gl := sc.perm(gain, 0)
	m = sc.perm(v.Mountain, 0)
	if ok, _ := sc.s.windowTap(sc.k, 0, &c, false); !ok || !sc.tapped(gl) || sc.tapped(m) || sc.s.pl[0].pool[kop.R] != 1 {
		t.Fatalf("gain land: tapped %v %v pool %v; want the gain land's {R}", sc.tapped(gl), sc.tapped(m), sc.s.pl[0].pool)
	}
}
