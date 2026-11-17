package oracle

import "testing"

// CR 602.2b with CR 601.2g-h: an activation whose cost taps its own source can
// not also tap that source for the mana that pays the rest of the cost, so
// gorge's prefix search (scriptSearch.visit) never activates the play's own
// {T} source. Dining Room's "{4}, {T}: Investigate" under the opponent's Chronic
// Flooding is the shape: the Aura's Taps watcher defers a Dining Room's tap in
// gorge's planner, so each Dining Room is a census gap, and an activation of
// one is paid by tapping the other gaps for mana first. The kernel's
// gap-order search (dirac scriptDFS) tapped the play's own Dining Room in its
// prefix when that Dining Room came first in battlefield order, and withheld
// the activation gorge offers (decision 21: two flooded Dining Rooms and
// three Mountains; gorge offers both investigates, the kernel one).
// Venser, Fervent Forger ({4}{R}{R}, flash) is the card the six-land cast
// shape of the originating ticket rides on.
//
// The game is a custom-deck recording (kfuzz -synth tail-63 no longer deals
// Venser beside Dining Room): seat 0 plays Venser, Dining Room, Mountains and
// Forests; seat 1 plays Chronic Flooding and Islands.
func TestMBCVenserCastAffordableReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "venser_cast_affordable")
}
