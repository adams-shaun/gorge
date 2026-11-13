package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	// MustTarget: the Flagbearer rule ("While an opponent is choosing targets
	// as part of casting a spell they control or activating an ability they
	// control, that player must choose at least one Flagbearer on the
	// battlefield if able", Coalition Honor Guard, Standard Bearer), read by
	// applyMustTarget at the cast-time target asks.
	effects.RegisterNonAPI("stat:MustTarget")
}

// applyMustTarget narrows the cast-time target ask d for pc (the root ask or
// a chained sub-ability's) to obey every live MustTarget static: when an
// offered target matches a static's ValidTarget$ in its ValidZone$, the
// answer must include at least one such target. As in Forge, the rule binds
// each targeting ability on its own, only a player choosing for themselves
// (a TargetingPlayer$ ask is exempt), and only while one is offered: "if
// able" is the offer itself.
//
// A single-target ask keeps only the matching options. A multi-target ask
// keeps every option and publishes the floor as Decision.MinSum over a Value
// of 1 on each matching option, so Validate rejects, and the shared repair
// fixes, an answer without one, while the rest of the selection stays free.
// A multi-target ask whose Value already carries a budget (a total-power or
// mana-value cap) cannot carry the floor too; no corpus Flagbearer game
// reaches one, and it is left unrestricted rather than over-restricted.
func (e *Engine) applyMustTarget(d *decision.Decision, pc *pendingCast) {
	if d.Player != pc.player || len(d.Options) == 0 {
		return
	}
	must := e.mustTargetOptions(d, pc.player, pc.isAbility())
	if must == nil {
		return
	}
	budgeted := d.HasBudget() || d.MinSum > 0
	for i := range d.Options {
		budgeted = budgeted || d.Options[i].Value != 0
	}
	if d.Max > 1 && budgeted {
		return
	}
	if d.Max > 1 {
		for i := range d.Options {
			if must[i] {
				d.Options[i].Value = 1
			}
		}
		d.MinSum = 1
		d.Min = max(d.Min, 1)
		return
	}
	kept := d.Options[:0]
	for i, o := range d.Options {
		if must[i] {
			o.Index = len(kept)
			kept = append(kept, o)
		}
	}
	d.Options = kept
	d.Min, d.Max = 1, 1
}

// mustTargetOptions marks the options of d a live MustTarget static obliges
// actor to choose among, or nil when none applies. A static applies when
// its gates hold and its ValidSA$ names the targeting action -- a spell, or
// an activated ability, controlled by the player its qualifier names
// relative to the static's controller.
func (e *Engine) mustTargetOptions(d *decision.Decision, actor state.PlayerID, ability bool) []bool {
	var must []bool
	for _, sv := range e.activeStatics("MustTarget") {
		if !e.restrictionGateHolds(sv, sv.Source) || !e.checkSVarHolds(sv) {
			continue
		}
		if !e.mustTargetSAMatches(sv.Params["ValidSA"], actor, ability, sv) {
			continue
		}
		valid, zone := sv.Params["ValidTarget"], sv.Params["ValidZone"]
		if valid == "" {
			continue
		}
		// Every corpus carrier names the battlefield; another zone is a
		// scope this reader does not model, so it obliges nothing.
		if zone != "" && zone != "Battlefield" {
			continue
		}
		for i, o := range d.Options {
			obj := e.G.Obj(o.Obj)
			if o.Obj == 0 || obj == nil || obj.Zone != state.ZBattlefield || !e.hasType(obj, valid) {
				continue
			}
			if must == nil {
				must = make([]bool, len(d.Options))
			}
			must[i] = true
		}
	}
	return must
}

// mustTargetSAMatches reads a MustTarget ValidSA$: a comma list of
// Spell[.Qualifier] / Activated[.Qualifier] alternatives, where the
// qualifier OppCtrl or YouCtrl names the action's controller relative to the
// static's controller. Any other spelling matches nothing, so a scope this
// build cannot read never obliges a choice.
func (e *Engine) mustTargetSAMatches(spec string, actor state.PlayerID, ability bool, sv staticView) bool {
	if spec == "" {
		return true
	}
	kind := "Spell"
	if ability {
		kind = "Activated"
	}
	for start, i := 0, 0; i <= len(spec); i++ {
		if i < len(spec) && spec[i] != ',' {
			continue
		}
		alt := spec[start:i]
		start = i + 1
		head, qual := alt, ""
		for j := 0; j < len(alt); j++ {
			if alt[j] == '.' {
				head, qual = alt[:j], alt[j+1:]
				break
			}
		}
		if head != kind {
			continue
		}
		var who string
		switch qual {
		case "":
			return true
		case "OppCtrl":
			who = "Opponent"
		case "YouCtrl":
			who = "You"
		default:
			continue
		}
		if effects.MatchesPlayerSpecCtx(e.G, who, actor, sv.Controller, e.playerSpecCtx(sv.Source)) {
			return true
		}
	}
	return false
}
