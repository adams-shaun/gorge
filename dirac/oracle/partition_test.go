package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The partition lockstep: Soul Partition exiles a nonland
// permanent; for as long as it remains exiled its owner may cast it, an
// opponent of the caster paying {2} more. Both decks play Soul Partition,
// so a seat exiles its own permanents too (no surcharge), and creatures, an
// Aura and an artifact give it targets. The expected answer at every
// decision is gorge's.
var kpartitionDecks = [2][]cardset.Entry{
	{
		{Name: "Soul Partition", Count: 6}, {Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 3},
		{Name: "Pacifism", Count: 3}, {Name: "Mind Stone", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Plains", Count: 12}, {Name: "Island", Count: 8},
	},
	{
		{Name: "Soul Partition", Count: 4}, {Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 4},
		{Name: "Wind Drake", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 7},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 3},
	},
}

// Soul Partition's Duration$ Permanent may-play survives cleanup from gorge
// dcadfc2f7 (effectUntilEOT read the duration after the instant/sorcery test),
// carried by the pin.
//
// gorgeMayPlayOwnerFixTime is gorge cr-mayplay-cardowner (82e371193,
// 2026-10-26 09:01:16 UTC): before it gorge withheld the MayPlayPlayer$
// CardOwner permission.
const gorgeMayPlayOwnerFixTime = "20261026090116"

func TestPartitionLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeMayPlayOwnerFixTime) {
		t.Skip("gorge before cr-mayplay-cardowner (82e371193): the owner's may-play is withheld")
	}
	lockstepSet(t, "partition", kpartitionDecks, 800, "KPARTITION_SEEDS")
}

// TestPartitionLockstepMutation proves the pool casts a partitioned card
// with the surcharge: without it, some seed diverges.
func TestPartitionLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	if !gorgePinAtLeast(t, gorgeMayPlayOwnerFixTime) {
		t.Skip("gorge before cr-mayplay-cardowner (82e371193)")
	}
	dirac.DebugNoPartitionRaise = true
	defer func() { dirac.DebugNoPartitionRaise = false }()
	if !lockstepDiverges(t, "partition", kpartitionDecks, 200) {
		t.Fatal("no seed diverged without the surcharge: the pool never casts a partitioned card as an opponent")
	}
}
