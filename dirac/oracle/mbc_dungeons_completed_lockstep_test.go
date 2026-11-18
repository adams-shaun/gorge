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

// Undermountain Adventurer's "{T}: Add {G}{G}. If you've completed a dungeon,
// add six {G} instead" reads DungeonsCompleted$Amount (CR 309.7). Gorge's count
// evaluator had no such head and priced the tap at {G}{G}, so with the Adventurer
// and five other lands it never offered Orthion, Hero of Lavabrink's
// {6}{R}{R}{R},{T} ability, which the six {G} pay for (decision 400, tail-69).
func TestMBCOrthionHeroOfLavabrinkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dungeons_completed_count_head")
}
