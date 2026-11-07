package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func TestAegisHoldSkipsMarkerLayerExceptions(t *testing.T) {
	sc, k, ov := aegisExcScen(t,
		kop.Override{AddKw: kop.KwHaste},
		kop.Override{AddKw: kop.KwFlying})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	p := &s.perms[sl]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = v.HillGiant
	s.copyExcAdd(k, sl, NoSlot, ov[0])

	if !s.aegisUnderPush(sl) {
		t.Fatal("no base")
	}
	a1 := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 10)
	s.aegisRefresh(k, sl, NoSlot)

	tmpl := sc.perm(v.GrizzlyBears, 1)
	tsl := sc.slot(tmpl)
	s.copyExcAdd(k, tsl, NoSlot, ov[1])
	tmplCard := s.cardAt(tsl)
	if _, ok := s.cloneLayerPrep(k, sl, true, tmplCard, s.perms[tsl].obj, 0, 20); !ok {
		t.Fatal("eot")
	}
	s.copyExcAdd(k, sl, tsl, 0)
	setCopy(sc, sl, tmplCard)

	a2 := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 30)
	s.aegisRefresh(k, sl, NoSlot)
	if l := s.copyTop(sl, NoSlot); l.kind != copyAegis {
		t.Fatalf("top %d", l.kind)
	}

	s.aegisUnlink(k, a2, sl, false)
	if l := s.copyTop(sl, NoSlot); l.kind != copyEOT {
		t.Fatalf("after a2 top %d", l.kind)
	}
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("EOT flying lost")
	}

	s.cloneCleanup(k)
	if l := s.copyTop(sl, NoSlot); l.kind != copyAegis {
		t.Fatalf("after cleanup top %d", l.kind)
	}

	s.aegisUnlink(k, a1, sl, false)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("card %d", got)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("haste lost")
	}
	if s.kw(k, sl)&kop.KwFlying != 0 {
		t.Fatalf("stale EOT flying leaked into base: kw %b", s.kw(k, sl))
	}
}
