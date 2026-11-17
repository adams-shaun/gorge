package oracle

import (
	"testing"
)

// pair 488 of the tail-27 sweep (autopay surface, seed 6042300421767332857)
// parted at decision 190, p1's upkeep: gorge offered [pass] while the kernel
// offered [pass cast(#76{Tendrils of Corruption},-,0)]. p1's payment for the
// {3}{B} instant is four untapped sources (Plains, Swamp, Contested Cliffs,
// Tainted Isle), but Tainted Isle carries Brink of Disaster's "when enchanted
// permanent becomes tapped, destroy it" trigger, and the payment planner's
// source-interference classifier could not vet that trigger: it deferred the
// source ("source:interference:BrinkD"), the cast fell to the prefix-script
// search, and the autopay surface's vocabulary (payment actions with a plan,
// then the offered options) drops a scripted plain cast -- so gorge offered
// only pass while the kernel's census priced the four units and offered the
// cast.
//
// The destroy is a post-payment consequence, not interference: the mana was
// produced at the tap (CR 605.4a) and the trigger resolves after the payment
// that caused it (CR 405.6c/603.2), touching no payment resource. The
// classifier now vets exactly that shape (a `Mode$ Taps` trigger whose body
// is `DB$ Destroy | Defined$ TriggeredCardLKICopy`, engine/rules/pay
// planshape.go), the source stays an ordinary unit, and both engines offer
// the cast.
func TestTapDestroyTriggerSourceStaysPricedReproLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tap_destroy_trigger_source_stays_priced")
}
