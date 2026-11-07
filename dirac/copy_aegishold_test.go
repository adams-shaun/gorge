package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// aegisExcScen is a scene whose table appends the copy-exception overrides
// overrides; the returned indexes are their (index+1) names.
func aegisExcScen(t *testing.T, overrides ...kop.Override) (*scen, *prep, []uint8) {
	t.Helper()
	tbl := *v.Table
	first := len(tbl.Overrides)
	tbl.Overrides = append(append([]kop.Override{}, tbl.Overrides...), overrides...)
	if first+len(overrides) > 255 {
		t.Fatal("too many overrides")
	}
	sc := newScenTable(t, &tbl, Config{})
	idx := make([]uint8, len(overrides))
	for i := range idx {
		idx[i] = uint8(first + i + 1)
	}
	return sc, genPrepOf(sc), idx
}

// TestAegisEndGivesBackCoveredCopyExceptions: a token copy with an "except it
// has haste" gets an Aegis copy, then loses it. The Aegis copy hides the
// exception while it shows (CR 707.2, 613.7e) and the covered copy, which has
// no marker to rebuild it from, gets it back when the Aegis copy ends.
func TestAegisEndGivesBackCoveredCopyExceptions(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{AddKw: kop.KwHaste})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	p := &s.perms[sl]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = v.HillGiant
	s.copyExcAdd(k, sl, NoSlot, ov[0]) // the token's own basis: "except it has haste"

	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("precondition: the token copy lacks its haste exception")
	}
	if !s.aegisUnderPush(sl) {
		t.Fatal("the effect pool refused the covered copy")
	}
	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 10)
	s.aegisRefresh(k, sl, NoSlot)

	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("with the Aegis copy the bearer shows card %d, want %d", got, v.SerraAngel)
	}
	if s.kw(k, sl)&kop.KwHaste != 0 {
		t.Fatal("the Aegis copy still shows the covered copy's haste exception")
	}
	if s.copyRec(sl, fxAegisHeld) < 0 {
		t.Fatal("the covered copy's exception was not held aside")
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after the Aegis copy ends the bearer shows card %d, want the covered copy %d", got, v.HillGiant)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 {
		t.Fatal("the covered copy's haste exception did not come back")
	}
	if s.copyRec(sl, fxCopyExc) < 0 || s.copyRec(sl, fxAegisHeld) >= 0 {
		t.Fatal("the held exception records were not restored")
	}
}

// TestAegisEndKeepsEOTInheritedExceptions: a newer until-end-of-turn copy
// with no override of its own carries its template's exceptions (CR 707.3).
// While it is the top layer, the older Aegis copy ending must not drop them;
// when the EOT copy later ends, the covered token copy's own exception comes
// back.
func TestAegisEndKeepsEOTInheritedExceptions(t *testing.T) {
	sc, k, ov := aegisExcScen(t,
		kop.Override{AddKw: kop.KwHaste},
		kop.Override{AddKw: kop.KwFlying})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	p := &s.perms[sl]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = v.HillGiant
	s.copyExcAdd(k, sl, NoSlot, ov[0]) // "except it has haste"
	if !s.aegisUnderPush(sl) {
		t.Fatal("the effect pool refused the covered copy")
	}
	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 10)
	s.aegisRefresh(k, sl, NoSlot)

	tmpl := sc.perm(v.GrizzlyBears, 1)
	tsl := sc.slot(tmpl)
	s.copyExcAdd(k, tsl, NoSlot, ov[1]) // the template: "except it has flying"
	tmplCard := s.cardAt(tsl)
	if _, ok := s.cloneLayerPrep(k, sl, true, tmplCard, s.perms[tsl].obj, 0, 20); !ok {
		t.Fatal("the effect pool refused the EOT layer")
	}
	s.copyExcAdd(k, sl, tsl, 0)
	setCopy(sc, sl, tmplCard)

	if l := s.copyTop(sl, NoSlot); l.kind != copyEOT || l.idx != 0 {
		t.Fatalf("precondition: top layer kind %d idx %d, want the EOT copy with no override", l.kind, l.idx)
	}
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("precondition: the EOT copy lacks its inherited flying exception")
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != tmplCard {
		t.Fatalf("after the Aegis copy ends the bearer shows card %d, want the EOT copy %d", got, tmplCard)
	}
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("the EOT copy's inherited exception was dropped when the Aegis copy ended")
	}

	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after cleanup the bearer shows card %d, want the covered copy %d", got, v.HillGiant)
	}
	if s.kw(k, sl)&kop.KwHaste == 0 || s.kw(k, sl)&kop.KwFlying != 0 {
		t.Fatalf("after cleanup the bearer has keywords %b, want the covered copy's haste only", s.kw(k, sl))
	}
}

// TestAegisRefreshSkipsMatchingPermExceptions: a permanent copy layer whose
// template has exceptions and whose own override is none already shows the
// records the refresh would re-push, so an ending Aegis copy beneath it must
// leave the bearer untouched.
func TestAegisRefreshSkipsMatchingPermExceptions(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{AddKw: kop.KwFlying})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 10)
	tmpl := sc.perm(v.GrizzlyBears, 1)
	tsl := sc.slot(tmpl)
	s.copyExcAdd(k, tsl, NoSlot, ov[0]) // the perm marker's template: flying
	tmplCard := s.cardAt(tsl)
	if _, ok := s.cloneLayerPrep(k, sl, false, tmplCard, s.perms[tsl].obj, 0, 20); !ok {
		t.Fatal("the effect pool refused the perm layer")
	}
	s.copyExcAdd(k, sl, tsl, 0)
	s.clonePermNote(sl, s.perms[tsl].obj, 0, 20)
	setCopy(sc, sl, tmplCard)

	if l := s.copyTop(sl, NoSlot); l.kind != copyPerm || l.idx != 0 {
		t.Fatalf("precondition: top layer kind %d idx %d, want the perm copy with no override", l.kind, l.idx)
	}
	if !s.excShows(sl, copyLayer{kind: copyPerm, idx: 0, src: s.perms[tsl].obj}) {
		t.Fatal("the perm layer's exception records are not the ones its restore pushes")
	}
	before := s.Touches()
	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != tmplCard {
		t.Fatalf("after the Aegis copy ends the bearer shows card %d, want the perm copy %d", got, tmplCard)
	}
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("the perm copy's inherited exception was dropped")
	}
	if s.Touches() != before {
		t.Fatal("the refresh re-did work the matching exception records already showed")
	}
}
