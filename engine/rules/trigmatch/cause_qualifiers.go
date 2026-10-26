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
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// exiledAsCostText is the Text every cost exile carries (rules' costExileEvent).
const exiledAsCostText = "exiled as a cost"

// stackCauseAdmits evaluates a ValidCause$ stack spec against the stack object
// o that caused an event, from you's perspective. Alternatives are OR. It
// fails closed on a qualifier it cannot evaluate: the shared StackKindTokenOf
// parser skips what it does not know, which would widen the match here.
func stackCauseAdmits(g *state.Game, spec string, o *state.Object, you state.PlayerID) bool {
	if o == nil {
		return false
	}
	for alt := range strings.SplitSeq(spec, ",") {
		tok, ok := state.StackKindTokenOf(alt)
		if !ok || tok.NonCreature || tok.Colorless || tok.Legendary || !causeQualifiersKnown(alt) {
			continue
		}
		if !state.StackKindAdmits([]state.StackKindToken{tok}, state.StackKindOf(g, o), o, o.Controller, you) {
			continue
		}
		if tok.SingleTarget && len(o.Targets) != 1 {
			continue
		}
		if tok.NumTargetsOp != "" && !numTargetsCompare(len(o.Targets), tok.NumTargetsOp, tok.NumTargets) {
			continue
		}
		return true
	}
	return false
}

// causeQualifiersKnown reports whether every qualifier of one stack-spec
// alternative is one stackCauseAdmits evaluates.
func causeQualifiersKnown(alt string) bool {
	_, rest, _ := strings.Cut(strings.TrimSpace(alt), ".")
	for part := range strings.SplitSeq(rest, ".") {
		for q := range strings.SplitSeq(part, "+") {
			q = strings.TrimSpace(q)
			switch q {
			case "", "YouCtrl", "OppCtrl", "YouDontCtrl", "singleTarget", "Instant", "Sorcery":
			default:
				if f := strings.Fields(q); len(f) != 2 || f[0] != "numTargets" {
					return false
				}
			}
		}
	}
	return true
}

func numTargetsCompare(count int, op string, want int) bool {
	switch effects.CmpOpOf(op) {
	case effects.CmpEQ:
		return count == want
	case effects.CmpNE:
		return count != want
	case effects.CmpGE:
		return count >= want
	case effects.CmpGT:
		return count > want
	case effects.CmpLE:
		return count <= want
	case effects.CmpLT:
		return count < want
	}
	return false
}

// turnFaceUpCauseAdmits is TurnFaceUp's ValidCause$. An effect that turned the
// permanent up is the resolving stack object. Otherwise the turn-up is the
// special action its controller took (CR 116.2b, no stack object): only a
// `SpellAbility` alternative admits it, with YouCtrl/OppCtrl read against
// the turner (the permanent's controller). Anything else fails closed.
func turnFaceUpCauseAdmits(e Board, spec string, source state.ObjID, turned *state.Object) bool {
	you := e.ControllerOf(source)
	if e.InResolution() {
		cause := e.ActionCause()
		return cause != 0 && stackCauseAdmits(e.Game(), spec, e.Game().Obj(cause), you)
	}
	for alt := range strings.SplitSeq(spec, ",") {
		base, rest, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if base != "SpellAbility" {
			continue
		}
		switch rest {
		case "":
			return true
		case "YouCtrl":
			if turned.Controller == you {
				return true
			}
		case "OppCtrl", "YouDontCtrl":
			if turned.Controller != you {
				return true
			}
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
		e.MatchesSpec(relative, ev.Obj, cause, ctrl, SpecOpts{TargetsBound: true, Targets: co.Targets})
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
	if ev.Text != exiledAsCostText && !events.IsSacrifice(ev) {
		return false
	}
	kw, isSpell, open := e.PayingCostOf()
	if !open {
		return false
	}
	kind, rest, _ := strings.Cut(spec, ".")
	switch {
	case kind == "Ability" && !isSpell:
		return rest != "" && rest == kw
	case kind == "Spell" && isSpell:
		w, found := strings.CutPrefix(rest, "with")
		return found && w != "" && w == kw
	}
	return false
}
