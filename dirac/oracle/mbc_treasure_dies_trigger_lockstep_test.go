package oracle

import "testing"

// A Treasure sacrificed for mana at priority fires Scrap Trawler's "another
// artifact you control dies" trigger; the Treasure token's mana value of 0
// leaves it no legal target, so gorge puts it on the stack and removes it
// (CR 603.3d) before priority returns. Two such Treasures spent one after the
// other, then a kicked Spell Contortion paid from their floating mana: the
// kernel asked for an order between two Scrap Trawler triggers where gorge
// had none left (dr tail-19 pair 92, decision 543 of 543).
func TestMBCSpellContortionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "treasure_dies_trigger_dropped_at_priority")
}
