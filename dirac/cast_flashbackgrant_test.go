package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFBGrantPlanes pins the granted-flashback count encoding: two bit
// planes hold 0..3 (lo bit 0, hi bit 1), grantFB saturates at 3, setFBGrants
// round-trips every count, and clearFB zeroes both planes.
func TestFBGrantPlanes(t *testing.T) {
	var s State
	k := arenaPrep(&s, fbGrantTable())
	o, other := Obj(7), Obj(8)
	for i := 1; i <= 4; i++ {
		s.grantFB(k, o)
		want := uint8(i)
		if want > 3 {
			want = 3
		}
		if got := s.fbGrants(k, o); got != want {
			t.Fatalf("after %d grants: fbGrants = %d, want %d", i, got, want)
		}
		if got := s.fbGrants(k, other); got != 0 {
			t.Fatalf("after %d grants on %d: fbGrants(%d) = %d, want 0", i, o, other, got)
		}
	}
	if !s.mObj(k, mechFBLo, o) || !s.mObj(k, mechFBHi, o) {
		t.Fatalf("four grants left planes lo=%v hi=%v, want both set (saturated 3)",
			s.mObj(k, mechFBLo, o), s.mObj(k, mechFBHi, o))
	}

	s.clearFB(k, o)
	if got := s.fbGrants(k, o); got != 0 {
		t.Fatalf("clearFB: fbGrants = %d, want 0", got)
	}
	if s.mObj(k, mechFBLo, o) || s.mObj(k, mechFBHi, o) {
		t.Fatal("clearFB left a plane bit set")
	}

	wantLo := [4]bool{false, true, false, true}
	wantHi := [4]bool{false, false, true, true}
	for n := uint8(0); n <= 3; n++ {
		s.setFBGrants(k, o, n)
		if got := s.fbGrants(k, o); got != n {
			t.Fatalf("setFBGrants(%d): fbGrants = %d", n, got)
		}
		if s.mObj(k, mechFBLo, o) != wantLo[n] || s.mObj(k, mechFBHi, o) != wantHi[n] {
			t.Fatalf("setFBGrants(%d): planes lo=%v hi=%v, want lo=%v hi=%v",
				n, s.mObj(k, mechFBLo, o), s.mObj(k, mechFBHi, o), wantLo[n], wantHi[n])
		}
	}

	s.setFBGrants(k, o, 9) // a restored count above three saturates, as grantFB does
	if got := s.fbGrants(k, o); got != 3 {
		t.Fatalf("setFBGrants(9): fbGrants = %d, want saturated 3", got)
	}
}

// fbGrantTable is a table with one program granting flashback (its own mana
// cost) until end of turn: the only thing that gives a pool the flashback
// planes.
func fbGrantTable() *kop.Table {
	t := arenaTable(kop.OpGrantKeyword)
	t.Cards[0].Abilities[0].Prog[0].B.N = int16(kop.KwFlashbackOwnCost >> 16)
	return t
}
