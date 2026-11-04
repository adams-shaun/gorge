package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The first-spell-this-game rider (First Contact): a static SpellCast trigger
// with ValidCard$ Card.Self whose DB$ StoreSVar writes Count$YouCastThisGame
// into an SVar at cast (Type$ Calculate), and a chain node gated by
// ConditionCheckSVar$ on that SVar with ConditionSVarCompare$ EQ1. The pair
// lowers to ExFirstCast on the resolving spell's entry: dirac stamps the
// spell when its cast completes (dirac/cast_firstcast.go), so the trigger
// itself lowers to nothing. The kernel keeps no count, so the SVar is read
// only as "equals 1"; any other read refuses by name.

// firstCastStamp reports the SVar a trigger of exactly that shape writes.
func (lc *lowerCtx) firstCastStamp(t cards.Trigger) (string, bool) {
	if t.Mode != "SpellCast" || t.Effect == nil || t.Effect.API != "StoreSVar" {
		return "", false
	}
	for k, v := range t.Params {
		switch k {
		case "Mode", "Execute", "TriggerDescription":
		case "ValidCard":
			if strings.TrimSpace(v) != "Card.Self" {
				return "", false
			}
		case "Static":
			if !strings.EqualFold(strings.TrimSpace(v), "True") {
				return "", false
			}
		default:
			return "", false
		}
	}
	if _, ok := t.Params["ValidCard"]; !ok {
		return "", false
	}
	if _, ok := t.Params["Static"]; !ok {
		return "", false
	}
	sp := t.Effect.Params
	name := strings.TrimSpace(sp["SVar"])
	if name == "" || t.Effect.Sub != nil || strings.TrimSpace(sp["Type"]) != "Calculate" ||
		strings.TrimSpace(sp["Expression"]) != "Count$YouCastThisGame" {
		return "", false
	}
	for k := range sp {
		switch k {
		case "SVar", "Type", "Expression":
		default:
			return "", false
		}
	}
	if _, has := lc.face.SVars[name]; has {
		return "", false
	}
	return name, true
}

// scanFirstCast finds the face's stamping trigger before any ability is
// lowered: the spell's chain reads the SVar the trigger writes.
func (lc *lowerCtx) scanFirstCast() {
	lc.firstCastSVar, lc.firstCastTrig = "", 0
	for i, t := range lc.face.Triggers {
		if name, ok := lc.firstCastStamp(t); ok {
			lc.firstCastSVar, lc.firstCastTrig = name, i+1
			return
		}
	}
}

// firstCastCond lowers a ConditionCheckSVar$ naming the stamped SVar.
func (lc *lowerCtx) firstCastCond(ps *params) (lCond, error) {
	if lc.ab == nil || lc.ab.Kind != akSpell {
		// the stamp is the resolving spell's own entry; a permanent's
		// ability would read an object that was never stamped
		return lCond{}, refuse("Count$YouCastThisGame SVar %s read outside a spell ability", lc.firstCastSVar)
	}
	cmp, _ := ps.get("ConditionSVarCompare")
	if strings.TrimSpace(cmp) != "EQ1" {
		return lCond{}, refuse("Count$YouCastThisGame SVar %s compared with %q: only EQ1 (the first spell cast this game) lowers", lc.firstCastSVar, cmp)
	}
	return lCond{Kind: cValue, A: lValue{Kind: vFirstCast, Sub: []lValue{cst(1), cst(0)}}, Op: opGE, B: cst(1)}, nil
}
