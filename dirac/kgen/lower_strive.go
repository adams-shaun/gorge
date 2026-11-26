package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

func init() { kwHandlers["Strive"] = (*lowerCtx).lowerStriveKw }

// lowerStriveKw lowers a printed K:Strive:<cost> (CR 702.52a, "this spell
// costs <cost> more to cast for each target beyond the first") as the card's
// own per-target raise: the S:Mode$ RaiseCost | Relative$ True | Amount$
// TargetedObjects$Amount/Minus.1 shape Officious Interrogation prints
// (raisePerTarget, kop.StRaisePerTarget). gorge folds the Strive charge into
// the cast's base cost after CR 601.2c (rules foldStriveCost, len(targets)-1
// payments of the parameter) and the raise group joins the same cost before
// the reductions, so both read the same total. Only a plain mana parameter
// (generic digits and W U B R G C pips, at most three of each per charge)
// lowers; anything else (hybrid, {X}, a non-mana part) refuses by name, as
// does a Strive face whose spell targets in no root slot of its own or
// whose sub-abilities target too (the kernel counts every slot of the cast,
// gorge only the root ask's targets).
func (lc *lowerCtx) lowerStriveKw(node, param string) {
	cost := strings.TrimSpace(param)
	if !striveCostPlain(cost) {
		lc.refuseNode(node, "Strive cost %q is not plain mana", cost)
		return
	}
	if sa := lc.striveSpell(); sa == nil || sa.Params["ValidTgts"] == "" {
		lc.refuseNode(node, "Strive on a face whose spell has no root target")
		return
	}
	for _, body := range lc.face.SVars {
		if b := strings.TrimSpace(body); strings.HasPrefix(b, "DB$") && strings.Contains(b, "ValidTgts$") {
			lc.refuseNode(node, "Strive beside a targeting sub-ability")
			return
		}
	}
	s := cards.Static{Mode: "RaiseCost", Params: map[string]string{
		"Mode": "RaiseCost", "ValidCard": "Card.Self", "Type": "Spell", "EffectZone": "All",
		"Relative": "True", "Cost": cost, "Amount": "TargetedObjects$Amount/Minus.1",
	}}
	lc.guard(node, func() error { return lc.lowerStatic(s) })
}

// striveSpell is the face's one spell ability (A:SP$), nil when it has none
// or several.
func (lc *lowerCtx) striveSpell() *cards.SA {
	var sp *cards.SA
	for _, a := range lc.face.Abilities {
		if a == nil || a.Kind != "SP" {
			continue
		}
		if sp != nil {
			return nil
		}
		sp = a
	}
	return sp
}

// striveCostPlain reports whether a Strive parameter is plain mana: generic
// digits and single W U B R G C pips, at most three of one colour.
func striveCostPlain(cost string) bool {
	if cost == "" {
		return false
	}
	var cnt [6]int
	for _, tok := range strings.Fields(cost) {
		if n, err := strconv.Atoi(tok); err == nil {
			if n < 0 || n > 255 || tok[0] == '+' || tok[0] == '-' {
				return false
			}
			continue
		}
		if len(tok) != 1 {
			return false
		}
		i := strings.IndexByte("WUBRGC", tok[0])
		if i < 0 {
			return false
		}
		if cnt[i]++; cnt[i] > 3 {
			return false
		}
	}
	return true
}
