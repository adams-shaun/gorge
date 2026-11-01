package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMovedTargetLastStatus: a filter over a moved target reads the tapped and
// attacking status of its snapshot (CR 608.2h).
func TestMovedTargetLastStatus(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Shoreline Looter", Count: 1}, {Name: "Plains", Count: 8}})
	s := sc.s
	o := sc.perm(id["Shoreline Looter"], 1)
	for _, tc := range []struct {
		flags     uint16
		want, not kop.FilterFlags
	}{
		{0, kop.FUntapped | kop.FNotAttacking, kop.FTapped | kop.FAttacking | kop.FAttackingOrBlocking},
		{pfTapped, kop.FTapped | kop.FNotAttacking, kop.FUntapped | kop.FAttacking},
		{pfAttacking, kop.FUntapped | kop.FAttacking | kop.FAttackingOrBlocking, kop.FTapped | kop.FNotAttacking},
	} {
		s.perms[sc.slot(o)].flags = tc.flags
		s.g.tli = s.snapLKI(o)
		for bit := kop.FilterFlags(1); bit != 0; bit <<= 1 {
			if tc.want&bit != 0 && !s.lkiStatusOK(&kop.Filter{Flags: bit}) {
				t.Errorf("flags %#x: filter %#x does not match", tc.flags, bit)
			}
			if tc.not&bit != 0 && s.lkiStatusOK(&kop.Filter{Flags: bit}) {
				t.Errorf("flags %#x: filter %#x matches", tc.flags, bit)
			}
		}
	}
}
