package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckChosenWidePairCensusFirst is the kernel-side half of
// TestChosenWidePairCensusFirst: a chosen-colour widened pair whose larger
// alternative gorge's census omits (Sunken Citadel) is no unit of the witness
// while the plain census pays, so two Citadels plan two one-mana census taps,
// ObjID-lowest first. A fixed-colour larger alternative keeps its unit: the
// Steelswarm Operator's activation-only {U}{U} is still the plan.
func CheckChosenWidePairCensusFirst(t *testing.T, tbl *kop.Table, citadel, opCard, mountain, stone, keep CardID) {
	t.Helper()
	sc := newScenTableLib(t, tbl, mountain, Config{})
	var cits [2]Obj
	for i := range cits {
		cits[i] = sc.perm(citadel, 0)
		sc.s.perms[sc.slot(cits[i])].chosen = 3 // red
		sc.s.perms[sc.slot(cits[i])].flags &^= pfTapped
	}
	land := sc.perm(mountain, 0) // the activation's land source
	dAct := payDesc{class: pdAct, src: sc.slot(land)}
	own := sc.s.manaPCGen(sc.k, sc.slot(cits[0]))
	if v, kd, _ := sc.s.restrView(sc.k, 0, sc.slot(cits[0]), own, &dAct); v == nil || kd != rkWiden || !v.wideChosen {
		t.Fatalf("precondition: the land activation does not admit Citadel's chosen pair (kind %d)", kd)
	}
	// A live ability build makes the plan's payment this activation.
	sc.s.ctl = ctlBuild
	*sc.s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0, obj: land, srcVer: sc.s.objs[land].ver}}
	var c kop.ManaCost
	c.Generic, c.Pips[kop.R] = 1, 1 // {1}{R}
	var pk planner
	sc.s.tapNeed(&pk, 0, &c, 0, false)
	sc.s.plan(sc.k, 0, &pk, NoObj)
	if !pk.found {
		t.Fatal("no plan for {1}{R} from two Citadels")
	}
	if pk.bestN != 2 {
		t.Fatalf("plan taps %d sources, want the two census Citadels: types %v", pk.bestN, pk.bestTy[:pk.bestN])
	}
	for i := 0; i < pk.bestN; i++ {
		u := pk.bestSeq[i]
		ty := &pk.types[pk.bestTy[i]]
		if got := sc.s.cardAt(pk.units[u]); got != citadel {
			t.Fatalf("plan unit %d is card %d, want Sunken Citadel %d", u, got, citadel)
		}
		if ty.n != 1 || ty.rb || ty.alt {
			t.Fatalf("plan unit %d is the restricted pair (n %d rb %v alt %v), want the census one-mana ability", u, ty.n, ty.rb, ty.alt)
		}
	}
	if pk.uobj[pk.bestSeq[0]] > pk.uobj[pk.bestSeq[1]] {
		t.Fatalf("plan taps ObjID %d before %d, want the lowest first", pk.uobj[pk.bestSeq[0]], pk.uobj[pk.bestSeq[1]])
	}

	// With the census unable to pay, the pair is the plan: one Citadel and a
	// Kher Keep tapped for the activation's own {T} has no other mana source.
	sc = newScenTableLib(t, tbl, mountain, Config{})
	cit := sc.perm(citadel, 0)
	sc.s.perms[sc.slot(cit)].chosen = 3 // red
	sc.s.perms[sc.slot(cit)].flags &^= pfTapped
	kp := sc.perm(keep, 0)
	sc.s.perms[sc.slot(kp)].flags |= pfTapped // the activation's {T} cost
	sc.s.ctl = ctlBuild
	*sc.s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0, obj: kp, srcVer: sc.s.objs[kp].ver}}
	var cf kop.ManaCost
	cf.Generic, cf.Pips[kop.R] = 1, 1 // {1}{R}
	var pkf planner
	sc.s.tapNeed(&pkf, 0, &cf, 0, false)
	sc.s.plan(sc.k, 0, &pkf, NoObj)
	if !pkf.found || pkf.bestN != 1 {
		t.Fatalf("one-Citadel plan: found %v taps %d, want the one pair", pkf.found, pkf.bestN)
	}
	uf := pkf.bestSeq[0]
	tf := &pkf.types[pkf.bestTy[0]]
	if got := sc.s.cardAt(pkf.units[uf]); got != citadel {
		t.Fatalf("one-Citadel plan taps card %d, want Sunken Citadel %d", got, citadel)
	}
	if tf.n != 2 || !tf.rb {
		t.Fatalf("one-Citadel plan type n %d rb %v, want the restricted {C}{C}", tf.n, tf.rb)
	}

	// A fixed-colour larger alternative stays a unit: the Operator's
	// activation-only {U}{U} pays an artifact ability's {U}{U}.
	sc = newScenTableLib(t, tbl, mountain, Config{})
	sc.perm(opCard, 0)
	art := sc.perm(stone, 0)
	sc.s.ctl = ctlBuild
	*sc.s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0, obj: art, srcVer: sc.s.objs[art].ver}}
	var cu kop.ManaCost
	cu.Pips[kop.U] = 2
	var pku planner
	sc.s.tapNeed(&pku, 0, &cu, 0, false)
	sc.s.plan(sc.k, 0, &pku, NoObj)
	if !pku.found || pku.bestN != 1 {
		t.Fatalf("Operator plan: found %v taps %d, want the one {U}{U} unit", pku.found, pku.bestN)
	}
	u := pku.bestSeq[0]
	ty := &pku.types[pku.bestTy[0]]
	if got := sc.s.cardAt(pku.units[u]); got != opCard {
		t.Fatalf("Operator plan taps card %d, want Steelswarm Operator %d", got, opCard)
	}
	if ty.n != 2 || !ty.rb {
		t.Fatalf("Operator plan type n %d rb %v, want the restricted {U}{U}", ty.n, ty.rb)
	}
}
