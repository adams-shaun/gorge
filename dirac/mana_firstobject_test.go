package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// firstObjActTable is dynPayTable's pool table (with its dynamic {T},
// sacrifice petal source) plus two activators: a {1}: gain 1 life artifact
// and a Pay 2 life: gain 1 life artifact.
func firstObjActTable(t *testing.T) (tbl *kop.Table, lotus, manaAct, lifeAct CardID) {
	t.Helper()
	tbl, _, lotus, _, _, _ = dynPayTable(t)
	manaAct = CardID(len(tbl.Cards))
	lifeAct = manaAct + 1
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test First Mana Act", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind: kop.AbActivated, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}}, Prog: gain}}},
		kop.CardDef{Name: "Test First Life Act", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostPayLife, N: 2}, Prog: gain}}},
	)
	return tbl, lotus, manaAct, lifeAct
}

// firstObjScen puts object 0 (seat 0's first library card, card c) onto seat
// 0's battlefield and a second copy of c beside it.
func firstObjScen(t *testing.T, tbl *kop.Table, c CardID) (*scen, Obj) {
	t.Helper()
	sc := newScenTableLib(t, tbl, c, Config{NoElide: true})
	const first = Obj(0)
	if ob := &sc.s.objs[first]; ob.card != c || ob.owner() != 0 {
		t.Fatalf("precondition: object 0 is card %d owner %d, want seat 0's card %d", ob.card, ob.owner(), c)
	}
	sc.s.detach(first)
	sc.s.enter(sc.k, first, 0, false, kop.ZoneNone)
	sc.s.perms[sc.slot(first)].flags &^= pfSick
	sc.s.npend = 0
	other := sc.perm(c, 0)
	if !sc.onBF(first) || !sc.onBF(other) || other == first {
		t.Fatalf("precondition: object 0 and object %d are not both on the battlefield", other)
	}
	return sc, other
}

// TestManaActivationFromFirstObject: object 0 is a card instance like any
// other (the kernel's no-object sentinel is NoObj), so an activation whose
// source is object 0 is priced as on any other object: the dynamic {T},
// sacrifice source's prefix-script pricing (dynSacScriptPriced, CR 601.2g,
// 605.1a) and the pay-life survival check (playLifeNotSurvived, CR 119.4).
func TestManaActivationFromFirstObject(t *testing.T) {
	tbl, lotus, manaAct, lifeAct := firstObjActTable(t)

	// {1} activation paid only by the petal source (two petals)
	sc, other := firstObjScen(t, tbl, manaAct)
	l := sc.perm(lotus, 0)
	sc.s.perms[sc.slot(l)].setCtrN(sc.s, int(kop.CtrPetal), 2)
	if c := sc.s.cardAt(sc.slot(l)); sc.k.gen.flags[c]&gfManaVar == 0 {
		t.Fatal("precondition: the petal source is not a dynamic-amount source")
	}
	if !sc.s.dynSacScriptPriced(sc.k, MakeAction(AkActivate, TCode(other), 0, 0), 0) {
		t.Fatalf("precondition: object %d's {1} activation is not priced through the petal source", other)
	}
	if !sc.s.dynSacScriptPriced(sc.k, MakeAction(AkActivate, 0, 0, 0), 0) {
		t.Fatal("object 0's {1} activation is not priced through the petal source")
	}

	// Pay 2 life at 2 life
	sc, other = firstObjScen(t, tbl, lifeAct)
	sc.s.pl[0].life = 2
	if !sc.s.playLifeNotSurvived(sc.k, 0, MakeAction(AkActivate, TCode(other), 0, 0)) {
		t.Fatalf("precondition: object %d's Pay 2 life at 2 life is survived", other)
	}
	if !sc.s.playLifeNotSurvived(sc.k, 0, MakeAction(AkActivate, 0, 0, 0)) {
		t.Fatal("object 0's Pay 2 life at 2 life is survived")
	}
}
