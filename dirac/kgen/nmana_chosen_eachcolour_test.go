package kgen

import (
	"strings"
	"testing"
)

// TestLowerNmanaChosenEachcolour: Tarnation Vista ("{T}: Add one mana of the
// chosen color" beside "{1}, {T}: For each color among monocolored permanents
// you control, add one mana of that color") is refused by name: the second
// ability is a fee-paid each-colour census gap the kernel does not model
// (refuseMultiMana). Out-of-shape variants keep the generic refusal: a
// fee-free each-colour ability beside the chosen one, and the fee-paid
// each-colour ability beside a fixed-colour first ability that is not chosen.
func TestLowerNmanaChosenEachcolour(t *testing.T) {
	const (
		named   = "a fee-paid each-colour gap the kernel does not model"
		generic = "2 mana abilities:"
		chosen  = "A:AB$ Mana | Cost$ T | Produced$ Chosen"
		feeEach = "A:AB$ Mana | Cost$ 1 T | Produced$ Special EachColorAmong_Valid Permanent.YouCtrl+MonoColor"
	)
	// Split so no source line repeats a Forge script line (GPL boundary).
	choose := []string{"K:ETBReplacement:Other:" + "ChooseColor", "SVar:ChooseColor:DB$ ChooseColor | " + "Defined$ You"}
	for _, c := range []struct {
		name, want string
		named      bool
		lines      []string
	}{
		{"Tarnation Vista", named, true, append([]string{chosen, feeEach}, choose...)},
		{"fee-free each-colour beside chosen", generic, false,
			append([]string{chosen, "A:AB$ Mana | Cost$ T | Produced$ Special EachColorAmong_Valid Permanent.YouCtrl+MonoColor"}, choose...)},
		{"fee-paid each-colour beside a sacrifice-cost chosen", generic, false,
			append([]string{"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Chosen", feeEach}, choose...)},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := stationProbeRefusals(t, c.lines...)
			if got == "" || !strings.Contains(got, c.want) {
				t.Fatalf("want a refusal containing %q, got %q", c.want, got)
			}
			if strings.Contains(got, named) != c.named {
				t.Errorf("named refusal present = %v, want %v: %q", !c.named, c.named, got)
			}
		})
	}
}
