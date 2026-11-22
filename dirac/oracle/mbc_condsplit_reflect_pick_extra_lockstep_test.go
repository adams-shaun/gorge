package oracle

import "testing"

// A lucky Gemstone Caverns' any-colour arm beside an opponent's Mana Flare:
// the arm's colour ask picks W, so the Flare's reflected extra is one more W
// and the {1}{W} Rescue the Foal cast is payable from the one activation. The
// kernel's Taps-deferred search floated the arm's mana without that pick
// extra (addExtra stalls on a reflected pick) and offered only pass
// (dr-mbc-abbb, gorge leading, decision 110, options).
func TestMBCPegasusGuardianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "condsplit_reflect_pick_extra")
}
