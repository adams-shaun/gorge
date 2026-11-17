package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const ignoreLandwalkWall = "Name:Wall of Plains\nManaCost:2 W\nTypes:Enchantment\n" +
	"S:Mode$ IgnoreLandWalk | ValidKeyword$ Landwalk:Plains | Description$ x\nOracle:x\n"

// CR 702.14c + Great Wall: creatures with plainswalk can be blocked as though
// they didn't have plainswalk. Only the named walk is switched off.
func TestGreatWallLetsPlainswalkBeBlocked(t *testing.T) {
	t.Parallel()
	for _, wallSeat := range []state.PlayerID{0, 1} {
		e := combatEngine(t)
		a := landwalkAttacker(t, e, corpusCardAtPath(t, "b/boggart_arsonists.txt"))
		blocker := onBoard(t, e, 0, "Name:Blocker\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n")
		if land := onBoard(t, e, 0, "Name:Pl\nTypes:Basic Land Plains\nOracle:x\n"); e.G.Obj(land).Zone != state.ZBattlefield {
			t.Fatal("land precondition failed")
		}
		if e.canBlock(blocker, a) {
			t.Fatal("precondition: plainswalk attacker was blockable without Great Wall")
		}
		wall := onBoard(t, e, wallSeat, ignoreLandwalkWall)
		if e.G.Obj(wall).Zone != state.ZBattlefield {
			t.Fatal("wall precondition failed")
		}
		if !e.canBlock(blocker, a) {
			t.Fatalf("wall under seat %d: plainswalk attacker stayed unblockable", wallSeat)
		}
	}
}

// A wall naming a different walk leaves plainswalk intact; a bare static
// cancels every landwalk line.
func TestIgnoreLandWalkNamesItsKeyword(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, static string
		blockable    bool
	}{
		{"other walk", "S:Mode$ IgnoreLandWalk | ValidKeyword$ Landwalk:Swamp\n", false},
		{"bare", "S:Mode$ IgnoreLandWalk\n", true},
	} {
		e := combatEngine(t)
		a := landwalkAttacker(t, e, corpusCardAtPath(t, "b/boggart_arsonists.txt"))
		blocker := onBoard(t, e, 0, "Name:Blocker\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n")
		onBoard(t, e, 0, "Name:Pl\nTypes:Basic Land Plains\nOracle:x\n")
		onBoard(t, e, 0, "Name:W\nTypes:Enchantment\n"+tc.static+"Oracle:x\n")
		if got := e.canBlock(blocker, a); got != tc.blockable {
			t.Errorf("%s: canBlock = %v, want %v", tc.name, got, tc.blockable)
		}
	}
}

// ValidBlocker$ (Street Savvy): only the matching blocker ignores the walk.
func TestIgnoreLandWalkValidBlocker(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	a := landwalkAttacker(t, e, corpusCardAtPath(t, "b/boggart_arsonists.txt"))
	plain := onBoard(t, e, 0, "Name:Blocker\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n")
	special := onBoard(t, e, 0, "Name:Special\nManaCost:1\nTypes:Creature Elf\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 0, "Name:Pl\nTypes:Basic Land Plains\nOracle:x\n")
	onBoard(t, e, 0, "Name:W\nTypes:Enchantment\nS:Mode$ IgnoreLandWalk | ValidBlocker$ Creature.Elf | Secondary$ True\nOracle:x\n")
	if e.canBlock(plain, a) {
		t.Error("a non-Elf blocker ignored the walk")
	}
	if !e.canBlock(special, a) {
		t.Error("the Elf blocker did not ignore the walk")
	}
}

func TestIgnoreLandWalkCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Great Wall", "Crevasse", "Quagmire"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
