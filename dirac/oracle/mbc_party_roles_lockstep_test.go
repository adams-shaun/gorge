package oracle

import (
	"testing"
)

// CR 700.8: Cascade Seer's "scry X, where X is the number of creatures in your
// party" counts a Cleric (Ajani's Sunstriker) and a Wizard (the Seer itself) as
// two. A matchup's subtype table keeps only the subtypes a filter or an effect
// names, and the party count names none, so Cleric, Rogue and Warrior dropped
// out and a Cleric filled no role: the kernel scried 1 where gorge scried 2, and
// the arrange ask offered one card to gorge's two (decision 503, pair 106 of
// pool tail-39, surface autopay).
func TestMBCTail39ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "party_roles_kept_in_the_matchup_table")
}
