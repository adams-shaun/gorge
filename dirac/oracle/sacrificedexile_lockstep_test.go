package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Mode$ Sacrificed trigger whose script names Origin$ Graveyard (It That
// Betrays: "put that card onto the battlefield under your control") finds the
// card in exile when Rest in Peace sent it there (CR 603.10a, 603.6c, 400.7e).
// gorge's changeZoneMoveSkipFor and the kernel's OpReturnEx (which gates only a
// library origin) agree: the opponent's sacrificed Flooded Strand returns
// under the Betrays controller. Biolume Egg and Riveteers Ascendancy carry the
// same Mode$ Sacrificed + Origin$ Graveyard text with no TriggeredCard move
// (a delayed-remembered return and a targeted one), so they ride along as
// controls. Seat 0 plays the one Rest in Peace, seat 1 the Betrays.
func ksacExileDecks() [2][]cardset.Entry {
	// One Rest in Peace in the whole game: a second copy makes a sacrificed
	// Rest in Peace face a replacement choice that includes itself, which
	// gorge offers and the kernel does not (a separate divergence, not this
	// test's subject).
	rip := []cardset.Entry{
		{Name: "Rest in Peace", Count: 1},
		{Name: "Flooded Strand", Count: 4},
		{Name: "Biolume Egg", Count: 2},
		{Name: "Riveteers Ascendancy", Count: 2},
		{Name: "Plains", Count: 15},
		{Name: "Island", Count: 10},
		{Name: "Swamp", Count: 3},
		{Name: "Mountain", Count: 3},
	}
	betrays := []cardset.Entry{
		{Name: "It That Betrays", Count: 4},
		{Name: "Biolume Egg", Count: 2},
		{Name: "Riveteers Ascendancy", Count: 2},
		{Name: "Plains", Count: 10},
		{Name: "Island", Count: 10},
		{Name: "Swamp", Count: 6},
		{Name: "Mountain", Count: 6},
	}
	return [2][]cardset.Entry{rip, betrays}
}

// ksacExilePolicy casts, activates and plays lands whenever it can, and counts
// the decisions that offer a Flooded Strand sacrifice while the opponent
// controls It That Betrays and some Rest in Peace is on the battlefield.
func ksacExilePolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && !counted[c.Seq] {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Kind != "ability" {
						continue
					}
					g := c.G.E.G
					ob := g.Obj(o.Obj)
					if ob == nil || ob.Face() == nil || ob.Zone != state.ZBattlefield || ob.Face().Name != "Flooded Strand" {
						continue
					}
					var betrays, rip bool
					for i := range g.Objs {
						x := &g.Objs[i]
						if x.Zone != state.ZBattlefield || x.Face() == nil {
							continue
						}
						switch x.Face().Name {
						case "It That Betrays":
							betrays = betrays || x.Controller != ob.Controller
						case "Rest in Peace":
							rip = true
						}
					}
					if betrays && rip {
						if *st == nil {
							*st = map[string]int{}
						}
						(*st)["strands"]++
						counted[c.Seq] = true
						break
					}
				}
			}
			return inner.Pick(c)
		})
	}
}

func TestSacrificedTriggerExileLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "sacrificedexile", ksacExileDecks(), 200, true, nil, nil, ksacExilePolicy(st))
	t.Logf("Flooded Strand sacrifice offers under It That Betrays and Rest in Peace: %d", (*st)["strands"])
	if (*st)["strands"] == 0 {
		t.Fatal("no game offered a Flooded Strand sacrifice under It That Betrays and Rest in Peace; the lockstep proves nothing")
	}
}
