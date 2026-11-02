package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPaySacSelf: kop.PaySacSelf is payable while the payer controls the
// resolving ability's source (the same object), and paying sacrifices it.
func TestPaySacSelf(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl, sc.s.res.e.kind = 0, seTrigger
	sc.s.res.e.obj, sc.s.res.e.card, sc.s.res.e.srcVer = a, fc, sc.s.objs[a].ver
	if !sc.s.canPayGen(sc.k, 0, kop.PaySacSelf, 1, 0, 0) || sc.s.canPayGen(sc.k, 1, kop.PaySacSelf, 1, 0, 0) {
		t.Fatal("PaySacSelf: payable by the source's controller only")
	}
	sc.s.paySacSelf(sc.k)
	if z := sc.s.objs[a].zone; z != kop.ZoneGraveyard {
		t.Fatalf("source zone %d after paying, want the graveyard", z)
	}
	if sc.s.canPayGen(sc.k, 0, kop.PaySacSelf, 1, 0, 0) {
		t.Fatal("PaySacSelf payable with the source gone")
	}
}

// TestPayDraw: kop.PayDraw is always payable and paying it draws A cards
// (gorge's Draw-bearing triggered cost).
func TestPayDraw(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	if !sc.s.canPayGen(sc.k, 0, kop.PayDraw, 2, 0, 0) {
		t.Fatal("a draw cost is not payable")
	}
	h, l := sc.s.pl[0].handN, sc.s.pl[0].libHi-sc.s.pl[0].libLo
	sc.s.payDraw(0, 2)
	if sc.s.pl[0].handN != h+2 || sc.s.pl[0].libHi-sc.s.pl[0].libLo != l-2 {
		t.Fatalf("hand %d -> %d, library %d -> %d", h, sc.s.pl[0].handN, l, sc.s.pl[0].libHi-sc.s.pl[0].libLo)
	}
}
