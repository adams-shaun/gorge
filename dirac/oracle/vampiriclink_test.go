package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Vampiric Link lockstep: "Whenever enchanted creature deals damage, you
// gain that much life" (CR 120.3: the trigger fires for each damage event the
// enchanted creature is the source of, whatever deals it) on Plague Spitter,
// whose upkeep trigger is a DamageAll (1 damage to each creature and each
// player). One DamageAll call is ONE damage batch for the DamageDealtOnce
// latch (gorge effDamageAll's BeginDamageBatch): the once line queues once,
// its amount the batch's total. gorge leads; the expected answer at every
// decision is gorge's. The second matchup adds the opponent's creatures, so a
// sweep has more recipients (and its once line a bigger total).
var kvampiricMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Plague Spitter", Count: 4}, {Name: "Vampiric Link", Count: 8}, {Name: "Swamp", Count: 28},
	},
	{
		{Name: "Mountain", Count: 40},
	},
}, {
	{
		{Name: "Plague Spitter", Count: 4}, {Name: "Vampiric Link", Count: 8}, {Name: "Swamp", Count: 28},
	},
	{
		{Name: "Giant Cindermaw", Count: 8}, {Name: "Mountain", Count: 32},
	},
}}

func TestVampiricLinkDamageAllLockstep(t *testing.T) {
	t.Parallel()
	n := sonRun(t, "mbc", kvampiricMatchups, 30, true, nil, nil)
	if n != 0 {
		t.Errorf("%d of 60 games diverged", n)
	}
}
