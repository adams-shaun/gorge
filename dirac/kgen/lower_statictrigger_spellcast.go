package kgen

import (
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// staticSpellCastBody settles a Static$ True SpellCast trigger's lowered
// program, the SpellCast sibling of staticTriggerBody. gorge's
// staticTriggerAskFree runs the body off the stack, before the CR 603.3b
// ordering ask, when it is provably ask-free. Two rider shapes carry that
// proof: the enters-with-counters Effects (EffEtbCounters / EffFinalityCtr,
// lower_effetbctr.go), and an Animate of the triggering spell that only
// grants keywords (Thundermane Dragon).
//
// The keyword Animate is a timed grant on the triggering spell that gorge
// carries onto the permanent the spell becomes (CR 400.7a;
// engine/effects/combatfx.go ExileOnMovedStackCarry and
// engine/rules/layers_effect.go spellCarrySweep). The kernel mirrors it by
// stamping the spell entry with the override record's index and applying the
// record as the permanent enters (dirac/static_spellcarry.go): the instruction is
// kept and marked spellCarry, not dropped.
func (lc *lowerCtx) staticSpellCastBody(ab *lAbility, t cards.Trigger) error {
	if len(ab.Targets) != 0 {
		return refuse("static SpellCast trigger body carries an unannounced target")
	}
	if len(ab.Prog) == 1 {
		switch ab.Prog[0].Op {
		case "EffEtbCounters", "EffFinalityCtr":
			return nil
		case "Override":
			if lc.staticSpellCastKeywordAnimate(t, &ab.Prog[0]) {
				ab.Prog[0].CS = "EOT|spellCarry"
				return nil
			}
		}
	}
	return refuse("static SpellCast trigger body is not the proven ask-free rider shape")
}

// staticSpellCastKeywordAnimate reports whether in is the plain keyword
// Animate of the triggering spell's last known information: DB$ Animate |
// Defined$ TriggeredCardLKICopy | Keywords$ <words>, with the Animate API's
// default until-end-of-turn duration and no other term (no P/T, types,
// colours, granted abilities or statics). The record comparison against an
// AddKw-only lOverride refuses every other Animate body by construction.
func (lc *lowerCtx) staticSpellCastKeywordAnimate(t cards.Trigger, in *lInstr) bool {
	if in.Obj.Kind != rTriggered || in.CS != "EOT" {
		return false
	}
	if t.Effect.API != "Animate" || strings.TrimSpace(t.Effect.ParamStr(cards.PKDefined)) != "TriggeredCardLKICopy" {
		return false
	}
	if in.C < 0 || in.C >= len(lc.out.overrides) {
		return false
	}
	o := lc.out.overrides[in.C]
	return len(o.AddKw) > 0 && reflect.DeepEqual(o, &lOverride{AddKw: o.AddKw})
}
