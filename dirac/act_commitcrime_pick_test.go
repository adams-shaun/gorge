package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCrimeAbilityPickSetsLedger: an ability's target ask sets the per-turn
// crime ledger when its picks complete (gorge records each ask's
// TargetsChosen as it is applied, and commits a multi-pick window at Done or
// Max), mid-way through a multi-target activation and before payment, while
// TrigCommitCrime still waits for placement (once per recorded stack
// object, act_commitcrime.go).
func TestCrimeAbilityPickSetsLedger(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	march := sc.perm(id("Marchesa, Dealer of Death"), 0) // my permanent: not a crime
	theirs := sc.perm(id("Hardbristle Bandit"), 1)       // theirs: a crime
	s.npend, s.g.crime = 0, 0
	*s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0, obj: march, trigObj: NoObj, srcVer: s.objs[march].ver}}
	s.bld().e.tgt = noTargets
	s.pushBuild(sc.k)
	s.ctl = ctlBuild
	if s.bld().e.kind != seAbility {
		t.Fatal("precondition: the build under test is an activated ability")
	}
	if s.g.crime == 1 {
		t.Fatal("precondition: the ledger is clear before the picks")
	}
	// a single-pick ask (the plain target step) completes at the pick
	s.crimePick(sc.k, TCode(march), true)
	if s.g.crime != 0 {
		t.Fatalf("a pick at your own permanent set the ledger: %b", s.g.crime)
	}
	s.crimePick(sc.k, TCode(theirs), true)
	if s.g.crime != 1 {
		t.Fatalf("after the criminal pick of a completed ask: ledger %b, want bit 0", s.g.crime)
	}
	if s.npend != 0 {
		t.Fatalf("the pick queued %d triggers, want 0 (placement fires)", s.npend)
	}
	// a multi-pick window: an open ask's pick sets nothing, the completion
	// (finishSlot) judges the slot's whole recorded set
	s.g.crime = 0
	s.g.mt = multiTgt{active: true, slot: 0, min: 1, max: 2}
	s.applyMultiTarget(sc.k, MakeAction(AkTarget, 0, TCode(theirs), 0))
	if s.g.crime != 0 || s.g.mt.picked != 1 {
		t.Fatalf("an open window's criminal pick: ledger %b picked %d, want 0 and 1", s.g.crime, s.g.mt.picked)
	}
	s.applyMultiTarget(sc.k, MakeAction(AkTarget, 0, TCode(march), 0))
	if s.g.crime != 0 || s.g.mt.picked != 2 {
		t.Fatalf("the second pick of an open window: ledger %b picked %d, want 0 and 2", s.g.crime, s.g.mt.picked)
	}
	if s.finishSlot(sc.k, nil, nil) != mtNext {
		t.Fatal("the completed slot did not advance the build")
	}
	if s.g.crime != 1 {
		t.Fatalf("after the window committed: ledger %b, want bit 0", s.g.crime)
	}
	if s.npend != 0 {
		t.Fatalf("the committed window queued %d triggers, want 0 (placement fires)", s.npend)
	}
	// placement: the walk judges the entry and queues each listener once
	e := stackEntry{kind: seAbility, ctrl: 0, nt: 2, obj: march, trigObj: NoObj}
	e.tgt = [kop.MaxTargets]TCode{TCode(theirs), TCode(march), TCode(NoTarget)}
	s.checkWard(sc.k, &e)
	if s.npend != 1 || s.g.crime != 1 {
		t.Fatalf("placement queued %d triggers (ledger %b), want 1 (ledger 1)", s.npend, s.g.crime)
	}
}
