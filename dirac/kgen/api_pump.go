package kgen

import (
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
	return lc.pumpRememberTargeted(ps, t0) // lower_remembered.go
}
