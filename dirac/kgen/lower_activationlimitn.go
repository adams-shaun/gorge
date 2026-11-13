package kgen

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
)

// activationLimit reads an activated ability's ActivationLimit$ (gorge
// legal_loyalty.go activationLimitBlocked: the offer is withheld once the
// ability's activations this turn, counted over the object's current stint
// by activationUsedCount, reach the limit; CR 602.5b).
//
// "1" is the once-each-turn record (TurnOnce: perm.actUsed, kop.
// CostOncePerTurn). A literal N >= 2 ("activate no more than twice each
// turn": Fire-Belly Changeling, Sewer Rats) returns the activation
// condition "this source's non-mana activations this turn < N" (kop.
// ExActivations N 1, read off the object's per-turn census object.acts at
// the offer), which the caller ANDs onto the ability's other activation
// conditions: gorge reads the limit beside them as an independent offer
// gate. The census is per object, not per ability, so the shape lowers only
// where the two counts agree: a printed battlefield non-mana ability that is
// the card's one non-mana activated ability (gorge's expanded keyword lines,
// Equip, Outlast, ..., are A: lines and count), on a face with no keyword
// the kernel lowers into another battlefield activation.
func (lc *lowerCtx) activationLimit(sa *cards.SA, ps *params, ab *lAbility) (lCond, error) {
	v, ok := ps.get("ActivationLimit")
	if !ok {
		return lCond{}, nil
	}
	if v == "1" {
		ab.TurnOnce = true
		return lCond{}, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 2 || n >= int(objActsLimit) {
		return lCond{}, refuse("ActivationLimit %q", v)
	}
	if ab.Kind != akActivated || sa.API == "Mana" || sa.API == "ManaReflected" || ab.Latent || ab.Zone != 0 && ab.Zone != zBattlefield {
		return lCond{}, refuse("ActivationLimit %q outside a printed battlefield non-mana ability", v)
	}
	if k := nonManaActivations(lc.face); k != 1 {
		return lCond{}, refuse("ActivationLimit %q on a card with %d non-mana activated abilities (the kernel counts activations per object)", v, k)
	}
	for _, kw := range lc.face.Keywords {
		if h := cards.KeywordHead(kw); activatedKeywords[h] {
			return lCond{}, refuse("ActivationLimit %q beside K:%s (the kernel counts activations per object)", v, h)
		}
	}
	return lCond{Kind: cValue, A: lValue{Kind: vActivations, N: 1}, Op: opLT, B: cst(n)}, nil
}

// objActsLimit bounds a lowered ActivationLimit$ N: the kernel's per-object
// census keeps six bits at four seats (dirac seats_mp.go objActsMask), and a
// limit it can reach stays exact.
const objActsLimit = 63

// nonManaActivations counts the face's AB$ lines that are not mana
// abilities (gorge's expanded keyword lines included).
func nonManaActivations(face *cards.Face) int {
	n := 0
	for _, sa := range face.Abilities {
		if sa != nil && sa.Kind == "AB" && sa.API != "Mana" && sa.API != "ManaReflected" {
			n++
		}
	}
	return n
}

// activatedKeywords are the keyword heads gorge reads directly (no A: line)
// that give the permanent a battlefield activated ability of its own.
var activatedKeywords = map[string]bool{
	"Crew": true, "Reconfigure": true, "Saddle": true, "Station": true, "Level up": true, "Fortify": true,
}
