package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fwSharesProducedColour: FwSharesProducedColour, the candidate battlefield
// permanent's colours intersect the colours of the mana a mana-ability tap
// produced (gorge's matchSharesColorWith over the trigger context's
// TriggerMana set, engine/effects/filter.go; C.A.M.P.'s ConditionPresent$
// Card.SharesColorWith TriggeredProduced on its Junk-token rider). The set
// rides the resolving trigger's entry in the tap amount's colour bits above
// mtTapBit (manaTapped's fire, trig_taps.go; gorge binds the activation's
// actual ManaAdd batch with stampTriggeredManaProduced -- the kernel's
// mana model is the paid ability's static production, the same mask the
// census adds). Colourless is not a colour and contributes nothing; an empty
// set, no resolving entry, or a candidate with no colours is "shares
// nothing", a resolved false, never an unresolved gate (gorge's convention
// for the empty TriggerMana set).
//
//kmanifest:fword FwSharesProducedColour
func (s *State) fwSharesProducedColour(c *fctx, l *kop.FLit) bool {
	if !c.perm() || !s.res.active {
		return false
	}
	col := uint8(s.res.e.trigAmt>>mtColShift) & 0x1f
	if col == 0 {
		return false // a colourless tap, or a Taps trigger no mana tap queued
	}
	return s.colorsAt(c.k, c.sl)&kop.ColorMask(col) != 0
}

// cardUsesProducedColour reports whether one of c's abilities reads a filter
// program's FwSharesProducedColour word: through an instruction filter, a
// trigger filter, or a condition's filter (the Conds' nested Sub tree). The
// prepareGen flag gfTrigProduced records it per card, and objTrigGen carries
// a mana tap's colour bits only for such a card, so every other table keeps
// the TrigTaps entry's amount exactly 0.
func cardUsesProducedColour(t *kop.Table, c *kop.CardDef) bool {
	ok := func(f kop.FilterID) bool {
		if f == 0 || int(f) >= len(t.Filters) || t.Filters[f].Pred != kop.PredProg {
			return false
		}
		if int(t.Filters[f].PredN) >= len(t.FilterProgs) {
			return false
		}
		for _, l := range t.FilterProgs[t.Filters[f].PredN].Lits {
			if l.W == kop.FwSharesProducedColour {
				return true
			}
		}
		return false
	}
	var condOK func(id kop.CondID) bool
	condOK = func(id kop.CondID) bool {
		if id == 0 || int(id) >= len(t.Conds) {
			return false
		}
		cd := &t.Conds[id]
		if ok(cd.F) {
			return true
		}
		return cd.Kind == kop.CondAnd && (condOK(cd.Sub[0]) || condOK(cd.Sub[1]))
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if ok(a.Trig.Filter) || condOK(a.Trig.Cond) || condOK(a.Static.Cond2) || condOK(a.Repl.Cond) {
			return true
		}
		for i := range a.Prog {
			in := &a.Prog[i]
			if ok(in.F) {
				return true
			}
			if in.Op == kop.OpSkipUnless && condOK(kop.CondID(in.C)) {
				// a ConditionPresent$/ConditionDefined$ guard (the Conds a
				// program step carries, dirac/kgen/emit.go's walker)
				return true
			}
		}
	}
	return false
}
