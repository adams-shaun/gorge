package kgen

import (
	"strings"
	"testing"
)

// The hosted-Commander unread-param and counter shapes of ticket
// cli-20261127T065129Z-e7b92793: each card below was refused at lowering by
// an unread Forge param (or a PutCounter without a literal CounterType), so
// its deck blocked on dirac. The acceptance arm requires the corpus card to
// lower with no refusals; the lockstep arm (unreadHostedLockstep) then plays
// the card's deck against gorge seed by seed, and gorge leads: any
// decision-for-decision divergence fails the run.

// unreadHostedCards are the nine hosted cards and the shape each carries:
// the kgen api node, the param the lowering must read, and the gorge
// resolution it mirrors (engine/effects). The lockstep table keys off the
// card name.
var unreadHostedCards = []struct {
	name  string
	shape string // census reason fragment, for the failure message
}{
	{"Captain Marvel, Apex Avenger", "DB$ PutCounter: TriggeredCounterMap"},
	{"Moira and Teshar", "DB$ Pump: LeaveBattlefield"},
	{"Filigree Vector", "DB$ Pump: RememberTargets"},
	{"Nick Valentine, Private Eye", "DB$ Investigate: Optional"},
	{"Synth Eradicator", "DB$ PutCounter: Optional RememberPut"},
	{"Rise and Shine", "SP$ Animate: RememberAnimated"},
	{"Chandra, Awakened Inferno", "AB$ Effect: EffectOwner"},
	{"Jeweled Amulet", "AB$ PutCounter: RememberCostMana"},
	{"Urza's Bauble", "AB$ Reveal: Random ValidTgts"},
}

// TestUnreadParamsHostedLockstep: every hosted card of the ticket lowers with
// no refusals (the acceptance half of TestUnreadParamsHostedLockstep's
// behaviour; the lockstep arms below play each shape in a real game against gorge).
func TestUnreadParamsHostedLockstep(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(unreadHostedCards))
	for _, c := range unreadHostedCards {
		names = append(names, c.name)
	}
	shapes := map[string]string{}
	for _, c := range unreadHostedCards {
		shapes[c.name] = c.shape
	}
	for _, c := range lowerNamed(t, names) {
		if len(c.Refusals) == 0 {
			continue
		}
		var rs []string
		for _, r := range c.Refusals {
			rs = append(rs, r.Reason)
		}
		t.Errorf("%s (%s): refused: %s", c.Name, shapes[c.Name], strings.Join(rs, "; "))
	}
}
