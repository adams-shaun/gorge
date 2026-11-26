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
// ability reduction (Bladegraft Aspirant, Power Artifact), the library-top
// look (Case of the Locked Hothouse, Field of Dreams) and the enduring-story
// carriers (Bifur, Melodic Rider's Panharmonicon; Dáin, Lord of the Iron
// Hills' attack tax; Fíli the Pathfinder's pump -- each prints K:Storied,
// the CR 702.195 latch the whole board's legendary bodies cross quickly).
// Burn, Unsummon-class bounce, pump and Swords to Plowshares read the target
// gates; Dawnglade's enters-trigger makes its controller the monarch so the
// monarch condition and Ramonda's cant-attack-you gate hold; the Dwarfs'
// entry and attack triggers are what Bifur's gated echo doubles. The deck
// stays 40 cards: Solitary Confinement and the Thought Eater pair, and an Island, made room
// (their classes keep Ivory Mask and the Minamo/Trusted pair). The
// FatefulHour / ExtraTurn / Night carriers are not in the pool: their
// statics refuse (gorge fails closed on the condition, reported). Kíli the
// Resourceful is not in the pool either: its AlternativeCost of activated
// equip abilities lowers nothing until the kernel prices an attach-cost
// alternative (gorge activate_static_altcost.go; dirac ticket
// dirac-kili-attach-alternative), so its EnduringStory condition has no
// lowered static to read.
// Seat 0's deck adds As Foretold, the {0} alternative over the full zone
// list (the hand alternative the kernel offers beside the plain cast,
// cast_mayzeroalt.go's hand hook); seat 1's adds Dracogenesis's free Dragon
// cast, Squee, the Immortal's two-zone self grant (one static per zone) and Dragon
// Whelp to feed both. Per controller exactly one alternative/free static
// family: the kernel's canon cannot name a second alternative index beside
// one (StallZeroAltShared); a second copy of As Foretold or Dracogenesis on
// the battlefield is a second such static, so each runs once. Conspiracy
// Unraveler's collect-evidence alternative rides its own mirror
// (staticzones_mayplay_test.go): a free plain cast beside it would share the
// cast's mode (StallZeroAltShared), so the free family and the evidence one
// do not share a deck.
var kstaticzonesDecks = [2][]cardset.Entry{
	{
		{Name: "As Foretold", Count: 1},
		{Name: "Ivory Mask", Count: 1}, {Name: "Solitary Confinement", Count: 1},
		{Name: "Minamo Scrollkeeper", Count: 2}, {Name: "Trusted Advisor", Count: 1}, {Name: "Thought Eater", Count: 1},
		{Name: "Dawnglade Regent", Count: 2}, {Name: "Queen Mother Ramonda", Count: 2}, {Name: "Entourage of Trest", Count: 1},
		{Name: "Spire Serpent", Count: 1}, {Name: "Slithering Shade", Count: 1}, {Name: "Geist of the Lonely Vigil", Count: 1},
		// the AffectedZone$ Stack keyword carriers (lower_stackkw.go): Cast
		// Through Time grants rebound to this deck's instants and sorceries
		{Name: "Cast Through Time", Count: 1}, {Name: "Opt", Count: 1}, {Name: "Sol Ring", Count: 1},
		{Name: "Dragon's Rage Channeler", Count: 2},
		{Name: "Case of the Locked Hothouse", Count: 1}, {Name: "Power Artifact", Count: 1},
		{Name: "Defensive Formation", Count: 1},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 2}, {Name: "Forest", Count: 2}, {Name: "Mountain", Count: 2},
		{Name: "Fíli the Pathfinder", Count: 2},
		{Name: "Bifur, Melodic Rider", Count: 2},
		{Name: "Dáin, Lord of the Iron Hills", Count: 2},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Swords to Plowshares", Count: 3}, {Name: "Giant Growth", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Dragon's Rage Channeler", Count: 2}, {Name: "Spire Serpent", Count: 2},
		{Name: "Case of the Locked Hothouse", Count: 2},
		{Name: "True Believer", Count: 2}, {Name: "Trusted Advisor", Count: 2},
		{Name: "Dracogenesis", Count: 1}, {Name: "Squee, the Immortal", Count: 2}, {Name: "Dragon Whelp", Count: 2},
		{Name: "Defensive Formation", Count: 2},
		{Name: "Plains", Count: 1}, {Name: "Island", Count: 1}, {Name: "Mountain", Count: 4},
	},
}

func TestStaticZonesLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kstaticzonesDecks, 200, "KSTATICZONES_SEEDS")
}

// TestStaticZonesMutation proves the pool exercises the new kernel reads: a
// kernel that reads no monarch makes the monarch-condition gates (Dawnglade
// Regent's hexproof, Entourage of Trest's extra block, Queen Mother
// Ramonda's cant-attack-you) differ from gorge's, so some seed diverges; so
// does a kernel that grants no enduring story (Bifur's gated echo, Dáin's
// tax and Fíli's pump go silent).
func TestStaticZonesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCondMonarchNone = true
	defer func() { dirac.DebugCondMonarchNone, dirac.DebugStoriedNone = false, false }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 64) {
		t.Fatal("DebugCondMonarchNone: no divergence in 64 seeds")
	}
	dirac.DebugCondMonarchNone = false // each mutation must diverge on its own
	dirac.DebugStoriedNone = true
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 64) {
		t.Fatal("DebugStoriedNone: no divergence in 64 seeds")
	}
}
