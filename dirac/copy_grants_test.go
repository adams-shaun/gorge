package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// An as-enters copy's AddAbilities$ grant (Mercurial Pretender) is one of the
// copy's copiable values (CR 707.9b): a newer copy layer covers it with the
// rest of the copy, and the layer under comes back with it. HungryGhoul stands
// in for the Pretender (its ability 0 is the granted one), HillGiant for the
// creature it copied.

// grantCopyScen puts a permanent whose printed card is the Ghoul on seat 0 as
// an as-enters copy of a Hill Giant that carries the grant (what
// becomeCopyEnter leaves), with its permanent copy marker when marker.
func grantCopyScen(t *testing.T, marker bool) (*scen, *prep, Slot, uint8) {
	t.Helper()
	sc, k, ov := aegisExcScen(t, kop.Override{Grant: [2]uint8{1, 0}})
	s := sc.s
	sl := sc.slot(sc.perm(v.HungryGhoul, 0))
	tmpl := sc.perm(v.HillGiant, 1)
	setCopy(sc, sl, v.HillGiant)
	s.copyGrantPut(k, sl, cgrantRef{ov[0], v.HungryGhoul})
	if marker {
		s.clonePermNote(sl, tmpl, v.HillGiant, 0, 10)
		s.clonePermGrant(sl, ov[0])
	}
	if n := s.copyGrantRecs(sl, new([maxOvr]cgrantRef)); n != 1 {
		t.Fatalf("precondition: %d grant records on the copy, want 1", n)
	}
	return sc, k, sl, ov[0]
}

func wantGrants(t *testing.T, s *State, sl Slot, want ...cgrantRef) {
	t.Helper()
	var got [maxOvr]cgrantRef
	n := s.copyGrantRecs(sl, &got)
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride && r.r.copyGrant() && r.r.fl&ovReps != 0 {
			t.Fatalf("grant record %d stands for %d applications: the grant was applied twice", i, r.r.reps())
		}
	}
	if n != len(want) {
		t.Fatalf("%d grant records %v, want %v", n, got[:n], want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("grant record %d is %v, want %v", i, got[i], want[i])
		}
	}
}

// TestCopyGrantCoveredByEOTCopyComesBackAtCleanup: a Pretender copy under an
// until-end-of-turn copy has no grant while it lasts, and has it again when the
// cleanup step re-bases it on the permanent copy marker under the ended layer.
func TestCopyGrantCoveredByEOTCopyComesBackAtCleanup(t *testing.T) {
	sc, k, sl, ov := grantCopyScen(t, true)
	s := sc.s
	tmpl := sc.perm(v.GrizzlyBears, 1)
	tsl := sc.slot(tmpl)
	if _, ok := s.cloneLayerPrep(k, sl, true, v.GrizzlyBears, tmpl, 0, 20); !ok {
		t.Fatal("the effect pool refused the layer")
	}
	s.copyExcAdd(k, sl, tsl, 0)
	s.copyGrantsAdd(k, sl, tsl)
	setCopy(sc, sl, v.GrizzlyBears)
	wantGrants(t, s, sl) // covered: the grant went with the copy it was a value of

	s.cloneCleanup(k)
	if l := s.copyTop(sl, NoSlot); l.kind != copyPerm || l.grant != ov {
		t.Fatalf("after cleanup the top layer is kind %d grant %d, want the permanent marker with grant %d", l.kind, l.grant, ov)
	}
	wantGrants(t, s, sl, cgrantRef{ov, v.HungryGhoul})
}

// TestCopyGrantCarriedByATemplateAndNotTwice: a copy layer whose template is a
// Pretender copy has its grant (CR 707.3), once even when the layer's own
// exception is the same grant.
func TestCopyGrantCarriedByATemplateAndNotTwice(t *testing.T) {
	sc, k, tsl, ov := grantCopyScen(t, false)
	s := sc.s
	sl := sc.slot(sc.perm(v.HungryGhoul, 0))
	setCopy(sc, sl, v.HillGiant)
	s.copyGrantsAdd(k, sl, tsl)
	s.copyGrantPut(k, sl, cgrantRef{ov, v.HungryGhoul}) // a Pretender copying a Pretender copy
	wantGrants(t, s, sl, cgrantRef{ov, v.HungryGhoul})
	if k.t.Overrides[ov-1].Grant[0] != 1 {
		t.Fatal("precondition: the grant override grants ability 0")
	}
}

