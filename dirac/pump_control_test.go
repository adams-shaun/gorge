package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pumpCtrlScen is a two-player scenario whose table carries one override row
// with the Aegis Angel pump's shape: a host-bound (WhileHost) keyword grant
// whose duration is AsLongAsControl. The spell program is not lowered here:
// the tests apply the row the way the interpreter's Override instruction does
// (applyWhileHost over the resolving entry).
func pumpCtrlScen(t *testing.T) (*scen, CardID, int) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lc := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Savannah Lions" {
			lc = CardID(i)
		}
	}
	tbl.Overrides = append(tbl.Overrides, kop.Override{
		AddKw: kop.KwCantBlock, CantAttack: true,
		WhileHost: true, AsLongAsControl: true,
	})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyWhileHost {
		t.Fatal("prep.anyWhileHost is false with a host-bound override")
	}
	return sc, lc, len(tbl.Overrides) - 1
}

// pumpOn applies the scenario's AsLongAsControl override to target with src
// as the resolving entry's host (the ability's source) under player ctrl.
func (sc *scen) pumpOn(target, src Obj, ctrl uint8, idx int) {
	e := &sc.s.res.e
	e.ctrl, e.obj = ctrl, src
	e.srcVer = sc.s.objs[src].ver
	sc.s.applyWhileHost(sc.k, sc.slot(target), idx, e)
}

// pumpCtrlCount counts the scenario's AsLongAsControl override records live
// on target.
func (sc *scen) pumpCtrlCount(target Obj, idx int) int {
	n := 0
	for i := 0; i < int(sc.s.g.nfx); i++ {
		if r := &sc.s.g.fx[i]; r.kind == fxOverride && r.sl == sc.slot(target) && int(r.r.idx)-1 == idx {
			n++
		}
	}
	return n
}

// pumpCtrlHeld reports whether the grant shows on target: the override's
// KwCantBlock bit in the layer-6 walk.
func (sc *scen) pumpCtrlHeld(target Obj) bool {
	return sc.s.kw6(sc.k, sc.slot(target))&kop.KwCantBlock != 0
}

// pumpCtrlCheckEndsOnHostControlChange (TestPumpControlEndsOnHostControlChange) pins CR 611.2b for a host-anchored
// Pump Duration$ AsLongAsControl (gorge expireAsLongAsControl on the host's
// ControlChange): the grant ends for good when the host changes controller
// and does not come back, and the pumped object's own control change leaves
// it alone.
func pumpCtrlCheckEndsOnHostControlChange(t *testing.T) string {
	sc, lc, idx := pumpCtrlScen(t)
	src, tgt := sc.perm(lc, 0), sc.perm(lc, 1)

	sc.pumpOn(tgt, src, 0, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 1 {
		return fmt.Sprintf("records %d, want 1", n)
	}
	if !sc.pumpCtrlHeld(tgt) {
		return fmt.Sprint("precondition: the granted can't block does not show")
	}

	// the pumped object's own control change is not the host's rule to end by
	sc.s.ctrlMove(sc.slot(tgt), 0)
	if n := sc.pumpCtrlCount(tgt, idx); n != 1 {
		return fmt.Sprintf("records %d after the pumped object changed controller, want 1", n)
	}

	// the host changes controller: the grant ends for good
	sc.s.ctrlMove(sc.slot(src), 1)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d after the host changed controller, want 0", n)
	}
	if sc.pumpCtrlHeld(tgt) {
		return fmt.Sprint("the can't block grant survived its host's control change")
	}

	// control of the host returns: the grant does not come back (CR 611.2b)
	sc.s.ctrlMove(sc.slot(src), 0)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d after the host's controller came back, want 0", n)
	}

	// the activator is seat 1: the record answers to seat 1, so the host
	// moving to seat 0 ends it (the seat rides the high byte of the record)
	sc, lc, idx = pumpCtrlScen(t)
	src, tgt = sc.perm(lc, 1), sc.perm(lc, 0)
	sc.pumpOn(tgt, src, 1, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 1 {
		return fmt.Sprintf("seat 1 activator: records %d, want 1", n)
	}
	sc.s.ctrlMove(sc.slot(src), 0)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("seat 1 activator: records %d after the host moved to seat 0, want 0", n)
	}
	return ""
}

