package kgen

import (
	"strings"
	"testing"
)

// TestGraveManaRefusal: a mana ability activated from the graveyard whose cost
// is a mana fee beside the CARDNAME exile (Jack-o'-Lantern: "{1}, Exile
// Jack-o'-Lantern from your graveyard: Add one mana of any color") is refused
// by name. gorge lists it as a priority "activate" only while a mana floats,
// pays the fee from that pool and asks a colour (TestGraveManaGorgeProbe); the
// kernel has no graveyard source whose fee the pool pays, so the reason names
// the fee. A fee-less graveyard exile keeps the generic graveyard reason.
func TestGraveManaRefusal(t *testing.T) {
	const fee = "mana ability with a forage / exile-from-graveyard cost: graveyard mana ability with a mana fee"
	got := keclReason(exileGraveLower(t, "Jack-o'-Lantern").Refusals)
	if !strings.Contains(got, fee) {
		t.Errorf("Jack-o'-Lantern: refusals %q, want one naming %q", got, fee)
	}

	const body = "Name:Grave Mana Probe\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ %s | ActivationZone$ Graveyard | Produced$ Any | Amount$ 1\n"
	for _, c := range []struct {
		cost    string
		wantFee bool
	}{
		{"1 ExileFromGrave<1/CARDNAME>", true},
		{"G ExileFromGrave<1/CARDNAME>", true},
		{"ExileFromGrave<1/CARDNAME>", false},
	} {
		got := keclReason(discardHandSynth(t, strings.Replace(body, "%s", c.cost, 1)))
		if got == "" {
			t.Errorf("cost %q: lowered with no refusal, want a graveyard mana refusal", c.cost)
		}
		if has := strings.Contains(got, "a mana fee"); has != c.wantFee {
			t.Errorf("cost %q: refusals %q, names the fee = %v, want %v", c.cost, got, has, c.wantFee)
		}
	}
}
