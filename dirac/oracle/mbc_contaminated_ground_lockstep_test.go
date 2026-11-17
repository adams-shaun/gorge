package oracle

import "testing"

// CR 305.7 with CR 605.3b: Contaminated Ground enchants a land the opponent's
// deck owns and turns it into a Swamp (its rules-text mana ability gone, the
// intrinsic {B} in its place), and its "whenever enchanted land becomes
// tapped" watcher puts gorge's payment planner off tapping that land (the
// Defer class). Two lockstep halves:
//
//   - The priority offer. The land-type setter's view of the land names no
//     mana ability of its card, which kept the land out of the Taps-deferred
//     census gaps (and a lost-abilities land reads as unactivatable), so the
//     kernel priced a plain Aura cast with the deferred land and offered it
//     where gorge's planner finds the cast insufficient and the autopay
//     surface drops the scripted prefix (decision 79 of 263).
//   - The manual payment window. A madness cast posed mid-resolution has no
//     plan and no auto-fill, so gorge's window answer is the first source
//     listed: the deferred land in its battlefield place, which the kernel's
//     window walk skipped (decision 263 of 263).
func TestMBCContaminatedGroundManualWindowLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "land_set_taps_deferred_source")
}
