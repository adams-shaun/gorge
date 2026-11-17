package oracle

import (
	"math/rand/v2"
	"strings"
)

// snowWatchPolicy plays randomly but, at priority with an empty stack, floats
// mana by hand so a {S} activation has snow units to pay with: three times in
// four it activates a watched card, else half the time it activates a land (or
// Coldsteel Heart, whose typed mana is no snow), else three times in four it
// casts a watched card. Everything else is RandomPolicy's.
func snowWatchPolicy(seed uint64, watch map[string]bool, casts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x736e6f77)) // "snow"
	isLand := func(n string) bool {
		switch n {
		case "Forest", "Island", "Mountain", "Swamp", "Plains", "Coldsteel Heart":
			return true
		}
		return strings.HasPrefix(n, "Snow-Covered")
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			pick := func(kind CanonKind, ok func(string) bool) (Canon, bool) {
				for _, a := range c.D.Options {
					if a.Kind() == kind && ok(refCardName(c.G, a.Src())) {
						return a, true
					}
				}
				return 0, false
			}
			if prng.IntN(4) != 0 {
				if a, ok := pick(ActActivate, func(n string) bool { return watch[n] }); ok {
					return a
				}
			}
			if prng.IntN(2) == 0 {
				if a, ok := pick(ActActivate, isLand); ok {
					return a
				}
			}
			if prng.IntN(4) != 0 {
				if a, ok := pick(ActCast, func(n string) bool { return watch[n] }); ok {
					casts[refCardName(c.G, a.Src())]++
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}
