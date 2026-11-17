package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The ChangeZoneAll LibraryPosition$ / RandomOrder$ lockstep: Harmonic
// Convergence puts every enchantment on top of its owner's library
// (LibraryPosition$ 0, the CR 401.4 answer), and Zimone's Experiment puts the
// cards it did not keep on the bottom in a random order (LibraryPosition$ -1,
// RandomOrder$ True, the same Fisher-Yates draws the kernel's ExtLibPlace arm
// makes). Pacifism is the enchantment Harmonic sweeps, Terminus is the
// non-random bottom sweep (LibraryPosition$ -1 without RandomOrder$), and the
// vanilla creatures and lands fill the decks.
var kczallposDecks = [2][]cardset.Entry{
	{
		{Name: "Harmonic Convergence", Count: 4}, {Name: "Zimone's Experiment", Count: 4},
		{Name: "Pacifism", Count: 4}, {Name: "Terminus", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 11}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Harmonic Convergence", Count: 4}, {Name: "Zimone's Experiment", Count: 4},
		{Name: "Pacifism", Count: 4}, {Name: "Terminus", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 11}, {Name: "Plains", Count: 7},
	},
}

// kczallposTally counts the carriers' casts the policy chose (gorge options),
// so the test can fail when the pool never reaches them.
type kczallposTally struct {
	harmonic, zimone, terminus int
}

func kczallposCastName(c *PickCtx, a Canon) string {
	if c.G == nil {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok || o.Kind != "cast" {
		return ""
	}
	if f := c.G.E.G.Obj(o.Obj).Face(); f != nil {
		return f.Name
	}
	return ""
}

// kczallposPolicy casts Harmonic Convergence or Zimone's Experiment whenever
// one is offered, else picks at random.
func kczallposPolicy(tl *kczallposTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			var want []Canon
			for _, a := range c.D.Options {
				switch kczallposCastName(c, a) {
				case "Harmonic Convergence", "Zimone's Experiment", "Terminus":
					want = append(want, a)
				}
			}
			if len(want) == 0 {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = want
			a := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			switch kczallposCastName(c, a) {
			case "Harmonic Convergence":
				tl.harmonic++
			case "Zimone's Experiment":
				tl.zimone++
			case "Terminus":
				tl.terminus++
			}
			return a
		})
	}
}

func TestChangeZoneAllLibraryPositionLockstep(t *testing.T) {
	t.Parallel()
	tl := &kczallposTally{}
	lockstepRunPolicy(t, "czallpos", kczallposDecks, 60, true, nil, nil, kczallposPolicy(tl))
	if tl.harmonic == 0 || tl.zimone == 0 || tl.terminus == 0 {
		t.Fatalf("carriers never cast (Harmonic %d, Zimone %d, Terminus %d); the lockstep proves nothing", tl.harmonic, tl.zimone, tl.terminus)
	}
	t.Logf("carrier casts: Harmonic %d, Zimone %d, Terminus %d", tl.harmonic, tl.zimone, tl.terminus)
}
