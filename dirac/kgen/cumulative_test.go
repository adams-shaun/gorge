package kgen

import (
	"strings"
	"testing"
)

// TestCumulativeUpkeepLowering pins the lowering of a cumulative upkeep with
// a plain generic cost (CR 702.24a; gorge kwCumulativeUpkeep's expansion):
// Mystic Remora (and every card with K:Cumulative upkeep:<N>) is the age
// counter (ExtCumulative), the cost once per age counter as the trigger's own
// pay-or-decline ask, and the sacrifice of the source behind it.
func TestCumulativeUpkeepLowering(t *testing.T) {
	cards := lowerNamed(t, []string{"Mystic Remora", "Fyndhorn Pollen"})
	if len(cards) != 2 {
		t.Fatalf("lowered %d cards, want 2", len(cards))
	}
	for _, c := range cards {
		if len(c.Refusals) != 0 {
			t.Fatalf("%s refused: %v", c.Name, c.Refusals)
		}
		var dump strings.Builder
		dumpLIR(&dump, c)
		got := dump.String()
		for _, want := range []string{
			"Cumulative C=2",
			"UnlessPaid obj=You A=CountersOn(Self,AGE)*",
			"CS=Mana skip=1",
			"Sacrifice obj=You A=1 F={types=0x0 Self}",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, want, got)
			}
		}
	}
}

// TestCumulativeUpkeepRefusesOtherCosts: only a plain generic cost lowers. A
// coloured cost, a life payment and the action costs of gorge's
// parseCumulativeAction (sacrifice a land) stay refused by name, never read
// as a generic one.
func TestCumulativeUpkeepRefusesOtherCosts(t *testing.T) {
	for _, name := range []string{"Illusionary Wall", "Arnjlot's Ascent", "Breath of Dreams"} {
		cards := lowerNamed(t, []string{name})
		if len(cards) != 1 {
			t.Fatalf("%s: lowered %d cards", name, len(cards))
		}
		found := false
		for _, r := range cards[0].Refusals {
			found = found || strings.Contains(r.Reason, "cumulative upkeep cost")
		}
		if !found {
			t.Errorf("%s: want a refusal naming the cumulative upkeep cost, got %v", name, cards[0].Refusals)
		}
	}
}
