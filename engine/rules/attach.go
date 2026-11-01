package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
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
//   - "anything attached to an object that left the battlefield": broken on
//     the FIRST state-based check after the departure. events' Move fold
//     (CR 400.7: a flicker can return the bearer under the same ObjID before
//     any checkpoint) marks each attachment to the departing permanent
//     UnattachPending at the move itself -- recording the departed bearer as
//     LastBearer while deliberately leaving the live AttachedTo link
//     standing, so the mid-resolution window before the next checkpoint
//     still shows the attachment fastened -- and when the bearer RE-ENTERS
//     the battlefield the entry arm breaks that link (the returned permanent
//     is a new object nothing is attached to; the link must not survive to
//     the SBA pass where destroyLethalDamage runs before this walk). The
//     pending arm at the top of this walk then publishes the queued detach
//     with the same Unattached / graveyard MoveZone the bearer-left read
//     produces.
//
// The bearer-left arm below (`bearer == nil || bearer.Zone != ZBattlefield`
// with AttachedTo still non-zero) is therefore reachable only for a link the
// fold never touched: the graveyard-enchant family, whose Aura attached to a
// card that was ALREADY off the battlefield -- every link the fold marked
// reaches the pending arm above first and never falls through.
//
// Reports whether anything changed, so it plugs into checkStateBased's own
// pass loop (a graveyard-bound Aura can itself trigger on its zone change,
// so "changed" keeps the loop going until the board is stable).
func (e *Engine) attachmentSBAs(b *sbaBatch) {
	// The battlefield snapshot the walk ranges lives in the Engine's SBA
	// scratch (sbaIDBuf).
	ids := e.sbaIDBuf
	e.sbaIDBuf = nil
	// The board's layer-4 presence for sbaIsAura.
	anyLType := e.activeHasLType()
	for _, p := range e.G.AliveFrom(0) {
		ids = append(ids[:0], e.G.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.PhasedOut {
				// CR 702.25b/d: a phased-out permanent is treated as though it
				// does not exist, so attachment SBAs do not fire on it (it and
				// anything attached to it phase out together).
				continue
			}
			if o.UnattachPending {
				// CR 400.7/704.5m-n: the bearer-departure fold in events'
				// Move (engine/events/apply_zone.go) marked this attachment
				// at its bearer's departure -- a flicker can return the
				// bearer under the same ObjID before any SBA checkpoint, so
				// the bearer-left arm below, which reads the live link, can
				// never see the departure; the queued mark survives the
				// round trip and publishes here. The fold left the live
				// AttachedTo link standing for the window while the bearer
				// was off the battlefield (the Murderous Spoils /
				// Witness Protection mid-chain read), and the bearer's
				// re-entry -- when the flicker has one -- already broke it
				// (breakQueuedAttachmentsOnEntry), so the publication below
				// is what spends the queue either way, with the same
				// Unattached / graveyard MoveZone the
				// bearer-left arm emits, so Mode$ Unattached triggers
				// (Grafted Exoskeleton) and the CR 702.114b bestowed detach
				// log keep firing exactly as before. The graveyard-enchant
				// exemption is deliberately NOT re-checked: a queued detach
				// means the bearer DEPARTED the battlefield while this
				// object was attached, and the exemption's window (this Aura
				// entered attached to a card that was already in a
				// graveyard, whose derived enchant still names that zone)
				// can only exist while no such departure has happened --
				// Animate Dead's family rewrites its enchant to the
				// remembered-bearer spec before it attaches to the
				// reanimated creature, so a pending Aura is never in that
				// window.
				bestowed := false
				if f := o.Face(); f != nil {
					_, bestowed = f.KeywordParam("Bestow")
				}
				switch {
				case bestowed:
					b.add(events.Event{Kind: events.Unattached, Obj: id,
						IDs:  []state.ObjID{o.LastBearer},
						Text: "bestowed attachment ended"})
				case e.isAura(o):
					b.add(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard,
						Text: "attached to an object that left the battlefield"})
				default:
					b.add(events.Event{Kind: events.Unattached, Obj: id,
						IDs:  []state.ObjID{o.LastBearer},
						Text: "bearer left the battlefield"})
				}
				continue
			}
			if o.HasAttachedPlayer {
				// Enchant:Player Auras have no object bearer. A lost seat
				// cannot remain enchanted (CR 800.4), and an illegal
				// attachment takes the same graveyard SBA as an object Aura.
				p := o.AttachedPlayer
				if !e.isAura(o) || int(p) >= len(e.G.Players) || e.G.Players[p].Lost ||
					!e.playerAuraStillMatchesEnchant(o, p) || e.playerProtectedFrom(p, o.ID) {
					b.add(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "illegal player attachment"})
				}
				continue
			}
			if o.AttachedTo == 0 {
				// A detached Aura has nothing legal to do on the battlefield.
				if e.sbaIsAura(o, anyLType) {
					b.add(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "Aura attached to nothing"})
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
					b.add(events.Event{Kind: events.Unattached, Obj: id,
						IDs:  []state.ObjID{o.AttachedTo},
						Text: "bestowed attachment ended"})
				}
				continue
			}
			if bearer == nil || bearer.Zone != state.ZBattlefield {
				// The bearer is off the battlefield while AttachedTo is still
				// set: since the CR 400.7 departure fold queues every
				// departed-bearer link as UnattachPending -- published by the
				// pending arm above, which the bearer's re-entry (when the
				// flicker has one) reaches first after breaking the link --
				// this arm serves only the links the fold never marks -- the
				// graveyard-enchant family (Animate
				// Dead, Dance of the Dead), whose Aura attached to a card
				// that was already in a graveyard at resolution. (A bearer
				// that left the battlefield while attached detaches through
				// the UnattachPending arm above instead.) The
				// graveyard-enchant family is the measured exception: the
				// Aura is LEGALLY a creature card in a graveyard -- CR 704.5m
				// sweeps an attachment that is no longer legal, and the
				// Aura's current enchant spec still admits exactly this
				// bearer here. The test is deliberately zone-POSITIVE (the
				// spec must match the bearer AND carry an inZone<X> word whose
				// X names the bearer's current zone), because
				// MatchesSpecFrom's bare type words are zone-blind (spec
				// `Creature` matches a graveyard bear too) and the exemption
				// must never save an ordinary Aura whose bearer died.
				if e.isAura(o) && bearer != nil && e.auraEnchantZoneAdmits(o, bearer) {
					continue
				}
				if e.isAura(o) {
					b.add(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard, Text: "attached to an object that left the battlefield"})
				} else {
					b.add(events.Event{Kind: events.Unattached, Obj: id,
						IDs:  []state.ObjID{o.AttachedTo},
						Text: "bearer left the battlefield"})
				}
				continue
			}
			if why := e.attachedKindIllegal(o); why != "" {
				b.add(events.Event{Kind: events.Unattached, Obj: id,
					IDs:  []state.ObjID{o.AttachedTo},
					Text: why})
				continue
			}
			if e.isAura(o) && (!e.auraStillMatchesEnchant(o, bearer) || e.protectedFrom(bearer.ID, o.ID)) {
				b.add(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZBattlefield, To: state.ZGraveyard,
					Text: "attached to something it can no longer legally enchant"})
				continue
			}
			if e.isEquipment(o) && (bearer.ReconfiguredAttached() || bearer.ImpendingDormant() ||
				(!e.IsCreature(bearer.ID) && !e.equipmentEquipSpecAdmits(o, bearer))) {
				// CR 702.150c: an attached Reconfigure card is not a creature,
				// so another Equipment riding it detaches like from any other
				// non-creature bearer (CR 301.5c / 704.5n). The creature read
				// is the DERIVED type list, as the Fortification arm below
				// reads Land: an animated land (Soulstone Sanctuary after its
				// {4}) is a creature and keeps its Equipment, a creature a
				// layer-4 effect turned into a noncreature drops it -- unless
				// the Equipment says otherwise (CR 301.5c): a bearer its Equip
				// keyword names (Luxior's "Equip planeswalker") stays equipped
				// even after it stops being a creature, exactly as it was
				// equipped while one.
				b.add(events.Event{Kind: events.Unattached, Obj: id,
					IDs:  []state.ObjID{o.AttachedTo},
					Text: "Equipment bearer is no longer a creature"})
				continue
			}
			if e.isFortification(o) && !e.IsLand(bearer.ID) {
				// CR 704.5n's Fortification half (CR 702.67b): the bearer is no
				// longer a land, so the Fortification detaches and stays on the
				// battlefield. The read is the DERIVED type list -- a bearer that
				// lost its Land type to a layer-4 static is as illegal as one
				// printed without it.
				b.add(events.Event{Kind: events.Unattached, Obj: id,
					IDs:  []state.ObjID{o.AttachedTo},
					Text: "Fortification bearer is no longer a land"})
			}
		}
	}
	e.sbaIDBuf = ids[:0]
}