// TestPumpControlEndsOnHostLeaving pins the host's battlefield departure
// ending the grant (gorge effectMoveSweep over the remembered host, CR
// 400.7): the record is gone, and the card that returns is a new object the
// old grant does not revive.
func pumpCtrlCheckEndsOnHostLeaving(t *testing.T) string {
	sc, lc, idx := pumpCtrlScen(t)
	src, tgt := sc.perm(lc, 0), sc.perm(lc, 1)

	sc.pumpOn(tgt, src, 0, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 1 {
		return fmt.Sprintf("records %d, want 1", n)
	}
	sc.s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d with the host in the graveyard, want 0", n)
	}
	if sc.pumpCtrlHeld(tgt) {
		return fmt.Sprint("the can't block grant survived its host's departure")
	}

	// a host already gone adds nothing
	sc.pumpOn(tgt, src, 0, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d from a host in the graveyard, want 0", n)
	}

	// it returns: the same card is a new object and a fresh grant holds
	sc.enterAs(src, 0)
	sc.pumpOn(tgt, src, 0, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 1 {
		return fmt.Sprintf("records %d after the host returned, want 1", n)
	}
	return ""
}

// TestPumpControlNotAppliedWithoutHostControl pins gorge checkValidDuration
// (combatfx.go registerPumpEffects): an AsLongAsControl pump whose host is
// not a battlefield permanent the activator controls applies nothing.
func pumpCtrlCheckNotAppliedWithoutHostControl(t *testing.T) string {
	sc, lc, idx := pumpCtrlScen(t)
	src, tgt := sc.perm(lc, 1), sc.perm(lc, 0)

	// the activator does not control the host
	sc.pumpOn(tgt, src, 0, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d from a host an opponent controls, want 0", n)
	}

	// the host is gone
	src2 := sc.perm(lc, 0)
	sc.s.leave(sc.k, sc.slot(src2), kop.ZoneGraveyard, false)
	e := &sc.s.res.e
	e.ctrl, e.obj = 0, src2
	e.srcVer = sc.s.objs[src2].ver
	sc.s.applyWhileHost(sc.k, sc.slot(tgt), idx, e)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d from a host in the graveyard, want 0", n)
	}
	return ""
}

// TestPumpControlEndsOnHostPhaseOut pins the host's phase-out ending the
// grant for good (gorge expireAsLongAsControl on the PhaseOut fold, CR
// 611.2b, 702.26g): it does not come back when the host phases in.
func pumpCtrlCheckEndsOnHostPhaseOut(t *testing.T) string {
	sc, lc, idx := pumpCtrlScen(t)
	src, tgt := sc.perm(lc, 0), sc.perm(lc, 1)

	sc.pumpOn(tgt, src, 0, idx)
	if n := sc.pumpCtrlCount(tgt, idx); n != 1 {
		return fmt.Sprintf("records %d, want 1", n)
	}
	sc.s.phaseOut(sc.k, sc.slot(src), false)
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d after the host phased out, want 0", n)
	}
	if sc.pumpCtrlHeld(tgt) {
		return fmt.Sprint("the can't block grant survived its host's phase-out")
	}

	// the host phases in: the grant stays gone (CR 611.2b)
	sc.s.phaseIn(sc.k, sc.slot(src))
	if n := sc.pumpCtrlCount(tgt, idx); n != 0 {
		return fmt.Sprintf("records %d after the host phased back in, want 0", n)
	}
	return ""
}

func TestPumpControlEndsOnHostControlChange(t *testing.T) {
	if m := pumpCtrlCheckEndsOnHostControlChange(t); m != "" {
		t.Fatal(m)
	}
}

func TestPumpControlEndsOnHostLeaving(t *testing.T) {
	if m := pumpCtrlCheckEndsOnHostLeaving(t); m != "" {
		t.Fatal(m)
	}
}

func TestPumpControlNotAppliedWithoutHostControl(t *testing.T) {
	if m := pumpCtrlCheckNotAppliedWithoutHostControl(t); m != "" {
		t.Fatal(m)
	}
}

func TestPumpControlEndsOnHostPhaseOut(t *testing.T) {
	if m := pumpCtrlCheckEndsOnHostPhaseOut(t); m != "" {
		t.Fatal(m)
	}
}

// TestPumpControlMutation: with each hook disabled by its DebugNo... switch,
// the test that owns the hook's rule fails (and the others' rules are
// untouched).
func TestPumpControlMutation(t *testing.T) {
	for _, c := range []struct {
		name string
		sw   *bool
		fn   func(*testing.T) string
	}{
		{"creation check", &DebugNoOvCtrlCheck, pumpCtrlCheckNotAppliedWithoutHostControl},
		{"control-change drop", &DebugNoOvCtrlChange, pumpCtrlCheckEndsOnHostControlChange},
		{"host-left drop", &DebugNoOvCtrlLeft, pumpCtrlCheckEndsOnHostLeaving},
		{"phase-out drop", &DebugNoOvCtrlPhase, pumpCtrlCheckEndsOnHostPhaseOut},
	} {
		if m := c.fn(t); m != "" {
			t.Fatalf("%s: the check fails without the mutation: %s", c.name, m)
		}
		*c.sw = true
		m := c.fn(t)
		*c.sw = false
		if m == "" {
			t.Errorf("%s disabled: its test still passes, it does not reach the hook", c.name)
		}
	}
}
