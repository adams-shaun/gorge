package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Vizier of the Menagerie's conversion (kop.StatAnyTypeCreatures; CR 609.4b):
// its controller may spend mana as though it were mana of any type to cast
// creature spells. gorge reads it as the payment's ManaConvert set (rules
// mana_convert.go manaConversionParts): its planner funds the pips as generic
// as under a may-play any-type rider (pay.ConvRider), so the kernel rides the
// same sfAnyMana flag, and only the floating pool's spend differs
// (payConvPips). The fact is read from the battlefield: no State bytes.

// convAnyType reports whether p's cast of o is under the conversion: o is a
// creature card and p controls a functioning StatAnyTypeCreatures static.
func (s *State) convAnyType(k *prep, p uint8, o Obj) bool {
	if k.gen == nil || !k.gen.anyTypeCreat || o == NoObj || !k.cards[s.objs[o].card].creature {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatAnyTypeCreatures && st.ctrl == SeatIx(p) && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// payConvPips is payAnyTypePips for a cast under the conversion alone: gorge
// resolves it with its exact pips (pay.ResolveManaWith under a wild Conv), so
// each pip, in W U B R G C order, takes its own colour first and else the
// first of W U B R G C the pool holds. It returns what is left of cost.
//
//go:noinline
func (s *State) payConvPips(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	b := s.bld()
	fl, bf, fg := b.e.flags, b.bfl, b.fixedGen
	b.e.flags &^= sfAnyMana
	b.bfl &^= bflCostFixed
	pc := s.buildCost(k)
	b.e.flags, b.bfl, b.fixedGen = fl, bf, fg
	pl := &s.pl[p]
	for pip, n := range pc.Pips {
		for ; n > 0 && cost.Generic > 0; n-- {
			c := pip
			if pl.pool[c] == 0 {
				c = -1
				for _, d := range [6]int{kop.W, kop.U, kop.B, kop.R, kop.G, kop.C} {
					if pl.pool[d] > 0 {
						c = d
						break
					}
				}
				if c < 0 {
					return cost // (no pool unit left: the plain spend reports it)
				}
			}
			pl.pool[c]--
			cost.Generic--
			if k.gen.anyColSpent && c != kop.C {
				b.e.spc = spcAdd(b.e.spc, c, 1)
			}
		}
	}
	return cost
}
