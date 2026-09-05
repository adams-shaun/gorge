package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// attachmentSBAs implements CR 704.5m/n, the state-based actions that keep
// attachments legal. This build's attachments are Auras and Equipment; a
// permanent is "attached" exactly when its state.Object.AttachedTo is
// non-zero. Four of the five rules are the same "it became illegal later"
// shape that lands here rather than being checked only at attach time:
//
//   - CR 704.5m, "an Aura is attached to something it can no longer legally
//     be attached to": the bearer is gone (moved zones -- its state-based
//     action below), or the bearer proved not to be a permanent whose types
//     still match the Aura's Enchant spec, the "becomes illegal later" half
//     of CR 704.5m the dispatch names. Either way the Aura goes to its
//     owner's graveyard.
//   - CR 704.5m, "an Aura attached to nothing": an Aura on the battlefield
//     with AttachedTo == 0 goes to the graveyard. (An Equipment is allowed to
//     sit unattached, so the "attached to nothing" wording is Aura-only.)
//   - CR 704.5n, "an Equipment attached to something invalid": an Equipment
//     whose bearer is now a non-creature is detached (an Attach with no IDs),
//     and so is one whose bearer left the battlefield entirely.
//   - "anything attached to an object that left the battlefield": the same
//     detached-for-Equipment, destroyed-Aura handling CR 704.5m's own
//     "attached to nothing" already gives the Aura (once its bearer left,
//     AttachedTo points at no battlefield permanent), so the Aura half does
//     not need a separate clause -- but the case is listed here so the
//     reader finds all five rules in one place.
//
// Reports whether anything changed, so it plugs into checkStateBased's own
// pass loop (a graveyard-bound Aura can itself trigger on its zone change,
// so "changed" keeps the loop going until the board is stable).
func (e *Engine) attachmentSBAs() bool {
	changed := false
	for _, p := range e.G.AliveFrom(0) {
		ids := append([]state.ObjID(nil), e.G.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.AttachedTo == 0 {
				// A detached Aura has nothing legal to do on the battlefield.
				if isAura(o) {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "Aura attached to nothing"})
					changed = true
				}
				continue
			}
			bearer := e.G.Obj(o.AttachedTo)
			if bearer == nil || bearer.Zone != state.ZBattlefield {
				// The bearer left the battlefield: an Equipment detaches
				// (CR 704.5n), an Aura goes to the graveyard (CR 704.5m).
				if isAura(o) {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "attached to an object that left the battlefield"})
				} else {
					e.emit(events.Event{Kind: events.Attach, Obj: id})
				}
				changed = true
				continue
			}
			if isAura(o) && !e.auraStillMatchesEnchant(o, bearer) {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZBattlefield, To: state.ZGraveyard,
					Text: "attached to something its Enchant no longer allows"})
				changed = true
				continue
			}
			if isEquipment(o) && bearer.Face() != nil && !bearer.Face().IsCreature() {
				e.emit(events.Event{Kind: events.Attach, Obj: id,
					Text: "Equipmentbearer is no longer a creature"})
				changed = true
			}
		}
	}
	return changed
}

// auraStillMatchesEnchant reports whether bearer still satisfies the Aura's
// Enchant keyword's restriction (the "creature" of K:Enchant:Creature, or the
// more specific "Creature.YouCtrl" a real card spells; the first field after
// the colon is the spec, any later fields are the human prompt and dropped).
// An Aura with no Enchant keyword is treated as still-matching so it is never
// spuriously destroyed -- nothing in the corpus prints an Aura without one,
// but an Enchant-less Aura should not be the thing this SBA guesses about.
func (e *Engine) auraStillMatchesEnchant(o, bearer *state.Object) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	param, ok := f.KeywordParam("Enchant")
	if !ok || strings.TrimSpace(param) == "" {
		return true
	}
	spec, _, _ := strings.Cut(param, ":")
	return effects.MatchesSpecFrom(e.G, strings.TrimSpace(spec), bearer.ID, o.Controller, o.ID)
}

// isAura reports whether a permanent has the Aura subtype.
func isAura(o *state.Object) bool { return hasType(o, "Aura") }

// isEquipment reports whether a permanent has the Equipment subtype.
func isEquipment(o *state.Object) bool { return hasType(o, "Equipment") }

// hasType is the rules-package view of an object's printed types, mirroring
// the effects-package hasType (effects/filter.go). Faced-less objects have no
// types.
func hasType(o *state.Object, t string) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	for _, x := range f.Types {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}

// hasLegalTarget reports whether the activated ability's ValidTgts$ spec has
// at least one legal candidate right now (a player the spec accepts, or an
// object on any battlefield it accepts). legalActions calls it before offering
// a non-mana activate option, so Equip is only offered when there is actually
// a creature to equip; askTarget would otherwise create-and-fizzle on take.
func (e *Engine) hasLegalTarget(p state.PlayerID, source state.ObjID, spec string) bool {
	if targetsPlayers(spec) {
		for _, q := range e.G.AliveFrom(0) {
			if effects.MatchesPlayerSpec(e.G, spec, q, p) {
				return true
			}
		}
	}
	for _, q := range e.G.AliveFrom(0) {
		for _, oid := range e.G.Zone(state.ZBattlefield, q) {
			if effects.MatchesSpecFrom(e.G, spec, oid, p, source) {
				return true
			}
		}
	}
	return false
}

// activateAbility starts an activated ability (an Equip, today) on p's behalf:
// pay its cost, mint the ability's stack object (events.AbilityPush, so a
// log-only replay creates the same object -- Ruling T20-a's shape), and, if
// it names a target, hand p the same target decision a spell's cast asks
// (askTarget scoped to the stack object just pushed). handleTarget records the
// choice onto that stack object's Targets and grants p priority (CR 117.3c).
// A targetless ability grants p priority directly once it is on the stack.
func (e *Engine) activateAbility(p state.PlayerID, id state.ObjID, ai int) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || ai < 0 || ai >= len(o.Face().Abilities) {
		return
	}
	ab := o.Face().Abilities[ai]
	cost := ParseCost(ab.Params["Cost"])
	if !e.castable(p, id, cost) || !e.payMana(p, cost) {
		return
	}
	e.emit(events.Event{Kind: events.AbilityPush, Player: p, Obj: id, Amount: int32(ai)})
	if len(e.G.Stack) == 0 {
		return
	}
	top := e.G.Stack[len(e.G.Stack)-1]
	if e.G.Obj(top) != nil && ab.Params["ValidTgts"] != "" {
		e.askTarget(p, top, ab)
		return
	}
	e.emit(events.Event{Kind: events.Priority, Player: p, Amount: 0})
}
