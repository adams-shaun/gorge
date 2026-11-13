package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The first-strike damage step is a step of its own, so what floats in it is gone
// before regular damage (CR 500.4, 510.4), and the end of the step is what moves on.
func TestFirstStrikeStepEmptiesPools(t *testing.T) {
	var s State
	s.step = kop.StepFirstStrikeDamage
	s.pl[0].pool[kop.R] = 2
	s.pl[1].pool[kop.G] = 1
	s.advance(&prep{})
	if s.step != kop.StepDamage || s.ctl != ctlStepBegin {
		t.Fatalf("step %v ctl %v, want the regular damage step beginning", s.step, s.ctl)
	}
	if s.pl[0].pool[kop.R] != 0 || s.pl[1].pool[kop.G] != 0 {
		t.Fatalf("pools %v %v, want empty", s.pl[0].pool, s.pl[1].pool)
	}
}

// A pick already a target stays where it is, the others take the places left in
// order, and a place no pick reaches keeps its target (CR 115.7e).
func TestRetargetedKeepsAPickInPlace(t *testing.T) {
	for _, c := range []struct {
		name       string
		old, picks []TCode
		want       []TCode
	}{
		{"one moved", []TCode{1}, []TCode{2}, []TCode{2}},
		{"one kept", []TCode{1}, []TCode{1}, []TCode{1}},
		{"second kept, first moved", []TCode{1, 2}, []TCode{2, 3}, []TCode{3, 2}},
		{"both kept whatever the order", []TCode{1, 2}, []TCode{2, 1}, []TCode{1, 2}},
		{"partial answer", []TCode{1, 2}, []TCode{3}, []TCode{3, 2}},
		{"partial answer naming a target", []TCode{1, 2}, []TCode{2}, []TCode{1, 2}},
	} {
		var old, out [MaxOptions]TCode
		var picks [8]TCode
		copy(old[:], c.old)
		copy(picks[:], c.picks)
		ksrRetargeted(&old, len(c.old), &picks, len(c.picks), &out)
		for i, w := range c.want {
			if out[i] != w {
				t.Errorf("%s: %v, want %v", c.name, out[:len(c.want)], c.want)
				break
			}
		}
	}
}
