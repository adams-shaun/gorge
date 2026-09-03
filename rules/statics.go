// Static play restrictions and cost modifiers: the six S: modes besides
// stat:Continuous (layers.go's own concern). These change what is legal and
// what things cost rather than what a permanent's characteristics are, so
// they hook into legalActions and ParseCost rather than the layer system.
package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// staticView is one S: line together with where it came from, so the filter
// predicates that are relative to a source (Self, Other) and the ones
// relative to a controller (YouCtrl, OppCtrl) resolve correctly.
type staticView struct {
	Source     state.ObjID
	Controller state.PlayerID
	Params     map[string]string
}

// activeStatics collects every S:Mode$ <mode> line from a permanent on the
// battlefield. The order is deterministic: AliveFrom(0) walks seats in fixed
// APNAP order, each seat's battlefield zone is a slice built by ordinary
// append (never a map), and each object's Statics is the slice order its
// card script parsed in — nothing here ever ranges a map, so cost adjustment
// and the resulting option list are stable run to run, which is what
// TestActiveStaticsIsDeterministicallyOrdered checks for.
func (e *Engine) activeStatics(mode string) []staticView {
	var out []staticView
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			f := o.Face()
			if f == nil {
				continue
			}
			for _, st := range f.Statics {
				if st.Mode == mode {
					out = append(out, staticView{Source: id, Controller: o.Controller, Params: st.Params})
				}
			}
		}
	}
	return out
}

// actorMatches implements the Caster$/Activator$ parameter, which scopes a
// restriction to whose action it is. A restriction with no such parameter
// applies regardless of actor.
func (e *Engine) actorMatches(sv staticView, key string, actor state.PlayerID) bool {
	spec, ok := sv.Params[key]
	if !ok {
		return true
	}
	return effects.MatchesPlayerSpec(e.G, spec, actor, sv.Controller)
}

// castRestricted reports whether p is forbidden from casting id (CantBeCast).
func (e *Engine) castRestricted(p state.PlayerID, id state.ObjID) bool {
	for _, sv := range e.activeStatics("CantBeCast") {
		if !e.actorMatches(sv, "Caster", p) {
			continue
		}
		if effects.MatchesSpecFrom(e.G, sv.Params["ValidCard"], id, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// abilityRestricted reports whether id's (mana) ability is forbidden from
// being activated (CantBeActivated). A nonexistent object has no ability to
// restrict, so it degrades to false rather than dereferencing a nil Object.
func (e *Engine) abilityRestricted(id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	for _, sv := range e.activeStatics("CantBeActivated") {
		if !e.actorMatches(sv, "Activator", o.Controller) {
			continue
		}
		if effects.MatchesSpecFrom(e.G, sv.Params["ValidCard"], id, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// adjustedCost applies RaiseCost and ReduceCost to id's printed mana cost.
// Both modes only ever touch the Generic component (never Colored), so
// clamping Generic at zero is already CR 601.2f-safe on its own: a reduction
// can consume the generic requirement down to nothing but can never reach
// into the coloured pips to reduce those, because this function never
// writes to c.Colored at all.
//
// A missing object or a Face()-less one (an ability object or a token
// mid-resolution) degrades to the zero Cost rather than panicking; nothing
// in this build calls adjustedCost with such an id today; the guard exists
// because a caller might one day compute a would-be cost speculatively.
func (e *Engine) adjustedCost(p state.PlayerID, id state.ObjID) Cost {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Cost{}
	}
	c := ParseCost(o.Face().ManaCost)
	apply := func(mode string, sign int32) {
		for _, sv := range e.activeStatics(mode) {
			if !e.actorMatches(sv, "Activator", p) {
				continue
			}
			if !effects.MatchesSpecFrom(e.G, sv.Params["ValidCard"], id, sv.Controller, sv.Source) {
				continue
			}
			c.Generic += sign * parseAmount(sv.Params["Amount"], 1)
		}
	}
	apply("RaiseCost", +1)
	apply("ReduceCost", -1)
	if c.Generic < 0 {
		c.Generic = 0
	}
	return c
}

// alternativeCosts lists extra ways to cast id, each becoming its own
// "cast" option in legalActions so the client can present the choice
// without knowing any rules. Two sources: another permanent's static
// granting the alternative (activeStatics, battlefield-only), and a static
// the card carries on itself, which activeStatics alone would never see
// while the card is still in hand.
//
// p is unused today: the table this task implements gives AlternativeCost
// only ValidCard$/Cost$, no Activator$-style actor scoping. The parameter is
// kept for symmetry with adjustedCost and because Forge does have
// AlternativeCost lines gated by who is casting; adding that scoping later
// is then a one-line change here rather than a signature change at every
// call site.
func (e *Engine) alternativeCosts(p state.PlayerID, id state.ObjID) []Cost {
	var out []Cost
	for _, sv := range e.activeStatics("AlternativeCost") {
		if !effects.MatchesSpecFrom(e.G, sv.Params["ValidCard"], id, sv.Controller, sv.Source) {
			continue
		}
		out = append(out, ParseCost(sv.Params["Cost"]))
	}
	if o := e.G.Obj(id); o != nil {
		if f := o.Face(); f != nil {
			for _, st := range f.Statics {
				if st.Mode == "AlternativeCost" {
					out = append(out, ParseCost(st.Params["Cost"]))
				}
			}
		}
	}
	return out
}

// blockRestricted reports whether blocker is forbidden from blocking
// attacker (CantBlock, CantBlockBy). Nothing calls this yet: real
// declare-blockers option generation is Task 21/22's (askBlockers and
// handleBlockers are still stubs.go's no-ops). This is defined and tested
// here in isolation so those tasks have a working primitive to wire in, the
// same shape layers.go's EndOfTurnCleanup already used for Task 21.
func (e *Engine) blockRestricted(blocker, attacker state.ObjID) bool {
	for _, sv := range e.activeStatics("CantBlock") {
		if effects.MatchesSpecFrom(e.G, sv.Params["ValidCard"], blocker, sv.Controller, sv.Source) {
			return true
		}
	}
	for _, sv := range e.activeStatics("CantBlockBy") {
		if !effects.MatchesSpecFrom(e.G, sv.Params["ValidCard"], attacker, sv.Controller, sv.Source) {
			continue
		}
		spec, ok := sv.Params["ValidBlocker"]
		if !ok {
			return true
		}
		if effects.MatchesSpecFrom(e.G, spec, blocker, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// parseAmount reads an Amount$ parameter, falling back to def for anything
// that is not a plain integer (missing, empty, or a malformed value from a
// card script this build cannot otherwise validate).
func parseAmount(s string, def int32) int32 {
	n := def
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		n = int32(v)
	}
	return n
}

func init() {
	effects.RegisterNonAPI("stat:CantBeCast", "stat:CantBeActivated", "stat:RaiseCost",
		"stat:ReduceCost", "stat:AlternativeCost", "stat:CantBlock", "stat:CantBlockBy",
		"stat:Continuous")
}

// altCostLabel names the nth (0-indexed) alternative-cost option for a
// spell, distinct from the base "Cast <name>" label and from each other when
// a card somehow offers more than one alternative.
func altCostLabel(name string, i int) string {
	if i == 0 {
		return "Cast " + name + " (alternative cost)"
	}
	return fmt.Sprintf("Cast %s (alternative cost %d)", name, i+1)
}
