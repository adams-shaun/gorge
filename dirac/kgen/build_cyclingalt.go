package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The cycling alternative cost's twin abilities (kop.Ability.CycAltOf).
//
// A StatCyclingAlt static (static_cyclingalt.go) lets its controller pay the
// static's own Cost$, the discard alone, rather than a cycling activation's
// printed cost (CR 118.9). The kernel prices an activation off its ability's
// Cost at every stage, so the alternative is a second ability of the same card:
// the printed cycling or typecycling body again, ability for ability, with
// only the cost replaced. The kernel offers the twin beside the printed one
// while a live static holds (dirac kw_cyclingalt.go) and never otherwise.
// Only a pool that lowered such a static builds twins, so every other table
// is unchanged.

// poolHasCyclingAlt reports whether some card of the pool lowered a
// StatCyclingAlt static.
func poolHasCyclingAlt(cs []*lCard) bool {
	for _, c := range cs {
		if len(c.Refusals) != 0 {
			continue
		}
		for _, s := range c.Statics {
			if s.Kind == "CyclingAlt" {
				return true
			}
		}
	}
	return false
}

// cyclingTwins appends, after the card's abilities, one twin for each printed
// hand cycling ability (a latent granted body is no printed ability), carrying
// the printed one's gorge origin so a stack ability reads the same index. The
// canonical action of an activation of a twin is told apart by CycAltOf.
func (cb *cardBuilder) cyclingTwins() {
	if !cb.pt.cyclingAlt {
		return
	}
	d := &cb.out.def
	n := len(d.Abilities)
	for i := 0; i < n; i++ {
		a := &d.Abilities[i]
		if a.Kind != kop.AbActivated || a.ActZone != kop.ZoneHand || a.AFlags&kop.AfCycling == 0 || a.AFlags&kop.AfLatent != 0 || a.CycAltOf != 0 {
			continue
		}
		tw := *a
		tw.Cost = kop.Cost{Flags: kop.CostDiscardSelf}
		tw.CostProg = 0
		tw.CycAltOf = uint8(i + 1)
		d.Abilities = append(d.Abilities, tw)
		cb.out.stAddSub = append(cb.out.stAddSub, nil)
		cb.out.origins = append(cb.out.origins, cb.out.origins[i])
	}
}
