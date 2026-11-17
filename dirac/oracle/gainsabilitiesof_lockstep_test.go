package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// GainsAbilitiesOf$'s lockstep (kgen lower_gainsof.go; kernel
// static_gainsof.go): Robaran Mercenaries has every activated ability of
// the legendary creatures its controller controls. The decks pair it with
// Prava of the Steel Legion ({3}{W}: a Soldier token) and Jeska, Warrior
// Adept ({T}: 1 damage to any target), so gorge offers Robaran each of
// them as an "ability" option with GainedSource set, and the kernel the
// AkActivate mode 0xC0 activation. The mutation test turns the kernel walk
// off (dirac.DebugNoGainsOf) and expects a divergence.
func kgainsofDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Robaran Mercenaries", Count: 4},
		{Name: "Prava of the Steel Legion", Count: 4},
		{Name: "Jeska, Warrior Adept", Count: 4},
		{Name: "Plains", Count: 14},
		{Name: "Mountain", Count: 14},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// kgainsofPolicy takes a gained activation whenever gorge offers one, else
// casts, activates and plays lands whenever it can, else random; it counts
// the priority decisions where gorge offered a gained activation.
func kgainsofPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			var gained []Canon
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					if o, ok := c.G.GorgeOption(a); ok && o.Kind == "ability" && o.GainedSource != 0 {
						gained = append(gained, a)
					}
				}
				if len(gained) > 0 && !counted[c.Seq] {
					if *st == nil {
						*st = map[string]int{}
					}
					(*st)["gained"]++
					counted[c.Seq] = true
				}
			}
			if len(gained) > 0 {
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

func TestGainsAbilitiesOfLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "gainsof", kgainsofDecks(), 40, true, nil, nil, kgainsofPolicy(st))
	if (*st)["gained"] == 0 {
		t.Fatal("gorge never offered a gained activation in the games; the lockstep proves nothing")
	}
	t.Logf("priority decisions offering a gained activation: %d", (*st)["gained"])
}

// TestGainsAbilitiesOfMutation: with the kernel's gained-ability walk off
// (dirac.DebugNoGainsOf), some seed diverges (gorge offers the gained
// activations; the kernel would not). Not parallel: the flag is global.
func TestGainsAbilitiesOfMutation(t *testing.T) {
	old := dirac.DebugNoGainsOf
	dirac.DebugNoGainsOf = true
	defer func() { dirac.DebugNoGainsOf = old }()
	st := new(map[string]int)
	if n := lockstepRunPolicy(t, "gainsof", kgainsofDecks(), 40, false, nil, nil, kgainsofPolicy(st)); n == 0 {
		t.Fatal("no seed diverged with the gained-ability walk off")
	}
}
