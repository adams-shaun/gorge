package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// grantRestrScen builds a generated table with one source whose only mana
// ability is ab (restricted by a "you control an artifact" filter unless the
// caller set one) beside a static grant of a {T} mana ability of grantColors
// to its controller's creatures. It places the grant and then the source, and
// returns the source's slot.
func grantRestrScen(t *testing.T, ab kop.Ability, grantColors kop.ColorMask) (*scen, Slot) {
	t.Helper()
	tbl := *v.Table
	// a generated table (prepareGen returns nil for a hand-written one)
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters,
		kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou},
		kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield},
	)
	artifacts := kop.FilterID(len(tbl.Filters) - 2)
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	if ab.ManaRestrict == 0 {
		ab.ManaRestrict = artifacts
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Restricted Source", Types: kop.TCreature | kop.TArtifact,
		Power: 1, Toughness: 1, Abilities: []kop.Ability{ab},
	})
	grant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Granted Mana", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: creatures, Grant: 2}},
			{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: grantColors, ManaN: 1},
		},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(grant, 0)
	return sc, sc.slot(sc.perm(src, 0))
}

// TestGrantRestrOwnFirstLeavesDynamicToDynWalk: a printed restricted ability
// whose amount is dynamic (Helga's evaluated N) beside an any-colour grant is
// not the fixed-colour ability the new ordering takes. kgen writes ManaN = 1
// for every dynamic ability, so taking it here would float 1 instead of the
// evaluated N; the pre-hunk walk leaves it to windowRestrDyn, which floats
// the evaluated amount. A generic-only payment owes no fixed pip, the shape
// that sends the fixed {C} source down grantRestrOwnFirst.
func TestGrantRestrOwnFirstLeavesDynamicToDynWalk(t *testing.T) {
	ab := kop.Ability{
		Kind:       kop.AbMana,
		AFlags:     kop.AfManaDynamic | kop.AfManaNonSpell,
		Cost:       kop.Cost{Flags: kop.CostTap},
		ManaColors: kop.ColU,
		ManaN:      1, // kgen's placeholder, not the evaluated amount
		ManaV:      kop.Const(3),
	}
	sc, sl := grantRestrScen(t, ab, kop.ColAny)
	s := sc.s
	if s.ownRestrAb(sc.k, sl) == nil {
		t.Fatal("precondition: the source has no printed restricted ability")
	}
	if s.ownManaUsable(sc.k, sl) {
		t.Fatal("precondition: the source has a usable own ability, not the grant-only shape")
	}
	if len(sc.k.gen.grantPC) == 0 || !s.grantMay(sc.k, sl) || s.windowGrantMask(sc.k, sl)&0x1f == 0 {
		t.Fatal("precondition: the source is not under the any-colour grant")
	}
	c := cost(1)
	if _, _, ok := s.windowRestrOwed(sc.k, 0, sl, &c); ok {
		t.Fatal("windowRestrOwed takes a dynamic restricted ability ahead of the grant")
	}
	if !s.windowRestrDyn(sc.k, 0, &c, sl) {
		t.Fatal("windowRestrDyn did not take the dynamic restricted source")
	}
	if n := s.restrOf(0, int(kop.U)); n != 3 {
		t.Fatalf("the dynamic source floated %d restricted {U}, want the evaluated 3 (not ManaN's 1)", n)
	}
}

// TestGrantRestrOwnFirstKeepsFixedPipBesideHybrid: a flexible pip leaves
// windowUseful undecided, but gorge's owedAbilitiesFirst still sorts by the
// fixed pips the payment owes (paymentUsefulColours' demand outlives its
// decided flag). With a fixed {U} owed and a {U/R} hybrid, a grant that makes
// {U} must sort ahead of a printed restricted {C}: this branch must not read
// the undecided clamp as "no fixed pip owed" and take the {C}.
func TestGrantRestrOwnFirstKeepsFixedPipBesideHybrid(t *testing.T) {
	ab := kop.Ability{
		Kind:       kop.AbMana,
		Cost:       kop.Cost{Flags: kop.CostTap},
		ManaColors: kop.ColC,
		ManaN:      1,
	}
	sc, sl := grantRestrScen(t, ab, kop.ColU)
	s := sc.s
	c := cost(0, kop.U)
	c.Hyb[0] = kop.ColU | kop.ColR
	if _, decided := s.windowUseful(sc.k, 0, &c); decided {
		t.Fatal("precondition: the hybrid pip did not leave the clamp undecided")
	}
	if w := s.windowDemand(sc.k, 0, &c); w != 1<<kop.U {
		t.Fatalf("precondition: fixed demand %#x, want the owed {U}", w)
	}
	if _, _, ok := s.windowRestrOwed(sc.k, 0, sl, &c); ok {
		t.Fatal("windowRestrOwed takes the printed {C} while the grant owes {U}")
	}
}

// TestGrantRestrOwnFirstLeavesAnyColourToAdmission: a printed restricted
// any-colour ability beside a grant keeps windowRestrOwed's admission walk,
// not the fixed single-colour ordering: the ask offers it only when the
// payment's restriction admits it. The fixed {U} sits beside a hybrid pip, so
// the admission walk also needs windowDemand's fixed demand (windowUseful
// leaves the clamp undecided), or it bails on w == 0 first.
func TestGrantRestrOwnFirstLeavesAnyColourToAdmission(t *testing.T) {
	unadmitted := kop.Ability{
		Kind:       kop.AbMana,
		Cost:       kop.Cost{Flags: kop.CostTap},
		ManaColors: kop.ColAny,
		ManaN:      1,
	}
	admitted := unadmitted
	admitted.AFlags = kop.AfManaNonSpell
	for _, tc := range []struct {
		name string
		ab   kop.Ability
		ok   bool
	}{
		{"unadmitted", unadmitted, false},
		{"admitted", admitted, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, sl := grantRestrScen(t, tc.ab, kop.ColW)
			s := sc.s
			costU := cost(0, kop.U)
			costU.Hyb[0] = kop.ColU | kop.ColR
			if _, decided := s.windowUseful(sc.k, 0, &costU); decided {
				t.Fatal("precondition: the hybrid pip did not leave the clamp undecided")
			}
			if w := s.windowDemand(sc.k, 0, &costU); w != 1<<kop.U {
				t.Fatalf("precondition: fixed demand %#x, want the owed {U}", w)
			}
			if _, ok := s.grantRestrOwnFirst(sc.k, sl, 1<<kop.U); ok {
				t.Fatal("the fixed-colour ordering took a printed any-colour ability")
			}
			ab, _, ok := s.windowRestrOwed(sc.k, 0, sl, &costU)
			if ok != tc.ok {
				t.Fatalf("windowRestrOwed ok %v, want %v", ok, tc.ok)
			}
			if ok && ab.ManaColors != kop.ColAny {
				t.Fatalf("windowRestrOwed took %#x, want the any-colour ability", ab.ManaColors)
			}
		})
	}
}
