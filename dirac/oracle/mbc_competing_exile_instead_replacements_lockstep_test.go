package oracle

import "testing"

// CR 616.1: when more than one replacement effect would modify one event, the
// affected object's controller chooses which applies first. Two Samurai of the
// Pale Curtain each exile a permanent that would be put into a graveyard from
// the battlefield, so one creature's death meets two competing "exile instead"
// replacements and gorge asks the order (decision 444, dr-mbc-baaa pair 52).
func replayCompetingExileInsteadReplacements(t *testing.T) {
	t.Helper()
	stdReproNamed(t, "competing_exile_instead_replacements_for_one_death")
}
