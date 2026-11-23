package oracle

import (
	"testing"
)

// CR 701.38a with CR 603.3c: a council's dilemma ballot is asked at
// resolution, each voter in turn starting with the trigger's controller --
// a triggered Vote's Choices$ are not CR 603.3c modal placement (gorge
// trigger_queue.go askTriggerModes excludes API "Vote"). The kernel lowered a
// triggered StoreVoteNum$ vote once per Choices$ option behind a placement
// Charm (gorge's old spurious placement mode, which ceace7cc00 removed), so
// on the ETB of Orchard Elemental it posed the controller a DkModes pick
// where gorge grants priority: tail-59-c000 pair 1 parts at decision 333
// (gorge [pass], kernel [mode 0 mode 1]). The lowering now emits the vote
// once, behind no Charm block.
func TestMBCOrchardElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "triggered_vote_placement_mode")
}
