package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters copy's "except it enters with N additional +1/+1 counters"
// rider (Altered Ego: a Clone whose sub-ability puts X +1/+1 counters on
// itself as it enters, an ETB PutCounter). The rider is part of the copy election, not of the
// entry: CR 707.9 makes it an exception of the copy effect, and CR 614.1c
// makes the whole "you may have it enter as a copy" replacement optional,
// so a declined election (or one with nothing to copy) enters the object as
// itself with NO extra counters.

// cloneETBCounterSubKeys are the keys an admitted rider link may carry.
var cloneETBCounterSubKeys = state.NewNameSet(
	"Defined", "CounterType", "ETB", "CounterNum", "EachFromSource", "SubAbility",
)

// cloneETBReflexiveKeys are the keys the admitted "When you do, exile that
// card" link may carry (Superior Spider-Man's DBImmediateTrig).
var cloneETBReflexiveKeys = state.NewNameSet(
	"ConditionDefined", "ConditionPresent", "ConditionCompare", "Execute",
	"RememberObjects", "SubAbility", "TriggerDescription",
)

// CloneETBSubChainOK reports whether an as-enters Clone body's SubAbility$
// chain lies inside the implemented shapes: any number of unconditional
// as-it-enters PutCounter links on itself with no asking modifier, then
// optionally Superior Spider-Man's reflexive exile (CloneETBReflexiveExileOK)
// with its trailing Cleanup that clears the remembered set, and
// Undercover Operative's condition-gated shield-counter rider
// (cloneETBConditionalCounterOK) as the first link. A body with no SubAbility$ is trivially inside it. Any other chain
// (Moritte of the Frost's DB$ Effect replacement) fails closed, keeping the
// body out of the
// ETB election. The key scan only answers a boolean, so map order cannot
// reach an event.
func CloneETBSubChainOK(sa *cards.SA, svars map[string]string) bool {
	if sa == nil {
		return false
	}
	if _, has := sa.Param(cards.PKSubAbility); !has {
		return true
	}
	if sa.Sub == nil {
		// An unlinked chain cannot be classified.
		return false
	}
	for link, d := sa.Sub, 0; link != nil; link, d = link.Sub, d+1 {
		if d >= maxChain {
			return false
		}
		if link.API == "ImmediateTrigger" {
			// The reflexive exile is the second-to-last link, closed by Cleanup.
			return CloneETBReflexiveExileOK(link, svars) && cloneETBCleanupOnly(link.Sub)
		}
		if !IsPutCounter(link) {
			return false
		}
		pc := PutCounterOf(link)
		if !pc.ETB || len(pc.Unread) > 0 || DefinedOf(link).Defined.Text != "Self" {
			return false
		}
		if _, has := link.Param(cards.PKSubAbility); has && link.Sub == nil {
			return false
		}
		// Undercover Operative's shield-counter rider: the condition-gated
		// link shape is classified on its own before the unconditional key
		// whitelist, which a Condition* key would fail. The shared PutCounter
		// shape above (Defined$ Self, ETB$ True, fully read) already ran.
		if _, gated := link.Param(cards.PKConditionDefined); gated {
			return cloneETBConditionalCounterOK(sa, link, pc)
		}
		for k := range link.Params {
			if !cloneETBCounterSubKeys.Has(k) {
				return false
			}
		}
		// Dominion Saboteur's rider: "it enters with additional counters on
		// it equal to the same number and kinds of counters the copied
		// permanent has on it". The origin is the RememberCloneOrigin$ copy
		// choice, so the link reads EachFromSource$ Remembered; the chain is
		// exactly that one link, optionally closed by ClearRemembered$.
		if strings.EqualFold(pc.CounterType, "EachFromSource") {
			if link != sa.Sub || pc.EachFromSource != "Remembered" || !CloneOf(sa).RememberCloneOrigin {
				return false
			}
			if _, has := link.Param(cards.PKCounterNum); has {
				return false
			}
			return link.Sub == nil || cloneETBCleanupOnly(link.Sub)
		}
		if pc.EntryFoldBlocked {
			return false
		}
	}
	return true
}

