package dirac_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// podBase is a four-seat world of twenty Plains a seat (ten in the hand, ten in
// the library) on turn 1 main phase, seat 3 departed: its cards ceased.
func podBase() dirac.World {
	w := dirac.World{Tbl: vanilla.Table, Cfg: dirac.LockstepConfig(dirac.SurfaceFull), Seed: 1, NSeat: 4, Turn: 1, Step: kop.StepMain1}
	for p := uint8(0); p < 4; p++ {
		w.Life[p] = 20
		for i := uint8(0); i < 20; i++ {
			o := dirac.WorldObj{Card: vanilla.Plains, Owner: p, Ctrl: p, Zone: kop.ZoneHand, Pos: i}
			if i >= 10 {
				o.Zone, o.Pos = kop.ZoneLibrary, i-10
			}
			if p == 3 {
				o.Zone, o.Pos = kop.ZoneNone, 0
			}
			w.Objs = append(w.Objs, o)
		}
	}
	w.Lost[3] = true
	return w
}

// TestStageDepartedRules: a world with a seat that has left stages with that
// seat out of the rotation, and each position the rules refuse returns its
// reason.
func TestStageDepartedRules(t *testing.T) {
	t.Parallel()
	w := podBase()
	s, err := dirac.NewFromWorld(&w)
	if err != nil {
		t.Fatalf("the departed world does not stage: %v", err)
	}
	var p dirac.Projection
	s.Project(&p)
	if !p.Lost[3] || p.Lost[0] || p.LibN[3] != 0 || p.HandN[3] != 0 {
		t.Errorf("projection: lost %v, seat 3 library %d hand %d", p.Lost, p.LibN[3], p.HandN[3])
	}
	// seat 2 holds priority after seats 0 and 1 passed; seat 3's turn slot is skipped
	w = podBase()
	w.Active, w.Prio = 2, 0
	if s, err = dirac.NewFromWorld(&w); err != nil {
		t.Fatalf("the holder after the departed seat does not stage: %v", err)
	}
	s.Project(&p)
	if p.Prio != 0 {
		t.Errorf("holder %d, want 0", p.Prio)
	}
	cases := map[string]struct {
		reason string
		mut    func(*dirac.World)
	}{
		"ceased object of a live seat": {dirac.ReasonZone, func(w *dirac.World) { w.Objs[3].Zone = kop.ZoneNone }},
		"lost seat in a library":       {dirac.ReasonZone, func(w *dirac.World) { w.Objs[60].Zone = kop.ZoneLibrary }},
		"ceased token":                 {dirac.ReasonToken, func(w *dirac.World) { w.Objs[79].Token = true }},
		"active seat lost":             {dirac.ReasonSeats, func(w *dirac.World) { w.Active, w.Prio = 3, 3 }},
		"priority holder lost":         {dirac.ReasonSeats, func(w *dirac.World) { w.Prio = 3 }},
		"one seat left":                {dirac.ReasonSeats, func(w *dirac.World) { w.Lost[1], w.Lost[2] = true, true }},
		"short lost deck":              {dirac.ReasonLayout, func(w *dirac.World) { w.Objs = w.Objs[:63] }},
		"lost seat out of range":       {dirac.ReasonSeats, func(w *dirac.World) { w.NSeat = 3 }},
	}
	for name, c := range cases {
		w := podBase()
		c.mut(&w)
		s, err := dirac.NewFromWorld(&w)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != c.reason || s != nil {
			t.Errorf("%s: got state %v, error %v, want reason %q", name, s != nil, err, c.reason)
		}
	}
}