// TestCopyGrantOfASelfCopyIsHeldBesideTheLayer: a Pretender copy that copies
// itself (CR 707.2) keeps its grant, a newer layer covers it, and the cleanup
// step gives it back from the records held beside the self layer.
func TestCopyGrantOfASelfCopyIsHeldBesideTheLayer(t *testing.T) {
	sc, k, sl, ov := grantCopyScen(t, true)
	s := sc.s
	host := s.perms[sl].obj
	var kept [maxOvr]cgrantRef
	n := s.copyGrantRecs(sl, &kept)
	if _, ok := s.cloneLayerPrep(k, sl, false, v.HillGiant, host, 0, 20); !ok { // cloneCopy's self layer, as selfCopyLayer
		t.Fatal("the effect pool refused the layer")
	}
	wantGrants(t, s, sl)
	for i := 0; i < n; i++ {
		s.copyGrantPut(k, sl, kept[i])
	}
	s.clonePermNote(sl, host, v.HillGiant, 0, 20)
	s.cloneSelfGrantsPush(sl, &kept, n, 20)
	wantGrants(t, s, sl, cgrantRef{ov, v.HungryGhoul})

	// an until-end-of-turn copy of another creature covers the self layer
	tmpl := sc.perm(v.GrizzlyBears, 1)
	if _, ok := s.cloneLayerPrep(k, sl, true, v.GrizzlyBears, tmpl, 0, 30); !ok {
		t.Fatal("the effect pool refused the layer")
	}
	setCopy(sc, sl, v.GrizzlyBears)
	wantGrants(t, s, sl)
	s.cloneCleanup(k)
	wantGrants(t, s, sl, cgrantRef{ov, v.HungryGhoul})
}

// TestCopyGrantHeldUnderAnAegisCopy: an Aegis copy covers a Pretender copy that
// has no marker; the grant is held aside while it shows and comes back as a
// live grant record when the Aegis copy ends.
func TestCopyGrantHeldUnderAnAegisCopy(t *testing.T) {
	sc, k, sl, ov := grantCopyScen(t, false)
	s := sc.s
	if !s.aegisUnderPush(sl) {
		t.Fatal("the effect pool refused the covered copy")
	}
	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 40)
	s.aegisRefresh(k, sl, NoSlot)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("with the Aegis copy the bearer shows card %d, want %d", got, v.SerraAngel)
	}
	wantGrants(t, s, sl)
	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after the Aegis copy the bearer shows card %d, want the Hill Giant", got)
	}
	wantGrants(t, s, sl, cgrantRef{ov, v.HungryGhoul})
}

// TestCopyGrantOnAPermanentShowingItsPrintedCard: a permanent whose printed
// card is the one it shows (a Hill Giant that a permanent copy effect made a
// copy of a Pretender copy of a Hill Giant) holds the Pretender's grant: the
// ability is the Ghoul's, hosted on the slot, never the Hill Giant's own
// ability 0 (which it has none of).
func TestCopyGrantOnAPermanentShowingItsPrintedCard(t *testing.T) {
	sc, k, ov := aegisExcScen(t, kop.Override{Grant: [2]uint8{1, 0}})
	s := sc.s
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	if s.cardAt(sl) != s.perms[sl].card || len(k.t.Cards[v.HillGiant].Abilities) != 0 {
		t.Fatal("precondition: the permanent shows another card or the Hill Giant has abilities")
	}
	s.copyGrantPut(k, sl, cgrantRef{ov[0], v.HungryGhoul})
	if got := s.selfGrantHost(k, sl, v.HungryGhoul, 0); got != sl {
		t.Fatalf("the Ghoul's granted ability 0 is hosted by slot %d, want %d", got, sl)
	}
	if got := s.grantCardAt(k, sl, 0); got != v.HungryGhoul {
		t.Fatalf("the granted ability 0 reads from card %d, want the Ghoul %d", got, v.HungryGhoul)
	}
}
