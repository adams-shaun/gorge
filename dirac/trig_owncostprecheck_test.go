package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOwnCostPrecheckTriggersWithoutCounter: Sun Droplet's "At the beginning
// of each upkeep, you may remove a charge counter from Sun Droplet. If you do,
// you gain 1 life" has no intervening-if, so with no charge counter the
// trigger still goes on the stack (CR 603.2; gorge presentGateApplies,
// TestFarmSunDroplet) and resolves without gaining life: the script's
// IsPresent$ only mirrors the SubCounter cost (kgen triggerIsPresentConds).
func TestOwnCostPrecheckTriggersWithoutCounter(t *testing.T) {
	sc, id := ddoScen(t) // trig_damagedonce_test.go: Sun Droplet and a Forest library
	o := sc.perm(id["Sun Droplet"], 0)
	if n := sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrCharge)); n != 0 {
		t.Fatalf("precondition: Sun Droplet has %d charge counters, want none", n)
	}
	sc.passUntil(t, kop.StepUpkeep)
	sc.s.Step()
	if sc.s.sp != 1 || sc.s.stack[0].obj != o || sc.s.stack[0].kind != seTrigger {
		t.Fatalf("want Sun Droplet's upkeep trigger alone on the stack, have %d entries", sc.s.sp)
	}
	s := sc.s
	life := s.pl[0].life
	declined := false
	for i := 0; i < 8 && s.sp > 0; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkPriority:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && d.Sub == ChGenPay:
			// "you may remove a charge counter": with no counter only
			// "don't pay" is offered, so no life is gained
			if d.N != 1 || d.Options[0].Mode() != 0 {
				t.Fatalf("pay choice offers %d options (first mode %d), want only don't-pay", d.N, d.Options[0].Mode())
			}
			declined = true
			s.Apply(d.Options[0])
		default:
			t.Fatalf("decision kind %d sub %d while the trigger waits", d.Kind, d.Sub)
		}
	}
	if !declined {
		t.Fatalf("precondition: the trigger resolved without reaching its SubCounter cost")
	}
	if s.sp != 0 || s.pl[0].life != life {
		t.Fatalf("stack %d, life %d -> %d: want the trigger resolved with no life gained", s.sp, life, s.pl[0].life)
	}
}
