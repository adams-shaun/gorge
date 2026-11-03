package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A counter-removal ask names each option's kind; the named counters the
// kernel keeps above its featurized kinds must map to their own mode, or two
// kinds on different permanents canonicalise alike.
func TestCounterKindModeNamesTheNamedCounters(t *testing.T) {
	for name, want := range map[string]kop.CounterKind{
		"INCUBATION": kop.CtrIncubation, "BAIT": kop.CtrBait, "SOUL": kop.CtrSoul,
		"FELLOWSHIP": kop.CtrFellowship, "REVIVAL": kop.CtrRevival, "STASH": kop.CtrStash,
	} {
		if got := counterKindMode(name); got != int(want) {
			t.Errorf("counterKindMode(%q) = %d, want %d", name, got, want)
		}
	}
}
