package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kmarvinDecks: Marvin, Murderous Mimic gains every activated ability of a
// creature its controller controls that does not share its name
// (GainsAbilitiesOf$ Creature.YouCtrl+!sharesNameWith Self). Each seat has
// Marvin (two copies) beside Prodigal Sorcerer and Prodigal Pyromancer,
// whose ping abilities are non-mana, so each live Marvin is offered their
// activations. Neither Marvin may gain the other's abilities: a gained
// activation whose source is a Marvin is the same-name exclusion failing.
func kmarvinDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Marvin, Murderous Mimic", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Prodigal Pyromancer", Count: 4},
		{Name: "Island", Count: 15},
		{Name: "Mountain", Count: 15},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// kmarvinPolicy takes a gained activation whenever gorge offers one, else
// casts, activates and plays lands whenever it can, else random. It counts
// the priority decisions that offered a gained activation, the ones where
// the recipient was a Marvin, and the ones where a Marvin's gained ability
// came from another Marvin.
func kmarvinPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			var gained []Canon
			marvinRecv, marvinSrc := false, false
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Kind != "ability" || o.GainedSource == 0 {
						continue
					}
					gained = append(gained, a)
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Marvin, Murderous Mimic" {
						marvinRecv = true
						if src := c.G.E.G.Obj(o.GainedSource); src != nil && src.Face() != nil && src.Face().Name == "Marvin, Murderous Mimic" {
							marvinSrc = true
						}
					}
				}
			}
			if len(gained) > 0 {
				if *st == nil {
					*st = map[string]int{}
				}
				if !counted[c.Seq] {
					(*st)["gained"]++
					counted[c.Seq] = true
				}
				if marvinRecv {
					(*st)["marvin"]++
				}
				if marvinSrc {
					(*st)["marvinFromMarvin"]++
				}
				d := *c.D
				d.Options = gained
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

// TestGainsAbilitiesOfMarvinLockstep: the battlefield walk of
// dirac/static_gainsof.go agrees with gorge on the sharesNameWith Self
// filter, gorge offered a Marvin a gained activation, and no Marvin gained
// an ability whose source is another Marvin.
func TestGainsAbilitiesOfMarvinLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "marvingainsof", kmarvinDecks(), 40, true, nil, nil, kmarvinPolicy(st))
	if (*st)["gained"] == 0 {
		t.Fatal("gorge never offered a gained activation in the games; the lockstep proves nothing")
	}
	if (*st)["marvin"] == 0 {
		t.Fatal("gorge never offered a Marvin a gained activation; the lockstep does not reach the filter")
	}
	if n := (*st)["marvinFromMarvin"]; n != 0 {
		t.Fatalf("%d priority decisions offered a Marvin an ability gained from another Marvin; !sharesNameWith Self did not exclude it", n)
	}
	t.Logf("gained-activation decisions: %d (%d for a Marvin, %d from a Marvin)",
		(*st)["gained"], (*st)["marvin"], (*st)["marvinFromMarvin"])
}
