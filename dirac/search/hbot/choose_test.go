package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestFirstPip: a hybrid symbol offers its colours clockwise from the first the
// symbol prints, whichever the kernel lists first.
func TestFirstPip(t *testing.T) {
	const w, u, b, r, g = 0, 1, 2, 3, 4
	for _, tc := range []struct{ x, y, want uint8 }{
		{w, u, w}, {u, b, u}, {b, r, b}, {r, g, r}, {g, w, g},
		{w, b, w}, {u, r, u}, {b, g, b}, {r, w, r}, {g, u, g},
	} {
		var d dirac.Decision
		d.N = 2
		d.Options[0] = dirac.MakeAction(dirac.AkChoose, 0, 0, tc.x)
		d.Options[1] = dirac.MakeAction(dirac.AkChoose, 0, 0, tc.y)
		if got := d.Options[firstPip(&d)].Mode(); got != tc.want {
			t.Errorf("{%d/%d}: first %d, want %d", tc.x, tc.y, got, tc.want)
		}
	}
}
