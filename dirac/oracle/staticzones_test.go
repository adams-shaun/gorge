package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The gap-static-zones slice's sample lockstep (fixture staticzones): the
// shared-gate conditions (Monarch on Dawnglade Regent, Queen Mother Ramonda
// and Entourage of Trest; Delirium, Metalcraft and Hellbent on the
// combat-restriction modes of Dragon's Rage Channeler, Spire Serpent,
// Slithering Shade and Geist of the Lonely Vigil), player shroud (Ivory
// Mask, True Believer, Solitary Confinement), the additive maximum hand size
// (Minamo Scrollkeeper, Trusted Advisor, the Thought trio), the target-gated
// ability reduction (Bladegraft Aspirant, Power Artifact) and the library-top
// look (Case of the Locked Hothouse, Field of Dreams). Burn, Unsummon-class
// bounce, pump and Swords to Plowshares read the target gates; Dawnglade's
// enters-trigger makes its controller the monarch so the monarch condition
// and Ramonda's cant-attack-you gate hold. The FatefulHour / ExtraTurn /
// Night carriers are not in the pool: their statics refuse (gorge fails
// closed on the condition, reported).
var kstaticzonesDecks = [2][]cardset.Entry{
	{
		{Name: "Ivory Mask", Count: 3}, {Name: "Solitary Confinement", Count: 2},
		{Name: "Minamo Scrollkeeper", Count: 2}, {Name: "Trusted Advisor", Count: 2}, {Name: "Thought Eater", Count: 2},
		{Name: "Dawnglade Regent", Count: 2}, {Name: "Queen Mother Ramonda", Count: 2}, {Name: "Entourage of Trest", Count: 2},
		{Name: "Spire Serpent", Count: 2}, {Name: "Slithering Shade", Count: 2}, {Name: "Geist of the Lonely Vigil", Count: 1},
		{Name: "Dragon's Rage Channeler", Count: 2},
		{Name: "Case of the Locked Hothouse", Count: 1}, {Name: "Power Artifact", Count: 1},
		{Name: "Defensive Formation", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Swords to Plowshares", Count: 3}, {Name: "Giant Growth", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Dragon's Rage Channeler", Count: 2}, {Name: "Spire Serpent", Count: 2},
		{Name: "Case of the Locked Hothouse", Count: 2},
		{Name: "True Believer", Count: 2}, {Name: "Trusted Advisor", Count: 2},
		// Squee, the Immortal: the two-zone self MayPlay (AffectedZone$
		// Exile,Graveyard + EffectZone$ Exile,Graveyard), the slice's
		// self-grant carrier (ticket dirac-squee-two-zone-self-mayplay).
		{Name: "Squee, the Immortal", Count: 2},
		{Name: "Defensive Formation", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 1}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 2},
	},
}

func TestStaticZonesLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kstaticzonesDecks, 200, "KSTATICZONES_SEEDS")
}

// TestStaticZonesMutation proves the pool exercises the new kernel reads: a
// kernel that reads no monarch makes the monarch-condition gates (Dawnglade
// Regent's hexproof, Entourage of Trest's extra block, Queen Mother
// Ramonda's cant-attack-you) differ from gorge's, so some seed diverges.
func TestStaticZonesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCondMonarchNone = true
	defer func() { dirac.DebugCondMonarchNone = false }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 64) {
		t.Fatal("DebugCondMonarchNone: no divergence in 64 seeds")
	}
}