// attachedKindIllegal implements CR 704.5p: an attached object that is a
// creature or a battle, or that is none of Aura, Equipment and Fortification,
// becomes unattached and stays on the battlefield (an Aura that is also a
// creature then takes the CR 303.4d graveyard arm on the next pass, once it
// is attached to nothing). An attached Reconfigure permanent is not a creature
// (CR 702.151b) and a bestowed one is handled by its own arm. The derived type
// list is read, so a layer-4 animation or subtype loss is seen. Returns the
// detach reason, or "" when the attachment is legal.
func (e *Engine) attachedKindIllegal(o *state.Object) string {
	if !o.ReconfiguredAttached() && (e.IsCreature(o.ID) || e.hasType(o, "Battle")) {
		return "attached creature or battle becomes unattached"
	}
	if !e.isAura(o) && !e.isEquipment(o) && !e.isFortification(o) && printsAttachmentType(o) {
		return "non-attachment permanent becomes unattached"
	}
	return ""
}

// printsAttachmentType reports whether o's printed face is an Aura, Equipment
// or Fortification, so a derived list without all three means a layer-4
// effect removed the subtype. A permanent that never printed one (an artifact
// granted Equip by an Animate effect) is left attached: the grant is the only
// route that attaches it.
func printsAttachmentType(o *state.Object) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	for _, t := range f.Types {
		switch t {
		case "Aura", "Equipment", "Fortification":
			return true
		}
	}
	return false
}

