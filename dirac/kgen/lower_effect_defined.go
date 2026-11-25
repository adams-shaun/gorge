package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"strings"
)

// noDefinedTargetSlot is the refusal of a Defined$ Targeted/ParentTarget(ed)
// that finds no slot: for Targeted, neither the node's own nor any ancestor's
// in the chain (lastTarget reads the chain, so a slot an earlier link
// declared is found).
func noDefinedTargetSlot(v string) error {
	if v == "Targeted" {
		return refuse("Defined$ Targeted with no ancestor target slot")
	}
	return refuse("Defined$ %s with no target slot", v)
}

// defined lowers a Defined$ value to a ref.
func (lc *lowerCtx) defined(v string) (lRef, error) {
	if lc.emblemBlocks && emblemBlocksAttackerRead(v) {
		return lRef{}, refuse("Effect Blocks trigger reads the attacker")
	}
	switch v {
	case "RememberedController":
		return lc.rememberedController() // lower_stacktgt.go
	case "ImprintedController":
		if lc.repeatRem { // lower_repeatremembered.go
			return lRef{Kind: rEach}, nil
		}
	case "Self", "Parent":
		// gorge's defined-spec table reads "Parent" as definedSpecSelf
		// (effects/context.go), the same one alias it gives "Self" and
		// "EffectSource": the parent of a resolving sub-ability is the
		// ability itself, and Defined() names its source -- for View from
		// Above's and Molten Birth's DB$ ChangeZone Origin$ Stack
		// Destination$ Hand the spell on the stack returns to its owner's
		// hand. ParentTarget(ed) above is the different, target-reading
		// spelling.
		return lRef{Kind: rSelf}, nil
	case "OriginalHost":
		return lRef{Kind: rHost}, nil
	case "You":
		return lRef{Kind: rYou}, nil
	case "Opponent", "Player.Opponent":
		return lRef{Kind: rEachOpp}, nil
	case "Player":
		return lRef{Kind: rEachPlayer}, nil
	case "Player.Other":
		// gorge definedSpecOpponent: every player but you, which is each
		// opponent at any seat count (seat-exact)
		return lRef{Kind: rEachOpp}, nil
	case "Targeted", "ParentTarget", "ParentTargeted":
		if v == "Targeted" {
			lc.unionProbe("defined:Targeted")
		}
		// ParentTarget(ed): the nearest earlier link's target (gorge
		// parentLinkTargets; definedSpecParentTarget reads both spellings):
		// Mind Spiral's stun counter goes on the creature its DBTap link
		// targets, not on the root's player
		t := lc.lastTarget()
		if v != "Targeted" {
			t = lc.parentLastTarget() // never the node's own slot (lower_owntarget.go)
		}
		if t < 0 {
			return lRef{}, noDefinedTargetSlot(v)
		}
		if v == "Targeted" && len(lc.chainT) > 1 {
			// gorge's Targeted is the union of the chain's targets
			// (effects context.go refTargetUnion), not the last slot: an
			// effect's subject lowers it as rTargetUnion (objOrTarget,
			// lower_targetunion.go); every other reader is refused
			return lRef{}, refuse("Defined$ Targeted over %d target slots outside an effect's subject", len(lc.chainT))
		}
		return lRef{Kind: rTarget, N: t}, nil
	case "TargetedPlayer":
		lc.unionProbe("defined:TargetedPlayer")
		// the chain's player target (Thought-Stalker Warlock's discard arms)
		t := lc.lastTarget()
		if t < 0 || lc.ab.Targets[t].F == nil || lc.ab.Targets[t].F.Players == 0 || len(lc.ab.Targets[t].F.Clauses) != 0 {
			return lRef{}, refuse("Defined$ %s without a player target slot", v)
		}
		return lRef{Kind: rTarget, N: t}, nil
	case "TargetedController", "TargetedOrController":
		// TargetedOrController: gorge definedAliases (effects context.go)
		// spells it TargetedController, which maps a player target to itself
		lc.unionProbe("defined:TargetedController")
		t := lc.lastTarget()
		if t < 0 {
			return lRef{}, refuse("Defined$ %s with no target slot", v)
		}
		return lRef{Kind: rTargetController, N: t}, nil
	case "TargetedOwner":
		lc.unionProbe("defined:TargetedOwner")
		t := lc.lastTarget()
		if t < 0 {
			return lRef{}, refuse("Defined$ %s with no target slot", v)
		}
		return lRef{Kind: rTargetOwner, N: t}, nil
	case "ThisTargetedCard":
		return lc.thisTargetedCard()
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredCards":
		// gorge definedSpecTriggeredCard reads the card forms
		return lRef{Kind: rTriggered}, nil
	case "TriggeredCardOwner":
		return lc.triggeredCardOwnerRef(), nil // lower_modeparams.go
	case "TriggeredObject", "TriggeredObjectLKICopy":
		// gorge definedSpecTriggeredObject(LKICopy) reads the OBJECT that
		// caused the trigger: for Mode$ Unattached (the Captain's Hook
		// family) the FORMER BEARER the queue binds from the event's ID
		// list (gorge trigger_match.go triggerRemembered's Unattached arm),
		// not the event's Obj (the attachment). The kernel's trigger entry
		// carries it as RefTriggerObj, with the queue-time LKI snapshot the
		// referent reads on the bearer-left path (CR 608.2h: a dead bearer
		// is a no-op, as gorge's LKI reference is)
		return lRef{Kind: rTriggered}, nil
	case "TriggeredAttacker", "TriggeredAttackerLKICopy":
		// gorge binds the one attacker that raised the event: an Attacks
		// trigger queues one trigger per matching attacker
		// (CR 508.3a/603.2c), as the kernel does. An AttackersDeclared trigger
		// is a player event: the kernel binds NoObj there and cannot name
		// the batch
		if tr := lc.ab.Trigger; tr != nil {
			if tr.Event == "AttackersDeclared" {
				return lRef{}, refuse("Defined$ %s in an AttackersDeclared trigger", v)
			}
		}
		return lRef{Kind: rTriggered}, nil
	case "TriggeredAttackers":
		// gorge definedSpecTriggeredAttackers reads the trigger's Remembered
		// (the event's attacker group), not the matched subset
		// (lower_triggerattackers.go)
		if err := lc.triggerAttackersSubjectOK(); err != nil {
			return lRef{}, err
		}
		return lRef{Kind: rTriggerAttackers}, nil
	case "TriggeredAttackerController":
		// gorge definedSpecTriggeredCombatantController: the attacker's
		// controller, which an Attacks or Blocks trigger binds as its
		// trigger player (the kernel queues both with the attacker's
		// controller, so the Blocks pair's attacker too)
		if tr := lc.ab.Trigger; tr != nil && (tr.Event == "Attacks" || tr.Event == "Blocks") {
			return lRef{Kind: rTriggeredPlayer}, nil
		}
		// a printed become-blocked line (gorge attackerBlockedCtx, and the
		// blocker-anchored half of AttackerBlockedByCreature) captures
		// AttackingPlayer as the attacker's controller when it queues; the
		// kernel queues it with the same seat as its trigger player
		// (fireBlockedGen, firePairTrig's other member: the attacker). The
		// attacker-anchored pair half captures no attacking player in gorge,
		// and an Effect's emblem line queues its own way: both refuse.
		if tr := lc.ab.Trigger; tr != nil && tr.Event == "AttackerBlocked" && !lc.emblemBlocked && (!tr.ByCreature || tr.AnchorBlocker) {
			return lRef{Kind: rTriggeredPlayer}, nil
		}
		return lRef{}, refuse("Defined$ %q outside an Attacks, Blocks or become-blocked trigger", v)
	case "TriggeredBlockerController":
		return lc.triggeredBlockerController(v)
	case "TriggeredBlocker", "TriggeredBlockerLKICopy":
		// gorge TriggerBlocker: lower_trigrefs.go pairs it with the modes
		// whose kernel record carries the blocker
		r, _, err := lc.triggeredObjectRef(v)
		return r, err
	case "TriggeredNewCard", "TriggeredNewCardLKICopy":
		return lRef{Kind: rTriggeredNew}, nil
	case "ExiledCards":
		// the card a dies trigger's own ExileAnyGrave<1/Card.TriggeredNewCard>
		// CollectEvidence<N> cost exiled (Lamplight Phoenix): the triggered
		// card in exile
		if lc.ab == nil || lc.ab.Kind != akTriggered || len(lc.ab.Prog) == 0 || lc.ab.Prog[0].CS != "ExileTrigNewEvidence" {
			return lRef{}, refuse("Defined$ %q outside a dies trigger's own exile-and-evidence cost", v)
		}
		return lRef{Kind: rTriggeredNew}, nil
	case "TriggeredSourceSA":
		// gorge's targeting stack object (TriggerStack) on a BecomesTarget
		// trigger: the source of the event, not the targeted permanent. The
		// kernel records it in the queue's trigSA slot (trig_becomestarget.go)
		// and RefTriggerSource reads it before the trigger object.
		if tr := lc.ab.Trigger; tr != nil && tr.Event == "BecomesTarget" {
			return lRef{Kind: rTriggeredSource}, nil
		}
		return lRef{}, refuse("Defined$ %q outside a BecomesTarget trigger", v)
	case "TriggeredSource", "TriggeredSources", "TriggeredSourceLKICopy":
		// gorge definedSpecTriggeredSource. On a capturing emblem's
		// recipient-captured DamageDone line the kernel emblem hook queues
		// the DAMAGED object as the trigger object (dirac/
		// trig_effectuntileot.go emblemTriggerObj over embRecipientCapture),
		// so a read of the damage source would name the damaged creature:
		// refuse. Every other DamageDone queue carries the source as the
		// trigger object (dirac/trig_damagedonce.go ddoLine).
		if lc.emblemDmg && emblemDmgRecipientTrig(lc.ab.Trigger) {
			return lRef{}, refuse("Defined$ %q beside an emblem trigger that reads the damaged recipient as its trigger object", v)
		}
		return lRef{Kind: rTriggeredSource}, nil
	case "TriggeredPlayer":
		// gorge definedSpecTriggeredPlayer: the run's Player, which is the
		// kernel's trigger player only for these events (the active player
		// of a Phase trigger, the drawer, the player who gained/lost life, the caster)
		if tr := lc.ab.Trigger; tr != nil {
			switch tr.Event {
			case "Phase", "Drawn", "LifeGained", "LifeLost", "SpellCast", "FacesDilemma", "BecomeMonarch":
				// SpellCast: gorge trigger_referents.go:244-258 sets
				// TriggerPlayer to the caster, the TriggeredActivator below;
				// BecomeMonarch: the new monarch (gorge's MonarchChange
				// Player, lower_monarch.go)
				return lRef{Kind: rTriggeredPlayer}, nil
			}
		}
		return lRef{}, refuse("Defined$ %q outside a Phase/Drawn/LifeGained/LifeLost/SpellCast/FacesDilemma/BecomeMonarch trigger", v)
	case "TriggeredAttackingPlayer":
		return lc.triggeredAttackingPlayerDefined(v)
	case "TriggeredAttackedTarget":
		return lc.triggeredAttackedTargetDefined(v)
	case "TriggeredTarget", "TriggeredTargetLKICopy":
		// gorge definedSpecTriggeredTarget: the object or player the trigger's
		// event happened to (lower_trigrefs.go)
		r, _, err := lc.triggeredObjectRef(v)
		return r, err
	case "TriggeredTargetController":
		// gorge definedSpecTriggeredTargetController (context.go): the
		// controller of the trigger's target object, read live at
		// resolution -- a referent that left the battlefield since it was
		// dealt the damage reads its owner (CR 603.7a: the trigger fires
		// for the event, the read is the object's as it exists when the
		// ability resolves; Expedited Inheritance's "its controller may
		// exile that many" after the creature died from that damage).
		// The queue-time trigger player (trig_damagedonce.go ddoRecipient)
		// is the referent's controller as it was, so this spelling gets
		// its own ref; the role is exact only on an object referent.
		if tr := lc.ab.Trigger; ddoObjectReferent(tr) {
			return lRef{Kind: rTriggeredTgtCtrl}, nil
		}
		return lRef{}, refuse("Defined$ %q without a damage object referent", v)
	case "TriggeredSourceController", "TriggeredSourceSAController":
		// gorge maps both spellings to one resolver
		// (definedSpecTriggeredSourceSAController, effects/context.go).
		// Same emblem recipient-capture pin as TriggeredSource above: the
		// controller read is of the kernel's trigger object
		if lc.emblemDmg && emblemDmgRecipientTrig(lc.ab.Trigger) {
			return lRef{}, refuse("Defined$ %q beside an emblem trigger that reads the damaged recipient as its trigger object", v)
		}
		// gorge definedSpecTriggeredSourceController (lower_trigrefs.go)
		r, _, err := lc.triggeredObjectRef(v)
		return r, err
	case "TriggeredCardController", "TriggeredActivator":
		return lRef{Kind: rTriggeredPlayer}, nil
	case "Equipped", "Enchanted", "AttachedTo":
		// gorge definedSpecEquipped reads all three
		return lRef{Kind: rEquipped}, nil
	case "Remembered", "RememberedLKI", "RememberedCard":
		if r, ok := lc.rememberedAlias(v); ok {
			return r, nil
		}
		// RememberedCard: gorge definedSpecRememberedCard,
		// the non-player entries of the remembered list, which is all the
		// kernel's remembered set holds.
		// RememberedLKI: gorge rememberedLKIGroup, the remembered objects
		// with a RememberLKI$ move's captured controller substituted off
		// the battlefield (kernel tliRemLKI); every other characteristic
		// is read live, as Remembered's
		return lRef{Kind: rRemembered}, nil
	case "DirectRemembered":
		// gorge definedAliases: DirectRemembered is RememberedLKI. Lowered
		// only as the remembered-persist leave return (Fiend Hunter), where
		// both read the source's persistent set as Remembered does
		if !lc.remPersistShape() {
			return lRef{}, refuse("Defined$ %q outside the remembered-persist enters/leaves pair", v)
		}
		return lRef{Kind: rRemembered}, nil
	case "Imprinted":
		return lc.definedImprinted()
	case "Player.IsRemembered":
		if lc.repeatPl == 0 {
			return lRef{}, refuse("Defined$ %q outside a RepeatEach body", v)
		}
		return lRef{Kind: rRemembered}, nil
	case "DelayTriggerRememberedLKI":
		return lRef{Kind: rDelayedRemembered}, nil
	case "DelayTriggerRemembered":
		// gorge's plain delay-remembered read returns the set as-is, players
		// included (context.go definedSpecDelayTriggerRemembered); the kernel's
		// RefDelayedRemembered is objects only. Admit it only over an object-only
		// RememberObjects$ or a capturing reflexive, never generally.
		if lc.ab != nil && lc.ab.DelayedPlayer {
			return lRef{Kind: rTriggeredPlayer}, nil // lower_delayremplayer.go
		}
		if lc.ab == nil || (!lc.ab.DelayedObjOnly && !lc.ab.ReflexObj) {
			return lRef{}, refuse("Defined$ %q", v)
		}
		return lRef{Kind: rDelayedRemembered}, nil
	case "ReplacedCard":
		return lRef{Kind: rReplaced}, nil
	case "ChosenCard":
		return lRef{Kind: rChosen}, nil
	case triggeredDefenderOrPW:
		return lc.triggeredDefenderOrPW(v)
	case "TriggeredDefendingPlayer":
		if !lc.ownAttackTrigger() {
			return lRef{}, refuse("Defined$ %q outside an attack trigger of the controller's own attack", v)
		}
		return lRef{Kind: rDefending}, nil
	case "Spawner>TriggeredTarget":
		// the player the spawning trigger's source dealt combat damage to
		// (gorge spawnerChain reads TriggeredTarget in the spawner's
		// roles): the trigger entry's damage seat, at any seat count (CR
		// 506.4; the same reading as the ControlledBy form, lower_filterwords.go)
		if !lc.spawnDmg {
			return lRef{}, refuse("Defined$ %q outside an ImmediateTrigger spawned by Card.Self's combat damage to a player", v)
		}
		return lRef{Kind: rTriggeredPlayer}, nil
	case "Promised":
		// the opponent the gift was promised to (CR 702.174a; gorge
		// Object.GiftPromisedTo): the spell's election, or a permanent's
		// gift trigger's queue-time snapshot (kop.RefPromised)
		if !lc.out.Gift {
			return lRef{}, refuse("Defined$ Promised without a gift")
		}
		return lRef{Kind: rPromised}, nil
	}
	if lc.targetedDefined != "" && v == lc.targetedDefined {
		// Defined$ Targeted.<qualifier> (graveexile): the node's one target slot,
		// guarded by node() on the qualifier. Only the exact value the node
		// declared is accepted, so no other parameter can read it.
		t := lc.lastTarget()
		if t < 0 {
			return lRef{}, refuse("Defined$ %q with no target slot", v)
		}
		return lRef{Kind: rTarget, N: t}, nil
	}
	if r, ok, err := lc.playerRefDefined(v); ok || err != nil {
		return r, err // lower_defineduniononplayers.go
	}
	switch v {
	case "OppNonRememberedController":
		// gorge definedSpecNonRememberedController: the opponents minus every
		// remembered object's controller (the bare NonRememberedController,
		// every player, has no Standard carrier and stays refused)
		return lRef{Kind: rOppNonRemCtrl}, nil
	case "CardOwner":
		return lRef{Kind: rSrcOwner}, nil // gorge definedSpecCardOwner
	}
	return lRef{}, refuse("Defined$ %q", v)
}

