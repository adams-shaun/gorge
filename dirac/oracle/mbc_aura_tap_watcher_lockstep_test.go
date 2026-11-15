package oracle

import "testing"

// Contaminated Ground (CR 305.7: "enchanted land is a Swamp"; "whenever
// enchanted land becomes tapped, its controller loses 2 life") sits on a
// Swamp of the player beside Dauthi Jackal. gorge's payment planner vets only
// a literal-damage Taps body, so the LoseLife watcher defers the enchanted
// land: its census never lists it, and the autopay surface drops a plain cast
// only the prefix script that taps it pays. The kernel left the land out of
// its Taps-deferred gap mask, because the setter took the land's printed
// abilities (its intrinsic {B} is no lost ability), counted it as an ordinary
// unit and offered Cirith Ungol Patrol and Sever Soul (decision 328 of
// tail-55 pair 321). The recording is pooled custom: tail-55's membership
// moves with the lowered set, and the repro's own decks must not depend on it.
func TestMBCDauthiJackalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_tap_watcher_defers_enchanted_land")
}
