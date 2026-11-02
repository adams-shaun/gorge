package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Innkeeper's Talent lockstep: the Class's level-1 combat
// +1/+1 counter, level 2's ward {1} for your permanents with counters (a
// ClassBand$ 2 static) and level 3's "if you would put counters on a
// permanent or player, put twice that many" (ReplDoubleCountersYou under
// ClassBand$ 3), beside infect creatures on both sides (your -1/-1 counters
// and poison are doubled, the opponent's are not), targeted removal to pay
// ward against, and vanilla creatures. The expected answer at every
// decision is gorge's.
var kinnkeeperDecks = [2][]cardset.Entry{
	{
		{Name: "Innkeeper's Talent", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Plains", Count: 5},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 3},
		{Name: "Glistener Elf", Count: 4}, {Name: "Hill Giant", Count: 2}, {Name: "Swords to Plowshares", Count: 2},
	},
	{
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 7},
		{Name: "Walking Corpse", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Plague Stinger", Count: 4},
		{Name: "Ichorclaw Myr", Count: 2}, {Name: "Doom Blade", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Innkeeper's Talent", Count: 2},
	},
}

// gorgeInnkeeperFixTime is gorge cr-player-counter-doubling (4f1770faa), the
// commit time in UTC: before it gorge never doubles a player's counters
// (rules/replacement_match.go failed ValidObject$ ...,Player closed on a
// PlayerCounterChange), so the lockstep diverged on poison.
const gorgeInnkeeperFixTime = "20261026101743"

func TestInnkeeperLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeInnkeeperFixTime) {
		t.Skip("gorge pin predates cr-player-counter-doubling (4f1770faa): player counters are never doubled")
	}
	reg, set, decks, tbl := p3Setup(t, "innkeeper", kinnkeeperDecks)
	p3Lockstep(t, "innkeeper", reg, set, decks, tbl, 600)
}