// triggeredAttackingPlayerDefined is Defined$ TriggeredAttackingPlayer:
// gorge binds AttackingPlayer on a DeclareAttackers event to the declaring
// player, who is the game's active player (CR 508.1: only the active player
// declares attackers) -- the kernel's queue-time trigger player at every seat
// count (fireAtkDecl's who, fireDefPlayer's, combat.go stampDefSeat). On a
// combat-damage-to-you line gorge binds it to the damage source's controller,
// which the per-referent DamageDone queue carries as the trigger object
// whose controller RefTriggerTgtCtrl reads (rTriggeredTgtCtrl); the queue's
// trigger player there is the damaged seat, not the attacker.
func (lc *lowerCtx) triggeredAttackingPlayerDefined(v string) (lRef, error) {
	if tr := lc.ab.Trigger; tr != nil && tr.Event == "AttackersDeclared" {
		return lRef{Kind: rTriggeredPlayer}, nil
	}
	if tr := lc.ab.Trigger; ddoAttackingPlayer(tr) {
		return lRef{Kind: rTriggeredTgtCtrl}, nil
	}
	return lRef{}, refuse("Defined$ %q outside a combat damage-to-you trigger the kernel's attacking player names or an AttackersDeclared trigger", v)
}

// triggeredAttackedTargetDefined is Defined$ TriggeredAttackedTarget: gorge
// binds AttackedTarget on a DeclareAttackers event to the event's defender.
// The kernel names the seat RefDefendingPlayer: the queue-time defending seat
// (combat.go stampDefSeat, every seat count past two) and the one opponent at
// two (resolve.go oneOppOf: the active player is the only possible attacker,
// CR 508.1). A per-defender line only (Trigger.DefPlayer set): a batch line
// queues once for the whole declaration, whose stamp names only the first
// defender group.
func (lc *lowerCtx) triggeredAttackedTargetDefined(v string) (lRef, error) {
	if tr := lc.ab.Trigger; tr != nil && tr.Event == "AttackersDeclared" && tr.DefPlayer != 0 {
		return lRef{Kind: rDefending}, nil
	}
	return lRef{}, refuse("Defined$ %q outside a per-defender AttackersDeclared trigger", v)
}

