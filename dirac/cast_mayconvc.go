package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Abstruse Appropriation's colourless-mana conversion (ManaConversion$
// C->AnyColor, CR 609.4b): the controller may spend colourless mana as
// though it were mana of any colour to cast the card its Effect exiled. gorge
// reads it as the payment's ManaConvert set, whose Conv is wild for {C}
// alone (pay.ApplyConversionTo): the planner prices the printed pips, and the
// wildness applies as the pool is spent -- each coloured pip takes its own
// colour first, else a floating {C}; coloured mana never covers another
// colour's pip and a {C} pip is not widened. The kernel carries it as a
// permission-record rider (kop.MayPlayConvC on the OpMayPlay C, mayTurnConvC
// on the record's turn word, which a permanent record never reads as a turn)
// and, once the record is spent, on the build (build.convC); like Oath of
// Nissa's any-colour conversion (mana_convertcreature.go) it only widens a
// cast the floating pool pays, so the offer gate passes on convCPoolPays
// beside the plain price and the pool is spent through payConvCPips.

// mayTurnConvC marks a permission record that carries the colourless-mana
// conversion: mayPlay.turn's bit 24 (a permanent grant leaves the word's turn
// number, bits 0-15, unread; mayHeld packs object and version below bit 24).
const mayTurnConvC mayWord = 1 << 24

// useMayConvC: some program grants a play permission carrying the
// colourless-mana conversion (kop.MayPlayConvC).
var useMayConvC = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpMayPlay && in.C&kop.MayPlayConvC != 0 {
					return true
				}
			}
		}
	}
	return false
})

// convCMark is the record bit an OpMayPlay with C carries (0 for none).
func convCMark(c int16) mayWord {
	if c&kop.MayPlayConvC != 0 {
		return mayTurnConvC
	}
	return 0
}

// mayConvC reports whether p's cast of o runs through a live permission
// record carrying the colourless-mana conversion: the first record
// spendMayRecord would spend.
func (s *State) mayConvC(k *prep, p uint8, o Obj) bool {
	if k.gen == nil || !k.gen.uses.has(useMayConvC) || o == NoObj {
		return false
	}
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj != o || r.ctrl != p || !(r.follows() || r.ver == s.objs[o].ver) ||
			ownWalkerRecord(r) || s.mayLimitSpent(r) || k.gen.anyMayWhileSrc && s.maySrcDead(r) {
			continue
		}
		return r.turn&mayTurnConvC != 0
	}
	return false
}

// convCCast reports whether p's cast of o pays under the colourless-mana
// conversion: the build already spent the record (build.convC), or the
// record that would permit the offer carries it.
func (s *State) convCCast(k *prep, p uint8, o Obj) bool {
	if k.gen == nil || !k.gen.uses.has(useMayConvC) {
		return false
	}
	if b := s.bld(); s.buildLive() && b.e.kind == seSpell && b.e.obj == o && b.e.ctrl == p {
		return b.convC
	}
	return s.mayConvC(k, p, o)
}

// convCCover is cost with each coloured pip the pool holds its own colour for
// kept, and the rest of that colour's pips turned into {C} pips: the pool
// pays the result exactly when it pays cost under the conversion (a coloured
// pip takes its own colour first, else {C}). A cost the conversion does not
// price (hybrid, Phyrexian, snow, X) reports false.
func (s *State) convCCover(p uint8, cost kop.ManaCost) (kop.ManaCost, bool) {
	if cost.X != 0 || cost.XPip != 0 || cost.Snow != 0 || cost.Phyrexian() || cost.Hyb != ([4]kop.ColorMask{}) {
		return cost, false
	}
	pl := &s.pl[SeatIx(p)]
	for pip := 0; pip < kop.C; pip++ {
		if short := int(cost.Pips[pip]) - int(pl.pool[pip]); short > 0 {
			cost.Pips[pip] -= uint8(short)
			cost.Pips[kop.C] += uint8(short)
		}
	}
	return cost, true
}

// convCPoolPays reports whether p's floating pool alone pays cost under the
// colourless-mana conversion (convCCast): a coloured pip takes its own colour,
// else a {C}; {C} and the generic cost as printed. gorge's planner funds the
// printed pips (its ConvRider relaxes the AnyType->AnyType set alone), so the
// conversion widens an offer only through the pool (convPoolPays).
func (s *State) convCPoolPays(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if !s.convCCast(k, p, o) {
		return false
	}
	c, ok := s.convCCover(p, *cost)
	return ok && s.poolPays(p, &c)
}

// payConvCPips spends the pool for a cast under the colourless-mana
// conversion: each coloured pip, in W U B R G order, takes its own colour
// first and else a {C} (gorge pay.ResolveManaWith under a Conv wild for {C}),
// while a {C} pip and the generic cost are left to the plain spend. It
// returns what is left of cost.
//
//go:noinline
func (s *State) payConvCPips(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	b := s.bld()
	pl := &s.pl[p]
	for pip := 0; pip < kop.C; pip++ {
		for ; cost.Pips[pip] > 0; cost.Pips[pip]-- {
			c := pip
			if pl.pool[c] == 0 {
				c = kop.C
				if pl.pool[c] == 0 {
					return cost // (no unit left: the plain spend reports it)
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

// convCRoute reports whether the cast being built pays through the
// conversion's pool route: it carries the conversion (build.convC), the plain
// price is out of reach of the pool and the untapped sources, and the pool
// alone pays under the conversion (the plan gorge would have funded, as
// convRouteFlag's). The payment then taps nothing; the charge spends the pool
// through payConvCPips. cost is the build's cost with the announced x not yet
// folded.
func (s *State) convCRoute(k *prep, cost kop.ManaCost, x int) bool {
	if k.gen == nil || !k.gen.uses.has(useMayConvC) || s.ctl != ctlBuild || !s.bld().convC {
		return false
	}
	e := &s.bld().e
	if e.kind != seSpell {
		return false
	}
	c := xFolded(cost, x)
	if !s.convCPoolPays(k, e.ctrl, e.obj, &c) {
		return false
	}
	var av avail
	s.availFor(k, e.ctrl, &av)
	return !av.affords(&c, pipMaskOf(&c), 0)
}
