package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The paid exile-population lockstep (dirac/kgen/lower_playexilepop.go, the
// kernel's exile filter population in dirac/cast_freecast.go): Boiling Rock
// Rioter's attack trigger casts an Ally its controller owns from among the
// cards exiled with it (Valid$ Ally.YouOwn+ExiledWithSource, every seat's
// exile walked, the opponent's exiled Allies filtered out), and Sanwell,
// Avenger Ace's tap trigger casts a Vehicle or artifact creature from its six
// remembered exiled cards (Valid$ Vehicle.IsRemembered,Artifact.Creature+
// IsRemembered). Both are paid plays (no WithoutManaCost$ True): gorge's
// beginPlay begins the ordinary cast and an unpayable cost reverses it
// (CR 601.2g, 733.1). Run as its own shard (-run '^TestPlayExilePopLockstep$').
func playExilePopDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Boiling Rock Rioter", Count: 4},
		{Name: "Sanwell, Avenger Ace", Count: 4},
		{Name: "Hada Freeblade", Count: 4},
		{Name: "Ondu Cleric", Count: 3},
		{Name: "Kazandu Blademaster", Count: 3},
		{Name: "Ornithopter", Count: 2},
		{Name: "Memnite", Count: 2},
		{Name: "Bronze Sable", Count: 2},
		{Name: "Smuggler's Copter", Count: 2},
		{Name: "Plains", Count: 8},
		{Name: "Swamp", Count: 6},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func TestPlayExilePopLockstep(t *testing.T) {
	t.Parallel()
	tally := new(psTally)
	lockstepRunPolicy(t, "playexilepop", playExilePopDecks(), 200, true, nil, nil, playShapesPolicy(tally))
	reg, _, _ := lockstepPool(t, "playexilepop", playExilePopDecks())
	ally, art := 0, 0
	for _, n := range tally.paidNames {
		c, ok := reg.Lookup(n)
		if !ok {
			continue
		}
		if c.Faces[0].HasTypeWord("Ally") {
			ally++
		} else {
			art++
		}
	}
	t.Logf("paid plays %d (Ally %d via Rioter, Vehicle/artifact creature %d via Sanwell), free %d", tally.paid, ally, art, tally.free)
	if ally < 1 || art < 5 {
		t.Errorf("paid exile plays: Ally %d, artifact %d; want at least 1 and 5, the lockstep proves nothing", ally, art)
	}
}