// damagedPlayerIsTriggerPlayer reports whether the ability is a DamageDone
// trigger whose damaged player is the kernel's trigger player (1 - the
// source's controller): ValidTarget$ names players only, and the damage is
// combat damage (dealt to the defending player), or ValidTarget$ is
// Opponent and every ValidSource$ clause is the listener itself or a
// permanent its controller controls.
func (lc *lowerCtx) damagedPlayerIsTriggerPlayer() bool {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "DamageDone" || tr.F2 == nil || tr.F2.Players == 0 || len(tr.F2.Clauses) != 0 {
		return false
	}
	if tr.Combat == 1 {
		return true
	}
	if tr.F2.Players != plOpp || tr.F == nil || len(tr.F.Clauses) == 0 {
		return false
	}
	for _, c := range tr.F.Clauses {
		if c.Flags&fSelf == 0 && c.Ctrl != whoYou {
			return false
		}
	}
	return true
}

// ownAttackTrigger reports whether the ability is an Attacks trigger on the
// source itself (ValidCard$ Card.Self), or an AttackersDeclared trigger of
// its controller's attack: the defending player is then the seat the attack
// event names, which kop.RefDefendingPlayer resolves to. With more seats the
// kernel resolves it to the defending seat of the attack event that queued
// the trigger (stackEntry.defSeat: for a declaration, its first event's seat,
// as gorge's batch trigger latches there), else the defender of an attacking
// source. Another attacker's Attacks trigger is queued with its own event, so
// its defSeat is the seat that attacker attacks; another attacker's
// AttackerBlocked or AttackerUnblocked trigger is stamped with that attacker's
// defending seat as it is queued (trig_blocked.go fireBlockedGen,
// trig_unblocked.go fireUnblocked), so RefDefendingPlayer and
// defendingSeatF2 read the same seat.
func (lc *lowerCtx) ownAttackTrigger() bool {
	tr := lc.ab.Trigger
	if tr == nil {
		return false
	}
	switch tr.Event {
	case "Attacks", "AttackerBlocked":
		if tr.F == nil || len(tr.F.Clauses) == 0 {
			return false
		}
		for _, c := range tr.F.Clauses {
			if c.Flags&fSelf == 0 && c.Ctrl != whoYou {
				if !lc.enchantedByYouCtrl(c) {
					return false
				}
			}
		}
		return true
	case "AttackersDeclared":
		return tr.Whose == whoYou
	}
	return false
}

