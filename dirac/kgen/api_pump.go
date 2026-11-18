package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for pumps, animation, protection and text (lower_api.go).
func init() {
	apiOp("Pump", (*lowerCtx).apiPump)
	apiPS("PumpAll", (*lowerCtx).pumpAll)
	apiPS("Animate", (*lowerCtx).animate)
	apiPS("Phases", (*lowerCtx).phases)                     // lower_phasing.go
	apiPS("ChangeText", (*lowerCtx).changeText)             // textreach.go
	apiPS("ProtectionAll", (*lowerCtx).protectionAllChosen) // lower_choosecolorres.go
	apiPS("Protection", (*lowerCtx).protection)             // lower_protection.go
	apiPS("AlterAttribute", (*lowerCtx).alterAttribute)     // lower_tappower.go
	apiPS("SetState", (*lowerCtx).setState)                 // lower_setstate.go
	apiPS("AnimateAll", (*lowerCtx).animateAll)
	apiPS("Debuff", (*lowerCtx).debuff)
}

func (lc *lowerCtx) apiPump(sa *cards.SA, ps *params) error {
	if ps.str("RememberObjects") == "Imprinted" {
		return lc.pumpRememberImprinted(ps) // lower_storesvar.go
	}
	if v, ok := ps.get("ImprintCards"); ok {
		// Pump ImprintCards$ TriggeredCard (gorge imprint on the trigger
		// body, effects/context.go): the triggering object joins the
		// source's persistent Imprinted list. Only the TriggeredCard
		// spelling inside a trigger body of a card that keeps its list
		// lowers; every other shape stays refused by name.
		if strings.TrimSpace(v) != "TriggeredCard" {
			return refuse("Pump ImprintCards$ %q", v)
		}
		if !lc.impPersist || !lc.trigBody {
			return refuse("Pump ImprintCards$ outside a persistent trigger body")
		}
		lc.emit(lInstr{Op: "ImprintTriggered", Obj: lRef{Kind: rTriggered}})
	}
	if _, ok := ps.m["TargetType"]; ok {
		return lc.pumpSpellTarget(ps) // lower_stackexile.go
	}
	if _, ok := ps.get("DefinedKW"); ok {
		return lc.colorGrantPump(ps) // (Skrelv)
	}
	lc.chosenStrictPump(ps) // lower_choosecardenter.go
	if done, err := lc.pumpYouAndValid(ps); done || err != nil {
		return err // lower_definedheads.go
	}
	if done, err := lc.definedValid(sa, ps); done || err != nil {
		return err
	}
	t0 := lc.lastTarget()
	if err := lc.pump(ps); err != nil {
		return err
	}
	if err := lc.pumpRememberTargeted(ps, t0); err != nil { // lower_remembered.go
		return err
	}
	return lc.pumpForgetImprinted(ps)
}

// pumpForgetImprinted reads Pump's ForgetImprinted$: the imprinted card(s)
// the Defined$ spec names leave the source's imprinted list (gorge
// forgetImprinted on Pump: the o.Imprinted half is not auto-pruned on the
// card's move, so the forget keeps a later read from a stale imprint; Mirror
// Golem's DBForget). Only the TriggeredCard$ spelling lowers, on a card that
// keeps its Imprinted list: the op's Obj ref is the trigger's object.
func (lc *lowerCtx) pumpForgetImprinted(ps *params) error {
	v, ok := ps.get("ForgetImprinted")
	if !ok {
		return nil
	}
	if !lc.impPersist {
		return refuse("Pump ForgetImprinted$ %q outside the persist shape", v)
	}
	if v != "TriggeredCard" {
		return refuse("Pump ForgetImprinted$ %q", v)
	}
	lc.emit(lInstr{Op: "ForgetImprinted", Obj: lRef{Kind: rTriggered}})
	return nil
}
