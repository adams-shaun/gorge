package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The combat and history filter words: properties gorge reads that the kernel
// filter struct has no field for, each lowered as a kop.FilterWord literal in
// the clause's PredProg (filter_spill.go). A word is registered once, by its
// property spelling, with the literal it lowers to; only the exact spellings
// registered lower, every other still refuses as an unknown property.

// fwArms is the property spellings that lower to a literal.
var fwArms = map[string]lSpillLit{}

// registerFwArm adds the property spelling p, lowering to l, to the arm table
// and names the word's manifest entry for the context check (fwAllowed).
func registerFwArm(p string, l lSpillLit, entry string) {
	fwArms[p] = l
	fwName[l.W] = entry
}

func init() {
	// gorge engine/effects/filter.go positiveRecognised / matchPositive
	registerFwArm("IsGoaded", lSpillLit{W: kop.FwGoaded}, "FwGoaded")
	// gorge engine/effects/filter.go predicates "firstTurnControlled"; the
	// '!' of the word holds for every object that is not a summoning-sick
	// battlefield permanent, which the literal's Neg reads
	registerFwArm("firstTurnControlled", lSpillLit{W: kop.FwFirstTurnControlled}, "FwFirstTurnControlled")
	registerFwArm("!firstTurnControlled", lSpillLit{W: kop.FwFirstTurnControlled, Neg: true}, "FwFirstTurnControlled")
	// gorge engine/effects/filter_word.go wordThisTurnEnteredFrom; only the
	// graveyard origin is lowered (the Battlefield and Library origins are
	// kop.PredWord7 words, lower_filterwords.go)
	registerFwArm("ThisTurnEnteredFrom_Graveyard", lSpillLit{W: kop.FwEnteredFrom, N: int(kop.ZoneGraveyard)}, "FwEnteredFrom")
}

// wordOrUnknown is prop's last arm: the property p lowers to its registered
// literal, else it is an unknown property.
func (c *lClause) wordOrUnknown(p string) error {
	if l, ok := fwArms[p]; ok {
		return c.spillWord(l)
	}
	return fmt.Errorf("unknown property %q", p)
}

// spillWord adds the word literal l to the clause's program. The clause's one
// comparison, when it has one, moves into the program beside it (cmpLit); a
// comparison with no word refuses the clause as a second comparison would.
func (c *lClause) spillWord(l lSpillLit) error {
	if c.CmpStat == cmpNone {
		c.CmpStat = cmpSpill
	}
	if c.CmpStat != cmpSpill {
		first, ok := cmpLit(c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter)
		if !ok {
			return fmt.Errorf("two comparisons in one clause")
		}
		c.pushSpill(first)
		c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter = cmpSpill, 0, 0, "", ""
	}
	if !c.pushSpill(l) {
		return fmt.Errorf("two comparisons in one clause")
	}
	return nil
}
