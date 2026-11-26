package oracle

import (
	"testing"
)

// Emerald Dragonfly's "{G}{G}: gains first strike" is payable from the two
// untapped Eldrazi Spawn tokens (Elven Chorus grants them "{T}: Add one mana
// of any color") while all of the player's Forests are tapped (decision 968 of
// pair 66, pool tail-51, surface autopay): gorge offered pass only; the kernel
// offered the activation. gorge's prefix search branches a tapped Spawn's
// printed sacrifice and spends its potentialScriptNodes budget before reaching the play;
// the kernel's grantSrch skipped that branch (printedSacPass) and so reached
// it. Both engines now cut an indistinguishable funder twice over (gorge's
// scriptTwin, the grant search's fundTwinTried), reach the play inside the
// budget and offer it.
func TestMBCEmeraldDragonflyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_mana_tap_pays_activation")
	// the same activation a second time in one combat, the Forests tapped by
	// the first: gorge pays {G}{G} through Prismite's repeatable {2}: "add one
	// mana of any color" twice, the Eldrazi Spawn's {C} among the funding
	// (CR 605.3a, 602.2b). The kernel's search took the state after the
	// Spawn's sacrifice for the one before it (same pool, same tapped bits)
	// and dropped the line; gorge's key lists the battlefield objects, so the
	// two differ and its search funds the second {G} (pair 15, pool tail-51,
	// decision 882).
	stdReproNamed(t, "sacrificed_token_is_a_new_filter_search_state")
}
