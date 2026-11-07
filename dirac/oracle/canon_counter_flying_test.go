package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A RemoveAnyCounter ask naming a Flying counter (CR 122.1b) maps to the
// featurized CtrFlying, not to the P1P1 fallback, and the keyword and named
// counters above it keep their own modes.
func TestCounterKindModeNamesFlyingAndEveryKernelKind(t *testing.T) {
	for name, want := range map[string]kop.CounterKind{
		"Flying": kop.CtrFlying, "Loyalty": kop.CtrLoyalty, "LOYALTY": kop.CtrLoyalty,
		"Stun": kop.CtrStun, "Shield": kop.CtrShield, "M1M1": kop.CtrM1M1,
		"First Strike": kop.CtrFirstStrike, "CHARGE": kop.CtrCharge,
	} {
		if got := counterKindMode(name); got != int(want) {
			t.Errorf("counterKindMode(%q) = %d, want %d", name, got, want)
		}
	}
}
