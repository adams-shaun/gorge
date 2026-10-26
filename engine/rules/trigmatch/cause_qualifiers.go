// Cause qualifiers on trigger lines (CR 603.2): the clauses that name what
// CAUSED the trigger event rather than what the event happened to. A matcher
// that skips one over-fires on events the card text excludes.
//
//	ValidCause$             TurnFaceUp, DamageDone
//	TargetRelativeToCause$  DamageDone
//	WhileKeyword$           Exiled, Sacrificed
//	SearchOwnLibrary$       SearchedLibrary (read in searchedLibraryMatches)
//
// Discarded, BecomesTargetOnce, LifeLost and the Panharmonicon statics read
// their own ValidCause$ elsewhere.

package trigmatch

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// stackCauseAdmits evaluates a ValidCause$ stack spec against the stack object
// o that caused an event, from you's perspective. Alternatives are OR and an
// alternative it cannot evaluate fails closed (state.CauseStackTokens).
func stackCauseAdmits(g *state.Game, spec string, o *state.Object, you state.PlayerID) bool {
	if o == nil {
		return false
	}
	// A chain link's targets (Arc Trail's second target) are the spell's
	// targets too, though they ride SubTargets, not Targets.
	n := len(o.Targets) + len(o.SubTargets)
	for _, tok := range state.CauseStackTokens(spec) {
		if state.StackKindAdmits([]state.StackKindToken{tok}, state.StackKindOf(g, o), o, o.Controller, you) &&
			targetCountAdmits(tok, n) {
			return true
		}
	}
	return false
}

// targetCountAdmits is a token's singleTarget and numTargets qualifiers
// against a stack object with n targets.
func targetCountAdmits(tok state.StackKindToken, n int) bool {
	if tok.SingleTarget && n != 1 {
		return false
	}
	return tok.NumTargetsOp == "" || effects.CmpOpOf(tok.NumTargetsOp).Apply(n, tok.NumTargets)
}

// turnFaceUpCauseAdmits is TurnFaceUp's ValidCause$. An effect that turned the
// permanent up is the resolving stack object. Otherwise the turn-up is the
// special action its controller took (CR 116.2b, no stack object), read
// against the turner (the permanent's controller). Anything the spec cannot
// say about it fails closed.
func turnFaceUpCauseAdmits(e Board, spec string, source state.ObjID, turned *state.Object) bool {
	you := e.ControllerOf(source)
	if e.InResolution() {
		cause := e.ActionCause()
		return cause != 0 && stackCauseAdmits(e.Game(), spec, e.Game().Obj(cause), you)
	}
	for _, tok := range state.CauseStackTokens(spec) {
		if tok.AdmitsSpecialAction(turned.Controller == you) {
			return true
		}
	}
	return false
}

// damageCauseMatches reads DamageDone's ValidCause$ and TargetRelativeToCause$.
// The cause is the resolving stack object (ActionCause): combat damage, a
// static and a state-based action have none and fail both clauses. The damaged
// object is matched with the CAUSE as the filter source, so `targetedBy` reads
// the cause's targets; a player recipient fails an object spec.
func damageCauseMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	validCause := t.ParamStr(cards.PKValidCause)
	relative := t.Params["TargetRelativeToCause"]
	if validCause == "" && relative == "" {
		return true
	}
	// Outside a resolution a spell on the stack did not deal the damage (a
	// mana ability or cost), whatever ActionCause reads off the stack top.
	if !e.InResolution() {
		return false
	}
	cause := e.ActionCause()
	if cause == 0 {
		return false
	}
	ctrl := e.ControllerOf(source)
	if validCause != "" && !stackCauseAdmits(e.Game(), validCause, e.Game().Obj(cause), ctrl) {
		return false
	}
	if relative == "" {
		return true
	}
	co := e.Game().Obj(cause)
	return ev.Obj != 0 && co != nil &&
		e.MatchesSpec(relative, ev.Obj, cause, ctrl,
			SpecOpts{TargetsBound: true, Targets: append(slices.Clone(co.Targets), co.SubTargets...)})
}

// whileKeywordAdmits is Exiled/Sacrificed's WhileKeyword$ (Forge
// Trigger.whileKeywordCheck): the move must be a cost paid by the open cast
// flow, and that flow must be `Ability.<Kw>` (an activation of an ability with
// that Keyword$) or `Spell.with<Kw>` (a spell cast with that keyword). Any
// other spelling fails closed.
func whileKeywordAdmits(e Board, t cards.Trigger, ev events.Event) bool {
	spec, ok := t.Params["WhileKeyword"]
	if !ok {
		return true
	}
	if !events.IsExileAsCost(ev) && !events.IsSacrifice(ev) {
		return false
	}
	kw, isSpell, emergeSac, open := e.PayingCostOf()
	if !open {
		return false
	}
	// The emerge cost sacrifices one chosen permanent; a sacrifice paying a
	// mana ability (Ashnod's Altar) mid-cast is that ability's cost.
	if isSpell && events.IsSacrifice(ev) && ev.Obj != emergeSac {
		return false
	}
	wantSpell, want, parsed := state.WhileKeywordSpec(spec)
	return parsed && wantSpell == isSpell && want == kw
}
