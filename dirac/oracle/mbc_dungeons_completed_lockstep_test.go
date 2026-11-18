package oracle

import "testing"

// CR 309.7 / 700.4: Undermountain Adventurer's "{T}: Add {G}{G}. If you've
// completed a dungeon, add six {G} instead." Once its controller has completed
// a dungeon the untapped Adventurer pays Javelin of Lightning's {4} equip, so
// both engines offer the activation. Gorge read DungeonsCompleted$Amount as 0
// and offered only the two-mana production (decision 735 of tail-69-c000).
func TestMBCJavelinOfLightningReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dungeons_completed_mana")
}
