package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StatStackKw: AffectedZone$ Stack keyword grants to the
// spells the static's controller casts: Cascade (Quandrix, the Proof), read
// live when the cast trigger queues (rules cascade.go queueCascadeTriggers);
// Improvise (Ironheart, Inspiring Statuary) and Casualty:<n> (Silverquill),
// read wherever the printed CardDef.Improvise / Convoke / Casualty are
// (tapPays, isImprovise, casualtyN). gorge derives the spell under the
// stack-zone override (keywordmay.go stackKeywordPossibleH), so the kernel
// asks the live statics at the same moment. gorge offers a tap-paid or
// casualty cast only against a floating pool, which the canonical vocabulary
// never builds (mana abilities are not actions); the stackkw grant lockstep
// floats one with Iridescent Tiger's ETB mana.
// Affinity grants are an ordinary ReduceCost (kgen).
//
// Every caller is behind genPrep.anyStackKw, so FDN and BLB never get here.

// DebugStackKwOff makes stackKwN answer 0 (the oracle's
// mutation test: the grants are ignored, so the lockstep must diverge).
var DebugStackKwOff bool

// prepStackKw sets the StatStackKw gates: a Cascade grant runs the cascade
// trigger queue, an Improvise grant opens the convoke paths (anyConvoke), a
// Casualty grant the casualty offer (p.casualty); one restricted to spells
// cast from hand reads the cast provenance (stackEntry.prov, castemit).
func prepStackKw(t *Table, g *genPrep, p *prep) {
	if !tableHasStatic(t, kop.StatStackKw) {
		return
	}
	g.anyStackKw = true
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatStackKw {
				continue
			}
			if a.Static.Zone == kop.ZoneHand {
				g.anyCastProv = true
			}
			switch a.Static.A {
			case kop.StackKwCascade:
				g.anyCascade = true
			case kop.StackKwImprovise:
				g.anyConvoke = true
			case kop.StackKwCasualty:
				p.casualty = true
			case kop.StackKwLifelink:
				g.anyStackLife = true
			}
		}
	}
}

// stackKwN counts the live StatStackKw statics of keyword code that the
// caster controls and whose filter matches spell card c (object o), and
// returns the first match's B (the casualty threshold);
// fromHand answers a wasCastFromYourHand restriction. Walked like
// mayPlayStatics: the permanents that have not lost their abilities, then
// emblems. A spell on the stack grants nothing (CR 113.6).
func (s *State) stackKwN(k *prep, code int16, c CardID, o Obj, caster uint8, fromHand bool) (n int, b int16) {
	if DebugStackKwOff {
		return 0, 0
	}
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatStackKw || a.Static.A != code || st.ctrl != caster {
			return true
		}
		if a.Static.Zone == kop.ZoneHand && !fromHand {
			return true
		}
		// no statCond: kgen refuses every gated AffectedZone$ Stack grant
		if s.matchSpellCard(k, a.Static.Filter, c, caster, caster, o, st.sl) {
			if n == 0 {
				b = a.Static.B
			}
			n++
		}
		return true
	})
	return n, b
}

// grantImprovise: a live StatStackKw Improvise grant reaches p's spell card
// c (object o). Behind genPrep.anyStackKw.
func (s *State) grantImprovise(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if k.gen == nil || !k.gen.anyStackKw {
		return false
	}
	n, _ := s.stackKwN(k, kop.StackKwImprovise, c, o, p, fromHand)
	return n > 0
}

// tapPays is CardDef.TapPays for p's spell card c (object o): printed
// convoke or improvise, or a granted improvise (the cast announces tapped
// permanents toward its mana cost). fromHand: the spell is cast from hand.
func (s *State) tapPays(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return k.t.Cards[c].TapPays() || s.grantImprovise(k, p, c, o, fromHand)
}

// isImprovise is CardDef.Improvise for p's spell card c (object o): printed
// or granted improvise (the tap-paid permanents are artifacts, not creatures).
func (s *State) isImprovise(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return k.t.Cards[c].Improvise || s.grantImprovise(k, p, c, o, fromHand)
}

// casualtyN is the casualty threshold of p's spell card c (object o): the
// printed fixed N, else the first matching granted Casualty:<n> (gorge
// casualtySpec: the first parseable keyword line, printed before granted);
// 0 = none. Behind prep.casualty.
func (s *State) casualtyN(k *prep, p uint8, c CardID, o Obj, fromHand bool) int {
	if n := int(k.t.Cards[c].Casualty); n != 0 {
		return n
	}
	if k.gen == nil || !k.gen.anyStackKw {
		return 0
	}
	_, b := s.stackKwN(k, kop.StackKwCasualty, c, o, p, fromHand)
	return int(b)
}
