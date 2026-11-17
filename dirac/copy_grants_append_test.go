package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A Clone's GainThisAbility$ appends the resolving trigger to the copied face
// each time (gorge's fold), so the grant it leaves is kept once for every copy
// it passed through, where an as-enters copy's AddAbilities$ is kept once.
func TestCopyGrantAppendKeepsRepeats(t *testing.T) {
	sc, k, sl, ov := grantCopyScen(t, false)
	s := sc.s
	g := cgrantRef{ov, v.HungryGhoul}
	count := func() int { return s.copyGrantRecs(sl, new([maxOvr]cgrantRef)) }
	s.copyGrantAppend(k, sl, g)
	if n := count(); n != 2 {
		t.Fatalf("%d grants after an append beside one, want 2", n)
	}
	s.copyGrantPut(k, sl, g) // the as-enters put adds nothing the copy has
	if n := count(); n != 2 {
		t.Fatalf("%d grants after an as-enters put of one it has, want 2", n)
	}
	// a copy of this copy carries both
	to := sc.slot(sc.perm(v.HungryGhoul, 0))
	setCopy(sc, to, v.HillGiant)
	s.copyGrantsAdd(k, to, sl)
	if n := s.copyGrantRecs(to, new([maxOvr]cgrantRef)); n != 2 {
		t.Fatalf("%d grants carried onto a copy of it, want 2", n)
	}
}
