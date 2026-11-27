package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopyCtlExileOnly pins ExtCopyExiledWith's op-scoped B.N bit 15: the
// ValidExile form (kgen's CopyAllExile) walks only the resolving controller's
// exile zone, as gorge's definedZoneValidTargets does for the exile prefix,
// while the ordinary exiled-with form still reads every seat's exile. Both
// seats' exiles are populated before the op, so the copy count is the flag's
// doing, not an empty set.
func TestCopyCtlExileOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctl  bool
		want int
	}{
		{"every seat's exile (the exiled-with form)", false, 2},
		{"the controller's exile only (the ValidExile form)", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, in, id := copyExwScen(t)
			src := sc.perm(id["saga"], 0)
			exwExile(sc, id["bear"], 0, src)
			exwExile(sc, id["bear"], 1, src)
			if sc.s.pl[0].exileN != 1 || sc.s.pl[1].exileN != 1 {
				t.Fatalf("exile zones = %d/%d, want one card in each",
					sc.s.pl[0].exileN, sc.s.pl[1].exileN)
			}
			in.F = 0 // every exiled card matches
			if tc.ctl {
				in.B = kop.Const(kop.CopyNonRemCtrl) // bit 15: controller's exile only
			}
			sc.s.res.active, sc.s.res.e.obj, sc.s.res.e.ctrl = true, src, 0
			sc.s.copyExiledWith(sc.k, &in)
			if got := copyExwTokens(sc.s); got != tc.want || sc.s.g.stalled != 0 {
				t.Fatalf("copies %d stalled %d, want %d copies and no stall",
					got, sc.s.g.stalled, tc.want)
			}
		})
	}
}
