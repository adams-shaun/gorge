package oracle

import "testing"

// CR 601.2g-h with CR 605.3b: p's census lists neither a grant (Jaheira's
// "{T}: Add {G}" on its Crab token) nor a Taps-deferred source (Hickory
// Woodlot's "{T}, Remove a depletion counter: Add {G}{G}", a rider gorge's
// planner defers), so paying Jaheira's {2}{G} is one prefix search over both
// gaps in battlefield order: the token's {G} floats first, then the Woodlot's
// {G}{G}, and the cast pays from the pool alone. The kernel ran the
// Taps-deferred script before the grant search — the grant-first arm in
// applyPriority was activation-only — floated the Woodlot's {G}{G} first and
// then tapped the Forest for the pip, tapping a different pair of permanents
// (Virtue's Ruin cast, dr-mbc-aabb pair 130 before the fix).
func TestMBCVirtueSRuinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_beside_grant_tapsdefer_first")
}
