package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// aegisExcScen builds a vanilla table copy with one appended copy-exception
// override ("except it's 1/1 and it has haste"), so the copy-exception
// readers can be exercised without a corpus card that carries one. The
// appended override is referenced by no table ability, so the prep flags are
// turned on by hand.
func aegisExcScen(t *testing.T) (*scen, uint8) {
	t.Helper()
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1) // generated prep: sized gen.flags for ptGen/refreshTy
	tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
	tbl.Overrides = append(tbl.Overrides, kop.Override{HasPT: true, SetP: 1, SetT: 1, AddKw: kop.KwHaste})
	idx := uint8(len(tbl.Overrides)) // fxCopyExc stores index+1
	sc := newScenTable(t, &tbl, Config{})
	sc.k.gen.cpxPT = true // no table ability references the appended override
	return sc, idx
}

// TestAegisCopyDropsCloneExceptions: an Aegis copy over a permanent clone
// marker whose copy has an "except it's 1/1, except it has haste" exception
// reads the exiled card's copiable values while it is on top (CR 707.2); the
// marker's exceptions come back with the marker when the Aegis ends (CR
// 707.9b).
func TestAegisCopyDropsCloneExceptions(t *testing.T) {
	sc, idx := aegisExcScen(t)
	s, k := sc.s, sc.k
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	addCopyPerm(sc, sl, sc.perm(v.GrizzlyBears, 1), idx, 10)
	s.copyExcPush(sl, idx)
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	setCopy(sc, sl, v.GrizzlyBears)

	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("precondition: the clone is %d/%d, want the marker's 1/1 exception", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste == 0 {
		t.Fatal("precondition: the marker's haste exception is not settled")
	}

	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 20)
	s.aegisRefresh(k, sl, NoSlot)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("Aegis copy shows card %d, want the exiled card %d", got, v.SerraAngel)
	}
	if p, tt := s.ptGen(k, sl); p != 4 || tt != 4 {
		t.Fatalf("Aegis copy is %d/%d, want Serra Angel 4/4", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste != 0 {
		t.Fatal("Aegis copy keeps the covered clone's haste, want the exiled card's keywords only")
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want the marker's %d", got, v.GrizzlyBears)
	}
	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("after the Aegis ends the marker is %d/%d, want its 1/1 exception back", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste == 0 {
		t.Fatal("after the Aegis ends the marker's haste exception is missing")
	}
}

// TestAegisCopySameCardDropsCloneExceptions is the same scenario with the
// bearer's printed card, the marker's source and the exiled card all equal,
// so the copy write is a no-op and only the exception set distinguishes the
// Aegis copy from the clone beneath it. It must not take the early return.
func TestAegisCopySameCardDropsCloneExceptions(t *testing.T) {
	sc, idx := aegisExcScen(t)
	s, k := sc.s, sc.k
	sl := sc.slot(sc.perm(v.SerraAngel, 0))
	addCopyPerm(sc, sl, sc.perm(v.SerraAngel, 1), idx, 10)
	s.copyExcPush(sl, idx)
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	setCopy(sc, sl, v.SerraAngel)

	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("precondition: the clone is %d/%d, want the marker's 1/1 exception", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste == 0 {
		t.Fatal("precondition: the marker's haste exception is not settled")
	}

	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 20)
	s.aegisRefresh(k, sl, NoSlot)
	if p, tt := s.ptGen(k, sl); p != 4 || tt != 4 {
		t.Fatalf("same-card Aegis copy is %d/%d, want the exiled card's 4/4", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste != 0 {
		t.Fatal("same-card Aegis copy keeps the covered clone's haste")
	}

	s.aegisUnlink(k, a, sl, false)
	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("after the same-card Aegis ends the marker is %d/%d, want its 1/1 back", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste == 0 {
		t.Fatal("after the same-card Aegis ends the marker's haste exception is missing")
	}
}

// TestAegisCopyDropsEOTCloneExceptions: the covered clone is an
// until-end-of-turn clone, which is still live when the Aegis ends, so the
// Aegis branch (not cloneCleanup) is the only path that can drop and restore
// its exceptions.
func TestAegisCopyDropsEOTCloneExceptions(t *testing.T) {
	sc, idx := aegisExcScen(t)
	s, k := sc.s, sc.k
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	addCopyEOT(sc, sl, v.GrizzlyBears, sc.perm(v.GrizzlyBears, 1), idx, 10)
	s.copyExcPush(sl, idx)
	s.perms[sl].ovr.kw |= s.copyExcKw(k, sl)
	setCopy(sc, sl, v.GrizzlyBears)

	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("precondition: the EOT clone is %d/%d, want its 1/1 exception", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste == 0 {
		t.Fatal("precondition: the EOT clone's haste exception is not settled")
	}

	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 20)
	s.aegisRefresh(k, sl, NoSlot)
	if p, tt := s.ptGen(k, sl); p != 4 || tt != 4 {
		t.Fatalf("Aegis copy over the EOT clone is %d/%d, want 4/4", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste != 0 {
		t.Fatal("Aegis copy over the EOT clone keeps its haste")
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want the live EOT clone %d", got, v.GrizzlyBears)
	}
	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("after the Aegis ends the live EOT clone is %d/%d, want its 1/1 back", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste == 0 {
		t.Fatal("after the Aegis ends the live EOT clone's haste exception is missing")
	}
}

// TestAegisOverBaseKeepsBaseException guards the covered-clone condition: an
// Aegis copy over a copy no marker records must not drop that base copy's
// exceptions, because the base is not re-recorded and could not be restored
// (the known fxAegisBase gap). The Aegis copy itself still inherits them;
// that gap is not asserted here.
func TestAegisOverBaseKeepsBaseException(t *testing.T) {
	sc, idx := aegisExcScen(t)
	s, k := sc.s, sc.k
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	setCopy(sc, sl, v.GrizzlyBears)
	s.copyExcPush(sl, idx)
	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("precondition: the base copy is %d/%d, want its 1/1 exception", p, tt)
	}

	if !s.aegisUnderPush(sl) {
		t.Fatal("precondition: the base copy was not kept aside")
	}
	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 20)
	s.aegisRefresh(k, sl, NoSlot)
	if got := s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("Aegis copy shows card %d, want the exiled card %d", got, v.SerraAngel)
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the Aegis ends the base shows card %d, want %d", got, v.GrizzlyBears)
	}
	if p, tt := s.ptGen(k, sl); p != 1 || tt != 1 {
		t.Fatalf("base is %d/%d after the Aegis ends, want its 1/1 back", p, tt)
	}
}

// TestAegisCopyDropsBatchEOTCloneExceptions: a batch-made until-end-of-turn
// clone has no template object (src NoObj), so its exception set is its own
// override only. Slot 0 carries an unrelated exception; the Aegis refresh
// must not read it as the clone's template and hand it to the bearer when
// the Aegis ends.
func TestAegisCopyDropsBatchEOTCloneExceptions(t *testing.T) {
	sc, idx := aegisExcScen(t)
	s, k := sc.s, sc.k
	// Slot 0's live exception is what a zero Slot would name as the
	// template for a marker with no template of its own.
	s0 := sc.slot(sc.perm(v.HillGiant, 0))
	s.copyExcPush(s0, idx)
	if s.copyRec(s0, fxCopyExc) < 0 {
		t.Fatal("precondition: slot 0 has no copy exception record")
	}
	sl := sc.slot(sc.perm(v.HillGiant, 1))
	addCopyEOT(sc, sl, v.GrizzlyBears, NoObj, 0, 10)
	setCopy(sc, sl, v.GrizzlyBears)
	if p, tt := s.ptGen(k, sl); p != 2 || tt != 2 {
		t.Fatalf("precondition: the batch EOT clone is %d/%d, want Grizzly Bears 2/2", p, tt)
	}
	if p, tt := s.ptGen(k, s0); p != 1 || tt != 1 {
		t.Fatalf("precondition: slot 0 is %d/%d, want its 1/1 exception", p, tt)
	}

	a := addAegisLayer(t, sc, sl, v.SerraAngel, v.SerraAngel, 20)
	s.aegisRefresh(k, sl, NoSlot)
	if p, tt := s.ptGen(k, sl); p != 4 || tt != 4 {
		t.Fatalf("Aegis copy over the batch EOT clone is %d/%d, want 4/4", p, tt)
	}

	s.aegisUnlink(k, a, sl, false)
	if got := s.cardAt(sl); got != v.GrizzlyBears {
		t.Fatalf("after the Aegis ends the bearer shows card %d, want the live EOT clone %d", got, v.GrizzlyBears)
	}
	if p, tt := s.ptGen(k, sl); p != 2 || tt != 2 {
		t.Fatalf("batch EOT clone (src NoObj) is %d/%d after the Aegis ends, want Grizzly 2/2", p, tt)
	}
	if s.perms[sl].ovr.kw&kop.KwHaste != 0 {
		t.Fatal("the batch EOT clone kept slot 0's haste after the Aegis ends")
	}
}
