package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A basic under a granted basic land type lists its own colour before the granted ones
// (CR 305.6; gorge chars.appendLandTypes, owedAbilitiesFirst): an Island under Leyline owing
// {W}{U} floats {U}, a Forest owing {W}{G} floats {G}.
func TestBasicLandGrantWindowOwnColourFirst(t *testing.T) {
	for _, tc := range []struct {
		land string
		cost kop.ManaCost
		want int
	}{
		{"Island", cost(0, kop.W, kop.U), kop.U},
		{"Forest", cost(0, kop.W, kop.G), kop.G},
	} {
		for _, useful := range []bool{false, true} {
			sc, objs := blgScen(t, "Leyline of the Guildpact", tc.land)
			c := tc.cost
			if sc.s.windowOwnMask(sc.k, sc.slot(objs[1]))&0x1f != 0x1f || sc.tapped(objs[1]) {
				t.Fatalf("%s: precondition: the land must be untapped and make every colour under Leyline", tc.land)
			}
			if useful {
				sc.s.windowTapUseful(sc.k, 0, &c, &c, true, false, true, false)
			} else {
				sc.s.windowTap(sc.k, 0, &c, false)
			}
			if !sc.tapped(objs[1]) {
				t.Fatalf("%s (useful %v): not tapped", tc.land, useful)
			}
			var want [kop.NumManaKinds]uint8
			want[tc.want] = 1
			if sc.s.pl[0].pool != want {
				t.Errorf("%s (useful %v): pool %v, want %v", tc.land, useful, sc.s.pl[0].pool, want)
			}
		}
	}
}
