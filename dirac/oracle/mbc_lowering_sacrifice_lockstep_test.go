package oracle

import "testing"

// CR 616.1: a Treasure sacrificed for the mana of a buyback {X} cast, with two
// Samurai of the Pale Curtain (each exiles a permanent put into a graveyard)
// on the battlefield, poses the controller's replacement-order ask. gorge's
// lowering taps the witness's sources at priority, one activation at a time,
// so the ask comes before the cast; the kernel sacrificed the Treasure inside
// the cast's own lowering and stalled (replleave) at decision 578 of 578
// (dr-mbc-baaa, autopay, pair 20).
func replayLoweringSacrificeExileOrder(t *testing.T) {
	t.Helper()
	stdReproNamed(t, "lowering_sacrifice_two_exile_replacements_asks")
}
