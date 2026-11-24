package oracle

import "testing"

// TestMBCBloodAgeGeneralReprosLockstep replays the synthetic tail-08 game the
// evidence loop recorded. At decision 686 p1 controls eleven lands, The World
// Tree ("As long as you control six or more lands, lands you control have
// '{T}: Add one mana of any color'") and Rain of Filth's "Sacrifice this
// land: Add {B}" on every land; only a summoning-sick Swamp is untapped.
// gorge's prefix search sacrifices the census gaps in battlefield order and
// asks each source's abilities afresh at every node, so by the time it
// reaches that Swamp The World Tree is gone and its any-colour ability with
// it: no prefix pays Blood Age General {1}{R}, Iron Fist, Hero for Hire {1}{R}
// or Goblin Heelcutter {3}{R} / dash {2}{R}, and gorge leaves them out. The
// kernel's mirror of that search kept the grant lists it read at the root
// after a sacrifice step and tapped the Swamp for {R} below the sacrificed
// Tree, offering them (options options).
func TestMBCBloodAgeGeneralReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_lost_after_sac_step")
}
