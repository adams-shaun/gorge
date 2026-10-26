package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters copy's "except it enters with N additional +1/+1 counters"
// rider (Altered Ego: `DB$ Clone | ... | SubAbility$ DBAddCounter` with
// `DBAddCounter:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | ETB$ True
// | CounterNum$ X`). The rider is part of the copy election, not of the
// entry: CR 707.9 makes it an exception of the copy effect, and CR 614.1c
// makes the whole "you may have it enter as a copy" replacement optional,
// so a declined election (or one with nothing to copy) enters the object as
// itself with NO extra counters.

// cloneETBCounterSubKeys are the keys an admitted rider link may carry.
var cloneETBCounterSubKeys = state.NewNameSet(
	"Defined", "CounterType", "ETB", "CounterNum", "SubAbility",
)

// cloneETBReflexiveKeys are the keys the admitted "When you do, exile that
// card" link may carry (Superior Spider-Man's DBImmediateTrig).
var cloneETBReflexiveKeys = state.NewNameSet(
	"ConditionDefined", "ConditionPresent", "ConditionCompare", "Execute",
	"RememberObjects", "SubAbility", "TriggerDescription",
)

// CloneETBSubChainOK reports whether an as-enters Clone body's SubAbility$
// chain lies inside the implemented shapes: any number of unconditional
// `DB$ PutCounter | Defined$ Self | ETB$ True` rider links with no asking
// modifier, then optionally Superior Spider-Man's reflexive exile
// (CloneETBReflexiveExileOK) with its trailing `DB$ Cleanup | ClearRemembered$
// True`. A body with no SubAbility$ is trivially inside it. Any other chain
// (Moritte of the Frost's DB$ Effect replacement, Undercover Operative's
// ConditionPresent$ shield counter) fails closed, keeping the body out of the
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
		for k := range link.Params {
			if !cloneETBCounterSubKeys.Has(k) {
				return false
			}
		}
		pc := PutCounterOf(link)
		if !pc.ETB || pc.EntryFoldBlocked || len(pc.Unread) > 0 || DefinedOf(link).Defined.Text != "Self" {
			return false
		}
		if _, has := link.Param(cards.PKSubAbility); has && link.Sub == nil {
			return false
		}
	}
	return true
}

// CloneETBReflexiveExileOK reports whether link is exactly Superior
// Spider-Man's "When you do, exile that card": an ImmediateTrigger over the
// remembered copy origin whose Execute$ is a `DB$ ChangeZone | Defined$
// DelayTriggerRememberedLKI` from the graveyard to exile.
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
