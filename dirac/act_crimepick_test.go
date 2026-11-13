package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCrimeAbilityPickActivation: CR 700.13 judges a crime at the frame the
// target is chosen, and CR 601.2c/602.2a record an activated ability's
// targets at their answers, before payment (601.2h). Goblin Cratermaker's
// "{T}: deal 1 damage to target creature" names the opponent's Hardbristle
// Bandit on the root ask: from that answer on the ledger bit is set and
// Slickshot Vault-Buster's CheckSVar$Count$CommittedCrimeThisTurn static
// reads it 3-power (gorge's TargetsChosen fold, turn_ledgers.go, agrees at
// that frame; the Boom Box chain-link shape is the lockstep oracle's
// TestCrimeSubAbilityTargetsLockstep).
func TestCrimeAbilityPickActivation(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	maker, slick := sc.perm(id("Goblin Cratermaker"), 0), sc.perm(id("Slickshot Vault-Buster"), 0)
	bandit := sc.perm(id("Hardbristle Bandit"), 1)
	// preconditions: the target is in the opponent's zone and the two
	// compared powers differ
	if !sc.onBF(bandit) || !sc.onBF(slick) {
		t.Fatal("the Bandit or Slickshot is not on the battlefield")
	}
	s.pl[0].pool[kop.R] = 1 // the activation's {1}
	s.ctl = ctlPriority
	if s.CrimeLedger(0) {
		t.Fatal("the ledger is set before the activation")
	}
	// the activation is offered and taken through the real option flow
	found := false
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(maker)) {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Goblin Cratermaker's activation not offered: %v", d.Options[:d.N])
	}
	// precondition of the comparison: before the answer the ledger is clear
	// and Slickshot is at base power
	if p := s.power(sc.k, sc.slot(slick)); p != 1 {
		t.Fatalf("Slickshot power %d before the crime, want 1", p)
	}
	// the root ask: the opponent's creature. The bit moves at THIS answer:
	// the plain target step takes one pick per ask, so the ask completes
	// here (crimePick with done, cast.go applyBuild).
	found = false
	d = s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget && a.Target() == TCode(uint8(bandit)) {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("the Bandit not offered as the root target: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	if !s.CrimeLedger(0) {
		t.Fatalf("after the root answer: ledger %b, want player 0 set", s.g.crime)
	}
	if p := s.power(sc.k, sc.slot(slick)); p != 3 {
		t.Fatalf("Slickshot power %d after the root answer, want 3", p)
	}
}

// TestCrimeSubLinkPick is the gate itself: crimePick judges a spell's pick
// at the pick (BySpell counts it, bflCrime marks the entry) and a
// non-spell's completed ask at the ask's completion, bit only — an open
// multi-pick window's pick (done false) sets nothing, gorge commits the
// ask's TargetsChosen when the window completes at Done or Max
// (crimeAskDone/finishSlot, the charm mask). A trigger's completed ask also
// sets only the bit: TrigCommitCrime still waits for the S2 walk at
// placement (checkWard, once per recorded object), so the pick queues
// nothing.
func TestCrimeSubLinkPick(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	banditObj := sc.perm(id("Hardbristle Bandit"), 1)
	theirPerm := uint8(banditObj)
	// preconditions: the judged target is on the opponent's battlefield
	if !sc.onBF(banditObj) {
		t.Fatal("the Bandit is not on the battlefield")
	}
	if s.perms[sc.slot(banditObj)].ctrl != 1 {
		t.Fatalf("the Bandit's controller is %d, want 1", s.perms[sc.slot(banditObj)].ctrl)
	}
	st := &CrimeStats{}
	DebugCrimeStats = st
	defer func() { DebugCrimeStats = nil }()
	run := func(kind uint8, done bool) (ledger uint8, bySpell int, pending int, flagged bool) {
		s.g.crime, s.npend, st.BySpell = 0, 0, 0
		*s.bld() = build{e: stackEntry{kind: kind, ctrl: 0}}
		s.bld().e.tgt = noTargets
		s.crimePick(sc.k, TCode(theirPerm), done)
		return s.g.crime, st.BySpell, int(s.npend), s.bld().bfl&bflCrime != 0
	}
	// an ability's completed ask: the bit, no spell stat, no queued trigger
	// (TrigCommitCrime waits for the S2 walk at placement), no pick-judged
	// flag (the walk re-judges; noteCrime is idempotent)
	ledger, bySpell, pending, flagged := run(seAbility, true)
	if ledger != 1 {
		t.Fatalf("an ability's completed ask: ledger %b, want player 0 set", ledger)
	}
	if bySpell != 0 {
		t.Fatalf("an ability's ask counted %d spells, want 0 (the stat stays spell-only)", bySpell)
	}
	if pending != 0 {
		t.Fatalf("an ability's ask queued %d triggers, want 0 (placement fires)", pending)
	}
	if flagged {
		t.Fatal("an ability's ask marked the entry pick-judged, want the walk to re-judge")
	}
	// an open multi-pick window's pick (done false): gorge commits the ask's
	// TargetsChosen when the window completes, so this sets nothing
	ledger, _, _, _ = run(seAbility, false)
	if ledger != 0 {
		t.Fatalf("an open window's pick: ledger %b, want clear until the ask commits", ledger)
	}
	// a spell's pick is judged at the pick, stat and flag included
	ledger, bySpell, _, flagged = run(seSpell, true)
	if ledger != 1 || bySpell != 1 || !flagged {
		t.Fatalf("a spell's pick: ledger %b bySpell %d flagged %v, want 1, 1 and true", ledger, bySpell, flagged)
	}
	// a trigger's completed ask sets the bit too (gorge records the ask's
	// TargetsChosen at the answer), but queues nothing: the fire waits for
	// the S2 walk at placement
	ledger, bySpell, pending, _ = run(seTrigger, true)
	if ledger != 1 {
		t.Fatalf("a trigger's completed ask: ledger %b, want player 0 set", ledger)
	}
	if bySpell != 0 || pending != 0 {
		t.Fatalf("a trigger's ask: %d spells %d triggers queued, want 0 and 0 (placement fires)", bySpell, pending)
	}
}
