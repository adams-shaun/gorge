package oracle

import "testing"

// TestMBCPatagiaTigerReprosLockstep replays the tail-13-c000 lockstep game
// whose priority offer for p0 parted at casting Patagia Tiger ({4}{W}) and
// Westgate Regent ({3}{B}{B}) from five untapped mana sources (Tainted Field,
// a Swamp and three Treasures). One Treasure carried gorge's Relic
// Putrescence ("whenever enchanted artifact becomes tapped, its controller
// gets a poison counter"): gorge's payment planner deferred the enchanted
// source as tap interference and priced only four mana, so it offered neither
// cast, while the kernel priced all five and offered both. The trigger
// resolves after the payment that caused it (CR 405.6c, 603.2) and touches no
// payment resource, so the tap stays an ordinary unit and both casts are
// payable (CR 601.2b).
func TestMBCPatagiaTigerReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "patagia_tiger")
}
