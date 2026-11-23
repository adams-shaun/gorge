package oracle

import "testing"

// Dragon Tyrant's upkeep trigger ("sacrifice Dragon Tyrant unless you pay
// {R}{R}{R}{R}") is payable with three Mountains and a Graven Cairns
// ("{T}: Add {C}. {B/R}, {T}: Add {B}{B}, {B}{R}, or {R}{R}"): the payer taps
// the Mountains, pays the Cairns {B/R} fee from the floating {R} and taps it
// for {R}{R} (CR 605.3b, CR 107.4b). The kernel offered that pay/decline
// election; gorge's unless census priced no fee-carrying alt for a HYBRID
// activation fee (unlessFeeAlts's castWindowGenericCostShape gate) and offered
// decline only — decision 857 of the recorded game, tail-16 pair 16 seed
// 108315. The implicates line named Jayemdae Tome, the last-sourced cast, not
// the card at the divergence.
func TestMBCJayemdaeTomeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_hybrid_fee")
}

// CR 605.3a with CR 107.4b and CR 118.12: Minion of Tevesh Szat's "unless you
// pay {B}{B}" upkeep tax is payable by tapping Fetid Heath, whose "{W/B}, {T}:
// Add {B}{B}" activation is funded by any white or black mana (here a Plains).
// The gorge offer gate priced no hybrid-fee mana activation and posed only the
// decline option where dirac offered pay as well (decision 444 of 444, tail-01
// pair 20, mode(-,-,1) only one engine offers).
func TestMBCRathiTrapperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_hybrid_fee_filter_land")
}
