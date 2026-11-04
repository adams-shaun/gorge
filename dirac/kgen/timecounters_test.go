package kgen

import (
	"strings"
	"testing"
)

// TestTimeCountersOnSelf: a card printed without Impending may put time
// counters on itself (a clock that counts up and trips at a threshold); the
// kernel's impending dormancy reads the counter only on an Impending card, so
// nothing goes dormant. A time counter put on any other permanent could land
// on an Impending card cast for its full cost, so that stays refused.
func TestTimeCountersOnSelf(t *testing.T) {
	self := "T:Mode$ Phase | Phase$ End of Turn | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ ProbeTick\n" +
		"SVar:ProbeTick:DB$ PutCounter | Defined$ Self | CounterType$ TIME | CounterNum$ 1"
	if r := kcrimeLower(t, self); len(r) != 0 {
		t.Fatalf("a self time counter is refused: %v", r)
	}
	other := strings.Replace(self, "Defined$ Self", "ValidTgts$ Permanent", 1)
	r := kcrimeLower(t, other)
	hit := false
	for _, x := range r {
		hit = hit || strings.Contains(x.Reason, "time counters outside Impending")
	}
	if !hit {
		t.Fatalf("a time counter on another permanent: want the Impending refusal, got %v", r)
	}
}
