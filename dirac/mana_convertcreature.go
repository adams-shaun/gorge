package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Vizier of the Menagerie's conversion (kop.StatAnyTypeCreatures; CR 609.4b):
// its controller may spend mana as though it were mana of any type to cast
// creature spells. gorge reads it as the payment's ManaConvert set (rules
// mana_convert.go manaConversionParts): its planner funds the pips as generic
// as under a may-play any-type rider (pay.ConvRider), so the kernel rides the
// same sfAnyMana flag, and only the floating pool's spend differs
// (payConvPips). The fact is read from the battlefield: no State bytes.

// useAnyColorSpell: the table has a StatAnyColorSpells static (convAnyColor).
var useAnyColorSpell = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatAnyColorSpells) })

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

// convAnyColor is convAnyType's any-colour twin (kop.StatAnyColorSpells, Oath
// of Nissa; CR 609.4b): p's cast of o is under the conversion when p controls
// a functioning static whose Filter matches the cast card (read with p as "you":
// matchCard with no object, so the card's zone is not tested). A coloured pip
// then takes any mana; {C} and the generic cost stay as printed. Unlike
// Vizier's, gorge's planner does not relax it (pay.ConvRider is AnyType alone),
// so it only widens a cast the floating pool pays: the offer gate passes on
// convPoolPays beside the plain price, the build rides bflAnyColor when that is
// the route taken (convRouteFlag), and payConvColorPips spends the pool. A cast
// under both statics is the any-type one's (callers test convAnyType first).
func (s *State) convAnyColor(k *prep, p uint8, o Obj) bool {
	if k.gen == nil || !k.gen.uses.has(useAnyColorSpell) || o == NoObj {
		return false
	}
	c := s.objs[o].card
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatAnyColorSpells && st.ctrl == SeatIx(p) && s.statCond(k, st) &&
			s.matchCard(k, st.a.Static.Filter, c, p, p) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// payConvColorPips is payConvPips for a cast under the any-colour conversion
// (convAnyColor): gorge resolves it with its exact pips (pay.ResolveManaWith
// under a Conv that is wild for every colour but not for {C}), so each coloured
// pip, in W U B R G order, takes its own colour first and else the first of
// W U B R G C the pool holds, while a {C} pip and the generic cost are left to
// the plain spend. It returns what is left of cost.
//
//go:noinline
func (s *State) payConvColorPips(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	b := s.bld()
	pl := &s.pl[p]
	for pip := 0; pip < kop.C; pip++ {
		for ; cost.Pips[pip] > 0; cost.Pips[pip]-- {
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
			if k.gen.anyColSpent && c != kop.C {
				b.e.spc = spcAdd(b.e.spc, c, 1)
			}
		}
	}
	return cost
}

// convPoolPays reports whether p's floating pool alone pays cost under the
// any-colour conversion (convAnyColor): a coloured pip takes any mana, {C} and
// the generic cost as they are. gorge's planner funds the pips as printed (its
// ConvRider relaxes the AnyType->AnyType set alone), so the conversion widens
// an offer only through the pool, which the cast then spends under it
// (payConvColorPips): the offer gate passes on it beside the plain price.
func (s *State) convPoolPays(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if k.gen == nil || !k.gen.uses.has(useAnyColorSpell) || s.convAnyType(k, p, o) || !s.convAnyColor(k, p, o) {
		return false
	}
	c := planUnderRider(*cost, false, true)
	return s.poolPays(p, &c)
}

// convRouteFlag sets bflAnyColor on the cast being built (before its payment
// is planned, once: bflMsDone) when it pays through the any-colour
// conversion's pool route: the floating pool alone pays the cost under the
// conversion (convPoolPays) and the plain price is out of reach of the pool and
// the untapped sources, the plan gorge would have funded. The payment then taps
// nothing for the pips the pool converts.
func (s *State) convRouteFlag(k *prep, e *stackEntry, cost kop.ManaCost) {
	if k.gen == nil || !k.gen.uses.has(useAnyColorSpell) || s.bld().bfl&bflMsDone != 0 {
		return
	}
	c := xFolded(cost, int(e.x))
	if !s.convPoolPays(k, e.ctrl, e.obj, &c) {
		return
	}
	var av avail
	s.availFor(k, e.ctrl, &av)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		s.bld().bfl |= bflAnyColor
	}
}
