package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// genPrepOf is the scenario's prep with generated-table flags, for the copy
// layer paths whose refreshTy reads prep.gen. It is a copy: the table's prep
// is shared by every scenario.
func genPrepOf(sc *scen) *prep {
	k := *sc.k
	k.gen = new(genPrep)
	return &k
}

// addAegisLayer puts a live Assimilation Aegis copy on host: an attachment
// permanent at the battlefield, its exiled creature card, and the link that
// names it (zone_exilethenattach.go). ts is the attachment's layer stamp.
func addAegisLayer(t *testing.T, sc *scen, host Slot, attachment, exiled CardID, ts uint16) Slot {
	t.Helper()
	a := sc.perm(attachment, 1)
	sl := sc.slot(a)
	ex := sc.obj(exiled, 1)
	sc.s.place(ex, kop.ZoneExile, false)
	i := int(sc.s.g.nlinks)
	sc.s.g.links[i].src = a
	sc.s.g.links[i].sver = sc.s.objs[a].ver
	sc.s.g.links[i].obj = ex
	sc.s.g.nlinks++
	sc.s.g.linkAeg |= 1 << uint(i)
	sc.s.perms[sl].attached = host
	sc.s.lstamp[sl] = ts
	return sl
}

// addCopyPerm records host's permanent copy marker (fxCopyPerm).
func addCopyPerm(sc *scen, host Slot, src Obj, idx uint8, ts uint16) {
	sc.s.g.fx[sc.s.g.nfx] = fxRec{sl: host, kind: fxCopyPerm, r: ovRec{p: int16(src), idx: idx, ts: ts}}
	sc.s.g.nfx++
	sc.s.fxM.set(host)
}

// addCopyEOT records host's until-end-of-turn copies (fxCopyEOT).
func addCopyEOT(sc *scen, host Slot, card CardID, src Obj, idx uint8, ts uint16) {
	sc.s.g.fx[sc.s.g.nfx] = fxRec{sl: host, kind: fxCopyEOT, r: ovRec{src: card, p: int16(src), idx: idx, ts: ts}}
	sc.s.g.nfx++
	sc.s.fxM.set(host)
}

// setCopy shows card c as host's copy.
func setCopy(sc *scen, host Slot, c CardID) {
	p := &sc.s.perms[host]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = c
}

// TestCloneCleanupGivesBackAegisOverPermanentClone is the first review case:
// a permanent copy marker under an Aegis copy, then an until-end-of-turn
// clone on top. At cleanup the clone ends and the newest layer left is the
// Aegis copy (CR 613.7e), not the older permanent marker.
func TestCloneCleanupGivesBackAegisOverPermanentClone(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	bearer := sc.perm(v.HillGiant, 0)
	sl := sc.slot(bearer)
	y := sc.perm(v.GrizzlyBears, 1) // the permanent clone's source
	addCopyPerm(sc, sl, y, 0, 10)
	addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 20)
	addCopyEOT(sc, sl, v.GrizzlyBears, NoObj, 0, 30)
	setCopy(sc, sl, v.GrizzlyBears)
	k := genPrepOf(sc)

	if l := s.copyTop(sl, NoSlot); l.kind != copyEOT || l.card != v.GrizzlyBears {
		t.Fatalf("precondition: top layer kind %d card %d, want the EOT clone", l.kind, l.card)
	}
	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("after cleanup the bearer shows card %d, want the Aegis copy %d (the newer layer left)", got, v.SerraAngel)
	}
	if s.copyRec(sl, fxCopyEOT) >= 0 {
		t.Fatal("the ended until-end-of-turn layer is still recorded")
	}
	if s.copyRec(sl, fxCopyPerm) < 0 {
		t.Fatal("the permanent copy marker was lost")
	}
}

