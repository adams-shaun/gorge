package dirac

import "testing"

// TestAvailModeBitsAreDisjoint: every availX mode is its own bit. availOwn
// once shared availNoFilt's bit, so the bare-gaps census (availGaps|availNoFilt)
// kept an activation's own {T} source in it and the own-source walk
// (availGaps|availOwn) skipped the filter lands.
func TestAvailModeBitsAreDisjoint(t *testing.T) {
	modes := []struct {
		name string
		bit  uint16
	}{
		{"availGaps", availGaps}, {"availAny", availAny}, {"availPlain", availPlain},
		{"availWalk", availWalk}, {"availNoWide", availNoWide}, {"availWin", availWin},
		{"availWinCol", availWinCol}, {"availHyp", availHyp}, {"availNoFilt", availNoFilt},
		{"availLife", availLife}, {"availOwn", availOwn},
	}
	var seen uint16
	for _, m := range modes {
		if m.bit == 0 || m.bit&(m.bit-1) != 0 {
			t.Errorf("%s = %d is not a single bit", m.name, m.bit)
		}
		if seen&m.bit != 0 {
			t.Errorf("%s = %d shares a bit with an earlier avail mode", m.name, m.bit)
		}
		seen |= m.bit
	}
	if (availGaps|availNoFilt)&availOwn != 0 {
		t.Errorf("the bare-gaps census mask carries availOwn")
	}
	if (availGaps|availOwn)&availNoFilt != 0 {
		t.Errorf("the own-source walk mask carries availNoFilt")
	}
}
