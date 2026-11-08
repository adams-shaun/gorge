package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A self-copy layer (CR 707.2) copies the becomer's own copiable values, its
// current copy exceptions among them (CR 707.9b). Because its template is the
// becomer itself, the inherited exceptions cannot be re-read from the template
// when a newer layer covers the self layer and later ends: they are held
// beside the layer as fxCopySelf records and restored with it. These tests
// cover both re-base sites (aegisRefresh when an Aegis copy ends, cloneCleanup
// at the cleanup step) and the permanent and until-end-of-turn markers.

// selfCopyLayer lands a self-copy layer on slot sl the way cloneCopy does:
// cloneLayerPrep clears the slot for the layer, the inherited exceptions are
// captured in kept, the layer's live exceptions are the inherited ones and
// then the op's own idx, and the inherited ones are held beside the layer.
func selfCopyLayer(t *testing.T, sc *scen, k *prep, sl Slot, eot bool, kept []uint8, idx uint8, ts uint16) {
	t.Helper()
	s := sc.s
	host := s.perms[sl].obj
	c := s.cardAt(sl)
	old, ok := s.cloneLayerPrep(k, sl, eot, c, host, idx, ts)
	if !ok {
		t.Fatal("the effect pool refused the self layer")
	}
	for _, x := range kept {
		s.copyExcPush(sl, x)
	}
	if idx != 0 {
		s.copyExcPush(sl, idx)
	}
	s.perms[sl].ovr.kw = s.perms[sl].ovr.kw&^old | s.copyExcKw(k, sl)
	if !eot {
		s.clonePermNote(sl, host, c, idx, ts)
	}
	s.cloneSelfHeldPush(sl, kept, len(kept), ts)
	setCopy(sc, sl, c)
}

// TestSelfCopyEOTKeepsInheritedExceptionsUnderAegis: a permanent that already
// carries a copy exception copies itself until end of turn. A newer Aegis
// copy covers it and ends; the self layer is newest again, and its locked
// copiable face (CR 707.2b) still has the inherited exception.
func TestSelfCopyEOTKeepsInheritedExceptionsUnderAegis(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{AddKw: kop.KwHaste})
	s := sc.s
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	s.copyExcPush(sl, ov[0]) // the permanent's own copiable exception
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	selfCopyLayer(t, sc, k, sl, true, []uint8{ov[0]}, 0, 20)

	if s.copyRec(sl, fxCopySelf) < 0 {
		t.Fatal("precondition: the self layer's inherited exception was not held beside it")
	}
	if l := s.copyTop(sl, NoSlot); l.kind != copyEOT || l.src != s.perms[sl].obj {
		t.Fatalf("precondition: top layer kind %d src %d, want the self EOT layer", l.kind, l.src)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("precondition: the self layer lacks its inherited flying exception")
	}

	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 30)
	s.aegisRefresh(k, sl, NoSlot)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("the Aegis copy shows card %d, want %d", got, v.SerraAngel)
	}
	if s.kw(k, sl)&kop.KwHaste != 0 {
		t.Fatal("precondition: the Aegis copy still shows the covered self layer's exception")
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want the self layer's %d", got, v.HillGiant)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("the self EOT layer's inherited exception is gone after the Aegis ends")
	}
}

// TestSelfCopyEOTKeepsInheritedExceptionsUnderSameCardAegis is the same
// scenario with the Aegis's exiled card equal to the layer's locked card, so
// only the exception set distinguishes the two layers and the refresh cannot
// take its early return on card equality: the held exceptions must make the
// early return say the layer is not yet shown.
func TestSelfCopyEOTKeepsInheritedExceptionsUnderSameCardAegis(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{AddKw: kop.KwHaste})
	s := sc.s
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	s.copyExcPush(sl, ov[0])
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	selfCopyLayer(t, sc, k, sl, true, []uint8{ov[0]}, 0, 20)

	a := addAegisLayer(t, sc, sl, v.HillGiant, v.HillGiant, 30)
	s.aegisRefresh(k, sl, NoSlot)
	if s.kw(k, sl)&kop.KwHaste != 0 {
		t.Fatal("precondition: the Aegis copy still shows the covered self layer's exception")
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want %d", got, v.HillGiant)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("the self EOT layer's inherited exception is gone after the same-card Aegis ends")
	}
}

// TestSelfCopyPermanentKeepsInheritedExceptionsAtCleanup: a permanent self
// copy marker under a newer until-end-of-turn layer. When the cleanup step
// ends the EOT layer, the marker is the newest layer left and its inherited
// exception comes back with it (CR 707.2b), not just its own override.
func TestSelfCopyPermanentKeepsInheritedExceptionsAtCleanup(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{AddKw: kop.KwFlying})
	s := sc.s
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	s.copyExcPush(sl, ov[0])
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	selfCopyLayer(t, sc, k, sl, false, []uint8{ov[0]}, 0, 20)

	tmpl := sc.perm(v.GrizzlyBears, 1)
	tsl := sc.slot(tmpl)
	tmplCard := s.cardAt(tsl)
	if _, ok := s.cloneLayerPrep(k, sl, true, tmplCard, s.perms[tsl].obj, 0, 30); !ok {
		t.Fatal("the effect pool refused the EOT layer")
	}
	s.copyExcAdd(k, sl, tsl, 0)
	setCopy(sc, sl, tmplCard)
	if l := s.copyTop(sl, NoSlot); l.kind != copyEOT {
		t.Fatalf("precondition: top layer kind %d, want the newer EOT copy", l.kind)
	}

	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after cleanup the bearer shows card %d, want the self marker's %d", got, v.HillGiant)
	}
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("the self permanent marker's inherited exception is gone after cleanup")
	}
	if s.copyRec(sl, fxCopySelf) < 0 {
		t.Fatal("the live self marker's held exception was dropped")
	}
}

// TestSelfCopyEOTEndDropsHeldRecords: when the self layer itself ends with no
// layer beneath, its held exceptions go with it and the permanent is back on
// its printed card.
func TestSelfCopyEOTEndDropsHeldRecords(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{AddKw: kop.KwFlying})
	s := sc.s
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	s.copyExcPush(sl, ov[0])
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	selfCopyLayer(t, sc, k, sl, true, []uint8{ov[0]}, 0, 20)
	if s.copyRec(sl, fxCopySelf) < 0 {
		t.Fatal("precondition: the self layer's inherited exception was not held")
	}

	s.cloneCleanup(k)
	if s.copyRec(sl, fxCopySelf) >= 0 {
		t.Fatal("the ended self layer's held exception outlived it")
	}
	if s.kw(k, sl)&kop.KwFlying != 0 {
		t.Fatal("the ended self layer's inherited exception still shows")
	}
}
