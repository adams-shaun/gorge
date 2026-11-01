package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a ReduceCost static's PlayerTurn$ window, a
// Condition$ NotPlayerTurn on a non-self static and OnlyFirstSpell$ True.
//
// gorge reads all three against the CASTER: costConditionHolds
// (Active != p), costTurnGateHolds (Active == sv.Controller, and for a
// card in hand sv.Controller is the caster, castersView) and
// onlyFirstSpellUsed (an earlier PutOnStack of the caster's matching
// ValidCard$, or ValidSpell$ when no ValidCard$ is present, this turn).
// The kernel applies every ReduceCost static in
// adjustCostGen under the static's Cond2 with the payer as the evaluating
// controller for a self static, so each lowers to a condition over the
// controller exactly when the payer is the controller.

// reduceActivatorYou is a ReduceCost static whose payer is its controller
// (Activator$ You).
func reduceActivatorYou(mode string, params map[string]string) bool {
	return mode == "ReduceCost" && params["Activator"] == "You"
}

// reduceGates reads PlayerTurn$ and OnlyFirstSpell$ off a ReduceCost
// static and ANDs their conditions into st.Cond.
func (lc *lowerCtx) reduceGates(sp *params, params map[string]string, st *lStatic) error {
	self := params["ValidCard"] == "Card.Self"
	payer := params["Activator"] == "You" || self && params["EffectZone"] == "All"
	if pt, ok := sp.get("PlayerTurn"); ok {
		if !payer {
			return refuse("ReduceCost PlayerTurn %q: the payer is not the controller", pt)
		}
		switch pt {
		case "You", "True":
			st.Cond = andCond(st.Cond, lCond{Kind: cYourTurn})
		case "Opponent":
			st.Cond = andCond(st.Cond, lCond{Kind: cYourTurn, Neg: true})
		default:
			return refuse("ReduceCost PlayerTurn %q", pt)
		}
	}
	if ofs, ok := sp.get("OnlyFirstSpell"); ok {
		if ofs != "True" {
			return refuse("ReduceCost OnlyFirstSpell %q", ofs)
		}
		spec, has := params["ValidCard"]
		if !has || spec == "" {
			// gorge's onlyFirstSpellUsed matches ValidCard$ when present,
			// then ValidSpell$ when present: the ledger counts prior casts
			// of the ValidSpell$ card spec, so an earlier creature does not
			// spend Melek's first instant/sorcery discount.
			if vs := sp.str("ValidSpell"); vs != "" {
				spec, has = stripSpellBase(vs), true
			}
		}
		switch {
		case !has || spec == "":
			// neither ValidCard$ nor ValidSpell$: no carrier names this shape
			return refuse("ReduceCost OnlyFirstSpell without ValidCard")
		case params["Activator"] != "You":
			return refuse("ReduceCost OnlyFirstSpell Activator %q", params["Activator"])
		case self:
			return refuse("ReduceCost OnlyFirstSpell on a self cost static")
		}
		lc.spellBaseOK = true
		v, err := lc.thisTurnCastValue(spec)
		lc.spellBaseOK = false
		if err != nil {
			return err
		}
		// gorge filters the ledger on ev.Player == p, not on a YouCtrl word
		v.N |= kop.CastSpecYou
		st.Cond = andCond(st.Cond, lCond{Kind: cValue, A: v, Op: opEQ, B: cst(0)})
	}
	return nil
}

// stripSpellBase drops a leading "Spell." from each alternative: every
// object on the cast ring is a spell, so the head is implied there, while
// the ring matcher reads the object where it now lives (a resolved
// instant/sorcery is a card in the graveyard).
func stripSpellBase(spec string) string {
	alts := strings.Split(spec, ",")
	for i, a := range alts {
		if r, ok := strings.CutPrefix(a, "Spell."); ok {
			alts[i] = r
		}
	}
	return strings.Join(alts, ",")
}
