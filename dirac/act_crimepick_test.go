package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// boomScen builds a scen over the std-otj pool (the Boom Box + Slickshot
// Vault-Buster lockstep fixture's table).
func boomScen(t *testing.T) (*scen, func(string) CardID) {
	t.Helper()
	set := cardsettest.Synth(t, "std-otj")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	sc := newScenTableLib(t, tbl, id("Mountain"), Config{})
	if !sc.k.gen.crimeOn {
		t.Fatal("the std-otj table does not turn crimeOn on")
	}
	return sc, id
}

// TestCrimeAbilityPickActivation: CR 700.13 judges a crime at the frame the
// target is chosen, and CR 601.2c/602.2a record an activated ability's
// targets at their answers, before payment (601.2h) — the root's ask and
// the SubAbility$ chain links' alike. Boom Box ({6},{T}, sac it: pump an
// artifact; then its chain asks a creature, a land, and destroys the
// targeted) names the opponent's Boom Box on the root ask: from that answer
// on the ledger bit is set and Slickshot Vault-Buster's
// CheckSVar$Count$CommittedCrimeThisTurn static reads it 3-power (gorge's
// TargetsChosen fold, turn_ledgers.go, agrees at that frame).
func TestCrimeAbilityPickActivation(t *testing.T) {
	sc, id := boomScen(t)
	s := sc.s
	boom, slick := sc.perm(id("Boom Box"), 0), sc.perm(id("Slickshot Vault-Buster"), 0)
	theirBoom, omen := sc.perm(id("Boom Box"), 1), sc.perm(id("Omenport Vigilante"), 1)
	// preconditions: the target is in the opponent's zone and the two
	// compared powers differ
	if !sc.onBF(theirBoom) || !sc.onBF(omen) {
		t.Fatal("the opponent's Boom Box or creature is not on the battlefield")
	}
	s.pl[0].pool[kop.R] = 8
	s.ctl = ctlPriority
	if s.CrimeLedger(0) {
		t.Fatal("the ledger is set before the activation")
	}
	if p := s.power(sc.k, sc.slot(slick)); p != 1 {
		t.Fatalf("Slickshot power %d before the crime, want 1", p)
	}
	// the activation is offered and taken through the real option flow
	found := false
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == uint8(boom) {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Boom Box's activation not offered: %v", d.Options[:d.N])
	}
	// the root ask: the opponent's Boom Box. The bit moves at THIS answer.
	found = false
	d = s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget && a.Target() == uint8(theirBoom) {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("the opponent's Boom Box not offered as the root target: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	if !s.CrimeLedger(0) {
		t.Fatalf("after the root answer: ledger %b, want player 0 set", s.g.crime)
	}
	if p := s.power(sc.k, sc.slot(slick)); p != 3 {
		t.Fatalf("Slickshot power %d after the root answer, want 3", p)
	}
	// the chain's creature link names the opponent's creature: the bit stays
	sc.do(t, AkTarget, 0xff, uint8(omen))
	if !s.CrimeLedger(0) {
		t.Fatalf("after the creature link's answer: ledger %b, want player 0 set", s.g.crime)
	}
	if p := s.power(sc.k, sc.slot(slick)); p != 3 {
		t.Fatalf("Slickshot power %d after the link's answer, want 3", p)
	}
}

// TestCrimeSubLinkPick is the gate itself: crimePick judges a spell's or an
// activated ability's pick — the root's slot and a chain link's slot alike
// (gorge's recordChosenTargets / recordSubTargets emit TargetsChosen at
// either answer) — and sets the ledger bit there; a trigger's pick is still
// judged on checkWard's S2 walk at placement. The BySpell stat stays
// spell-only.
func TestCrimeSubLinkPick(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	theirPerm := uint8(sc.perm(id("Hardbristle Bandit"), 1))
	st := &CrimeStats{}
	DebugCrimeStats = st
	defer func() { DebugCrimeStats = nil }()
	run := func(kind uint8) (ledger uint8, bySpell int, flagged bool) {
		s.g.crime = 0
		*s.bld() = build{e: stackEntry{kind: kind, ctrl: 0}}
		s.bld().e.tgt = noTargets
		s.crimePick(sc.k, theirPerm)
		return s.g.crime, st.BySpell, s.bld().bfl&bflCrime != 0
	}
	ledger, bySpell, flagged := run(seAbility)
	if ledger != 1 {
		t.Fatalf("an ability's pick: ledger %b, want player 0 set", ledger)
	}
	if !flagged {
		t.Fatal("an ability's pick did not mark the entry pick-judged")
	}
	if bySpell != 0 {
		t.Fatalf("an ability's pick counted %d spells, want 0 (the stat stays spell-only)", bySpell)
	}
	ledger, bySpell, _ = run(seSpell)
	if ledger != 1 || bySpell != 1 {
		t.Fatalf("a spell's pick: ledger %b bySpell %d, want 1 and 1", ledger, bySpell)
	}
	ledger, _, _ = run(seTrigger)
	if ledger != 0 {
		t.Fatalf("a trigger's pick: ledger %b, want clear (the S2 walk judges it at placement)", ledger)
	}
}
