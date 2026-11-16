package oracle

import "testing"

// CR 406.3 with CR 118.9: Extract Power exiles the top card of each library
// face down and its controller may play them without paying their mana costs
// for as long as they remain exiled. The kernel lowered the Effect's grant
// (MayPlayWithoutManaCost$ beside a plain Card.IsRemembered) as an emblem
// static that remembers nothing, so it offered neither exiled card while
// gorge offered Facet Reader and Planar Despair (pair 890 of tail-17, decision
// 380 of 380). The recording plays on the custom table its own decks need: a
// tail pool's bucket membership is a property of the build that lowered the
// cards, and land's advancing lowering moved Catapult Fodder to tail-18, which
// failed the gate's pool check on a recording that pins a rule, not a deal.
func TestMBCFacetReaderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_facedown_free_play_grant")
}