// roleSBA implements CR 704.5z (repeated by CR 303.7a): if a permanent has
// more than one Role attached to it controlled by the SAME player, each of
// those Roles except the one with the most recent timestamp is put into its
// owner's graveyard. It is a state-based action, not an attach-time
// replacement: two Roles controlled by different players coexist on one
// bearer, and even two controlled by one player both stay until the next
// state-based check.
//
// Roles are grouped per bearer by their CURRENT controller (a control change
// can make a bearer's two Roles share one controller) and the survivor is
// the greatest LayerTimestamp (CR 613.7e restamps an Aura when it becomes
// attached, so the newest attach wins). Equal timestamps -- only reachable
// from a synthetic board, since an attach takes a fresh Game.Clock tick --
// keep the Role later in the battlefield walk order, which is deterministic:
// AliveFrom(0) seat order, then each battlefield zone slice's own order.
//
// The removal is a plain battlefield-to-graveyard MoveZone, NOT destruction:
// no replacement or regeneration path, and the recursive emit lets the
// swept Role's "leaves the battlefield" triggers fire normally (a Wicked
// Role's "each opponent loses 1 life").
func (e *Engine) roleSBA(b *sbaBatch) {
	// The battlefield snapshot the walk ranges lives in the Engine's SBA
	// scratch, taken so a re-entrant pass allocates its own (sbaIDBuf).
	ids := e.sbaIDBuf
	e.sbaIDBuf = nil
	rolesByBearer := make(map[state.ObjID][]state.ObjID)
	var bearers []state.ObjID
	for _, p := range e.G.AliveFrom(0) {
		ids = append(ids[:0], e.G.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			o := e.G.Obj(id)
			if o == nil || o.PhasedOut || o.AttachedTo == 0 || !e.isRole(o) {
				continue
			}
			if _, ok := rolesByBearer[o.AttachedTo]; !ok {
				bearers = append(bearers, o.AttachedTo)
			}
			rolesByBearer[o.AttachedTo] = append(rolesByBearer[o.AttachedTo], id)
		}
	}
	for _, bearer := range bearers {
		roles := rolesByBearer[bearer]
		if len(roles) < 2 {
			continue
		}
		// newest[i] is the surviving Role index for its controller; a map is
		// used only for membership, never ranged, so no map order can reach
		// an event.
		newest := make(map[state.PlayerID]int, len(roles))
		for i, id := range roles {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			j, ok := newest[o.Controller]
			if !ok {
				newest[o.Controller] = i
				continue
			}
			cur := e.G.Obj(roles[j])
			if cur == nil || o.LayerTimestamp() >= cur.LayerTimestamp() {
				newest[o.Controller] = i
			}
		}
		for i, id := range roles {
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			if newest[o.Controller] == i {
				continue
			}
			b.add(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZBattlefield, To: state.ZGraveyard,
				Text: "another Role on it: the old Role goes to the graveyard"})
		}
	}
	e.sbaIDBuf = ids[:0]
}

