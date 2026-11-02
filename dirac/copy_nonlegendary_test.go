package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// copyExcTySlots must read an override only for fxCopyExc records: the
// other kinds (fxLoseAll, fxKwGrant, fxControl) carry r.idx == 0, and
// r.idx-1 would wrap to 255.
func TestCopyExcTySlotsIgnoresOtherFx(t *testing.T) {
	k := &prep{t: &kop.Table{Overrides: make([]kop.Override, 3)}}
	s := new(State)
	s.g.fx[0] = fxRec{sl: 1, kind: fxLoseAll}
	s.g.fx[1] = fxRec{sl: 2, kind: fxKwGrant}
	s.g.fx[2] = fxRec{sl: 3, kind: fxControl}
	s.g.fx[3] = fxRec{sl: 4, kind: fxOverride}
	s.g.fx[3].r.idx = 2
	k.t.Overrides[1].RemoveLegendary = true
	s.g.fx[4] = fxRec{sl: 5, kind: fxCopyExc}
	s.g.fx[4].r.idx = 2
	s.g.nfx = 5
	m := s.copyExcTySlots(k)
	var want slotMask
	want.set(5)
	if m != want {
		t.Fatalf("copyExcTySlots = %v, want only slot 5 (%v)", m, want)
	}
}
