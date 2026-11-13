package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// donatedSrcScen: P1 controls a Twinflame Tyrant and a Grizzly Bears, and the
// Bears' resolving ability (controller 0, the player who activated it before
// P1 took the Bears) is about to deal damage. The source is on the
// battlefield under P1, so its current controller differs from the ability's.
func donatedSrcScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	sc, id := shieldScen(t, "Twinflame Tyrant", "Grizzly Bears")
	bear := sc.perm(id["Grizzly Bears"], 1)
	sc.perm(id["Twinflame Tyrant"], 1)
	e := &sc.s.res.e
	e.kind = seAbility
	e.obj = bear
	e.srcVer = sc.s.objs[bear].ver
	e.card = sc.s.objs[bear].card
	e.ctrl = 0
	return sc, bear
}

// TestDamageToDonatedSourceUsesSourceController: a source donated after its
// ability was activated reads the doubler of its current controller, not the
// ability controller's. P0's ability deals 3 to P0; P1's Twinflame Tyrant
// doubles it because the source is now P1's and P0 is P1's opponent.
func TestDamageToDonatedSourceUsesSourceController(t *testing.T) {
	sc, _ := donatedSrcScen(t)
	s := sc.s
	// Preconditions: the object is where the rule reads it, and the two
	// controllers actually differ.
	if s.objRef(sc.k, kop.RefSelf) == NoSlot {
		t.Fatal("the source is not the resolving entry's self on the battlefield")
	}
	if s.srcCtrl(sc.k) != 1 {
		t.Fatalf("srcCtrl = %d, want 1", s.srcCtrl(sc.k))
	}
	if s.res.e.ctrl != 0 {
		t.Fatalf("ability controller = %d, want 0", s.res.e.ctrl)
	}
	if sc.k.gen.anyRepl&rbDblDmg == 0 {
		t.Fatal("rbDblDmg unset with Twinflame Tyrant in the table")
	}
	life := s.pl[0].life
	if n := s.damageTo(sc.k, uint8(PlayerTarget0), 3); n != 6 {
		t.Fatalf("damageTo returned %d, want 6", n)
	}
	if got := life - s.pl[0].life; got != 6 {
		t.Fatalf("P0 lost %d, want 6", got)
	}
	// One Tyrant matches, so the direct path dealt it rather than parking.
	if s.g.ro.n != 0 {
		t.Fatalf("a replacement order ask was parked (%d candidates)", s.g.ro.n)
	}
}

// TestDamageBatchDonatedSourceUsesSourceController: the batch path reads the
// same current controller of the source as the single-target path.
func TestDamageBatchDonatedSourceUsesSourceController(t *testing.T) {
	sc, _ := donatedSrcScen(t)
	s := sc.s
	if s.objRef(sc.k, kop.RefSelf) == NoSlot {
		t.Fatal("the source is not the resolving entry's self on the battlefield")
	}
	if s.srcCtrl(sc.k) != 1 {
		t.Fatalf("srcCtrl = %d, want 1", s.srcCtrl(sc.k))
	}
	if s.res.e.ctrl != 0 {
		t.Fatalf("ability controller = %d, want 0", s.res.e.ctrl)
	}
	if sc.k.gen.anyRepl&rbDblDmg == 0 {
		t.Fatal("rbDblDmg unset with Twinflame Tyrant in the table")
	}
	life := s.pl[0].life
	s.damageBatch(sc.k, []uint8{uint8(PlayerTarget0)}, []int{3}, false)
	if got := life - s.pl[0].life; got != 6 {
		t.Fatalf("P0 lost %d, want 6", got)
	}
}

// TestSrcCtrlDepartedDonatedSourceUsesLastKnownController: a source donated
// after its ability was activated, then removed with the ability still on the
// stack, reads its last-known controller from the departed battlefield record,
// not the ability's (CR 702.15b, 608.2h).
func TestSrcCtrlDepartedDonatedSourceUsesLastKnownController(t *testing.T) {
	sc, bear := donatedSrcScen(t)
	s := sc.s
	// Preconditions: the source is live and its controller differs from the
	// ability's, so the two candidate answers actually differ.
	if s.srcCtrl(sc.k) != 1 {
		t.Fatalf("precondition: live srcCtrl = %d, want 1", s.srcCtrl(sc.k))
	}
	if s.res.e.ctrl != 0 {
		t.Fatalf("precondition: ability controller = %d, want 0", s.res.e.ctrl)
	}
	s.leave(sc.k, sc.slot(bear), kop.ZoneGraveyard, false)
	// Preconditions: the source is gone from the battlefield, the departed
	// record exists, and its controller is 1 (not the ability's 0).
	if s.objRef(sc.k, kop.RefSelf) != NoSlot {
		t.Fatal("precondition: the source is still the resolving entry's self on the battlefield")
	}
	dsl := s.departedSrcSlot(&s.res.e)
	if dsl == NoSlot {
		t.Fatal("precondition: no departed battlefield record for the source")
	}
	if s.perms[dsl].ctrl != 1 {
		t.Fatalf("precondition: departed slot controller = %d, want 1", s.perms[dsl].ctrl)
	}
	if s.res.e.ctrl != 0 {
		t.Fatalf("precondition: ability controller = %d, want 0", s.res.e.ctrl)
	}
	if got := s.srcCtrl(sc.k); got != 1 {
		t.Fatalf("srcCtrl after departure = %d, want its last-known controller 1", got)
	}
}

// TestDamageToDepartedDonatedSourceUsesLastKnownController: the single-target
// damage path reads the departed source's last-known controller too, so P1's
// Twinflame Tyrant doubles the damage the ability deals to P0 (CR 702.15b,
// 608.2h).
func TestDamageToDepartedDonatedSourceUsesLastKnownController(t *testing.T) {
	sc, bear := donatedSrcScen(t)
	s := sc.s
	if s.srcCtrl(sc.k) != 1 {
		t.Fatalf("precondition: live srcCtrl = %d, want 1", s.srcCtrl(sc.k))
	}
	s.leave(sc.k, sc.slot(bear), kop.ZoneGraveyard, false)
	if s.objRef(sc.k, kop.RefSelf) != NoSlot {
		t.Fatal("precondition: the source is still the resolving entry's self on the battlefield")
	}
	if dsl := s.departedSrcSlot(&s.res.e); dsl == NoSlot || s.perms[dsl].ctrl != 1 {
		t.Fatalf("precondition: departed record controller, want slot with controller 1")
	}
	if s.res.e.ctrl != 0 {
		t.Fatalf("precondition: ability controller = %d, want 0", s.res.e.ctrl)
	}
	if sc.k.gen.anyRepl&rbDblDmg == 0 {
		t.Fatal("rbDblDmg unset with Twinflame Tyrant in the table")
	}
	life := s.pl[0].life
	if n := s.damageTo(sc.k, uint8(PlayerTarget0), 3); n != 6 {
		t.Fatalf("damageTo returned %d, want 6", n)
	}
	if got := life - s.pl[0].life; got != 6 {
		t.Fatalf("P0 lost %d, want 6", got)
	}
	if s.g.ro.n != 0 {
		t.Fatalf("a replacement order ask was parked (%d candidates)", s.g.ro.n)
	}
}
