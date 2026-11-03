package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Static grants of a triggered ability to a class of permanents (Locutus of
// Borg, Supplementation Node, Co-Option Adjutant, Antagonist Incorporator,
// Hive Mind Coprocessor, Bio-Asset Allocator): "artifact creatures you
// control have 'whenever this creature ...'" (Unctus: other blue creatures). Each artifact creature, the
// granting one included, holds its own trigger (CR 613.1f, 603.2); an
// opponent's artifact creature holds none.
func TestArtifactCreatureTriggerGrantLockstep(t *testing.T) {
	t.Parallel()
	a := []cardset.Entry{
		{Name: "Locutus of Borg", Count: 3}, {Name: "Supplementation Node", Count: 3}, {Name: "Co-Option Adjutant", Count: 3},
		{Name: "Antagonist Incorporator", Count: 3}, {Name: "Hive Mind Coprocessor", Count: 3}, {Name: "Bio-Asset Allocator", Count: 3},
		{Name: "Memnite", Count: 3}, {Name: "Ornithopter", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Plains", Count: 14},
	}
	b := []cardset.Entry{
		{Name: "Locutus of Borg", Count: 2}, {Name: "Supplementation Node", Count: 2}, {Name: "Memnite", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 20},
	}
	c := []cardset.Entry{
		{Name: "Unctus, Grand Metatect", Count: 4}, {Name: "Wind Drake", Count: 6}, {Name: "Memnite", Count: 4},
		{Name: "Locutus of Borg", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Island", Count: 21},
	}
	lockstepSet(t, "artifactgrant", [2][]cardset.Entry{a, b}, 64, "KARTIFACTGRANT_SEEDS")
	lockstepSet(t, "artifactgrant", [2][]cardset.Entry{c, a}, 64, "KARTIFACTGRANT_SEEDS")
}
