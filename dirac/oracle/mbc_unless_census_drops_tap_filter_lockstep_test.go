package oracle

import "testing"

// CR 605.3b with CR 601.2g: a mana ability's own mana fee is paid from the
// floating mana an earlier same-window activation produced, so a payer with an
// Island, a Plains and an Unknown Shores ("{T}: Add {C}. {1}, {T}: Add one mana
// of any color") can pay Conversion's upkeep "sacrifice unless you pay {W}{W}":
// the Island's {U} funds the Shores' {1} fee and the Shores makes the second
// {W}. gorge's unless offer gate prices that chain (UnlessManaReachable sorts
// the fee-free units first) and offers Pay beside Decline; the kernel's
// unless census drops the tap filter (scriptFilt's anyFiltFirst partition
// keeps only first-only filters), so it offered Decline only (decision 275 of
// 275; kfuzz tail-34-c001 pair 29 before the fix).
func TestMBCBilliardRoomReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_census_drops_tap_filter")
}
