package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestWindowGrantedManaLockedByCantAct: Clarion Conqueror forbids the
// opponent's creatures' activated abilities, mana abilities included
// (ValidSA$ Activated), so a creature whose only mana ability is a
// Continuous grant is no source for the window walk. windowGrantPC must
// read the same lock manaPCGen reads (cast_cantactivate.go), or
// windowCanTap / windowDry tap a permanent gorge withholds.
func TestWindowGrantedManaLockedByCantAct(t *testing.T) {
	set := cardsettest.Set(t, "cantact")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the cantact pool", name)
		}
		return CardID(set.PoolID(i))
	}
	// The grant, for the test: "Creatures you control have '{T}: Add one
	// mana of any color.'" (Cryptolith Rite's shape).
	creatures := filter(kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rite", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: creatures, Grant: 2}},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 1},
	}})
	plains := id("Plains")
	sc := newScenTableLib(t, &tbl, plains, Config{NoElide: true})
	if !sc.k.gen.anyCantAct {
		t.Fatal("precondition: cantact does not arm anyCantAct")
	}
	sc.perm(rite, 0)
	bears := sc.perm(bear, 0)
	sl := sc.slot(bears)
	// The grant is a usable source while nothing locks it.
	if !sc.s.windowCanTap(sc.k, sl) {
		t.Fatal("precondition: the granted mana ability is not a window source")
	}
	sc.perm(id("Clarion Conqueror"), 1)
	if !sc.s.actForbidden(sc.k, sl, true) {
		t.Fatal("precondition: Clarion Conqueror does not forbid the granted mana ability")
	}
	if sc.s.windowCanTap(sc.k, sl) {
		t.Error("window taps a granted mana ability a CantBeActivated lock forbids")
	}
	if pc := sc.s.windowGrantPC(sc.k, sl); pc.manaAb >= 0 {
		t.Errorf("windowGrantPC returns a mana ability (%d) a CantBeActivated lock forbids", pc.manaAb)
	}
}

// TestWindowDryUsesAdmittedGrant: a source with two granted tap abilities
// whose first the payment refuses and whose later one it admits. The dry
// run must simulate the admitted ability the real window taps, not the
// first grant (manaPCGen), or a payable cost reads stranded and the cast
// is reversed. windowOwnMask, read by windowCol's no-pip fallback, must
// follow the same view.
func TestWindowDryUsesAdmittedGrant(t *testing.T) {
	tbl := *v.Table
	// a generated table (prepareGen returns nil for a hand-written one)
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rite", Types: kop.TEnchantment, Abilities: []kop.Ability{
		// The payment here is neither a spell nor an activated ability
		// (payDescNow without a build), so a restricted grant is refused
		// and the unrestricted {G}{G} grant admitted.
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColR, ManaN: 1, ManaRestrict: creatures},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 2},
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: creatures, Grant: 1}},
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: creatures, Grant: 2}},
	}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(rite, 0)
	bears := sc.perm(bear, 0)
	sl := sc.slot(bears)
	if !sc.s.windowCanTap(sc.k, sl) {
		t.Fatalf("precondition: the admitted grant is not a window source (grantPC %d anyGrant %v grantMay %v own %v)",
			len(sc.k.gen.grantPC), sc.k.gen.anyGrant, sc.s.grantMay(sc.k, sl), sc.s.ownManaUsable(sc.k, sl))
	}
	if first := sc.s.manaPCGen(sc.k, sl); first.manaN != 1 || srcMask(first) != 1<<kop.R {
		t.Fatalf("precondition: manaPCGen is not the refused first grant (n %d mask %#x)", first.manaN, srcMask(first))
	}
	pc := sc.s.windowGrantPC(sc.k, sl)
	if pc.manaN != 2 || srcMask(pc) != 1<<kop.G {
		t.Fatalf("windowGrantPC reads n %d mask %#x, want the admitted {G}{G} grant", pc.manaN, srcMask(pc))
	}
	if m := sc.s.windowOwnMask(sc.k, sl); m != 1<<kop.G {
		t.Errorf("windowOwnMask %#x, want the admitted grant's %#x", m, 1<<kop.G)
	}
	c := cost(2)
	ok, known := sc.s.windowDry(sc.k, 0, &c, false)
	if !ok || !known {
		t.Errorf("windowDry pays {2}: ok %v known %v, want true true", ok, known)
	}
}
