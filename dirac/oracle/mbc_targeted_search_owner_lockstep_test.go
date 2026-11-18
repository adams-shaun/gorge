package oracle

import "testing"

// Life's Finale's "search target opponent's library" is a chained ChangeZone
// whose opponent target is announced as the spell's sub-target (Ctx.SubPreAsk)
// rather than Ctx.Targets. The search-owner resolver read only Ctx.Targets, so
// it fell back to the spell's controller and offered that player's own library
// (dr-mbc pair 6, decision 477: the kernel searched the target's library).
func TestMBCASunbathingRootwallaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "targeted_search_owner")
}