// cloneETBConditionalCounterOK classifies the one condition-gated as-enters
// counter rider this build admits: Undercover Operative's "except it enters
// with a shield counter on it if you control that creature" (CR 707.9b). The
// caller has already run the shared PutCounter rider shape (Defined$ Self |
// ETB$ True, fully read). The root Clone remembers its copy origin
// (RememberCloneOrigin$ True), and the FIRST chain link's gate is exactly
// ConditionDefined$ Remembered -- the remembered copy origin -- narrowed by
// a present ConditionPresent$ spec, with ConditionCompare$ absent or the GE1
// count. Every other key on the link stays out: the counter keys plus the
// three Condition* keys cloneETBReflexiveKeys already names (the reflexive
// exile's gate's own vocabulary), with the reflexive shape's non-condition
// keys -- Execute$, RememberObjects$, TriggerDescription$ -- rejected typed,
// so the link stays the bare counter rider and any other Condition* key
// (ConditionZone$, ConditionPresent2$, ...) fails. The gate is evaluated by
// the ordinary chain walk (registry.go's linkConditionMet) against the
// chain-local remembered set, so a copy of an opponent's creature -- and a
// declined election, where the whole replacement is not applied (CR 614.1c)
// -- gets no counter. The link's chain is empty or the ClearRemembered$
// Cleanup.
func cloneETBConditionalCounterOK(sa, link *cards.SA, pc *PutCounterParams) bool {
	if link != sa.Sub || !CloneOf(sa).RememberCloneOrigin {
		return false
	}
	if definedSpecCodes.Code(link.ParamStr(cards.PKConditionDefined)) != definedSpecRemembered {
		return false
	}
	if strings.TrimSpace(link.ParamStr(cards.PKConditionPresent)) == "" {
		return false
	}
	if cmp := link.ParamStr(cards.PKConditionCompare); cmp != "" {
		if len(cmp) < 3 || CmpOpOf(cmp[:2]) != CmpGE {
			return false
		}
		if n, err := strconv.Atoi(strings.TrimSpace(cmp[2:])); err != nil || n != 1 {
			return false
		}
	}
	for k := range link.Params {
		if !cloneETBCounterSubKeys.Has(k) && !cloneETBReflexiveKeys.Has(k) {
			return false
		}
	}
	if link.HasParam(cards.PKExecute) || link.HasParam(cards.PKRememberObjects) ||
		link.HasParam(cards.PKTriggerDescription) {
		return false
	}
	if n, ok := literalInt(pc.CounterNum); pc.CounterNum.Present && (!ok || n < 0) {
		return false
	}
	if pc.EntryFoldBlocked {
		return false
	}
	return link.Sub == nil || cloneETBCleanupOnly(link.Sub)
}

// CloneETBReflexiveExileOK reports whether link is exactly Superior
// Spider-Man's "When you do, exile that card": an ImmediateTrigger over the
// remembered copy origin whose Execute$ is a ChangeZone of the delayed
// trigger's remembered card (its last known information) from the graveyard
// to exile.
func CloneETBReflexiveExileOK(link *cards.SA, svars map[string]string) bool {
	if link == nil || link.API != "ImmediateTrigger" {
		return false
	}
	for k := range link.Params {
		if !cloneETBReflexiveKeys.Has(k) {
			return false
		}
	}
	if link.ParamStr(cards.PKConditionDefined) != "Remembered" || link.ParamStr(cards.PKRememberObjects) != "RememberedCard" ||
		link.ParamStr(cards.PKConditionPresent) != "Card" || link.ParamStr(cards.PKConditionCompare) != "GE1" {
		return false
	}
	body := cards.ResolveSVar(svars, link.ParamStr(cards.PKExecute))
	if body == nil || body.API != "ChangeZone" || body.Sub != nil {
		return false
	}
	return body.ParamStr(cards.PKDefined) == "DelayTriggerRememberedLKI" &&
		body.ParamStr(cards.PKOrigin) == "Graveyard" && body.ParamStr(cards.PKDestination) == "Exile" &&
		len(body.Params) == 3
}

// cloneETBCleanupOnly reports whether link is the single trailing
// `DB$ Cleanup | ClearRemembered$ True` that ends the remembered window.
func cloneETBCleanupOnly(link *cards.SA) bool {
	return link != nil && link.API == "Cleanup" && link.Sub == nil && len(link.Params) == 1 &&
		link.ParamStr(cards.PKClearRemembered) == "True"
}

// CloneETBNoCopy reports that an as-enters copy election was recorded on the
// entering object and no copy will be made: the controller declined ("Enter
// as itself", also the only option with nothing to copy), or the chosen
// template is no longer a legal copy source when the replacement applies.
// The optional replacement is then not applied at all (CR 614.1c), so its
// SubAbility$ rider -- the copy's "enters with additional counters"
// exception -- must not run either. False when no election was recorded:
// that path keeps effClone's loud unimplemented-API fallback.
func CloneETBNoCopy(g *state.Game, c *Ctx, sa *cards.SA) bool {
	if c == nil || sa == nil || !c.CloneEnter.ETB || !c.CloneEnter.ChoiceValid || !isCloneSA(sa) {
		return false
	}
	return c.CloneEnter.Choice == 0 || !cloneETBTemplateLegal(g, c, CloneOf(sa))
}
