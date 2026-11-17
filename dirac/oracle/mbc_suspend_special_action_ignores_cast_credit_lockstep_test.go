package oracle

import "testing"

// CR 702.62a: suspend is a special action -- "If you could begin to cast
// this card by putting it onto the stack from your hand, you may pay [cost]
// and exile it" -- not a spell cast, so a cast-time mana credit such as
// Inspiring Statuary's improvise grant prices none of it. Infiltrator il-Kor
// (Suspend 2—{1}{U}) in a hand whose nonartifact spells have improvise is
// therefore still suspendable for {1}{U} from the Islands, and gorge lists
// the action beside the plain cast. The kernel's potential planner treated
// the improvise credit as part of the special action's price, reported
// "cost:credit" and never offered it (decision 403 of pool tail-53 pair 6:
// the kernel's option list held cast(#53,-,15), the suspend cast, beside the
// plain cast(#53,-,0) gorge offered alone).
func TestMBCInfiltratorIlKorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "suspend_special_action_ignores_cast_credit")
}
