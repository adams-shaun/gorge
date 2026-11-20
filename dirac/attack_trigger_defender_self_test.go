package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAtkPWGuardGoneSelfDefenderStalls: a TriggeredDefender attack trigger
// whose attacker has left before it resolves carries only the queue-time
// defSeat. Gorge's .Opponent qualifier names nobody when the attacked player
// is the trigger's own controller (a stolen equipped creature attacking the
// equipment's controller: Mage Slayer), so the kernel must stall there as the
// live-attacker arm does, not deal the controller damage from the attacker's
// last known information. A gone attacker that attacked another player still
// passes, on that bound seat.
func TestAtkPWGuardGoneSelfDefenderStalls(t *testing.T) {
	for _, tc := range []struct {
		name    string
		defSeat uint8
		ctrl    uint8
		stalls  bool
	}{
		{"gone, bound opponent player", defSeatPack(1, false), 0, false},
		{"gone, bound own controller", defSeatPack(1, false), 1, true},
		{"gone, no bound defender", 0, 1, true},
		{"gone, bound object", defSeatPack(1, true), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &State{}
			s.res.active = true
			s.res.e = stackEntry{kind: seTrigger, ctrl: tc.ctrl, defSeat: tc.defSeat, trigObj: NoObj}
			// Precondition: the attacker is gone, so the guard reads defSeat.
			if sl := s.objRef(&prep{}, kop.RefTriggerObj); sl != NoSlot {
				t.Fatalf("attacker slot %d, want the attacker gone", sl)
			}
			ok, done := s.execMcombat(&prep{}, &kop.Instr{Op: kop.OpAtkPWGuard})
			if !ok || !done {
				t.Fatalf("execMcombat = %v, %v, want the op handled", ok, done)
			}
			if got := s.StallReason(); (got == StallDefenderPW) != tc.stalls {
				t.Fatalf("stall %q (code %d), want stall=%v", StallName(got), got, tc.stalls)
			}
		})
	}
}
