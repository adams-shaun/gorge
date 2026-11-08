package oracle

import "testing"

// The farm's std-fdn lockstep (FDN-s409 game 16) parted from gorge on which land a
// cast tapped, with Dragonlord's Servant implicated: the Servant, enchanted by
// Imprisoned in the Moon, is a land whose only mana is the granted {T}: Add {C}, and
// Pyromancer's Goggles beside it is a deferred source. gorge's one prefix search
// taps the census gaps in battlefield order, the granted land before Goggles; the
// kernel tapped Goggles alone ahead of it. The pinned games have the Servant, the
// Aura and Goggles in play and must replay clean.
func TestPinnedFarmDragonlordsServant(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-fdn", Config{}, onBattlefield("Dragonlord's Servant"), onBattlefield("Imprisoned in the Moon"), onBattlefield("Pyromancer's Goggles"))
}
