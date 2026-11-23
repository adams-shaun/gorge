package kgen

import (
	"fmt"
	"strings"
	"testing"
)

// TestGraveManaRefusal: Jack-o'-Lantern's graveyard mana line ("{1}, Exile
// Jack-o'-Lantern from your graveyard: Add one mana of any color") lowers with
// no refusal: the line lowers onto the byte (lower_gravemana.go's
// graveManaLine; dirac/mana_gravemana.go reads it), not into a mana ability
// the kernel's mana walk would have to read. The near-miss shapes keep a
// refusal that names the mechanism they wait on: a coloured fee and a non-Any
// production are a graveyard mana ability with a mana fee, and the fee-less
// CARDNAME exile keeps the generic graveyard-activation reason.
func TestGraveManaRefusal(t *testing.T) {
	jack := exileGraveLower(t, "Jack-o'-Lantern")
	if jack.GraveMana == 0 {
		t.Error("Jack-o'-Lantern: no GraveMana byte")
	}
	if got := keclReason(jack.Refusals); got != "" {
		t.Errorf("Jack-o'-Lantern: refusals %q, want none", got)
	}

	const body = "Name:Grave Mana Probe\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ %s | ActivationZone$ Graveyard | Produced$ %s | Amount$ 1\n"
	for _, c := range []struct {
		cost, produced string
		wantFee        bool
	}{
		{"G ExileFromGrave<1/CARDNAME>", "Any", true},
		{"1 ExileFromGrave<1/CARDNAME>", "Combo", true},
		{"ExileFromGrave<1/CARDNAME>", "Any", false},
	} {
		got := keclReason(discardHandSynth(t, fmt.Sprintf(body, c.cost, c.produced)))
		if got == "" {
			t.Errorf("cost %q produced %q: lowered with no refusal, want a graveyard mana refusal", c.cost, c.produced)
			continue
		}
		if has := strings.Contains(got, "a mana fee"); has != c.wantFee {
			t.Errorf("cost %q produced %q: refusals %q, names the fee = %v, want %v", c.cost, c.produced, got, has, c.wantFee)
		}
	}
}
