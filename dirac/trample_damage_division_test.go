package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTrampleDamageDivisionIsAsked pins the kernel side of CR 702.19b: a
// blocked trampler with excess power poses the controller's damage division,
// whose legal splits run from "lethal to the blocker, the rest tramples" to
// "everything to the blocker". The sixth decision option must not be assumed
// from the gorge side only: the kernel's own ask is the lockstep contract.
func TestTrampleDamageDivisionIsAsked(t *testing.T) {
	for _, tc := range []struct {
		name     string
		option   int
		wantLife int16
	}{
		// 6/6 trample blocked by a 0/4: extra 2 over three ways:
		// 4/2, 5/1, 6/0 (blocker share / trample share).
		{"lethal then trample", 0, 18},
		{"all to the blocker", 2, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScen(t, Config{NoElide: true})
			dm := sc.perm(v.ColossalDreadmaw, 0) // 6/6 trample
			wall := sc.perm(v.TundraWall, 1)     // 0/4
			sc.passUntil(t, kop.StepAttackers)
			sc.do(t, AkAttack, TCode(dm), scAny)
			sc.do(t, AkDone, scAny, scAny)
			sc.passUntil(t, kop.StepBlockers)
			sc.do(t, AkBlock, TCode(wall), TCode(dm))
			sc.do(t, AkDone, scAny, scAny)

			// Drive to the damage division by hand: passUntil answers a
			// default trample division itself, and this test inspects it.
			var d Decision
			for i := 0; i < 20; i++ {
				d = sc.s.Step()
				if d.Kind == DkChoose && d.Sub == ChDivide {
					break
				}
				if d.Kind == DkPriority {
					sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
				}
			}
			if d.Kind != DkChoose || d.Sub != ChDivide || d.Player != 0 || d.N != 3 {
				t.Fatalf("division decision kind=%d sub=%d player=%d n=%d, want ChDivide by seat 0 with 3 options", d.Kind, d.Sub, d.Player, d.N)
			}
			sc.s.Apply(d.Options[tc.option])
			sc.passUntil(t, kop.StepMain2)
			if got := sc.s.pl[1].life; got != tc.wantLife {
				t.Fatalf("defender life %d, want %d", got, tc.wantLife)
			}
			if tc.wantLife == 20 && sc.onBF(wall) {
				t.Fatal("the 0/4 wall survived all 6 damage")
			}
		})
	}
}
