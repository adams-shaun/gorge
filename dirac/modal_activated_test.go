package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// mode applies the offered mode m of the pending DkModes decision.
func (sc *scen) mode(t *testing.T, m uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkMode && a.Mode() == m {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("mode %d not offered: %v", m, d.Options[:d.N])
}

// TestModalActivatedAbility: a modal activated ability announces its mode as it is
// activated, before its cost is paid (CR 602.2b).
func TestModalActivatedAbility(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	bow := sc.perm(id("Bow of Nylea"), 0)
	sc.perm(id("Forest"), 0)
	sc.perm(id("Forest"), 0)
	life := s.pl[0].life
	sc.do(t, AkActivate, uint8(bow), 0xff)
	d := sc.expect(t, DkModes, 0)
	if s.perms[sc.slot(bow)].flags&pfTapped != 0 {
		t.Fatal("the cost was paid before the modes")
	}
	if sc.count(&d, AkMode) != 2 { // the two with no target; no creature is out
		t.Fatalf("%d modes offered, want 2", sc.count(&d, AkMode))
	}
	sc.mode(t, 2) // you gain 3 life
	s.Step()
	if s.pl[0].life != life+3 || s.sp != 0 {
		t.Fatalf("life %d (was %d), stack %d: the chosen mode did not resolve", s.pl[0].life, life, s.sp)
	}
}

// TestModalActivatedTarget: a mode-bound target is chosen with the mode, as the
// ability is activated; mode 0's counter lands on the creature picked then.
func TestModalActivatedTarget(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	bow := sc.perm(id("Bow of Nylea"), 0)
	sc.perm(id("Forest"), 0)
	sc.perm(id("Forest"), 0)
	a := sc.perm(id("Hardbristle Bandit"), 0)
	b := sc.perm(id("Hardbristle Bandit"), 1)
	sc.do(t, AkActivate, uint8(bow), 0xff)
	sc.expect(t, DkModes, 0)
	sc.mode(t, 0) // +1/+1 counter on target creature
	d := sc.expect(t, DkTarget, 0)
	if d.N != 2 {
		t.Fatalf("%d candidates, want both creatures", d.N)
	}
	s.Apply(MakeAction(AkTarget, 0, TCode(b), 0))
	s.Step()
	if s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)) != 1 || s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Fatal("the counter did not land on the creature chosen")
	}
}

// TestModalStages: a spell, an ability and a trigger each announce their modes.
func TestModalStages(t *testing.T) {
	has := func(kind uint8) bool {
		for _, st := range buildProgram[kind] {
			if st == bsModes {
				return true
			}
		}
		return false
	}
	if !has(seSpell) || !has(seTrigger) || !has(seAbility) {
		t.Fatalf("bsModes: spell %v trigger %v ability %v, want all", has(seSpell), has(seTrigger), has(seAbility))
	}
}
