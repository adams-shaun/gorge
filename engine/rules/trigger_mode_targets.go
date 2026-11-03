package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// triggerModesWithTargets is CR 700.2a for a modal triggered ability's CR
// 603.3c placement: a mode whose mandatory targets have no legal candidate
// can't be chosen. It keeps choices' order (so the ask's option indices map
// back to the same SVar names) and judges each mode -- its own ValidTgts$
// and every chained SubAbility$ target -- through modeTargetsAvailable, the
// census the spell-side announcement (charmTargetsAvailable, castModeAsk)
// already uses, so the trigger and the spell share one legality rule. obj is
// the trigger's stack object: the same source the later target ask is posed
// for. A mode whose SVar does not resolve is kept (the caller has already
// established every Choices$ value resolves).
func triggerModesWithTargets(e *Engine, p state.PlayerID, obj state.ObjID, svars map[string]string, choices []string) []string {
	out := choices[:0:0]
	for _, name := range choices {
		if sub := cards.ResolveSVar(svars, name); sub != nil && !modeTargetsAvailable(e, p, obj, obj, sub, 0, false) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// placeRandomCharmMode is CR 700.2b for a triggered Charm whose mode is chosen
// at random: the draw is among the modes that can be chosen and happens as the
// ability goes on the stack, so a targeted mode's targets are chosen then too
// (CR 603.3d). It reports whether it settled the placement; false leaves it to
// askTriggerModes (not a random Charm, a failed comparison, or several slots).
func (e *Engine) placeRandomCharmMode(p state.PlayerID, obj state.ObjID, sa *cards.SA) bool {
	if sa.API != "Charm" {
		return false
	}
	source, ctx := e.triggerModeCtx(p, obj, sa)
	svars := ctx.SVars
	if !effects.CharmRandomChosen(e, ctx, sa) {
		return false
	}
	choices := effects.CharmOf(sa).Modes
	for _, name := range choices {
		if cards.ResolveSVar(svars, name) == nil {
			return false
		}
	}
	eligible := effects.CharmEligibleModes(e, source, sa, choices)
	eligible = triggerModesWithTargets(e, p, obj, svars, eligible)
	if len(eligible) == 0 {
		removeModelessTrigger(e, obj)
		return true
	}
	if min, max, repeat := effects.CharmModeBounds(e, ctx, sa, len(eligible)); min != 1 || max != 1 || repeat {
		return false
	}
	name := eligible[e.Rand(len(eligible))]
	mode := cards.ResolveSVar(svars, name)
	label := effects.CharmModeLabel(mode, name)
	e.emit(events.Event{Kind: events.Note, Obj: source, Text: "chose a mode at random: " + label})
	if effects.GameReadsDilemmas(e.G) {
		e.emit(events.Event{Kind: events.ModeChosen, Obj: obj, Player: p,
			Counter: events.DilemmaFacedAtRandom, Text: label})
	}
	names := []string{name}
	if o := e.G.Obj(obj); o != nil {
		o.ChosenModes = names
	}
	effects.RecordCharmChoices(e, source, sa, names)
	if effects.TargetsOf(mode).Targeted() {
		e.askTarget(p, obj, mode)
	}
	return true
}

// removeModelessTrigger is CR 603.3c's "if no mode is chosen, the ability is
// removed from the stack": a modal trigger left with fewer choosable modes
// than its minimum ceases to exist (an ability object rests in exile, the
// same resting zone every ability fizzle uses) instead of sitting on the
// stack to resolve as nothing.
func removeModelessTrigger(e *Engine, obj state.ObjID) {
	e.emit(events.Event{Kind: events.MoveZone, Obj: obj, From: state.ZStack,
		To: state.ZExile, Text: "removed: no legal mode"})
	e.ensureLeftTheStack(obj, state.ZExile, "a replacement discarded a modeless trigger's removal")
}