// attachRefused implements the CR 701.3a/b refusal half of emit's Attach
// gate: refused is true when an Attach fastening the on-battlefield object
// obj to bearerID would either do nothing (obj is already attached to that
// permanent) or fasten it to something the attachment SBAs would at once
// undo (an Aura whose current enchant spec does not admit the bearer, an
// Equipment whose bearer is not a creature its Equip keyword names or is an
// attached Reconfigure or an impending permanent, a Fortification whose
// bearer is not a land). The predicates are the same ones attachmentSBAs reads below, so an attach the
// SBA would undo is exactly an attach that is refused. note is true when the
// refusal is worth a Note (an illegal bearer) rather than silent (the
// same-bearer no-op of CR 701.3b's "the effect does nothing"). The gate is
// not judged for an attaching object off the battlefield (an Aura spell
// attaching while it resolves from the stack), a bestowed attachment, an
// off-battlefield bearer (Animate Dead's graveyard card) or a player bearer.
func (e *Engine) attachRefused(obj, bearerID state.ObjID) (refused, note bool) {
	o := e.G.Obj(obj)
	if o == nil || o.Zone != state.ZBattlefield || o.BestowedAttached() {
		return false, false
	}
	if o.AttachedTo == bearerID && !o.HasAttachedPlayer {
		return true, false
	}
	bearer := e.G.Obj(bearerID)
	if bearer == nil || bearer.Zone != state.ZBattlefield {
		return false, false
	}
	switch {
	case e.isAura(o):
		return !e.auraStillMatchesEnchant(o, bearer), true
	case e.isEquipment(o):
		// CR 702.150c: an attached Reconfigure card is not a creature, and
		// an impending permanent is not yet a creature either. A bearer the
		// Equipment's own Equip keyword names is legal anyway (CR 301.5c,
		// equipmentEquipSpecAdmits): Luxior's "Equip planeswalker" attaches
		// to a walker that is a creature only through Luxior's own static.
		return bearer.ReconfiguredAttached() || bearer.ImpendingDormant() ||
			(!e.IsCreature(bearer.ID) && !e.equipmentEquipSpecAdmits(o, bearer)), true
	case e.isFortification(o):
		return !e.IsLand(bearer.ID), true
	}
	return false, false
}

// equipmentEquipSpecAdmits is CR 301.5c's "unless the Equipment says
// otherwise" exception: a bearer that is not a creature is still a legal
// equip target when the Equipment's Equip keyword names it -- Luxior,
// Giada's Gift's "Equip planeswalker" attaches to a walker that is not yet a
// creature (the Equipment's own static makes the bearer one only once
// attached). The read is the DERIVED Equip lines (a layer-6 removal of the
// line removes the exception with it), each one's bearer restriction the
// field engine/cards/kw_equip.go's expansion writes the activation's
// ValidTgts$ from (cards.AttachBearerSpec); a line without one equips
// creatures only (the expansion's defaultTgts). Fortify's land
// restriction is not this exception -- a Fortification's bearer must be a
// land regardless (CR 702.67b), checked by the Fortification arms.
func (e *Engine) equipmentEquipSpecAdmits(o, bearer *state.Object) bool {
	for _, k := range e.Derived(o.ID).Keywords {
		if cardsKeywordHead(k) != kwhEquip.S {
			continue
		}
		spec := cards.AttachBearerSpec(k)
		if spec == "" {
			spec = "Creature.YouCtrl"
		}
		if e.matchesSpecFrom(spec, bearer.ID, o.Controller, o.ID) {
			return true
		}
	}
	return false
}

// auraStillMatchesEnchant reports whether bearer still satisfies the Aura's
// Enchant keyword's restriction (the "creature" of K:Enchant:Creature, or the
// more specific "Creature.YouCtrl" a real card spells; the first field after
// the colon is the spec, any later fields are the human prompt and dropped).
// The read is the DERIVED enchant (derivedKeywordParam: printed plus layer-6
// granted, minus layer-6 removed), not the printed face -- Animate Dead's
// RemoveKeywords$/Keywords$ pair rewrites the Enchant the SBA must honour, and
// a printed-only read would sweep the reanimated pair at the very next
// checkpoint. An Aura with no derived Enchant keyword is treated as
// still-matching so it is never spuriously destroyed; the intended side effect
// is that an Aura under a RemoveAbilities effect (Humility) -- which has no
// derived Enchant at all -- falls into that same still-matching arm, not a
// sweep. Nothing in the corpus prints an Aura without an Enchant keyword, so
// the arm only ever covers granted-absent shapes.
func (e *Engine) auraStillMatchesEnchant(o, bearer *state.Object) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	param, ok := e.derivedKeywordParamH(o.ID, kwhEnchant)
	if !ok || strings.TrimSpace(param) == "" {
		return true
	}
	spec, _, _ := strings.Cut(param, ":")
	return e.matchesSpecFrom(strings.TrimSpace(spec), bearer.ID, o.Controller, o.ID)
}

