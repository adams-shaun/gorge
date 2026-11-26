package cards_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestStationThresholdParamReadsAsCounterGate: Card-Forge acaef15177 spelled
// the Spacecraft STATION N+ static `Station$ N`; the parse reads it as the
// Affected$ Card.Self+counters_GE<N>_CHARGE gate it replaced, so a static
// reader that knows only the old spelling gates the effects on the charge
// counters. A threshold the gate cannot carry keeps its Station key.
func TestStationThresholdParamReadsAsCounterGate(t *testing.T) {
	line := func(rest string) map[string]string {
		s, ok := cards.ParseStaticLine("Mode$ Continuous | " + rest + " | AddKeyword$ Flying")
		if !ok {
			t.Fatalf("static line did not parse: %q", rest)
		}
		return s.Params
	}
	old := line("AffectedDefined$ Self | Affected$ Card.counters_GE8_CHARGE")
	nw := line("AffectedDefined$ Self | Station$ 8")
	if old["Affected"] != "Card.Self+counters_GE8_CHARGE" {
		t.Fatalf("precondition: old spelling reads %q", old["Affected"])
	}
	if nw["Affected"] != old["Affected"] {
		t.Errorf("Station$ 8 reads Affected %q, want %q", nw["Affected"], old["Affected"])
	}
	if _, left := nw["Station"]; left {
		t.Error("Station key survived the rewrite")
	}
	for _, rest := range []string{
		"AffectedDefined$ Self | Station$ 0", "AffectedDefined$ Self | Station$ +5", "AffectedDefined$ Self | Station$ 08", "AffectedDefined$ Self | Station$ 256", "AffectedDefined$ Self | Station$ X",
		"Affected$ Creature.YouCtrl | Station$ 3", "AffectedDefined$ Enchanted | Station$ 3",
	} {
		if _, kept := line(rest)["Station"]; !kept {
			t.Errorf("%q: Station key dropped for a shape the gate cannot carry", rest)
		}
	}
}
