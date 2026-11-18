package oracle

import "testing"

// CR 601.2g with 405.6c: a Taps watcher that would fire on a mana tap
// (Brink of Disaster's "when enchanted permanent becomes tapped, destroy it"
// on the enchanted land) resolves only after the spell's announcement is
// complete, so the enchanted land is an ordinary payment source: a cast that
// can pay only through it (Forerunner of the Heralds' {3}{G} beside the two
// Swamps) is offered. The kernel plans it in the second pass its Taps-deferred
// gap route runs when the first pass proves no plan (dirac planTiered); the
// gorge planner had no second pass and withheld the cast. The recording is the
// first divergence of the tail-46 evidence focus (pair 1129, decision 423:
// options options, autopay, gorge leading), shrunk.
func TestMBCForerunnerOfTheHeraldsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdeferred_second_pass")
}