// auraEnchantZoneAdmits is attachmentSBAs' off-battlefield-bearer exemption
// test: the Aura's CURRENT (derived) enchant spec matches the bearer AND
// carries an inZone<X> word whose X names the bearer's current zone. See
// attachmentSBAs for why the zone word is mandatory (a bare spec is
// zone-blind and would exempt ordinary dead-bearer Auras too).
func (e *Engine) auraEnchantZoneAdmits(o, bearer *state.Object) bool {
	param, ok := e.derivedKeywordParamH(o.ID, kwhEnchant)
	if !ok || strings.TrimSpace(param) == "" {
		return false
	}
	spec, _, _ := strings.Cut(param, ":")
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return false
	}
	zoneNamed := false
	for word := range strings.SplitSeq(spec, ".") {
		z, has := strings.CutPrefix(word, "inZone")
		if !has {
			continue
		}
		if zn, known := effects.ParseZoneWord(z); known && zn == bearer.Zone {
			zoneNamed = true
		}
	}
	if !zoneNamed {
		return false
	}
	return e.matchesSpecFrom(spec, bearer.ID, o.Controller, o.ID)
}

func (e *Engine) playerAuraStillMatchesEnchant(o *state.Object, p state.PlayerID) bool {
	if o.Face() == nil {
		return false
	}
	param, ok := o.Face().KeywordParam("Enchant")
	if !ok {
		return false
	}
	spec, _, _ := strings.Cut(param, ":")
	return effects.MatchesPlayerSpecFrom(e.G, strings.TrimSpace(spec), p, o.Controller, o.ID)
}

// isAura reports whether a permanent has the Aura subtype.
func (e *Engine) isAura(o *state.Object) bool { return e.hasType(o, "Aura") }

// isEquipment reports whether a permanent has the Equipment subtype.
func (e *Engine) isEquipment(o *state.Object) bool { return e.hasType(o, "Equipment") }

// isFortification reports whether a permanent has the Fortification subtype.
func (e *Engine) isFortification(o *state.Object) bool { return e.hasType(o, "Fortification") }

// isRole reports whether a permanent has the Role subtype. The nine
// `.cards/tokenscripts/role_*.txt` scripts each print
// `Types:Enchantment Aura Role`, so a minted Role token carries the word;
// real printed Role cards do NOT (Forge omits the subtype there), which is
// recorded as an open issue -- the exclusivity sweep below keys on this
// predicate, so it only ever fires for minted Role tokens.
func (e *Engine) isRole(o *state.Object) bool { return e.hasType(o, "Role") }

// hasType is the rules-package subtype read, mirroring the effects-package
// hasType (engine/effects/filter.go) off the battlefield: it scans the printed face.
// On the battlefield the read is typeCharacteristics -- the layer-4 walk's
// BASE plus changes -- because a permanent's current characteristics, not its
// printed face, are what CR 704.5m classifies. The measured case (CR 708.5
// with CR 704.5m): a face-down Gift of Doom is a vanilla 2/2 Creature while
// face down -- its printed Aura subtype does not exist -- so the unattached
// arm above must NOT sweep it to the graveyard as "Aura attached to
// nothing"; the printed read did exactly that, killing it at the next SBA
// checkpoint before its Morph turn-up cost could ever be paid. The same walk
// keeps every face-up read the printed face already answered (the fast path
// returns the base list when no layer-4 effect is active) and correctly
// follows a layer-4 type rewrite on a face-up permanent too. Faced-less
// objects have no types.
func (e *Engine) hasType(o *state.Object, t string) bool {
	if o == nil {
		return false
	}
	if o.Zone == state.ZBattlefield {
		for _, x := range e.typeCharacteristics(o.ID, 0) {
			if strings.EqualFold(x, t) {
				return true
			}
		}
		return false
	}
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
