package rules

import (
	"strings"

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
//     action below), the bearer no longer matches the Aura's Enchant spec,
//     or the bearer gained protection from the Aura. This is the "becomes
//     illegal later" half of CR 704.5m. Either way the Aura goes to its
//     owner's graveyard.
//   - CR 704.5m, "an Aura attached to nothing": an Aura on the battlefield
//     with AttachedTo == 0 goes to the graveyard. (An Equipment is allowed to
//     sit unattached, so the "attached to nothing" wording is Aura-only.)
//   - CR 704.5n, "an Equipment or Fortification attached to something
//     illegal": an Equipment whose bearer is now a non-creature is detached
//     (an Attach with no IDs), and so is one whose bearer left the
//     battlefield entirely; a Fortification whose bearer is no longer a land
//     detaches the same way (CR 702.67b's "attached to a nonland permanent
//     becomes unattached"). The land test reads the CURRENT derived type
//     list, not the printed face: a Darksteel Mutation-shaped layer-4 static
//     that strips the Land card type ends the attachment just as a printed
//     nonland bearer would.
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
			if o.BestowedAttached() {
				// CR 702.114b: a bestowed permanent that is no longer attached
				// to a legal creature becomes a creature again -- it DETACHES
				// and stays on the battlefield, never taking the "Aura attached
				// to nothing / bearer left / illegal bearer" graveyard arms
				// below. The illegal-bearer reading chosen here: the bearer
				// left the battlefield, is no longer a creature, or gained
				// protection from the bestowed card's colours -- any of the
				// three emits one detach Attach (no IDs) and the card is a
				// creature again from the derived type switch. (The Aura arm
				// binning it to the graveyard would contradict 702.114b's own
				// "becomes a creature again if it's not attached".)
				if bearer == nil || bearer.Zone != state.ZBattlefield ||
					!e.IsCreature(bearer.ID) || e.protectedFrom(bearer.ID, o.ID) {
					e.emit(events.Event{Kind: events.Unattached, Obj: id,
						IDs:  []state.ObjID{o.AttachedTo},
						Text: "bestowed attachment ended"})
					changed = true
				}
				continue
			}
			if bearer == nil || bearer.Zone != state.ZBattlefield {
				// The bearer left the battlefield: an Equipment detaches
				// (CR 704.5n), an Aura goes to the graveyard (CR 704.5m).
				if isAura(o) {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "attached to an object that left the battlefield"})
				} else {
					e.emit(events.Event{Kind: events.Unattached, Obj: id,
						IDs:  []state.ObjID{o.AttachedTo},
						Text: "bearer left the battlefield"})
				}
				changed = true
				continue
			}
			if isAura(o) && (!e.auraStillMatchesEnchant(o, bearer) || e.protectedFrom(bearer.ID, o.ID)) {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZBattlefield, To: state.ZGraveyard,
					Text: "attached to something it can no longer legally enchant"})
				changed = true
				continue
			}
			if isEquipment(o) && (bearer.ReconfiguredAttached() || !bearer.EffectiveIsCreature()) {
				// CR 702.150c: an attached Reconfigure card is not a creature,
				// so another Equipment riding it detaches like from any other
				// non-creature bearer (CR 301.5c / 704.5n).
				e.emit(events.Event{Kind: events.Unattached, Obj: id,
					IDs:  []state.ObjID{o.AttachedTo},
					Text: "Equipment bearer is no longer a creature"})
				changed = true
				continue
			}
			if isFortification(o) && !e.IsLand(bearer.ID) {
				// CR 704.5n's Fortification half (CR 702.67b): the bearer is no
				// longer a land, so the Fortification detaches and stays on the
				// battlefield. The read is the DERIVED type list -- a bearer that
				// lost its Land type to a layer-4 static is as illegal as one
				// printed without it.
				e.emit(events.Event{Kind: events.Unattached, Obj: id,
					IDs:  []state.ObjID{o.AttachedTo},
					Text: "Fortification bearer is no longer a land"})
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
	return e.matchesSpecFrom(strings.TrimSpace(spec), bearer.ID, o.Controller, o.ID)
}

// isAura reports whether a permanent has the Aura subtype.
func isAura(o *state.Object) bool { return hasType(o, "Aura") }

// isEquipment reports whether a permanent has the Equipment subtype.
func isEquipment(o *state.Object) bool { return hasType(o, "Equipment") }

// isFortification reports whether a permanent has the Fortification subtype.
func isFortification(o *state.Object) bool { return hasType(o, "Fortification") }

// isRole reports whether a permanent has the Role subtype. The nine
// `.cards/tokenscripts/role_*.txt` scripts each print
// `Types:Enchantment Aura Role`, so a minted Role token carries the word;
// real printed Role cards do NOT (Forge omits the subtype there), which is
// recorded as an open issue -- the exclusivity sweep below keys on this
// predicate, so it only ever fires for minted Role tokens.
func isRole(o *state.Object) bool { return hasType(o, "Role") }

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
