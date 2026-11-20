package oracle

import "testing"

// CR 601.2f: Tear Asunder's kicked cast beside Esior, Wardwing Familiar's
// "spells your opponents cast that target Esior cost {3} more to cast" is a
// target-priced cast, so gorge's potential planner prices it only from its
// census -- Song of Freyalise's granted "{T}: Add one mana of any color" is a
// census gap -- and the census/prefix fallback declines a target-priced
// cast's prefix search (potentialTargetPriced), leaving the play
// census_incomplete; the autopay surface drops it. The kernel offered the
// kicked cast off its potential bound (dr-mbc-bbab pair 41, decision 621).
func TestMBCTearAsunderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kick_tgtpriced_census_nosearch")
}
