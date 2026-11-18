package oracle

import (
	"testing"
)

// CR 605.3a: a player may activate a mana ability while paying a spell's
// cost, and the ability's own mana cost (Cave of Temptation's "{1}, {T}: Add
// one mana of any color") is paid from mana already floating or from other
// sources activated first. At tail-76 pair 9, decision 57, the UG seat holds
// Stealth Mission ({2}{U}, "Put two +1/+1 counters on target creature you
// control") with only Forest, Cave of Temptation and Tinder Wall
// (sacrifice: Add {R}{R}) on the battlefield: the cast is affordable only
// through the Cave's costed any-colour ability plus the Wall's sac mana. The
// kernel offers cast(Stealth Mission); gorge's planner never priced it and
// did not offer the cast at all.
func TestMBCStealthMissionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_costed_mana_ability")
}