// enchantedByYouCtrl reports whether c is the "enchanted creature" attacker
// of an Aura whose printed Enchant word names only a creature its controller
// controls. That attacker is then controlled by the Aura's controller, so
// the defending player of its attack is a specific opponent of the
// controller, which the kernel reads from the attacker's attack target
// (Cloak of Confusion's "defending player discards").
func (lc *lowerCtx) enchantedByYouCtrl(c lClause) bool {
	if c.Flags != fEnchantedBy || c.Ctrl != whoAny || c.Types != tCreature || lc.face == nil {
		return false
	}
	for _, k := range lc.face.Keywords {
		if cards.KeywordHead(k) != "Enchant" {
			continue
		}
		spec, _, _ := strings.Cut(strings.TrimPrefix(k, "Enchant:"), ":")
		return strings.TrimSpace(spec) == "Creature.YouCtrl"
	}
	return false
}

// emblemBlocksAttackerRead reports whether Defined$ v names the ATTACKER of a
// Blocks pair (or the trigger object/player a printed Blocks line binds to it):
// an Effect's Blocks emblem binds the blocker instead
// (dirac/trig_emblemblocks.go), so these refuse.
func emblemBlocksAttackerRead(v string) bool {
	return strings.HasPrefix(v, "TriggeredCard") || strings.HasPrefix(v, "TriggeredAttacker") || strings.HasPrefix(v, "TriggeredObject")
}