// TestAegisEndGivesBackCloneUnderSecondAegis is the second review case: a
// clone between two Aegis copies. The second Aegis copy is newer and shows
// while it lives; when it ends the clone (newer than the first Aegis copy)
// shows again (CR 613.7e), and its own cleanup gives the first Aegis copy
// back.
func TestAegisEndGivesBackCloneUnderSecondAegis(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 10)
	addCopyEOT(sc, sl, v.GrizzlyBears, NoObj, 0, 20)
	a2 := addAegisLayer(t, sc, sl, v.HillGiant, v.HillGiant, 30)
	setCopy(sc, sl, v.HillGiant)
	k := genPrepOf(sc)

	if l := s.copyTop(sl, NoSlot); l.kind != copyAegis || l.card != v.HillGiant {
		t.Fatalf("precondition: top layer kind %d card %d, want the second Aegis copy", l.kind, l.card)
	}
	s.aegisUnlink(k, a2, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the second Aegis ends the bearer shows card %d, want the live clone %d", got, v.GrizzlyBears)
	}
	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("after cleanup the bearer shows card %d, want the first Aegis copy %d", got, v.SerraAngel)
	}
}

// TestEOTEndsBeneathNewerAegis: the until-end-of-turn clone is under a newer
// Aegis copy when it ends; the cleanup step must leave the Aegis copy shown
// and, when the Aegis copy later ends, give back the next layer (the older
// Aegis copy here), not the printed card.
func TestEOTEndsBeneathNewerAegis(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	addAegisLayer(t, sc, sl, v.GrizzlyBears, v.SerraAngel, 10)
	addCopyEOT(sc, sl, v.HillGiant, NoObj, 0, 20)
	a2 := addAegisLayer(t, sc, sl, v.GrizzlyBears, v.HillGiant, 30)
	setCopy(sc, sl, v.HillGiant)
	k := genPrepOf(sc)

	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.HillGiant {
		t.Fatalf("after cleanup the bearer shows card %d, want the newer Aegis copy %d", got, v.HillGiant)
	}
	s.aegisUnlink(k, a2, sl, false)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("after the newer Aegis ends the bearer shows card %d, want the older Aegis copy %d", got, v.SerraAngel)
	}
}

// TestAegisEndKeepsNewerPermanentClone: a permanent copy marker newer than
// every Aegis copy stays shown when the until-end-of-turn layer above it
// ends and when the Aegis copy ends.
func TestAegisEndKeepsNewerPermanentClone(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	a1 := addAegisLayer(t, sc, sl, v.HillGiant, v.SerraAngel, 10)
	addCopyEOT(sc, sl, v.HillGiant, NoObj, 0, 20)
	y := sc.perm(v.GrizzlyBears, 1)
	addCopyPerm(sc, sl, y, 0, 30)
	setCopy(sc, sl, v.GrizzlyBears)
	k := genPrepOf(sc)

	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after cleanup the bearer shows card %d, want the newer permanent clone %d", got, v.GrizzlyBears)
	}
	s.aegisUnlink(k, a1, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want the permanent clone %d", got, v.GrizzlyBears)
	}
}

// TestCloneCleanupKeepsPermanentBetweenAegisAndClone: the permanent copy
// marker is newer than the Aegis copy and older than the until-end-of-turn
// clone. When the clone ends the marker shows (CR 613.7e), and it stays when
// the Aegis copy later ends.
func TestCloneCleanupKeepsPermanentBetweenAegisAndClone(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	bearer := sc.perm(v.SavannahLions, 0)
	sl := sc.slot(bearer)
	a1 := addAegisLayer(t, sc, sl, v.HillGiant, v.SerraAngel, 10)
	y := sc.perm(v.GrizzlyBears, 1)
	addCopyPerm(sc, sl, y, 0, 20)
	addCopyEOT(sc, sl, v.HillGiant, NoObj, 0, 30)
	setCopy(sc, sl, v.HillGiant)
	k := genPrepOf(sc)

	s.cloneCleanup(k)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after cleanup the bearer shows card %d, want the permanent clone %d", got, v.GrizzlyBears)
	}
	s.aegisUnlink(k, a1, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want the permanent clone %d", got, v.GrizzlyBears)
	}
}
