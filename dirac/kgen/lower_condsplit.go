package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lSplit is a Condition-split mana ability's arm (kop.Ability.SplitGate):
// River of Tears' "{T}: Add {U}. If you played a land this turn, add {B}
// instead" and Gemstone Caverns' "{T}: Add {C}. If this has a luck counter on
// it, instead add one mana of any color". gorge reads them as an AB$ Mana whose
// root is gated ConditionSVarCompare$ EQ0 on an SVar, SubAbility$ a DB$ Mana
// gated GE1 on the same SVar: the activation makes the root's mana or the
// sub's per the board (CR 605.1a, 106.1). The ability lowers as ONE mana
// ability (the root's production in lMana, the sub folded into Split); the
// kernel mirrors gorge's planner, which defers the source (pay.PaymentPlanShapeTierOf
// source:conditional) and counts the root's production plus the sub's while its
// gate holds (rules condSplitLiveSub; dirac mana_condsplit.go).
type lSplit struct {
	Gate uint8  // kop.SplitGateCounter or kop.SplitGateLands
	Ctr  string // the counter name of a counter gate ("LUCK")
	Cols string // the arm's colours: "WUBRG" (Produced$ Any) or one of W U B R G
}

// condSplitRoot reads the Condition-split shape of the AB$ Mana root sa (its
// params ps), nil when the root is gated by nothing (the caller's ordinary
// lowering). Exactly the two shapes above lower: a plain {T} root of one
// fixed mana (C or one of WUBRG), gated EQ0, a lone DB$ Mana sub of Produced$
// Any or one of WUBRG gated GE1 on the same SVar, the SVar exactly
// Count$CardCounters.<KIND> read on the source or PlayerCountPropertyYou$LandsPlayed.
// Anything else refuses, naming the unsupported part.
func (lc *lowerCtx) condSplitRoot(sa *cards.SA, ps *params, ab *lAbility) (*lSplit, error) {
	if ps.peek("ConditionCheckSVar") == "" {
		return nil, nil
	}
	const shape = "a Condition-gated mana ability beyond the Condition-split shape (root {T} one fixed mana gated EQ0, lone DB$ Mana sub gated GE1 on the same counter or lands-played SVar)"
	sub := sa.Sub
	if ab.Kind != akMana || ab.Loyalty || sub == nil || sub.API != "Mana" || sub.Sub != nil {
		return nil, refuse("%s: no lone DB$ Mana sub", shape)
	}
	if !ab.Cost.Tap || !plainTapCost(ab.Cost) {
		return nil, refuse("%s: root cost beyond {T}", shape)
	}
	for _, k := range sortedParamKeys(ps.m) {
		switch k {
		case "AB", "Cost", "Produced", "ConditionCheckSVar", "ConditionSVarCompare", "SubAbility", "SpellDescription":
		default:
			return nil, refuse("%s: root param %s$", shape, k)
		}
	}
	for _, k := range sortedParamKeys(sub.Params) {
		switch k {
		case "DB", "Produced", "ConditionCheckSVar", "ConditionSVarCompare":
		default:
			return nil, refuse("%s: sub param %s$", shape, k)
		}
	}
	root := strings.TrimSpace(ps.m["Produced"])
	if len(root) != 1 || !strings.Contains("WUBRGC", root) {
		return nil, refuse("%s: root Produced$ %q", shape, root)
	}
	armProd := strings.TrimSpace(sub.Params["Produced"])
	arm := armProd
	if armProd == "Any" {
		arm = "WUBRG"
	}
	if len(arm) != 1 && arm != "WUBRG" || !strings.Contains("WUBRG", arm) {
		return nil, refuse("%s: sub Produced$ %q", shape, armProd)
	}
	name := ps.peek("ConditionCheckSVar")
	if name == "" || name != strings.TrimSpace(sub.Params["ConditionCheckSVar"]) {
		return nil, refuse("%s: root and sub gate different SVars", shape)
	}
	if ps.peek("ConditionSVarCompare") != "EQ0" || strings.TrimSpace(sub.Params["ConditionSVarCompare"]) != "GE1" {
		return nil, refuse("%s: gate compare %q / %q", shape, ps.peek("ConditionSVarCompare"), strings.TrimSpace(sub.Params["ConditionSVarCompare"]))
	}
	body := strings.TrimSpace(lc.face.SVars[name])
	sp := &lSplit{Cols: arm}
	switch kind, found := strings.CutPrefix(body, "Count$CardCounters."); {
	case found:
		if _, err := counterKind(kind); err != nil {
			return nil, refuse("%s: gate counter %q", shape, kind)
		}
		sp.Gate, sp.Ctr = kop.SplitGateCounter, kind
	case body == "PlayerCountPropertyYou$LandsPlayed":
		sp.Gate = kop.SplitGateLands
	default:
		return nil, refuse("%s: gate SVar %q", shape, body)
	}
	ps.consume("ConditionCheckSVar", "ConditionSVarCompare")
	return sp, nil
}
