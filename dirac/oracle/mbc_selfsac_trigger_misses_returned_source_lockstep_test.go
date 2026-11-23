package oracle

import (
	"testing"
)

// CR 112.7a with CR 400.7: Hellspark Elemental's printed end-of-turn sacrifice
// trigger ("At the beginning of the end step, sacrifice CARDNAME") was waiting
// on the stack when Rescue the Foal exiled and returned it in the same end
// step. gorge (Forge's DB$ Sacrifice SacValid$ Self) sacrifices the card that
// came back to the battlefield; the kernel's RefSelf read pins the push-time
// version and resolves to nothing, so the unearthed Hellspark stayed on the
// battlefield with its haste and P/T (decision 296 of dr-mbc-abbb pair 760,
// seed 16801408355894942116). gorge's own incarnation gate
// (rules/stack.go, SourceIncarnation) is armed only for a keyword trigger that
// demonstrably acts on its source (engine/events/apply_trigger.go); a printed
// trigger is not stamped, so the Self read finds the returned object.
func TestMBCHellsparkElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "selfsac_trigger_misses_returned_source")
}
