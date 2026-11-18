package oracle

import (
	"testing"
)

// Life's Finale's "search target opponent's library" is a SubAbility$ whose
// ValidTgts$ is pre-asked at cast time (CR 601.2c), so its answer arrives at
// resolution in Ctx.SubPreAsk, not Ctx.Targets. The search path read only
// Ctx.Targets and fell back to the caster, offering their own library instead
// of the targeted opponent's (options options, decision 308).
func TestMBCAlphaMyrReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "targeted_search_subability_target")
}
