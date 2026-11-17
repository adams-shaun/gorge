package oracle

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMBCColorIdentityRiderReprosLockstep pins the trigger-cost potential gate
// for a Produced$ ColorIdentity mana source a rider keeps out of the kernel's
// ReflectedIdentity case: Path of Ancestry's "{T}: Add one mana of any color in
// your commander's color identity. When that mana is spent..." shape. Gorge's
// payment-bound potential fold prices the source at the commander's identity
// colours (rules potential.go addPotentialIdentityColours), so Tolarian Kraken's
// own "may pay {1}" draw trigger is offered with only the untapped Path of
// Ancestry to fund it; before the kernel read the shape, it offered decline only
// (CR 903.4, 118.3).
func TestMBCColorIdentityRiderReprosLockstep(t *testing.T) {
	const name = "coloridentity_rider_trigger_cost"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}

	// The precondition: the recording is the divergence this test pins, not
	// merely a clean game. Gorge's menu offered the trigger's own may-pay cost
	// and the kernel's did not, and the seat holds both the identity source and
	// the trigger.
	d := r.Divergence
	if d == nil || d.What != "options" || d.Field != "options" {
		t.Fatalf("divergence = %+v, want a recorded options difference", d)
	}
	diff := optionsOnlyIn(d.RefOptions, d.KerOptions)
	if len(diff) != 1 || !strings.Contains(diff[0], "trigger_cost_pay") {
		t.Fatalf("reference-only options = %v, want exactly the trigger_cost_pay arm", diff)
	}
	for _, card := range []string{"Path of Ancestry", "Tolarian Kraken"} {
		if !stringIn(r.Cards[0], card) {
			t.Fatalf("p0's deck has no %s; cards = %v", card, r.Cards[0])
		}
	}

	// The assertion: on the recorded surface the game now replays in lockstep
	// to its end, paying the cost from the identity source.
	stdReproNamed(t, name)
}
