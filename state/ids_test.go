package state

import "testing"

// TestPlayerRefRoundTripsAndRealObjectIDsNeverDecode is PlayerRef/
// ObjID.PlayerRef's own guard test (FL-41): the sentinel bit only exists
// because a real object id must never collide with it, so this pins both
// halves -- encode-then-decode returns the original PlayerID, and every
// ObjID an ordinary match could plausibly produce (including right up to
// the bit itself) decodes as "not a player reference".
func TestPlayerRefRoundTripsAndRealObjectIDsNeverDecode(t *testing.T) {
	for _, p := range []PlayerID{0, 1, 7, 255} {
		id := PlayerRef(p)
		got, ok := id.PlayerRef()
		if !ok || got != p {
			t.Errorf("PlayerRef(%d) round-trip = %d, %v, want %d, true", p, got, ok, p)
		}
	}
	for _, id := range []ObjID{0, 1, 42, playerRefBit - 1} {
		if p, ok := id.PlayerRef(); ok {
			t.Errorf("ObjID(%d), a real object id, decoded as player reference %d", id, p)
		}
	}
}
