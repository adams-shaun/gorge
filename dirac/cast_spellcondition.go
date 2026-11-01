package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A spell's own cast restriction (gorge spellTimingPre's IsPresent$
// gate, cr-spell-ispresent-cast-gate feb134b15, CR 601.3): "Cast this spell
// only if a creature is attacking you" (Confront the Assault). kgen puts the
// condition in the spell ability's otherwise unused Trig.Cond; castForbidden
// (the hand and flashback offers, only with prep.anyCantCast) withholds the
// cast while it fails, judged for the caster with the card as its source.

// prepSpellCond sets anySpellCond (and reports it for anyCantCast): some
// card's spell ability carries a cast restriction.
func prepSpellCond(t *kop.Table, g *genPrep) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbSpell && a.Trig.Cond != 0 {
				g.anySpellCond = true
				return true
			}
		}
	}
	return false
}

// spellCondFails reports whether card c's spell restriction fails for
// caster p (o: the card).
func (s *State) spellCondFails(k *prep, p uint8, c CardID, o Obj) bool {
	ab := spellAb(k, c)
	if ab == 255 {
		return false
	}
	a := &k.t.Cards[c].Abilities[ab]
	return a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: o, srcSl: NoSlot, ctrl: p})
}
