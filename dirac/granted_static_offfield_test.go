package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A rule static an Animate grants lives on the granting card's face, not on
// the holder's permanent: the combat guards that skip on an empty statM must
// also open when a live override record can switch the static on
// (State.ovGateReach). Each case puts the granting card off the battlefield,
// so the record is the only thing that can reach it; the holder is a vanilla
// permanent whose own card carries no static, leaving statM empty.

// TestGrantedMustAttackOffField: Carnage, Crimson Chaos's "attacks each
// combat if able" static read on the creature it animated after Carnage
// itself has left the battlefield.
func TestGrantedMustAttackOffField(t *testing.T) {
	tbl := v.NewTable()
	src := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "off-field MustAttack grantor", Abilities: []kop.Ability{{
		Kind:   kop.AbStatic,
		Static: kop.Static{Kind: kop.StatMustAttack, Filter: kop.FilterID(1), OvGate: 1}, // vanilla fCreature
	}}})
	tbl.Overrides = append(tbl.Overrides, kop.Override{GrantStatics: 1})
	sc := newScenTable(t, tbl, Config{})
	o := sc.perm(v.GrizzlyBears, 0)
	sl := sc.slot(o)
	if sc.s.statM.any() {
		t.Fatal("precondition: a battlefield static would mask the record read")
	}
	sc.s.applyOverridePT(sc.k, sl, len(tbl.Overrides)-1, src, 0, 0, 0)
	if !sc.s.staticFlag(sc.k, kop.StatMustAttack, sl) {
		t.Fatal("precondition: the granted MustAttack static does not apply to the holder")
	}
	if !sc.s.MustAttack(o) {
		t.Error("MustAttack = false with a live granted MustAttack record and no battlefield static")
	}
}

// TestGrantedCantBlockByOffField: Chomping Kavu's Backup rider ("can't be
// blocked by creatures with power 2 or less") read on another creature after
// Kavu itself has left the battlefield.
func TestGrantedCantBlockByOffField(t *testing.T) {
	tbl := v.NewTable()
	src := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "off-field CantBlockBy grantor", Abilities: []kop.Ability{{
		Kind:   kop.AbStatic,
		Static: kop.Static{Kind: kop.StatCantBlockBy, Filter: kop.FilterID(1), Filter2: kop.FilterID(1), OvGate: 1}, // both vanilla fCreature
	}}})
	tbl.Overrides = append(tbl.Overrides, kop.Override{GrantStatics: 1})
	sc := newScenTable(t, tbl, Config{})
	a := sc.perm(v.GrizzlyBears, 0)
	b := sc.perm(v.HillGiant, 1)
	as, bs := sc.slot(a), sc.slot(b)
	if sc.s.statM.any() {
		t.Fatal("precondition: a battlefield static would mask the record read")
	}
	sc.s.applyOverridePT(sc.k, as, len(tbl.Overrides)-1, src, 0, 0, 0)
	if !sc.s.blockForbiddenGen(sc.k, bs, as) {
		t.Fatal("precondition: the granted CantBlockBy static does not forbid the block")
	}
	if sc.s.canBlock(sc.k, bs, as) {
		t.Error("canBlock = true with a live granted CantBlockBy record and no battlefield static")
	}
}
