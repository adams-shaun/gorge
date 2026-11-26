package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// lethalDamageByPower reports whether a live Mode$ Continuous static with
// LethalDamageByPower$ True affects creature id (Zilortha, Strength
// Incarnate: "Lethal damage dealt to creatures you control is determined by
// their power rather than their toughness"). Forge's Card.getLethal returns
// the net power for such a card and every lethal-damage read goes through it:
// the CR 704.5g sweep (GameAction.checkStateEffects), the attacker's
// damage-assignment minimum (getLethalDamage, CR 510.1c / 702.19c) and the
// CR 120.10 excess (getExcessDamageValue). CR 704.5f (toughness 0 or less)
// still reads toughness.
//
// Printed statics and AddStaticAbility$-granted ones are walked in the
// deterministic APNAP battlefield order; a gate this build cannot evaluate
// fails closed (continuousGateHolds), so the creature keeps toughness.
func (e *Engine) lethalDamageByPower(id state.ObjID) bool {
	if !e.lethalByPowerOut() {
		return false
	}
	for _, list := range [2][]staticView{e.activeStatics("Continuous"), e.grantedStatics("Continuous")} {
		for i := range list {
			sv := list[i]
			if sv.ParamStr(cards.PKLethalDamageByPower) != "True" {
				continue
			}
			if !e.continuousGateHolds(sv) {
				continue
			}
			spec := sv.ParamStr(cards.PKAffected)
			if spec == "" {
				spec = "Card.Self"
			}
			if e.matchesSpec(spec, id, e.staticSpecCtx(sv)) {
				return true
			}
		}
	}
	return false
}

// LethalDamageThreshold is the damage that is lethal to creature id before any
// marked damage: its power under a LethalDamageByPower$ static, else its
// toughness (Forge Card.getLethal). effects' excess-damage reads reach it
// through effects.LethalThreshold.
func (e *Engine) LethalDamageThreshold(id state.ObjID) int32 {
	if e.lethalDamageByPower(id) {
		return e.Power(id)
	}
	return e.Toughness(id)
}

// lethalByPowerOut is lethalDamageByPower's cheap guard: some battlefield
// face names LethalDamageByPower$ (printed, or in the SVar text it grants),
// read off the memoised face scan, so a board without one never walks the
// Continuous statics on a lethal-damage read.
func (e *Engine) lethalByPowerOut() bool {
	g := e.G
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if o := g.Obj(id); o != nil && !o.PhasedOut {
				if f := o.Face(); f != nil && e.faceScanHas(f, faceScanLethalByPower) {
					return true
				}
			}
		}
	}
	return false
}
