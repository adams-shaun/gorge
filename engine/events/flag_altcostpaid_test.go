package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestFlagAltCostPaidRoundTrips: the appended altcostpaid name round-trips
// and leaves the earlier flags' spelling untouched.
func TestFlagAltCostPaidRoundTrips(t *testing.T) {
	if state.FlagAltCostPaid == 0 {
		t.Fatal("precondition: FlagAltCostPaid overflowed the CastFlags word")
	}
	s := FlagsString(state.FlagKicked | state.FlagAltCostPaid)
	if s != "kicked,altcostpaid" {
		t.Fatalf("FlagsString = %q, want kicked,altcostpaid", s)
	}
	if got := FlagsFrom(s); got != state.FlagKicked|state.FlagAltCostPaid {
		t.Fatalf("FlagsFrom(%q) = %b", s, got)
	}
	if got := FlagsString(state.FlagBeamedUp); got != "beamedup" {
		t.Fatalf("FlagsString(FlagBeamedUp) = %q", got)
	}
}