// triggeredBlockerController is Defined$ TriggeredBlockerController.
func (lc *lowerCtx) triggeredBlockerController(v string) (lRef, error) {
	if lc.emblemBlocks {
		// the Effect's Blocks emblem: the trigger object is the blocker
		return lRef{Kind: rTriggeredTgtCtrl}, nil
	}
	// gorge definedSpecTriggeredCombatantController: the controller of
	// the captured blocker, read live. A Blocks trigger over the
	// source's own host (ValidCard$ Card.AttachedBy: the enchanted or
	// equipped creature) names the host's controller, the one
	// RefAttachedCtrl reads (Contaminated Bond)
	if tr := lc.ab.Trigger; tr != nil && tr.Event == "Blocks" && tr.F != nil && len(tr.F.Clauses) == 1 &&
		tr.F.Players == 0 && tr.F.Clauses[0] == (lClause{Flags: fEquippedBy}) {
		return lRef{Kind: rAttachedCtrl}, nil
	}
	if tr := lc.ab.Trigger; tr != nil && tr.ByCreature && !tr.AnchorBlocker {
		// the attacker's "becomes blocked by a creature" trigger (Gloom
		// Sower): gorge captures no TriggerBlocker role there, so
		// TriggeredCardController reads the trigger card, the blocker
		// (TriggerCard = Remembered[0]), live. The kernel's trigger object
		// is that blocker; RefTriggerTgtCtrl reads its controller live (a
		// blocker gone from the battlefield reads its owner).
		return lRef{Kind: rTriggeredTgtCtrl}, nil
	}
	return lRef{}, refuse("Defined$ %q outside a Blocks trigger over the attached creature", v)
}
