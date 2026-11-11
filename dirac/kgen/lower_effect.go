package kgen

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// defined lowers a Defined$ value to a ref.
func (lc *lowerCtx) defined(v string) (lRef, error) {
	switch v {
	case "RememberedController":
		return lc.rememberedController() // lower_stacktgt.go
	case "Self":
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
			return lRef{}, refuse("Defined$ %s with no target slot", v)
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
	case "TriggeredBlocker", "TriggeredBlockerLKICopy":
		// gorge TriggerBlocker of a Blocks trigger: the source itself when
		// the trigger is "whenever this blocks" (ValidCard$ Card.Self)
		if tr := lc.ab.Trigger; tr != nil && tr.Event == "Blocks" && tr.F != nil && len(tr.F.Clauses) == 1 &&
			tr.F.Players == 0 && (tr.F.Clauses[0] == lClause{Flags: fSelf} || tr.F.Clauses[0] == lClause{Types: tCreature, Flags: fSelf}) {
			return lRef{Kind: rSelf}, nil
		}
		if tr := lc.ab.Trigger; tr != nil && tr.ByCreature {
			// gorge AttackerBlockedByCreature: the blocker's own trigger
			// names it through TriggerBlocker (the source); the attacker's
			// remembers the blocker, the trigger object
			if tr.AnchorBlocker {
				return lRef{Kind: rSelf}, nil
			}
			return lRef{Kind: rTriggered}, nil
		}
		return lRef{}, refuse("Defined$ %q outside a whenever-this-blocks trigger", v)
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
	case "TriggeredSource", "TriggeredSources", "TriggeredSourceLKICopy":
		// gorge definedSpecTriggeredSource
		return lRef{Kind: rTriggeredSource}, nil
	case "TriggeredPlayer":
		// gorge definedSpecTriggeredPlayer: the run's Player, which is the
		// kernel's trigger player only for these events (the active player
		// of a Phase trigger, the drawer, the player who gained/lost life, the caster)
		if tr := lc.ab.Trigger; tr != nil {
			switch tr.Event {
			case "Phase", "Drawn", "LifeGained", "LifeLost", "SpellCast", "FacesDilemma":
				// SpellCast: gorge trigger_referents.go:244-258 sets
				// TriggerPlayer to the caster, the TriggeredActivator below
				return lRef{Kind: rTriggeredPlayer}, nil
			}
		}
		return lRef{}, refuse("Defined$ %q outside a Phase/Drawn/LifeGained/LifeLost/SpellCast/FacesDilemma trigger", v)
	case "TriggeredTarget", "TriggeredTargetLKICopy":
		// gorge definedSpecTriggeredTarget of a damage trigger: the damaged
		// object or player. A per-referent DamageDone line whose ValidTarget$
		// is an object filter carries the damaged permanent as the trigger
		// object (gorge's LKI copy names the same object; a recipient that
		// already left is a gone object, and Destroy of a gone object is a
		// no-op in the kernel as in gorge).
		if tr := lc.ab.Trigger; ddoObjectReferent(tr) {
			return lRef{Kind: rTriggered}, nil
		}
		// The player dealt the damage. The kernel's trigger player there is
		// the opponent of the source's controller (two players), which is
		// the damaged player when the damage is combat damage to a player,
		// or damage to an opponent from a source the listener controls.
		// seat-exact: with more seats the kernel's trigger player is the
		// damaged seat itself (dirac dmgSeat).
		if lc.damagedPlayerIsTriggerPlayer() {
			return lRef{Kind: rTriggeredPlayer}, nil
		}
		return lRef{}, refuse("Defined$ %q outside a damage trigger the kernel's trigger object or player names", v)
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
	case "TriggeredSourceController":
		// gorge definedSpecTriggeredSourceController: the controller of the
		// spell or ability that targeted the watched permanent. The watcher
		// arm that sets Whose = opp is ValidSource$ SpellAbility.OppCtrl, so
		// the controller is an opponent of the listener's controller: in a
		// two-player game the one opponent (CR 102.2; Thunderbreak Regent).
		if tr := lc.ab.Trigger; tr != nil && tr.Event == "BecomesTarget" && tr.Watch && tr.Whose == whoOpp {
			lc.twoSeat("TriggeredSourceController read as the one opponent")
			return lRef{Kind: rEachOpp}, nil
		}
		return lRef{}, refuse("Defined$ %q outside an opponent-source BecomesTarget watcher", v)
	case "TriggeredAttackingPlayer":
		// gorge trigger_referents.go: the controller of the in-flight
		// combat source when it is attacking. A per-referent DamageDone
		// line with CombatDamage$ True and ValidTarget$ You is combat
		// damage to this listener's controller, so with two players the
		// source is an attacker the one opponent controls.
		if tr := lc.ab.Trigger; ddoAttackingPlayer(tr) {
			lc.twoSeat("TriggeredAttackingPlayer read as the one opponent")
			return lRef{Kind: rEachOpp}, nil
		}
		return lRef{}, refuse("Defined$ %q outside a combat damage-to-you trigger the kernel's attacking player names", v)
	case "TriggeredCardController", "TriggeredActivator":
		return lRef{Kind: rTriggeredPlayer}, nil
	case "Equipped", "Enchanted", "AttachedTo":
		// gorge definedSpecEquipped reads all three
		return lRef{Kind: rEquipped}, nil
	case "Remembered", "RememberedLKI", "RememberedCard":
		if v == "Remembered" && lc.remOppLink() {
			return lRef{Kind: rEachOpp}, nil // lower_remembered.go
		}
		if v == "Remembered" && lc.capRem && !lc.hasRememberWriter() {
			return lRef{Kind: rTriggeredSource}, nil // lower_effectcaptured.go
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
		// DigUntil's ImprintFound$ chain (lower_digimprint.go): gorge
		// effectRemembered reads the source's seek-found Imprint, which the
		// kernel holds in the chosen set (DuImprintFound).
		if lc.digImprintSetLive() { // lower_digimprintset.go
			return lRef{Kind: rImprinted}, nil
		}
		if !lc.digImprintChosen {
			return lRef{}, refuse("Defined$ Imprinted outside a DigUntil ImprintFound$")
		}
		return lRef{Kind: rChosen}, nil
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
		// roles): two players, the one opponent (CR 506.4; the same
		// reading as the ControlledBy form, lower_filterwords.go)
		if !lc.spawnDmg {
			return lRef{}, refuse("Defined$ %q outside an ImmediateTrigger spawned by Card.Self's combat damage to a player", v)
		}
		lc.twoSeat("Spawner>TriggeredTarget read as the one opponent")
		return lRef{Kind: rEachOpp}, nil
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
	if r, ok := lc.chosenPlayerDefined(v); ok {
		return r, nil // lower_definedheads.go
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
// its controller's attack: the defending player is then the controller's
// opponent (two players), which is what kop.RefDefendingPlayer resolves to.
// With more seats the kernel resolves it to the defending seat of the attack
// event that queued the trigger (stackEntry.defSeat: for a declaration, its
// first event's seat, as gorge's batch trigger latches there), else the
// defender of an attacking source. Another attacker's Attacks trigger is
// queued with its own event, so its defSeat is the seat that attacker
// attacks; another attacker's AttackerBlocked trigger is queued at blocks
// with no seat bound and marks the card two-seat only.
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
		self := true
		for _, c := range tr.F.Clauses {
			if c.Flags&fSelf == 0 && c.Ctrl != whoYou {
				return false
			}
			self = self && c.Flags&fSelf != 0
		}
		if !self && tr.Event == "AttackerBlocked" {
			lc.twoSeat("the defending player of another attacker read as the one opponent")
		}
		return true
	case "AttackersDeclared":
		return tr.Whose == whoYou
	}
	return false
}

// objOrTarget resolves the object(s) a node acts on: its own target slot
// (ValidTgts$) or Defined$ (default dflt). A target slot reads TargetMin$/
// TargetMax$/TgtZone$/TargetUnique$ and friends.
func (lc *lowerCtx) objOrTarget(ps *params, dflt string, zone int) (lRef, error) {
	return lc.objOrTargetOpt(ps, dflt, zone, false)
}

// objOrTargetParent is objOrTarget for an effect that may restrict a card
// target to the graveyard of an earlier player target (gorge's
// TargetsWithDefinedController$ ParentTarget).
func (lc *lowerCtx) objOrTargetParent(ps *params, dflt string, zone int) (lRef, error) {
	return lc.objOrTargetOpt(ps, dflt, zone, true)
}

func (lc *lowerCtx) objOrTargetOpt(ps *params, dflt string, zone int, allowParent bool) (lRef, error) {
	if vt, ok := ps.get("ValidTgts"); ok {
		var gxZones []kop.Zone
		if lc.graveExileCard {
			var err error
			if vt, gxZones, err = graveExileSplit(vt); err != nil { // lower_graveexiletgt.go
				return lRef{}, err
			}
		}
		f, err := lc.filter(vt)
		if err != nil {
			return lRef{}, err
		}
		t := lTarget{F: f, Zone: zone}
		if lc.graveExileCard {
			if f.Players != 0 || len(f.Clauses) != len(gxZones) {
				return lRef{}, refuse("ChangeZone Origin$ Graveyard,Exile ValidTgts$ %q does not lower to one clause per alternative", vt)
			}
			t.GraveExileCard = true
			for _, z := range gxZones {
				t.ClauseZones = append(t.ClauseZones, int(z))
			}
		}
		if lc.permOrGraveCard {
			if f.Players != 0 {
				return lRef{}, refuse("ChangeZone Origin$ Battlefield,Graveyard with a player target")
			}
			for _, c := range f.Clauses {
				if c.Ctrl != whoAny || c.Owner != whoAny {
					return lRef{}, refuse("ChangeZone Origin$ Battlefield,Graveyard with a controller/owner filter")
				}
			}
			t.PermOrGraveCard = true
		}
		if z, ok := ps.get("TgtZone"); ok {
			t.Zone = zoneNames[z]
			if t.Zone == 0 {
				return lRef{}, refuse("TgtZone %q", z)
			}
		}
		if f.Players != 0 && len(f.Clauses) == 0 {
			t.Zone = zNone
		}
		if lc.otherOneEach(ps, f, t.Zone) { // the bounds are set here; forEachOpp checks the rest
			t.Min, t.Max = cst(0), cst(1)
		} else {
			if v, ok := ps.get("TargetMin"); ok {
				if t.Min, err = lc.value(v); err != nil {
					return lRef{}, err
				}
			}
			if v, ok := ps.get("TargetMax"); ok {
				if t.Max, err = lc.value(v); err != nil {
					return lRef{}, err
				}
			}
			t.Min, t.Max = lc.oppCountBound(t.Min), lc.oppCountBound(t.Max)
		}
		if allowParent {
			if v, ok := ps.get("TargetsWithDefinedController"); ok && v != "TriggeredCardController" && v != "TriggeredTarget" { // those forms: lower_targetset.go
				if v != "ParentTarget" {
					return lRef{}, refuse("TargetsWithDefinedController$ %q", v)
				}
				parent := lc.lastTarget()
				if parent < 0 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget without an earlier target")
				}
				pt := &lc.ab.Targets[parent]
				if pt.F == nil || pt.F.Players == 0 || len(pt.F.Clauses) != 0 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: parent is not a player target")
				}
				// The kernel reads only the primary parent slot (e.tgt[parent-1]),
				// while gorge's filterParentControllerTargets uses every parent
				// target. A parent that can choose more than one player (or whose
				// target count is not a constant) would silently see only the
				// first. Min may be 0 (a declined parent names nobody: both
				// sides fail closed to no candidates) or 1.
				if !pt.Max.isConst() || pt.Max.N != 1 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: parent TargetMax$ %s is not 1", pt.Max)
				}
				if !pt.Min.isConst() || (pt.Min.N != 0 && pt.Min.N != 1) {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: parent TargetMin$ %s is not 0 or 1", pt.Min)
				}
				// The kernel's parentTargetOK accepts only a graveyard card
				// controlled by one player parent. gorge's
				// filterParentControllerTargets keeps object parents too,
				// and the effect's own zone may be wider, so read exactly
				// the supported shape.
				if f.Players != 0 {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: target filter %q names players", vt)
				}
				if t.Zone != zGraveyard {
					return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget: target zone %s is not Graveyard", zoneLabel[t.Zone])
				}
				for _, c := range f.Clauses {
					if c.Owner != 0 || c.Ctrl != 0 {
						return lRef{}, refuse("TargetsWithDefinedController$ ParentTarget with an owner/controller filter")
					}
				}
				t.ParentPlus1 = parent + 1
			}
		}
		if ps.is("TargetUnique", "True") {
			t.Unique = true
		}
		if ps.is("TargetsWithSameController", "True") {
			t.SameCtrl = true
		}
		if ps.is("TargetsWithDifferentControllers", "True") {
			t.DiffCtrl = true
		}
		if err := lc.tgtSetParams(ps, &t, allowParent); err != nil { // lower_targetset.go
			return lRef{}, err
		}
		if err := lc.forEachOpp(ps, f, &t); err != nil {
			return lRef{}, err
		}
		tr := lRef{Kind: rTarget, N: lc.addTarget(t)}
		if d, ok := ps.get("Defined"); ok {
			// gorge DefinedRef: a Defined$ selector wins over the chosen
			// targets as the effect's subject; the target is still
			// chosen (and checked on resolution) for the values and
			// later nodes that read it (Biomantic Mastery, Donate)
			if d == "Targeted" {
				lc.unionProbe("own-slot:Targeted")
				return tr, nil
			}
			return lc.defined(d)
		}
		return tr, nil
	}
	d := dflt
	if v, ok := ps.get("Defined"); ok {
		d = v
	}
	if d == "" {
		return lRef{}, refuse("no target and no Defined$")
	}
	if d == "Targeted" && len(lc.chainT) > 1 {
		return lc.targetUnion()
	}
	return lc.defined(d)
}

// targetedAndYou reads Defined$/TokenOwner$ TargetedAndYou (graveexile): gorge's
// alias for TargetedController & You (context.go:1834), deduplicated, the
// target player first then the controller. Only a target slot that is
// exactly ValidTgts$ Opponent is lowered: the dedupe can never merge the
// opponent with the controller (seat-exact), and any other filter (or a
// target-count rider) would change who receives the instruction.
func (lc *lowerCtx) targetedAndYou(ps *params, key string) (lRef, bool, error) {
	v, ok := ps.get(key)
	if !ok || strings.TrimSpace(v) != "TargetedAndYou" {
		return lRef{}, false, nil
	}
	vt, ok := ps.get("ValidTgts")
	if !ok || strings.TrimSpace(vt) != "Opponent" {
		return lRef{}, false, refuse("%s$ TargetedAndYou with ValidTgts$ %q", key, ps.str("ValidTgts"))
	}
	for _, k := range []string{"TargetMin", "TargetMax", "TgtZone", "TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers"} {
		if _, ok := ps.get(k); ok {
			return lRef{}, false, refuse("%s$ TargetedAndYou with %s$", key, k)
		}
	}
	f, err := lc.filter(vt)
	if err != nil {
		return lRef{}, false, err
	}
	if f.Players != plOpp || len(f.Clauses) != 0 {
		return lRef{}, false, refuse("%s$ TargetedAndYou target filter %q", key, vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zNone})
	ps.consume(key, "ValidTgts")
	return lRef{Kind: rTarget, N: t}, true, nil
}

// guardBlock emits a SkipIf for cond, runs fn, then patches the skip.
func (lc *lowerCtx) guardBlock(cond lCond, fn func() error) error {
	if cond.Kind == cNone {
		return fn()
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: cond})
	if err := fn(); err != nil {
		return err
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// nodeCondition reads a node's Condition*$ params.
func (lc *lowerCtx) nodeCondition(ps *params) (lCond, error) {
	if lc.clAt < 0 {
		lc.clAt = 0
		// the Effect after a Counter RememberCountered$: its ConditionPresent$
		// is read in counterLoseEffect (the kernel's CounterLose checks the
		// source's types itself)
		return lCond{}, nil
	}
	if lc.condDone {
		lc.condDone = false
		ps.consume("ConditionCheckSVar", "ConditionSVarCompare")
		return lCond{}, nil
	}
	if ps.str("Announce") == "NumTimes" && ps.str("ConditionCheckSVar") == "NumTimes" {
		// the announced count is at least 1 exactly when the cost was paid:
		// numTimesPay (lower_numtimes.go) reads it with the rest of the shape
		return lCond{}, nil
	}
	if c, ok, err := lc.giftCondition(ps); ok || err != nil {
		return c, err
	}
	if c, ok, err := lc.teamworkCondition(ps); ok || err != nil {
		return c, err
	}
	if c, ok := ps.get("Condition"); ok {
		return lc.bareCondition(ps, c)
	}
	if c, ok := ps.get("ConditionActivationLimit"); ok {
		return lc.activationLimitCondition(ps, c) // lower_activationcount.go
	}
	if c, ok := ps.get("ConditionPlayerTurn"); ok {
		return lc.playerTurnCondition(ps, c)
	}
	if c, ok := ps.get("ConditionPhases"); ok {
		// a lone ConditionPhases$ (the player-turn pair is handled above);
		// any other Condition*$ key beside it refuses
		if beside := conditionKeysBesides(ps, "ConditionPhases"); len(beside) > 0 {
			return lCond{}, refuse("ConditionPhases$ beside %s$", beside[0])
		}
		return lc.phasesCondition(c)
	}
	if err := lc.notPresentAsPresent(ps); err != nil { // lower_condnotpresent.go
		return lCond{}, err
	}
	if c, ok, err := lc.effectPairCondition(ps); ok || err != nil { // lower_effectremember.go
		return c, err
	}
	if cp, ok := ps.get("ConditionPresent"); ok {
		if d, ok := ps.get("ConditionDefined"); ok && strings.TrimSpace(d) == "Collected" {
			return lc.collectedCondition(ps, cp) // lower_collectedevidence.go
		}
		if d, ok := ps.get("ConditionDefined"); ok && strings.TrimSpace(d) == "CastSA>Collected" {
			return lc.castCollectedCondition(ps, cp) // lower_collectedevidence.go
		}
		// ConditionDefined$ Self + ConditionPresent$
		// Card.wasCastFromGraveyard reads the resolving spell's own
		// graveyard-origin cast flag (gorge state.ObjectWasCastFromGraveyard
		// through effects/filter.go's wasCastFromGraveyard predicate), the
		// same read as the Count$wasCastFromGraveyard.<yes>.<no> branch head
		// in countValue. It reads the cast flag OR the cast zone:
		// a may-play permission cast from a graveyard reads true; a stack
		// copy reads false (CR 707.10). Only
		// this exact spelling is lifted here; any other use of the word (a
		// ValidCard$, a target filter, a TriggeredCard$ subject) still goes
		// to lc.filter and refuses as an unknown property.
		if d, ok := ps.get("ConditionDefined"); ok && strings.TrimSpace(d) == "Self" && strings.TrimSpace(cp) == "Card.wasCastFromGraveyard" {
			if lc.ab == nil || lc.ab.Kind != akSpell {
				// gorge reads the resolving object's persisting CastFlags: a
				// permanent's ETB/static would read true off its cast spell.
				// The kernel reads the resolving stack entry, so only a spell
				// ability has the same object.
				return lCond{}, refuse("ConditionPresent$ Card.wasCastFromGraveyard outside a spell ability")
			}
			c := lCond{Kind: cValue, A: lValue{Kind: vCastFromGrave, Sub: []lValue{cst(1), cst(0)}}, Op: opGE, B: cst(1)}
			if cmp, ok := ps.get("ConditionCompare"); ok {
				switch strings.TrimSpace(cmp) {
				case "EQ1", "GE1":
					// the default: the flag reads 1, present
				case "EQ0":
					c.Op, c.B = opEQ, cst(0)
				default:
					return lCond{}, refuse("ConditionPresent$ Card.wasCastFromGraveyard with ConditionCompare$ %q", cmp)
				}
			}
			return c, nil
		}
		// ConditionDefined$ Self + a cast-provenance word
		// reads the resolving spell's own stack entry (kernel wasCastSpell);
		// only a spell ability has that object
		d0, hasD := ps.get("ConditionDefined")
		lc.spellProvOK = hasD && strings.TrimSpace(d0) == "Self" && lc.ab != nil && lc.ab.Kind == akSpell
		// the remembered-persist leave token asks whether the remembered card
		// is still exiled with the source (a resolution-time read of the
		// source's own exile link, lower_rempersist.go)
		lc.exwOK = hasD && strings.TrimSpace(d0) == "Remembered" && strings.TrimSpace(cp) == "Card.ExiledWithSource" && lc.remPersistShape()
		if z, ok := ps.m["ConditionZone"]; ok && strings.TrimSpace(z) == "Stack" && strings.TrimSpace(cp) == "Card" && hasD && strings.TrimSpace(d0) == "TriggeredCard" {
			// "the triggering card is on the stack" (gorge conditionMetZone
			// over the defined group): Card.inZoneStack's read, the spelling
			// Goliath Daydreamer uses (lower_effectstackexile.go)
			cp = "Card.inZoneStack"
			ps.consume("ConditionZone")
		}
		f, err := lc.filter(cp)
		lc.exwOK = false
		lc.spellProvOK = false
		if err != nil {
			return lCond{}, err
		}
		c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
		if d, ok := ps.get("ConditionDefined"); ok {
			var ref lRef
			if r, ok, err := lc.rememberedLKIRef(d); err != nil { // lower_handmove.go
				return lCond{}, err
			} else if ok {
				ref = r
			} else if strings.TrimSpace(d) == "Sacrificed" {
				// C: the cost's sacrificed object, matched as
				// the card it is now (kernel sacrificedObj)
				if err := lc.sacrificedGroup(f); err != nil {
					return lCond{}, err
				}
				ref = lRef{Kind: rSacrificed}
			} else {
				// TriggeredAttackerLKICopy is the
				// single attacker an Attacks trigger captured, read as it now
				// stands (an Ordeal's counter is already on it). defined()
				// binds it to the triggering object and refuses the shapes
				// where gorge has no single attacker (several matches, a
				// player-event trigger).
				var err error
				lc.condDefined = true
				ref, err = lc.defined(d)
				lc.condDefined = false
				if err != nil {
					return lCond{}, err
				}
			}
			c.Kind, c.Ref = cRefMatches, ref
		}
		if cmp, ok := ps.get("ConditionCompare"); ok {
			op, v, err := lc.cmpSpec(cmp)
			if err != nil {
				return lCond{}, err
			}
			c.Op, c.A = op, v
		}
		if c, err = lc.nodeConditionZone(ps, c, cp); err != nil { // lower_condkeys.go
			return lCond{}, err
		}
		// ConditionCheckSVar$ beside the present check (H.M.S. Bounty:
		// ConditionPresent$ Whale.YouCtrl with Count$YourExtraTurns EQ0).
		// Only that exact SVar and an equality-with-zero compare are read;
		// any other pair refuses rather than dropping the SVar gate.
		if cs, ok := ps.get("ConditionCheckSVar"); ok {
			v, err := lc.value(cs)
			if err != nil {
				return lCond{}, err
			}
			cmp, ok := ps.get("ConditionSVarCompare")
			if !ok {
				return lCond{}, refuse("ConditionCheckSVar$ %q without ConditionSVarCompare$", cs)
			}
			op, b, err := lc.cmpSpec(cmp)
			if err != nil {
				return lCond{}, err
			}
			if v.Kind != vYourExtraTurns || op != opEQ || !b.isConst() || b.N != 0 {
				return lCond{}, refuse("ConditionCheckSVar$ %q with ConditionSVarCompare$ %q beside ConditionPresent$ %q", cs, cmp, cp)
			}
			c = lCond{Kind: cAnd, Sub: []lCond{c, {Kind: cValue, A: v, Op: op, B: b}}}
		}
		// A self-referencing present check ("Card.Self+Scout") is a
		// property of the source.
		return c, nil
	}
	if cs, ok := ps.get("ConditionCheckSVar"); ok {
		if lc.firstCastSVar != "" && strings.TrimSpace(cs) == lc.firstCastSVar {
			return lc.firstCastCond(ps) // lower_firstcast.go
		}
		v, err := lc.value(cs)
		if err != nil {
			return lCond{}, err
		}
		c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
		if cmp, ok := ps.get("ConditionSVarCompare"); ok {
			op, b, err := lc.cmpSpec(cmp)
			if err != nil {
				return lCond{}, err
			}
			c.Op, c.B = op, b
		}
		if t, ok := ps.get("ConditionOpponentTurn"); ok {
			// gorge conditions.go: ConditionOpponentTurn$ True is
			// ConditionPlayerTurn$ False, ANDed with the SVar compare
			if !strings.EqualFold(strings.TrimSpace(t), "True") {
				return lCond{}, refuse("ConditionOpponentTurn$ %q", t)
			}
			c = lCond{Kind: cAnd, Sub: []lCond{{Kind: cYourTurn, Neg: true}, c}}
		}
		return c, nil
	}
	return lCond{}, nil
}

// node lowers one SA node (not its Sub).
func (lc *lowerCtx) node(sa *cards.SA, ps *params) error {
	if done, err := lc.putCounterUnion(sa, ps); done { // lower_definedheads.go
		return err
	}
	at := len(lc.ab.Prog)
	usN := len(lc.unlessSubs)
	guard := lCond{}
	// Peek, not get: get marks Defined$ read for every node, and the
	// effect must still read (or leave unread) its own Defined$.
	if d, ok := ps.m["Defined"]; ok && strings.HasPrefix(d, "Targeted.") && sa.API != "Animate" {
		// Defined$ Targeted.<qualifier> (graveexile): gorge definedSpec
		// (context.go:440-452) keeps the chain targets the qualifier
		// admits. Lower only exactly one target slot, so the node's
		// subject is that slot; a player target never matches the card
		// filter (countRefMatches yields 0), as in gorge. Animate lowers
		// this spelling itself (lower_zone.go:913), with its own guard.
		if _, ok := ps.get("ValidTgts"); ok {
			return refuse("Defined$ %q beside its own ValidTgts$", d)
		}
		if len(lc.chainT) != 1 {
			return refuse("Defined$ %q with %d target slots", d, len(lc.chainT))
		}
		t := lc.lastTarget()
		if t < 0 {
			return refuse("Defined$ %q with no target slot", d)
		}
		// The kernel keeps only a slot's first object in its target cell
		// (the rest live in xTargets), while gorge's definedSpec reads
		// every object of the slot. Lower only a slot that can never hold
		// more than one object.
		tg := lc.ab.Targets[t]
		if !tg.Min.isConst() || !tg.Max.isConst() || tg.Min.N > 1 || tg.Max.N > 1 {
			return refuse("Defined$ %q on a multi-target slot", d)
		}
		f, err := lc.filter("Card." + strings.TrimPrefix(d, "Targeted."))
		if err != nil {
			return err
		}
		guard = lCond{Kind: cRefMatches, Ref: lRef{Kind: rTarget, N: t}, F: f, Op: opGE, A: cst(1)}
		lc.targetedDefined = d
	}
	if guard.Kind == cNone {
		g, err := lc.triggeredQualGuard(sa, ps) // lower_definedheads.go
		if err != nil {
			return err
		}
		guard = g
	}
	guardAt := -1
	if guard.Kind != cNone {
		guardAt = len(lc.ab.Prog)
		lc.emit(lInstr{Op: "SkipIf", Cond: guard})
	}
	// Defined$ / TokenOwner$ Player.<q> / Opponent.<q> (B): the
	// node lowers over the plain set; the qualifier restricts it below
	var setPred *playerSetPred
	if key, ok := playerSetAPIs[sa.API]; ok {
		if v, ok := ps.m[key]; ok {
			p, matched, err := lc.playerSetSpec(sa.API, v)
			if err != nil {
				return err
			}
			if matched {
				if guardAt >= 0 {
					return refuse("%s$ %q beside a Targeted. guard", key, v)
				}
				setPred, ps.m[key] = &p, p.base
			}
		}
	}
	effAt := len(lc.ab.Prog)
	if err := lc.effect(sa, ps); err != nil {
		return err
	}
	lc.targetedDefined = ""
	if setPred != nil {
		if err := lc.wrapPlayerSet(sa.API, effAt, *setPred); err != nil {
			return err
		}
	}
	// The condition is read after the effect so it may name the node's
	// own target slot (Hidetsugu's Second Rite reads the targeted player's
	// life); its SkipIf goes in front of the node's instructions. Target
	// slots are declarative, so the guard never changes target selection.
	cond, err := lc.nodeCondition(ps)
	if err != nil {
		return err
	}
	if cond.Kind != cNone || guardAt >= 0 || setPred != nil {
		lc.markUnlessGuarded(usN) // a skip over the node alone (lower_unlesssubs.go)
	}
	if cond.Kind != cNone {
		n := len(lc.ab.Prog) - at
		lc.ab.Prog = append(lc.ab.Prog, lInstr{})
		copy(lc.ab.Prog[at+1:], lc.ab.Prog[at:])
		lc.ab.Prog[at] = lInstr{Op: "SkipIf", Cond: cond, Skip: n}
		if guardAt >= 0 {
			guardAt++ // the node guard moved behind the condition guard
		}
	}
	if guardAt >= 0 {
		lc.ab.Prog[guardAt].Skip = len(lc.ab.Prog) - guardAt - 1
	}
	return nil
}

// unless wraps fn in an "unless a player pays" block: fn runs unless payer
// pays (switched: fn runs only if payer pays).
func (lc *lowerCtx) unless(ps *params, fn func() error) error {
	uc, ok := ps.get("UnlessCost")
	if !ok {
		return fn()
	}
	var in lInstr
	ri, isRes, rerr := lc.resUnlessToken(strings.TrimSpace(uc)) // lower_costtokens.go
	if rerr != nil {
		return rerr
	}
	if isRes {
		in = ri
	} else if _, isVar := lc.face.SVars[strings.TrimSpace(uc)]; isVar {
		// an SVar-sized generic mana cost (Dazzling Denial's Y: {4} if you
		// control a Bird, else {2}); gorge evaluates it as the ask is posed
		v, err := lc.value(strings.TrimSpace(uc))
		if err != nil {
			return err
		}
		in = lInstr{CS: "Mana", A: v, B: cst(0)}
	} else {
		lc.wbOK = lc.ab.Kind != akLatent
		lc.unlessParse = lc.ab.Kind != akLatent
		cost, err := lc.costUnless(uc)
		lc.unlessParse = false
		lc.wbOK = false
		if err != nil {
			return err
		}
		if cost.SubCtr > 0 {
			// gorge's unless SubCounter drains the resolving
			// ability's host, and the corpus's only carrier pays as the
			// controller (UnlessPayer$ You). Any other payer is refused.
			if p, ok := ps.get("UnlessPayer"); !ok || p != "You" {
				return refuse("SubCounter unless cost without UnlessPayer$ You")
			}
		}
		lc.unlessCost = lc.ab.Kind != akLatent
		in, err = lc.payInstr(cost, lRef{}, cost.SubCtr > 0)
		lc.unlessCost = false
		if err != nil {
			return err
		}
	}
	if ps.is("UnlessSwitched", "True") {
		in.Op = "UnlessSwitched"
	} else {
		in.Op = "UnlessPaid"
	}
	gate, whenPaid, err := unlessSubsMode(ps)
	if err != nil {
		return err
	}
	if gate && lc.depth == 0 {
		return refuse("UnlessResolveSubs$ outside a chain")
	}
	at, nTgt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(in)
	if err := fn(); err != nil {
		return err
	}
	// gorge unlessProceed (Forge's default): UnlessPayer$ defaults to
	// TargetedController, the controller of the node's target -- read
	// after the node so its own target slot exists.
	up := "TargetedController"
	if p, ok := ps.get("UnlessPayer"); ok {
		up = p
	}
	if up == "ThisTargetedController" {
		// gorge's unlessPayerTargetsCodes maps ThisTargetedController to the
		// same selector code as TargetedController (effects/unless.go), so it
		// reads the node's OWN target set. A chained link that declared
		// exactly one target lowers exactly like the default: its own slot's
		// controller, or NO payer when that optional target answered empty
		// (gorge: unlessProceed returns the unpaid
		// orientation for a non-nil empty ownTargets, with no fallback ask).
		// The head SA's own target is bound at announcement (E), and a
		// slot allowing several targets names several payers gorge walks in
		// order: both stay refused.
		if lc.head || len(lc.ab.Targets)-nTgt != 1 {
			return refuse("ThisTargetedController unless payer outside a chained single own target")
		}
		if t := lc.ownTarget(); t < 0 {
			return refuse("ThisTargetedController unless payer without its own target slot")
		}
		if mx := lc.ab.Targets[len(lc.ab.Targets)-1].Max; !mx.isConst() || mx.N != 1 {
			return refuse("ThisTargetedController unless payer on a multi-target slot")
		}
		up = "TargetedController"
	}
	payer, err := lc.defined(up)
	if err != nil {
		return err
	}
	if gate && payer.Kind == rEachPlayer {
		return refuse("UnlessResolveSubs$ with an each-player payer")
	}
	lc.ab.Prog[at].Obj = payer
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	if gate {
		lc.unlessSubs = append(lc.unlessSubs, usGate{at: at, nodeEnd: len(lc.ab.Prog),
			same: ps.is("UnlessSwitched", "True") == whenPaid, nTgt: len(lc.ab.Targets)})
	}
	return nil
}

// payInstr encodes a resolution-time cost offer (unless/if-you-do).
// wardEvidenceRe is a ward cost of collect evidence N.
var wardEvidenceRe = regexp.MustCompile(`^CollectEvidence<([0-9]+)>$`)

// wardDiscManaRe is a ward cost of discarding a card or paying {N}
// (Forge's "Discard<1/Card>:N"; gorge beginWardPayment's ">:" arm).
var wardDiscManaRe = regexp.MustCompile(`^Discard<1/Card>:([0-9]+)$`)

func (lc *lowerCtx) payInstr(c lCost, payer lRef, subCtrOK bool) (lInstr, error) {
	in := lInstr{Obj: payer}
	if c.Forage != nil {
		// gorge (trigger_cost_forage.go): the triggered-cost window
		// pays a forage-only Cost$ of a printed trigger's own body ("you
		// may forage. If you do, ..."): PayForage, kernel act_forage.go's
		// resolution forage. A forage beside any other part, in an UnlessCost or
		// deeper in a chain stays refused.
		if !lc.trigOwnCost || !c.Mana.None || c.DiscardN > 0 || c.Sac != nil || c.PayLife > 0 || c.Energy > 0 ||
			c.Return != nil || c.ExileG != nil || c.Tap || c.SacSelf || c.Loyalty != 0 || c.SubCtr != 0 || c.TapN != 0 || c.TapHost {
			return in, refuse("resolution forage cost")
		}
		in.CS, in.A, in.F = "Forage", cst(1), c.Forage
		return in, nil
	}
	if c.Waterbend != 0 {
		// an UnlessCost$ Waterbend<N> (gorge pay.chargeUnlessWaterbend; kernel
		// cost_unlesswaterbend.go): alone, with N the cost's whole generic
		m, rest := c.Mana, c
		rest.Waterbend, rest.Mana = 0, manaCost{None: true}
		if !lc.unlessCost && !lc.wardCost || m.Generic != c.Waterbend || c.Waterbend > 255 || m.Pips != [5]int{} || m.X || m.XX || m.XXX || m.C != 0 ||
			len(m.Hybrid)+len(m.Twobrid)+len(m.Phyrexian) != 0 || !rest.pureMana() {
			return in, refuse("waterbend cost shape")
		}
		in.CS, in.A = "Waterbend", cst(c.Waterbend)
		return in, nil
	}
	n := 0
	hasGraveExile := c.ExileG != nil
	if c.ExileG != nil {
		// a printed trigger's own ExileFromGrave<N/Spec> (gorge's
		// triggered-cost window: trigger_cost pay/decline, then the
		// "exile_cost" pick over the payer's graveyard, unasked with
		// exactly N candidates; Masked Vandal): PayExileGrave
		if !lc.trigOwnCost && !lc.unlessCost {
			return in, refuse("resolution exile-from-graveyard cost")
		}
		for _, cl := range c.ExileG.Clauses {
			if cl.Flags&fSelf != 0 {
				return in, refuse("resolution exile-from-graveyard cost of the source")
			}
		}
		n++
		in.CS, in.A, in.F = "ExileGrave", cst(c.ExileGN), c.ExileG
		c.ExileG = nil
	}
	if c.ExileStackSelf {
		// ExileFromStack<1/Card.Self> beside the graveyard exile (Sphinx's
		// Approach, Dragon's Approach): the resolving spell itself leaves the
		// stack as the same unless cost is paid, after the grave picks.
		if !hasGraveExile || !lc.unlessCost {
			return in, refuse("resolution exile-from-stack cost shape")
		}
		in.CS = "ExileStackSelf"
		c.ExileStackSelf = false
	}
	if c.DrawN > 0 {
		// gorge's triggered-cost window: a Draw-bearing cost of a printed
		// trigger's own body (cumulative.go: offered when its counts
		// resolve; paying draws). Only alone and only there.
		if !lc.trigOwnCost {
			return in, refuse("resolution draw cost outside a trigger's own cost")
		}
		n++
		in.CS, in.A = "Draw", cst(c.DrawN)
	}
	if !c.Mana.None {
		n++
		if c.Mana.X && lc.trigXMax != nil {
			return lc.payManaX(c, in) // lower_triggerxcost.go
		}
		if c.Mana.X || len(c.Mana.Hybrid) > 0 || c.Mana.C > 0 {
			return in, refuse("resolution mana cost %s", c.Mana)
		}
		in.CS = "Mana"
		in.A = cst(c.Mana.Generic)
		in.B = cst(packPips(c.Mana.Pips))
	}
	if c.DiscardN > 0 {
		n++
		switch c.DiscardSpec {
		case "Random":
			in.CS, in.A = "DiscardRandom", cst(c.DiscardN)
		case "Hand":
			in.CS, in.A = "DiscardHand", cst(c.DiscardN)
		default:
			in.CS, in.A = "Discard", cst(c.DiscardN)
		}
	}
	if c.Sac != nil {
		n++
		in.CS, in.A, in.F = "Sacrifice", cst(c.SacN), c.Sac
	}
	if c.PayLife > 0 && in.CS == "Mana" && (lc.trigOwnCost || lc.wardCost) {
		// a printed trigger's own "pay {W}{B} and 2 life" (Zoraline; gorge
		// triggeredCostPayable's mana/life half, pay.CostPayableOther): the
		// mana pay carries the life in C (kernel F). A ward's
		// "{N} and pay L life" (Ovika; gorge AdvanceUnless charges both in
		// PayManaConv) carries it the same way (build_ab.go WardLife).
		in.C = c.PayLife
	} else if c.PayLife > 0 {
		n++
		in.CS, in.A = "PayLife", cst(c.PayLife)
	}
	if c.LifeBy != nil {
		// PayLife<X> of a trigger's own cost: the amount is the value read as
		// the cost is offered (kernel interp_ask.go execPay evaluates in.A);
		// beside any other part the count below refuses
		n++
		in.CS, in.A = "PayLife", *c.LifeBy
	}
	if c.Energy > 0 {
		n++
		in.CS, in.A = "PayEnergy", cst(c.Energy)
	}
	if c.Return != nil {
		// one pick: gorge settles all N at once, the kernel one per answer
		if c.ReturnN != 1 {
			return in, refuse("resolution return cost of %d", c.ReturnN)
		}
		n++
		in.CS, in.A, in.F = "Return", cst(1), c.Return
	}
	if c.SacSelf && n == 0 {
		// Sac<1/CARDNAME> alone (gorge's triggered-cost window: the one
		// candidate is the source, settled unasked): PaySacSelf
		n++
		in.CS, in.A = "SacrificeSelf", cst(1)
		c.SacSelf = false
	}
	if c.SacHost && n == 0 && lc.trigOwnCost && lc.grantAttached {
		// Sac<1/OriginalHost> as the own cost of a trigger an Equipment or Aura
		// grants (Sunfire Torch, Spare Dagger; gorge's triggered-cost window binds
		// the part to the granting host): the host is sacrificed, the one
		// candidate settled unasked (CR 701.21a)
		n++
		in.CS, in.A = "SacrificeHost", cst(1)
		c.SacHost = false
	}
	if c.BlightN > 0 {
		// Blight<N> as a trigger's own cost or an UnlessCost (gorge
		// trigger_cost_blight / unlessflow): the payer chooses a creature
		// they control (the source included) and puts N -1/-1 counters on
		// it at payment.
		if n != 0 {
			return in, refuse("resolution blight cost shape")
		}
		in.CS, in.A = "Blight", cst(c.BlightN)
		in.F = &lFilter{Clauses: []lClause{{Types: tCreature, Ctrl: whoYou}}}
		c.BlightN = 0
		n++
	}
	if c.EvidenceN > 0 {
		// CollectEvidence<N> as a printed trigger's own cost (gorge's
		// triggered-cost window: cumulative.go payability, then
		// evidence_cost.go evidenceStage): PayEvidence, kernel
		// cost_collectevidence.go. Alone, and only there: an UnlessCost
		// pays through unlessflow, not modelled.
		if !lc.trigOwnCost || n != 0 {
			return in, refuse("resolution evidence cost shape")
		}
		in.CS, in.A = "Evidence", cst(c.EvidenceN)
		if c.ExileTrigNew {
			// Lamplight Phoenix's ExileAnyGrave<1/Card.TriggeredNewCard>
			// CollectEvidence<N>: gorge reserves the triggered card's exile
			// first (cumulative.go evidenceReserveOrder), then the evidence
			// stage runs over the rest of the graveyard
			// (cost_collectevidence.go)
			if !lc.selfDiesTrigger() {
				return in, refuse("resolution ExileAnyGrave cost shape")
			}
			in.CS = "ExileTrigNewEvidence"
			c.ExileTrigNew = false
		}
		c.EvidenceN = 0
		n++
	}
	if c.EvidenceX {
		// CollectEvidence<X> as a printed trigger's own cost (Incinerator of
		// the Guilty): gorge's triggeredCostXAsk announces X over 0..the
		// graveyard's total mana value (capped at 64), then the ordinary
		// pay election and the evidence stage; the body's Count$xPaid is X
		// (kernel cost_collectevidencex.go). Alone, and only there.
		if !lc.trigOwnCost || n != 0 || !c.Mana.None {
			return in, refuse("resolution evidence X cost shape")
		}
		if x, ok := lc.face.SVars["X"]; !ok || strings.TrimSpace(x) != "Count$xPaid" {
			return in, refuse("trigger CollectEvidence<X> cost with SVar:X other than Count$xPaid")
		}
		in.CS, in.A = "EvidenceX", cst(trigXFoldCap)
		lc.trigXPaid = true
		c.EvidenceX = false
		n++
	}
	if c.SubAmongN > 0 {
		// RemoveAnyCounter<N/Any/CARDNAME> as a trigger's own cost: the
		// source's counters, any kind (gorge trigger_cost_subcounter.go's
		// subCounterKindAsk). A filtered among-part is a different payment.
		if c.SubAmongF != nil || n != 0 {
			return in, refuse("resolution RemoveAnyCounter cost shape")
		}
		var ck int
		if strings.EqualFold(c.SubAmongKnd, "Any") {
			ck = int(kop.CtrAll)
		} else {
			k, err := counterKind(c.SubAmongKnd)
			if err != nil {
				return in, refuse("resolution RemoveAnyCounter cost of kind %q", c.SubAmongKnd)
			}
			ck = int(k)
		}
		in.CS, in.A, in.B = "SubCounter", cst(c.SubAmongN), cst(ck)
		c.SubAmongN = 0
		n++
	}
	if c.SubCtr > 0 {
		// B: a fixed SubCounter<N/Kind> drained from the resolving
		// ability's own source (gorge trigger_cost_subcounter.go:31, the
		// trigger's own cost; unless_cost.go:47, an UnlessCost$). The cost
		// parser admits only the source form and rejects X/announced and
		// named targets; a wildcard "Any" kind is CtrAll.
		if !subCtrOK || n != 0 {
			return in, refuse("resolution SubCounter cost shape")
		}
		var ck int
		if strings.EqualFold(c.SubCtrKnd, "Any") {
			ck = int(kop.CtrAll)
		} else {
			k, err := counterKind(c.SubCtrKnd)
			if err != nil {
				return in, refuse("resolution SubCounter cost of kind %q", c.SubCtrKnd)
			}
			ck = int(k)
		}
		in.CS, in.A, in.B = "SubCounter", cst(c.SubCtr), cst(ck)
		c.SubCtr = 0
		n++
	}
	if c.TapXVar {
		// tapXType<X/Spec> as a printed trigger's own whole cost (lower_tapxvar.go)
		if !lc.trigOwnCost || n != 0 || c.TapF == nil {
			return in, refuse("resolution tapXType<X> cost shape")
		}
		in.CS, in.A, in.F = "TapXVar", cst(0), c.TapF
		lc.trigXPaid = true
		c.TapXVar = false
		n++
	}
	if c.TapN > 0 {
		// C: a literal tapXType<N/Spec> that is the WHOLE cost of a
		// printed trigger (gorge cumulative.go:1130-1137 and
		// triggered_literal_tap.go: one KChoose, Min=Max=N, over
		// TapCostCandidates; the tap election is the payment). As an
		// UnlessCost$ (gorge, unlessflow.go
		// AdvanceUnless's UnlessKindTap part) the pay / decline ask comes
		// first, then the pick, unasked with exactly N candidates.
		if !lc.trigOwnCost && !lc.unlessCost || n != 0 || c.TapF == nil {
			return in, refuse("resolution tapXType cost shape")
		}
		in.CS, in.A, in.F = "TapX", cst(c.TapN), c.TapF
		c.TapN = 0
		n++
	}
	if c.ExileTrigNew {
		// ExileAnyGrave<1/Card.TriggeredNewCard> as a printed dies
		// trigger's own cost (gorge's triggered-cost
		// window: the pay/decline ask, then the triggered card is exiled
		// unasked — one candidate, no pick). Alone, and only there: a cost
		// with another part (Lamplight Phoenix's evidence, Kinzu's life),
		// an unless cost (Flitting Guerrilla) or a non-dies trigger keeps
		// refusing.
		if !lc.trigOwnCost || n != 0 || !lc.selfDiesTrigger() {
			return in, refuse("resolution ExileAnyGrave cost shape")
		}
		in.CS, in.A = "ExileTrigNew", cst(1)
		c.ExileTrigNew = false
		n++
	}
	if c.ExileHandN > 0 {
		// ExileFromHand<N/Card> as a printed trigger's own cost (gorge's
		// triggered-cost window: the pay/decline ask, then the "exile_cost"
		// pick over the hand, unasked with exactly N cards): PayExileGrave
		// with the hand sentinel. Alone, and only there.
		if !lc.trigOwnCost || lc.unlessCost || n != 0 {
			return in, refuse("resolution exile-from-hand cost shape")
		}
		in.CS, in.A = "ExileHand", cst(c.ExileHandN)
		c.ExileHandN = 0
		n++
	}
	if c.BeholdN > 0 {
		// Behold<1/Type> as a resolution cost (CR 701.4a): an UnlessCost$ or
		// a printed trigger's own cost. gorge's unless and triggered-cost
		// flows pose the pay election, then the "beholdcost" pick over the
		// payer's matching permanents then hand cards, unasked with exactly
		// the count; nothing moves (kernel cost_unlessbehold.go). Alone, one.
		if !lc.trigOwnCost && !lc.unlessCost || n != 0 || c.BeholdN != 1 || c.BeholdF == nil {
			return in, refuse("resolution behold cost shape")
		}
		in.CS, in.A, in.F = "Behold", cst(1), c.BeholdF
		c.BeholdN, c.BeholdF = 0, nil
		n++
	}
	if c.AddCtr > 0 {
		// An UnlessCost$ AddCounter<N/KIND/recipient> (gorge
		// pay/costparts.go PayManaSourceParts' unless placement; kernel
		// cost_unlessaddcounter.go): alone, with the recipient the source
		// (CARDNAME/NICKNAME) or the parent's target. A two-field token
		// stays an activation cost.
		if !lc.unlessCost || c.AddCtrRef.Kind == rNone || n != 0 {
			return in, refuse("resolution AddCounter cost shape")
		}
		ck, err := counterKind(c.AddCtrKnd)
		if err != nil {
			return in, refuse("resolution AddCounter cost of kind %q", c.AddCtrKnd)
		}
		in.CS, in.A, in.B, in.Obj2 = "AddCounter", cst(c.AddCtr), cst(int(ck)), c.AddCtrRef
		c.AddCtr, c.AddCtrRef, c.AddCtrKnd = 0, lRef{}, ""
		n++
	}
	if n != 1 || c.Tap || c.SacSelf || c.Loyalty != 0 || c.SubCtr != 0 || c.SubAmongN != 0 || c.TapN != 0 || c.TapHost || c.SacHost || c.BlightN != 0 || c.EvidenceN != 0 || c.EvidenceX || c.AddCtr != 0 || c.AddCtrRef.Kind != rNone {
		return in, refuse("resolution cost shape")
	}
	return in, nil
}

// packPips packs WUBRG pip counts 3 bits each.
func packPips(p [5]int) int {
	v := 0
	for i := 4; i >= 0; i-- {
		v = v<<3 | p[i]
	}
	return v
}

// effect dispatches on the API.
func (lc *lowerCtx) effect(sa *cards.SA, ps *params) error {
	// An AB$ inside a trigger body (an optional cost paid on resolution).
	if lc.ab.Kind == akTriggered || lc.ab.Kind == akLatent {
		if c, ok := ps.get("Cost"); ok && sa.Kind == "AB" {
			if strings.HasSuffix(c, `\NumTimes>`) && strings.HasPrefix(c, "Mana<") {
				return lc.numTimesPay(sa, ps, c) // lower_numtimes.go
			}
			// A printed trigger's own optional cost reads the unless-cost
			// selection methods (gorge's triggered-cost window pays a Hand
			// discard with the whole hand whatever the count; CR 118.3,
			// 603.4). A latent body, an activated cost and a spell cost keep
			// refusing them.
			parse := lc.cost
			if lc.ab.Kind == akTriggered && len(lc.ab.Prog) == 0 {
				parse = lc.costUnless
				lc.trigBodyCost = true
			}
			cost, err := parse(c)
			lc.trigBodyCost = false
			if err != nil {
				return err
			}
			lc.trigOwnCost = lc.ab.Kind == akTriggered && len(lc.ab.Prog) == 0
			if lc.trigOwnCost && cost.Mana.X && !cost.Mana.None {
				// a printed trigger's own "you may pay {X}": gorge announces X
				// first, capped by the body's XMax$ (lower_triggerxcost.go)
				if err := lc.trigXCost(ps); err != nil {
					return err
				}
			}
			var chain *lInstr
			if lc.trigOwnCost && cost.ExileG != nil && !cost.Mana.None {
				if chain, cost, err = lc.exileChainSplit(cost); err != nil { // lower_exilechain.go
					lc.trigOwnCost = false
					return err
				}
			}
			in, err := lc.payInstr(cost, lRef{Kind: rYou}, lc.trigOwnCost)
			lc.trigOwnCost = false
			lc.trigXMax = nil
			if err != nil {
				return err
			}
			if _, ok := ps.m["UnlessResolveSubs"]; ok {
				return refuse("UnlessResolveSubs$ with a trigger's own cost")
			}
			in.Op = "IfPaid"
			at := len(lc.ab.Prog)
			lc.emit(in)
			if chain != nil {
				lc.emit(*chain)
			}
			if err := lc.unless(ps, func() error { return lc.api(sa, ps) }); err != nil {
				return err
			}
			// "If you do" covers the rest of the chain too.
			if !strings.HasPrefix(lc.ab.Prog[at].Note, noteRememberSacrificed) {
				// a sacrifice a value head reads keeps its remember note
				// the skip is patched either way
				lc.ab.Prog[at].Note = "covers the Sub chain"
			}
			lc.ab.Prog[at].Skip = -1 // patched by the trigger lowering to the chain end
			return nil
		}
	}
	if sa.API == "Ward" {
		// Ward's UnlessCost is the ward cost itself, not an unless wrapper.
		return lc.api(sa, ps)
	}
	// A Sacrifice's UnlessCost$ Discard<N/Card> lowers like any other unless
	// cost: gorge's shared unless gate prices the discard and pays it
	// (Masticore, The Great Juggernaut, Bebop & Rocksteady). A
	// filtered discard spec still refuses in the cost parser.
	return lc.unless(ps, func() error { return lc.api(sa, ps) })
}

func (lc *lowerCtx) api(sa *cards.SA, ps *params) error {
	ps.consume("Planeswalker")
	switch sa.API {
	case "EachDamage":
		return lc.eachDamage(ps)
	case "DealDamage":
		return lc.dealDamage(ps)
	case "DamageAll":
		return lc.damageAll(ps)
	case "Destroy":
		obj, err := lc.objOrTarget(ps, "", zBattlefield)
		if err != nil {
			return err
		}
		nr, err := noRegen(ps)
		if err != nil {
			return err
		}
		in := lInstr{Op: "Destroy", Obj: obj, C: nr}
		if done, err := lc.destroyRememberLKIDestroyed(ps, &in); err != nil { // lower_rememberwritersmass.go
			return err
		} else if !done {
			if err := destroyRememberLKI(ps, &in); err != nil { // lower_handmove.go
				return err
			}
		}
		if err := destroyRememberDestroyed(ps, &in); err != nil { // lower_remembered.go
			return err
		}
		lc.emit(in)
	case "DestroyAll":
		var f *lFilter
		var err error
		if vc := ps.str("ValidCards"); vc == "TargetedCard.Self,Permanent.NotDefinedTargeted+sharesNameWith Targeted" {
			f, err = lc.sameNameT0Filter() // lower_choosetype.go
		} else {
			f, err = lc.filter(vc)
		}
		if err != nil {
			return err
		}
		in := lInstr{Op: "DestroyAll", F: f}
		if v, ok := ps.get("Hidden"); ok && v != "True" {
			// Forge's UI flag: gorge's effDestroyAll (zone_bulk.go) never reads
			// Hidden$ (PKHidden is read only by ChangeZone); Steel Hellkite
			return refuse("DestroyAll Hidden %q", v)
		}
		if v, ok := ps.get("Zone"); ok && v != "Battlefield" {
			return refuse("DestroyAll Zone %q: only permanents are destroyed", v)
		}
		if v, ok := ps.get("RememberDestroyed"); ok {
			if v != "True" {
				return refuse("RememberDestroyed %q", v)
			}
			in.C = 1 // the destroyed cards put into a graveyard become the remembered set (Starfall Invocation)
		}
		nr, err := noRegen(ps)
		if err != nil {
			return err
		}
		in.C |= nr << 1
		lc.emit(in)
	case "ChangeZone":
		return lc.changeZone(ps)
	case "Assimilate":
		return lc.assimilate(ps) // lower_assimilate.go
	case "ChangeZoneAll":
		return lc.changeZoneAll(ps)
	case "Draw":
		remDrawn, remAll, err := rememberDrawn(ps)
		if err != nil {
			return err
		}
		if who, ok, err := lc.targetedAndYou(ps, "Defined"); err != nil {
			return err
		} else if ok {
			if remDrawn {
				return refuse("RememberDrawn$ on a two-player draw")
			}
			if _, ok := ps.get("OptionalDecider"); ok {
				return refuse("OptionalDecider$ on a two-player draw")
			}
			n, err := lc.amount(ps, "NumCards", 1)
			if err != nil {
				return err
			}
			lc.emit(lInstr{Op: "Draw", Obj: who, A: n})
			lc.emit(lInstr{Op: "Draw", Obj: lRef{Kind: rYou}, A: n})
			return nil
		}
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "NumCards", 1)
		if err != nil {
			return err
		}
		if err := lc.drawDecider(ps, who); err != nil { // lower_optional.go
			return err
		}
		in := lInstr{Op: "Draw", Obj: who, A: n}
		switch {
		case remAll:
			in.CS = "RememberAll"
		case remDrawn:
			in.CS = "Remember"
		}
		lc.emit(in)
	case "Discard":
		return lc.discard(ps)
	case "Recruit":
		return lc.recruit(sa, ps) // lower_recruit.go
	case "Vote":
		return lc.vote(sa, ps) // lower_vote.go
	case "Mill":
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "NumCards", 1)
		if err != nil {
			return err
		}
		in := lInstr{Op: "Mill", Obj: who, A: n}
		if ps.is("RememberMilled", "True") {
			in.CS = "Remember"
		}
		if err := lc.millImprint(ps, &in); err != nil { // lower_imprintzone.go
			return err
		}
		if err := lc.mayGate(ps, kop.MayGuardMill, who); err != nil {
			return err
		}
		lc.emit(in)
	case "Surveil":
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "Amount", 1)
		if err != nil {
			return err
		}
		in := lInstr{Op: "Surveil", Obj: who, A: n}
		if rem, err := rememberParam(ps, "RememberMoved"); err != nil {
			return err
		} else if rem {
			in.CS = "Remember" // the cards put into the graveyard
		}
		if kept, err := rememberParam(ps, "RememberKept"); err != nil {
			return err
		} else if kept {
			if in.CS != "" {
				return refuse("Surveil RememberKept$ beside RememberMoved$")
			}
			in.CS = "RememberKept" // the cards left on top
		}
		lc.emit(in)
	case "GainLife", "LoseLife":
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "LifeAmount", 1)
		if err != nil {
			return err
		}
		if sa.API == "GainLife" && n.Kind == vConst && lc.face.SVars[ps.str("LifeAmount")] == "Number$0" {
			// Exsanguinate's AFLifeLost: the life the previous LoseLife took.
			n = lValue{Kind: vLastLifeLost}
		}
		lc.emit(lInstr{Op: sa.API, Obj: who, A: n})
	case "Pump":
		if _, ok := ps.m["TargetType"]; ok {
			return lc.pumpSpellTarget(ps) // lower_stackexile.go
		}
		if _, ok := ps.get("DefinedKW"); ok {
			return lc.colorGrantPump(ps) // (Skrelv)
		}
		if done, err := lc.pumpYouAndValid(ps); done || err != nil {
			return err // lower_definedheads.go
		}
		if done, err := lc.definedValid(sa, ps); done || err != nil {
			return err
		}
		t0 := lc.lastTarget()
		if err := lc.pump(ps); err != nil {
			return err
		}
		return lc.pumpRememberTargeted(ps, t0) // lower_remembered.go
	case "PumpAll":
		return lc.pumpAll(ps)
	case "PutCounter":
		if done, err := lc.definedValid(sa, ps); done || err != nil {
			return err
		}
		return lc.putCounter(ps)
	case "RemoveCounter":
		if _, ok := ps.m["Choices"]; ok {
			return lc.rmctrChoices(ps) // lower_removecounterchoices.go
		}
		obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
		if err != nil {
			return err
		}
		if ps.is("CounterType", "Any") && !ps.is("CounterNum", "All") {
			// gorge removeCounterAny (cr-removecounter-any-upto): a kind
			// pick per counter, UpTo$ a Done at every step (Glissa
			// Sunslayer)
			n, err := lc.amount(ps, "CounterNum", 1)
			if err != nil {
				return err
			}
			if !n.isConst() || n.N < 1 || n.N > 8 {
				return refuse("RemoveCounter Any CounterNum %v", n)
			}
			in := lInstr{Op: "RemoveCounterAny", Obj: obj, B: n}
			if v, ok := ps.get("UpTo"); ok {
				if v != "True" {
					return refuse("RemoveCounter UpTo$ %q", v)
				}
				in.C = 1
			}
			if ps.is("RememberRemoved", "True") {
				return refuse("RemoveCounter Any RememberRemoved$")
			}
			lc.emit(in)
			return nil
		}
		var n lValue
		if ps.is("CounterNum", "All") {
			if ps.is("CounterType", "All") {
				return lc.clearCounters(ps, obj)
			}
			// gorge effRemoveCounter's NumAll: every counter of the kind the
			// object holds, read before the removal; a CounterType$ naming
			// no single kind (Any) refuses
			if k := ps.str("CounterType"); k == "" || k == "Any" {
				return refuse("RemoveCounter All of CounterType %q", ps.str("CounterType"))
			}
			n = lValue{Kind: vCountersOn, Ref: obj, S: ps.str("CounterType")}
		} else if n, err = lc.amount(ps, "CounterNum", 1); err != nil {
			return err
		}
		in := lInstr{Op: "RemoveCounter", Obj: obj, A: n, CS: ps.str("CounterType")}
		if ps.is("RememberRemoved", "True") {
			in.Note = "remember removed"
			in.C = 1
		}
		lc.emit(in)
	case "MultiplyCounter":
		if m := ps.str("Multiplier"); m != "" && m != "2" { // gorge effMultiplyCounter's default is 2
			return refuse("MultiplyCounter Multiplier$ %q", m)
		}
		if done, err := lc.definedValid(sa, ps); done || err != nil {
			return err
		}
		obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
		if err != nil {
			return err
		}
		in := lInstr{Op: "DoubleCounters", Obj: obj, CS: ps.str("CounterType")}
		if in.CS == "" {
			in.CS = "All"
		}
		lc.emit(in)
	case "Token":
		return lc.token(ps)
	case "Airbend":
		return lc.airbend(ps) // lower_airbend.go
	case "Tap", "Untap":
		if done, err := lc.untapChoose(sa, ps); done || err != nil { // lower_handmove.go
			return err
		}
		obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
		if err != nil {
			return err
		}
		if ps.is("ETB", "True") {
			return refuse("ETB tap outside a replacement")
		}
		lc.emit(lInstr{Op: sa.API, Obj: obj})
	case "Fight":
		return lc.fight(ps)
	case "Earthbend":
		return lc.earthbend(ps)
	case "Endure":
		return lc.endure(ps) // lower_endure.go
	case "Clone":
		return lc.cloneAttached(ps) // lower_cloneattached.go
	case "Goad":
		return lc.goad(ps) // lower_goad.go
	case "TakeInitiative":
		return lc.takeInitiative(ps) // lower_initiative.go
	case "Counter":
		if _, ok := ps.get("TargetType"); !ok {
			if d, ok := ps.get("Defined"); ok {
				if spec, ok := strings.CutPrefix(d, "ValidStack "); ok {
					return lc.counterAll(ps, strings.TrimSpace(spec)) // lower_stackrewrite.go
				}
				return lc.counterTriggered(d)
			}
		}
		tt := ps.str("TargetType")
		listF, trigOnly := "", false
		if tt != "Spell" && tt != "Spell,Activated,Triggered" && tt != "Activated,Triggered" {
			var terr error
			if listF, trigOnly, terr = counterTypeList(tt); terr != nil { // lower_counterspell.go
				return terr
			}
		}
		var vtg *lFilter
		if v, ok := ps.get("TargetValidTargeting"); ok {
			if tt != "Spell" || ps.str("ValidTgts") != "Card" {
				return refuse("Counter TargetValidTargeting beside TargetType %q ValidTgts %q", tt, ps.str("ValidTgts"))
			}
			var verr error
			if vtg, verr = lc.counterValidTargeting(strings.TrimSpace(v)); verr != nil {
				return verr
			}
		}
		if v, ok := ps.get("TgtZone"); ok && (tt != "Spell" || v != "Stack") {
			return refuse("Counter TgtZone$ %q beside TargetType %q", v, tt)
		}
		remMV := 0
		if v, ok := ps.get("RememberCounteredCMC"); ok {
			if v != "True" || tt != "Spell" {
				return refuse("Counter RememberCounteredCMC$ %q beside TargetType %q", v, tt)
			}
			remMV = kop.CtrRemMV
		}
		if v, ok := ps.get("RememberForCounter"); ok && v != "True" {
			return refuse("Counter RememberForCounter$ %q", v)
		}
		cl := false
		if v, ok := ps.get("RememberCountered"); ok {
			// Tishana's Tidebinder: the countered ability's source card
			// is remembered for the loss-of-abilities Effect after it
			if v != "True" || tt != "Activated,Triggered" || ps.str("Destination") != "" || ps.str("SubAbility") == "" {
				return refuse("Counter RememberCountered$ shape")
			}
			cl = true
		}
		vt := ps.str("ValidTgts")
		if tt != "Spell" {
			// gorge's stack census: a spell matching the spec, any ability
			// (Card,Emblem names every source; lower_stacktgt.go)
			if vt != "Card,Emblem" && vt != "Card" {
				return refuse("Counter TargetType %q ValidTgts %q", tt, vt)
			}
			vt = "Card"
		}
		if listF != "" {
			vt = listF
		}
		f, err := lc.filter(vt)
		if err != nil {
			return err
		}
		lt := lTarget{F: f, Zone: zStack, Spell: true, StackAb: tt != "Spell", AbOnly: tt == "Activated,Triggered", TrigOnly: trigOnly, VTgt: vtg}
		if v, ok := ps.get("TargetMin"); ok {
			if lt.Min, err = lc.value(v); err != nil {
				return err
			}
		}
		if v, ok := ps.get("TargetMax"); ok {
			if lt.Max, err = lc.value(v); err != nil {
				return err
			}
		}
		t := lc.addTarget(lt)
		if cl {
			lc.emit(lInstr{Op: "CounterLose", Obj: lRef{Kind: rTarget, N: t}, C: -1, Note: "counter the ability; its source permanent loses all abilities while the host stays"})
			lc.clAt = len(lc.ab.Prog)
			return nil
		}
		dest, err := counterDestination(ps)
		if err != nil {
			return err
		}
		in := lInstr{Op: "Counter", Obj: lRef{Kind: rTarget, N: t}, C: dest}
		if remMV != 0 {
			in.B = cst(remMV)
		}
		if ps.is("RememberForCounter", "True") {
			in.Note = rememberCounterNote // its controller is the chain's RememberedController
		}
		lc.emit(in)
	case "Sacrifice":
		if ps.str("Defined") == "DelayTriggerRememberedLKI" {
			// CR 702.181a: the group program's body (lowerCtx.groupDelayed
			// checked its shape); gorge sacrifices the survivors as one batch
			if !lc.inGroup {
				return refuse("Sacrifice Defined$ DelayTriggerRememberedLKI outside a Mobilize group")
			}
			lc.emit(lInstr{Op: "SacrificeDelayed", Obj: lRef{Kind: rDelayedRemembered}})
			return nil
		}
		if ps.str("Mode") == "TgtChoose" {
			// gorge effSacrifice never reads Mode$: a targeted player
			// chooses what to sacrifice whether or not it says so
			ps.consume("Mode")
		}
		_, hasD := ps.get("Defined")
		_, hasT := ps.get("ValidTgts")
		sv := strings.TrimSpace(ps.str("SacValid"))
		selfSac := sv == "" || sv == "Self" || sv == "Card.Self"
		if selfSac && !hasT && (!hasD || ps.str("Defined") == "Self") {
			// gorge effSacrifice: no Defined$/ValidTgts$ (or Defined$ Self)
			// and no SacValid$ other than Self resolves Defined() to the
			// source object, which is sacrificed: its controller sacrifices
			// the one permanent "Self" matches (no choice, count <= Amount).
			ps.consume("Defined")
			n, err := lc.amount(ps, "Amount", 1)
			if err != nil {
				return err
			}
			in := lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: n, F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}}
			if err := lc.sacrificeOptional(ps, &in); err != nil {
				return err
			}
			if err := rememberSacrificed(ps, &in); err != nil {
				return err
			}
			lc.emit(in)
			return nil
		}
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		f, err := lc.filter(ps.str("SacValid"))
		if err != nil {
			return err
		}
		n, err := lc.amount(ps, "Amount", 1)
		if err != nil {
			return err
		}
		if ann := ps.str("Annihilator"); ann != "" {
			// CR 702.86a: the Annihilator expansion's count rides the
			// marker and replaces Amount$ (gorge zone_bulk.go effSacrifice)
			v, aerr := strconv.Atoi(ann)
			if aerr != nil || v < 0 {
				return refuse("Annihilator count %q", ann)
			}
			ps.consume("Annihilator")
			n = cst(v)
		}
		in := lInstr{Op: "Sacrifice", Obj: who, A: n, F: f}
		if err := lc.sacrificeOptional(ps, &in); err != nil {
			return err
		}
		if err := rememberSacrificed(ps, &in); err != nil {
			return err
		}
		lc.emit(in)
	case "SacrificeAll":
		if ps.str("Defined") == "DelayTriggerRememberedLKI" {
			// a remembered group's delayed body (lowerCtx.delayedGroupSac:
			// Will Riker, First in the Field); the same ExtSacDelayed batch
			// as Mobilize's group, emitted by Sacrifice below
			if !lc.inGroup {
				return refuse("SacrificeAll Defined$ DelayTriggerRememberedLKI outside a delayed group")
			}
			lc.emit(lInstr{Op: "SacrificeDelayed", Obj: lRef{Kind: rDelayedRemembered}})
			return nil
		}
		if _, ok := ps.get("ValidCards"); !ok {
			// gorge effSacrificeAll's Defined$ path sacrifices the named
			// objects that are on the battlefield: Defined$ Self is the
			// permanent "Self" matches; Defined$ Targeted (this node's
			// target slot) the permanent the resolving ability targets
			d := ps.str("Defined")
			_, hasT := ps.get("ValidTgts")
			var cl lClause
			switch {
			case d == "Self" && !hasT:
				cl.Flags = fSelf
			case d == "Targeted" && hasT:
				if _, err := lc.objOrTarget(newParams(map[string]string{"ValidTgts": ps.str("ValidTgts")}), "", zBattlefield); err != nil {
					return err
				}
				ps.consume("TgtPrompt")
				cl.Flags = fTargetedBy
			default:
				return refuse("SacrificeAll without ValidCards$ (Defined$ %q)", d)
			}
			if ctl, ok := ps.get("Controller"); ok {
				// gorge effSacrificeAll keeps only the Defined$
				// objects a Controller$ player controls, judged against
				// the board before the sacrifice (controlledOK); You is
				// the resolving controller (definedPlayerIDs: the
				// delayed trigger's controller, Fog Elemental's "sacrifice
				// it at end of combat"), the filter's ctrl=You. The other
				// corpus spellings (TriggeredPlayer, OriginalHostController)
				// name no kernel player selector: refused.
				if strings.TrimSpace(ctl) != "You" {
					return refuse("SacrificeAll Controller$ %q", ctl)
				}
				cl.Ctrl = whoYou
			}
			in := lInstr{Op: "SacrificeAll", F: &lFilter{Clauses: []lClause{cl}}}
			if rem, err := rememberParam(ps, "RememberSacrificed"); err != nil {
				return err
			} else if rem {
				in.CS = "Remember"
			}
			lc.emit(in)
			return nil
		}
		f, err := lc.filter(ps.str("ValidCards"))
		if err != nil {
			return err
		}
		in := lInstr{Op: "SacrificeAll", F: f}
		if rem, err := rememberParam(ps, "RememberSacrificed"); err != nil {
			return err
		} else if rem {
			in.CS = "Remember"
		}
		lc.emit(in)
	case "Charm":
		return lc.charm(ps)
	case "Attach":
		return lc.attach(ps)
	case "Dig":
		return lc.dig(ps)
	case "DigUntil":
		return lc.digUntil(ps)
	case "Effect":
		if lc.prevColor {
			return lc.preventColorEffect(ps)
		}
		if lc.colGrant {
			return lc.colorGrantEffect(ps) // (Skrelv)
		}
		if lc.clAt > 0 {
			return lc.counterLoseEffect(ps) // (Tishana's Tidebinder)
		}
		return lc.effectAPI(ps)
	case "Animate":
		return lc.animate(ps)
	case "Phases":
		return lc.phases(ps) // lower_phasing.go
	case "CopyPermanent":
		return lc.copyPermanent(ps)
	case "GainControl":
		return lc.gainControl(ps)
	case "ExchangeControl":
		return lc.exchangeControl(ps)
	case "DelayedTrigger":
		return lc.delayedTrigger(ps)
	case "ImmediateTrigger":
		return lc.immediateTrigger(ps)
	case "Cleanup":
		if ps.is("ClearRemembered", "True") {
			lc.emit(lInstr{Op: "ClearRemembered"})
		}
		if ps.is("ClearChosenCard", "True") {
			lc.emit(lInstr{Op: "ClearChosen"})
		}
		lc.clearChosenPlayer(ps) // lower_definedheads.go
		if v, ok := ps.get("ClearChosenColor"); ok {
			// the chosen-colour grants read the colour at resolution (gorge
			// definedKWGrant, freezeChosenColor): clearing it after them
			// changes nothing
			if v != "True" || !lc.colGrant {
				return refuse("Cleanup ClearChosenColor$ outside a chosen-colour grant")
			}
			lc.colGrant = false
		}
		if lc.breachStage == 2 && ps.is("ClearImprinted", "True") && ps.is("ClearChosenCard", "True") {
			ps.consume("ClearImprinted") // Breach the Multiverse: the chosen set is the imprint (lower_repeatgravechoice.go)
			lc.breachStage = 0
		}
		if lc.exileLandStage == 2 && ps.is("ClearImprinted", "True") && ps.is("ClearChosenCard", "True") && ps.is("ClearRemembered", "True") {
			ps.consume("ClearImprinted") // Ensign Pavel Chekov: the chosen set is the imprint (lower_repeatexilechoice.go)
			lc.exileLandStage = 0
		}
		if lc.digImprintSetLive() && ps.is("ClearImprinted", "True") {
			ps.consume("ClearImprinted") // the Dig's imprinted set (lower_digimprintset.go)
			lc.emit(lInstr{Op: "ClearImprinted"})
		}
		if lc.digImprinted && ps.is("ClearImprinted", "True") {
			// the imprinted card is the kernel's remembered set
			// (lower_digimprint.go): ClearRemembered$ beside it clears it
			if !ps.is("ClearRemembered", "True") {
				return refuse("Cleanup ClearImprinted$ without ClearRemembered$")
			}
		}
		if lc.digImprintChosen && ps.is("ClearImprinted", "True") {
			// the imprint is the kernel's chosen set (lower_digimprint.go):
			// gorge's seek-found Imprint "clear" clears it
			ps.consume("ClearImprinted")
			lc.emit(lInstr{Op: "ClearChosen"})
		}
	case "ChooseCard":
		return lc.chooseCard(ps)
	case "ChoosePlayer":
		return lc.choosePlayer(ps) // lower_definedheads.go
	case "GenericChoice":
		return lc.genericChoice(ps)
	case "PeekAndReveal":
		return lc.peekAndReveal(ps)
	case "TwoPiles":
		return lc.twoPiles(ps)
	case "Play":
		return lc.play(ps)
	case "AddTurn":
		n, err := lc.amount(ps, "NumTurns", 1)
		if err != nil {
			return err
		}
		if !n.isConst() || n.N != 1 {
			return refuse("AddTurn %v", n)
		}
		if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
			return refuse("AddTurn Defined$ %q", d)
		}
		lc.emit(lInstr{Op: "AddTurn", Obj: lRef{Kind: rYou}})
	case "EndTurn":
		lc.emit(lInstr{Op: "EndTurn"})
	case "DamageResolve":
		lc.emit(lInstr{Op: "ResolveDamageMap", Note: "deal the damage map's queued damage at once"})
	case "Ward":
		uc := ps.str("UnlessCost")
		if m := wardEvidenceRe.FindStringSubmatch(uc); m != nil {
			// Ward—Collect evidence N (Axebane Ferox; gorge kw_ward.go
			// beginWardPayment's CollectEvidence arm: after "Pay", a
			// KChoose of 1..all of the payer's graveyard cards, zone
			// order; short of N mana value the ward counters)
			n, _ := strconv.Atoi(m[1])
			lc.emit(lInstr{Op: "Ward", CS: "CollectEvidence", A: cst(n), Obj: lRef{Kind: rTriggered}})
			break
		}
		if m := wardDiscManaRe.FindStringSubmatch(uc); m != nil {
			// Ward—discard a card or pay {N} (Titania, Rugged Rumbler; gorge
			// rules ward.go beginWardPayment's ">:" arm: after "Pay", one
			// KChoose of a hand card to discard or {N}, the {N} paid through
			// the ward mana window)
			n, _ := strconv.Atoi(m[1])
			lc.emit(lInstr{Op: "Ward", CS: "DiscardOrMana", A: cst(n), Obj: lRef{Kind: rTriggered}})
			break
		}
		if strings.HasPrefix(uc, "PayLife<X/") {
			// Ward—pay life equal to its power (Raubahn; gorge kw_ward.go
			// beginWardPayment: any PayLife<X/…> is life = the ward
			// permanent's power at payment, floored at 0, whatever X says)
			if x := lc.face.SVars["X"]; x != "Count$CardPower" {
				return refuse("ward life cost X %q", x)
			}
			lc.emit(lInstr{Op: "Ward", CS: "PayLifePower", A: cst(0), Obj: lRef{Kind: rTriggered}})
			break
		}
		if wi, ok, err := lc.wardPoison(uc); err != nil { // lower_costtokens.go
			return err
		} else if ok {
			lc.emit(wi)
			break
		}
		lc.wbOK = true
		cost, err := lc.cost(uc)
		lc.wbOK = false
		if err != nil {
			return err
		}
		if cost.Energy != 0 || cost.Return != nil {
			return refuse("ward energy / return cost")
		}
		lc.wardCost = true
		in, err := lc.payInstr(cost, lRef{Kind: rTriggeredPlayer}, false)
		lc.wardCost = false
		if err != nil {
			return err
		}
		if in.CS == "Blight" {
			return refuse("ward blight cost")
		}
		in.Op = "Ward"
		in.Obj = lRef{Kind: rTriggered}
		lc.emit(in)
	case "Repeat":
		return lc.repeat(ps)
	case "CopySpellAbility":
		return lc.copySpell(ps)
	case "Demonstrate":
		return lc.demonstrate(ps)
	case "ChooseColor":
		return lc.chooseColorPrevent(ps) // Prismatic Strands
	case "ChooseType":
		return lc.chooseType(ps)
	case "Scry":
		return lc.scry(ps)
	case "PutCounterAll":
		return lc.putCounterAll(ps)
	case "RemoveCounterAll":
		return lc.removeCounterAll(ps)
	case "UntapAll":
		return lc.untapAll(ps)
	case "TapAll":
		return lc.tapAll(ps)
	case "Seek":
		return lc.seek(ps)
	case "Reveal":
		return lc.reveal(ps)
	case "Amass":
		return lc.amass(ps)
	case "Empower":
		return lc.empower(ps) // lower_empower.go
	case "Incubate":
		return lc.incubate(ps)
	case "Manifest":
		return lc.manifest(ps)
	case "Cloak":
		return lc.cloak(ps)
	case "ManifestDread":
		return lc.manifestDread(ps)
	case "Proliferate":
		return lc.proliferate(ps)
	case "AlterAttribute":
		return lc.alterAttribute(ps) // lower_tappower.go
	case "ChangeSpeed":
		return lc.changeSpeed(ps) // lower_speed.go
	case "SetState":
		return lc.setState(ps) // lower_setstate.go
	case "Connive":
		return lc.connive(ps)
	case "FlipCoin":
		return lc.flipCoin(ps)
	case "RevealHand":
		return lc.revealHand(ps)
	case "Mana":
		return lc.manaEffect(ps)
	case "ElementalBend":
		return lc.elementalBend(ps)
	case "Branch":
		return lc.branch(ps)
	case "Regenerate":
		return lc.regenerate(ps)
	case "RepeatEach":
		return lc.repeatEachPlayers(ps)
	case "ClassLevelUp":
		return lc.classLevelUp(ps)
	case "Investigate":
		return lc.investigate(ps)
	case "AnimateAll":
		return lc.animateAll(ps)
	case "RollDice":
		return lc.rollDice(ps)
	case "TapOrUntap":
		return lc.tapOrUntap(ps)
	case "Debuff":
		return lc.debuff(ps)
	case "Explore":
		return lc.explore(ps)
	case "Discover":
		return lc.discover(ps)
	case "Hideaway":
		return lc.hideaway(ps) // lower_hideaway.go
	case "MoveCounter":
		return lc.moveCounter(ps)
	case "RearrangeTopOfLibrary":
		// gorge effRearrangeTopOfLibrary: the controller orders the top
		// NumCards$ of the one acting player's library (MayShuffle$ and
		// RearrangePlayer$ refuse; a constant count up to 8)
		if _, ok := ps.get("RearrangePlayer"); ok {
			return refuse("RearrangeTopOfLibrary RearrangePlayer$")
		}
		in := lInstr{Op: "Rearrange"}
		if ms, ok := ps.get("MayShuffle"); ok {
			if ms != "True" {
				return refuse("RearrangeTopOfLibrary MayShuffle %q", ms)
			}
			in.C = 1 // gorge rearrangeMayShuffleTape: a yes/no shuffle ask
		}
		n, err := lc.amount(ps, "NumCards", 1)
		if err != nil {
			return err
		}
		if !n.isConst() || n.N < 1 || n.N > 8 {
			return refuse("RearrangeTopOfLibrary NumCards %v", n)
		}
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		switch who.Kind {
		case rYou:
		case rTarget:
			if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N > 1 {
				return refuse("RearrangeTopOfLibrary of target slot %d", who.N)
			}
		default:
			return refuse("RearrangeTopOfLibrary of %s", who)
		}
		in.Obj, in.A = who, n
		lc.emit(in)
	case "LosesGame", "WinsGame":
		// gorge effLosesGame / effWinsGame (effects/dice.go): the Defined$
		// player (the controller when absent) loses / wins. A win is blocked
		// only by an "opponents can't win" replacement (kernel winGame).
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		switch who.Kind {
		case rYou, rEachOpp, rTriggeredPlayer:
		case rTarget:
			if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
				return refuse("%s of a non-player target", sa.API)
			}
		default:
			return refuse("%s Defined$ %q", sa.API, who)
		}
		in := lInstr{Op: "LoseGame", Obj: who}
		if sa.API == "WinsGame" {
			in.C = 1
		}
		lc.emit(in)
	case "Shuffle":
		// gorge effShuffle: the controller, or the one Defined$ / target
		// player (several players and object owners refuse)
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		switch who.Kind {
		case rYou, rTriggeredPlayer, rTargetController:
		case rTarget:
			if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N > 1 {
				return refuse("Shuffle of target slot %d", who.N)
			}
		default:
			return refuse("Shuffle of %s", who)
		}
		lc.emit(lInstr{Op: "Shuffle", Obj: who})
	case "SetLife":
		// gorge effSetLife: the one acting player's life becomes
		// LifeAmount$ (Redistribute$ and several players refuse)
		if _, ok := ps.get("Redistribute"); ok {
			return refuse("SetLife Redistribute$")
		}
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		switch who.Kind {
		case rYou, rTriggeredPlayer, rTargetController, rEachPlayer, rEachOpp:
		case rTarget:
			if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N > 1 {
				return refuse("SetLife of target slot %d", who.N)
			}
		default:
			return refuse("SetLife of %s", who)
		}
		n, err := lc.amount(ps, "LifeAmount", 0)
		if err != nil {
			return err
		}
		if _, ok := ps.m["LifeAmount"]; !ok {
			return refuse("SetLife without LifeAmount$")
		}
		lc.emit(lInstr{Op: "SetLife", Obj: who, A: n})
	case "Fog":
		// gorge effFog: a turn-long restriction the combat damage passes
		// read (fogActive); Duration$ refuses
		if _, ok := ps.get("Duration"); ok {
			return refuse("Fog Duration$")
		}
		lc.emit(lInstr{Op: "Fog"})
	case "Poison":
		// gorge effPoison: each Defined$ player gets Num$ poison counters
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		switch who.Kind {
		case rYou, rEachOpp, rEachPlayer, rTriggeredPlayer, rDefending, rTargetController:
		case rTarget:
			if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
				return refuse("Poison of a non-player target")
			}
		default:
			return refuse("Poison of %s", who)
		}
		n, err := lc.amount(ps, "Num", 1)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "Poison", Obj: who, A: n})
	case "Blight":
		// gorge effBlight (CR 701.68): each Defined$ player chooses a
		// creature they control and puts Num$ -1/-1 counters on it; the
		// ValidTgts$ fallback makes a targeted opponent the blighter.
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		switch who.Kind {
		case rYou, rEachOpp, rTriggeredPlayer, rDefending, rTargetController:
		case rTarget:
			if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
				return refuse("Blight of a non-player target")
			}
		default:
			return refuse("Blight of %s", who)
		}
		n, err := lc.amount(ps, "Num", 1)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "Blight", Obj: who, A: n})
	case "AddPhase":
		return lc.addPhase(ps) // lower_addphase.go
	case "ChangeTargets":
		return lc.changeTargets(ps) // lower_stackrewrite.go
	case "UnlockDoor":
		return lc.unlockDoor(ps) // lower_unlockdoor.go
	default:
		return refuse("api %s", sa.API)
	}
	return nil
}

// amount reads an amount param (default dflt when absent).
func (lc *lowerCtx) amount(ps *params, key string, dflt int) (lValue, error) {
	v, ok := ps.get(key)
	if !ok {
		return cst(dflt), nil
	}
	return lc.value(v)
}

func (lc *lowerCtx) dealDamage(ps *params) error {
	var (
		n      lValue
		dmgSrc lRef
		obj    lRef
		err    error
	)
	// a node with its own ValidTgts$ declares the slot first: its amount's
	// Targeted$ and its DamageSource$ name that slot (gorge c.Targets)
	own := ownTargetFirst(ps)
	if own {
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	if n, err = lc.amount(ps, "NumDmg", 0); err != nil {
		return err
	}
	if s, ok := ps.get("DamageSource"); ok {
		if dmgSrc, err = lc.defined(s); err != nil {
			return err
		}
	}
	if !own {
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	in := lInstr{Op: "DealDamage", Obj: obj, A: n}
	if dmgSrc.Kind != rNone {
		if ps.is("DamageMap", "True") {
			// the kernel flushes no damage map (Self-Destruct's source = target hit
			// plus a second node's "any other target" that may be a player)
			return refuse("DamageMap$ True with DamageSource$: the kernel has no damage-map flush")
		}
		if obj.Kind == rEachOpp {
			return lc.dmgSrcPlayers(ps, dmgSrc, obj, n) // lower_damagesourceplayers.go
		}
		if obj.Kind == rTarget {
			// Bite / DealDamageFrom hit permanent slots only (hitBatch); a
			// player the slot may name is offered by the kernel, then dropped
			if f := lc.ab.Targets[obj.N].F; f != nil && f.Players != 0 {
				if dmgSrc.Kind == rTriggered {
					return lc.dmgSrcAnyTarget(ps, dmgSrc, obj, n) // lower_damagesourceplayers.go
				}
				return refuse("DamageSource$ to a target slot that may name a player")
			}
		}
		// "Target creature you control deals damage equal to its power":
		// the kernel's Bite (one-sided fight).
		if n.Kind != vPowerOf || n.Ref != dmgSrc || n.Mul != 0 || n.Add != 0 || n.Neg {
			if obj.Kind != rTarget || dmgSrc.Kind != rTarget {
				return refuse("DamageSource$ with an amount other than the source's power")
			}
			lc.emit(lInstr{Op: "DealDamageFrom", Obj: dmgSrc, A: n, C: obj.N, Note: "source deals A"})
			return nil
		}
		if obj.Kind != rTarget {
			// the kernel's Bite names one object target slot; a source's
			// power dealt to a player or each opponent has no lowering
			return refuse("DamageSource$ to %s", obj)
		}
		in = lInstr{Op: "Bite", Obj: dmgSrc, A: lValue{}, Note: "source deals its power"}
		in.B = lValue{Kind: vNone}
		in.CS = "to=" + obj.String()
		in.C = obj.N
	}
	if d, ok := ps.get("DividedAsYouChoose"); ok {
		dv, err := lc.value(d)
		if err != nil {
			return err
		}
		if obj.Kind != rTarget {
			return refuse("divided damage without targets")
		}
		if dv.isConst() {
			lc.ab.Targets[obj.N].Divided = dv.N
		} else if sameValue(dv, lc.ab.Targets[obj.N].Max) {
			// A value total (Ureni's land count) is read where the ability is
			// put on the stack, exactly where gorge's divisionTotalFor reads
			// DividedAsYouChoose$ (CR 601.2d, 603.3d). The slot's TargetMax$
			// must be that same value: gorge clamps a divided slot's target
			// count to the announced total, and the kernel does not.
			lc.ab.Targets[obj.N].Divided = kop.DivValue
			lc.ab.Targets[obj.N].DividedV = dv
		} else {
			return refuse("divided over a value total that is not TargetMax$")
		}
		in.Op = "DealDamageDivided"
	}
	if de, ok := ps.get("DivideEvenly"); ok {
		if err := lc.divideEvenly(ps, de, &in); err != nil { // lower_divideevenly.go
			return err
		}
	}
	if ps.is("DamageMap", "True") {
		in.Note = "damage map: dealt together with the following DealDamage nodes"
	}
	if es, ok := ps.get("ExcessSVar"); ok {
		// "" and Card.targetedBy both hold for the damaged target (gorge
		// excessConditionHolds); anything else refuses
		if c := ps.str("ExcessSVarCondition"); c != "" && c != "Card.targetedBy" {
			return refuse("ExcessSVar condition")
		}
		if err := lc.dealExcess(obj, dmgSrc); err != nil {
			return err
		}
		in.CS = "TrackExcess"
		lc.excess = strings.TrimSpace(es)
	}
	if rd, ok := ps.get("ReplaceDyingDefined"); ok {
		// Remembered is the set this same node damaged (gorge
		// registerReplaceDying builds its ids from the damaged slice)
		if rd != "Targeted" && !(rd == "Remembered" && ps.is("RememberDamaged", "True")) {
			return refuse("ReplaceDyingDefined %q", rd)
		}
		in.CS = "ExileIfDiesThisTurn"
		if rd == "Remembered" {
			in.CS += "|ExileIfDealt" // only a recipient actually dealt damage (gorge damage.go)
		}
	}
	if rd, ok := ps.get("RememberDamaged"); ok {
		if err := lc.rememberDamaged(rd, &in); err != nil {
			return err
		}
	}
	if err := lc.damageDecider(ps, &in); err != nil { // lower_optional.go
		return err
	}
	lc.emit(in)
	return nil
}

// rememberDamaged reads DealDamage RememberDamaged$ (gorge effDealDamage:
// each object target on the battlefield is remembered when damage above 0 is
// actually dealt to it, CR 120.1; a player target is remembered as a
// player). The kernel's remembered set
// holds objects only (kop.DmgRemember), so a later node that can observe a
// remembered player refuses when the damage can go to a player.
func (lc *lowerCtx) rememberDamaged(rd string, in *lInstr) error {
	if rd != "True" {
		return refuse("RememberDamaged$ %q", rd)
	}
	if in.Op != "DealDamage" || strings.HasPrefix(in.Note, "damage map") {
		return refuse("RememberDamaged$ on a %s", in.Op)
	}
	admitsPlayer := in.Obj.Kind != rTarget
	if in.Obj.Kind == rTarget {
		f := lc.ab.Targets[in.Obj.N].F
		admitsPlayer = f == nil || f.Players != 0
	}
	if admitsPlayer && lc.restObservesRememberedPlayer() {
		return refuse("RememberDamaged$ with a remembered player observed")
	}
	in.CS = strings.TrimPrefix(in.CS+"|Remember", "|")
	return nil
}

// restObservesRememberedPlayer reports whether a later node of the chain
// could tell a remembered player apart from none: a Player-based
// ConditionPresent$, Defined$ Player.IsRemembered, or a count of the set
// (Remembered$Amount, Count$RememberedSize), an SVar's body included.
func (lc *lowerCtx) restObservesRememberedPlayer() bool {
	if lc.curNode == nil {
		return false
	}
	reads := func(v string) bool {
		return strings.Contains(v, "Player.IsRemembered") || strings.Contains(v, "Remembered$Amount") ||
			strings.Contains(v, "RememberedSize")
	}
	for n := lc.curNode.Sub; n != nil; n = n.Sub {
		for k, v := range n.Params {
			if k == "ConditionPresent" && strings.Contains(v, "Player") || reads(v) || reads(lc.face.SVars[strings.TrimSpace(v)]) {
				return true
			}
		}
	}
	return false
}

func (lc *lowerCtx) damageAll(ps *params) error {
	n, err := lc.amount(ps, "NumDmg", 0)
	if err != nil {
		return err
	}
	if ds, ok := ps.get("DamageSource"); ok {
		return lc.sweepFrom(ps, ds, n) // lower_sweepfrom.go
	}
	if vp := ps.str("ValidPlayers"); vp == "OppNonTriggeredTarget" && ps.m["ValidCards"] == "" {
		// every opponent but the damaged one (gorge validPlayers): in a
		// two-player game the damaged player is the one opponent, so no
		// player is left to hit (CR 102.2)
		if !lc.damagedPlayerIsTriggerPlayer() {
			return refuse("ValidPlayers$ OppNonTriggeredTarget outside a damage-to-player trigger")
		}
		lc.twoSeat("OppNonTriggeredTarget read as nobody")
		ps.consume("ValidPlayers")
		return nil
	}
	in := lInstr{Op: "DamageAll", A: n}
	if vc, ok := ps.get("ValidCards"); ok {
		if in.F, err = lc.filter(vc); err != nil {
			return err
		}
	}
	if vp, ok := ps.get("ValidPlayers"); ok {
		pf, err := lc.filter(vp)
		if err != nil {
			return err
		}
		if in.F == nil {
			in.F = &lFilter{}
		}
		in.F.Players |= pf.Players
	}
	if in.F == nil {
		return refuse("DamageAll with no filter")
	}
	lc.emit(in)
	return nil
}

// sacrificeOptional reads Sacrifice's Optional$ (gorge effSacrifice: a
// non-strict optional asks 0..min(Amount, eligible) over the matching
// permanents; an object target's ask is the same 0..1 shape). Any value
// but True refuses, and the Optional$+StrictAmount$ disjoint
// {0, Amount} shape (gorge's KModes yes/no then an exact batch) refuses:
// the corpus carries it on one card, whose follow-up token lines (its
// RememberSacrificed$ conditions) refuse anyway.
func (lc *lowerCtx) sacrificeOptional(ps *params, in *lInstr) error {
	v, ok := ps.get("Optional")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("Optional$ %q on a sacrifice", v)
	}
	if sa, ok := ps.get("StrictAmount"); ok {
		return refuse("Optional$ with StrictAmount$ %q on a sacrifice", sa)
	}
	in.C |= int(kop.OptAsk)
	return nil
}

func (lc *lowerCtx) discard(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Discard", Obj: who, A: n}
	// Optional$ True (gorge discardBounds: the pick's min lowers to 0; the
	// TgtChoose arm poses a may-election first, chooseMode's Min 0 pick
	// needs none). Any value but True refuses. AnyNumber$ rides the
	// same bounds (Min 0, Max the eligible count, no election); the corpus
	// population is Optional$-only, so AnyNumber$ without Optional$ keeps
	// refusing.
	if v, ok := ps.get("Optional"); ok {
		if v != "True" {
			return refuse("Optional$ %q on a discard", v)
		}
		in.C |= int(kop.OptAsk)
		if an, ok := ps.get("AnyNumber"); ok {
			if an != "True" {
				return refuse("AnyNumber$ %q on a discard", an)
			}
			in.C |= int(kop.OptAny)
		}
	}
	switch m := ps.str("Mode"); m {
	case "TgtChoose", "":
		in.CS = "DiscarderChooses"
	case "RevealYouChoose", "LookYouChoose", "YouChoose":
		// gorge effDiscard chooseMode: the three are one path (the
		// controller picks from the discarder's hand; reveal vs look is
		// information only)
		in.CS = "YouChoose"
	case "Hand":
		// gorge effDiscard Mode$ Hand: the whole hand in hand order, no
		// ask, NumCards$ never read (the corpus carries none). Optional$
		// True is a per-player may-election whose "yes" discards the whole
		// hand (effDiscardHand); the corpus population is each-player only,
		// whose per-player ask the kernel does not pose -- refused below.
		if _, ok := ps.get("NumCards"); ok {
			return refuse("discard mode Hand with NumCards")
		}
		if in.C&int(kop.OptAsk) != 0 && who.Kind != rYou {
			return refuse("Optional$ on a Hand discard of a non-You player (each-player election)")
		}
		in.A = cst(127)
		in.CS = "DiscarderChooses"
		if in.C&int(kop.OptAsk) != 0 {
			in.C |= int(kop.OptAll) // the whole-hand election discards every card
		}
	case "RevealDiscardAll":
		// gorge effDiscardRevealDiscardAll: every card DiscardValid$
		// allows, hand order, no ask (the kernel's filtered discard with
		// N >= hand size discards every match without an ask)
		if _, ok := ps.get("NumCards"); ok {
			return refuse("discard mode RevealDiscardAll with NumCards")
		}
		if _, ok := ps.get("DiscardValid"); !ok {
			return refuse("discard mode RevealDiscardAll without DiscardValid")
		}
		in.A = cst(127)
		in.CS = "DiscarderChooses"
	case "Random":
		// gorge effDiscardRandom (CR 701.8b): the engine RNG picks
		// NumCards$ cards from the whole hand, no ask. The kernel's
		// ChooseRandom arm reads the whole hand and poses no election, so
		// any rider that would narrow the eligible set or ask first
		// refuses; an each-player Defined$ has no kernel path for
		// ChooseRandom (interp.go's each-player branch is ChooseSelf only).
		if _, ok := ps.get("Optional"); ok {
			return refuse("discard mode Random with Optional$")
		}
		if _, ok := ps.get("AnyNumber"); ok {
			return refuse("discard mode Random with AnyNumber$")
		}
		if _, ok := ps.get("DiscardValid"); ok {
			return refuse("discard mode Random with DiscardValid$")
		}
		if who.Kind == rEachPlayer {
			return refuse("discard mode Random of an each-player Defined$")
		}
		in.CS = "Random"
	default:
		return refuse("discard mode %q", m)
	}
	if dv, ok := ps.get("DiscardValid"); ok {
		if in.F, err = lc.filter(dv); err != nil {
			return err
		}
		// gorge discardEligible (cardflow.go:442) matches DiscardValid$ with
		// matchesInOwnZone (filter.go:3382): a bare Permanent base is a
		// permanent card in hand (CR 110.4a), the kernel's
		// zone-scoped filter.
	}
	if who.Kind == rEachPlayer {
		// the kernel expands the unfiltered, non-optional, self-choosing
		// each-player discard player by player (act_eachdiscard.go);
		// a filtered, optional or controller-chooses shape still resolves
		// one player only
		if in.F != nil {
			return refuse("each-player discard with a filter (DiscardValid$)")
		}
		if in.C&int(kop.OptAsk) != 0 {
			return refuse("each-player optional discard")
		}
		if in.CS != "DiscarderChooses" {
			return refuse("each-player discard chooser %q", in.CS)
		}
	}
	if err := rememberDiscarded(ps, &in); err != nil {
		return err
	}
	if err := lc.discardUnless(ps, &in, who); err != nil { // lower_handmove.go
		return err
	}
	lc.emit(in)
	return nil
}

// chooseType lowers a mid-resolution ChooseType$ ("choose a creature
// type": gorge effects/choose.go effChooseType asks KChoose over the
// creature-type vocabulary). The answer is the resolution's chosen subtype,
// which a later filter's Creature.ChosenType reads (Kindred Charge's copy
// filter).
func (lc *lowerCtx) chooseType(ps *params) error {
	if ps.str("Type") == "Card" {
		return lc.chooseCardType(ps) // lower_choosetype.go
	}
	if ps.str("Defined") != "You" || ps.str("Type") != "Creature" {
		return refuse("ChooseType shape")
	}
	lc.emit(lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}})
	return nil
}

func (lc *lowerCtx) pump(ps *params) error {
	zone := zBattlefield
	if z, ok := ps.get("PumpZone"); ok {
		zone = zoneNames[z]
	}
	obj, err := lc.objOrTarget(ps, "Self", zone)
	if err != nil {
		return err
	}
	if kwc, ok := ps.get("KWChoice"); ok {
		return lc.pumpChoice(ps, obj, zone, kwc)
	}
	if _, ok := ps.get("ReplaceDyingDefined"); ok {
		// the mark follows the pump's own events (lower_exileexmark.go); a
		// Duration$ grant is not a pump of the target's characteristics
		if _, d := ps.get("Duration"); d || zone != zBattlefield {
			return refuse("ReplaceDyingDefined$ beside Duration$ or PumpZone$")
		}
	}
	if err := lc.pumpBody(ps, obj, zone, nil); err != nil {
		return err
	}
	return lc.replaceDyingMark(ps, obj, lRef{})
}

// pumpChoice lowers a Pump with KWChoice$ ("gains your choice of X or Y":
// gorge pump.go asks the pump's controller KModes first, then registers the
// pump with the chosen keyword). OpChooseBranch's chooser is the
// controller, not the pumped target; each branch is one candidate's full
// pump body. A fixed KW$ beside KWChoice$ and a duration other than the
// end-of-turn default refuse (the kernel's grant is until end of turn).
func (lc *lowerCtx) pumpChoice(ps *params, obj lRef, zone int, kwc string) error {
	if _, ok := ps.get("KW"); ok {
		return refuse("Pump with KW$ and KWChoice$")
	}
	if d, ok := ps.get("Duration"); ok && d != "" && d != "EndOfTurn" {
		return refuse("Pump KWChoice Duration %q", d)
	}
	choices := strings.Split(kwc, ",")
	if len(choices) < 1 {
		return refuse("Pump KWChoice with %d choices", len(choices))
	}
	kws := make([][]string, len(choices))
	for i, name := range choices {
		ks, err := pumpKeywords(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		kws[i] = ks
	}
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rYou}, A: cst(len(choices))})
	for i, ks := range kws {
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		if err := lc.pumpBody(ps, obj, zone, ks); err != nil {
			return err
		}
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	}
	return nil
}

// pumpObjectRef reports whether a Pump's subject is a battlefield object
// rather than a player. The keyword override record applies to a permanent;
// a player-scoped grant (`Override obj=You`, The One Ring and The Stasis
// Coffin's "you gain protection from everything") has no kernel record and
// must refuse instead of lowering to a no-op. A target slot must be an
// object filter; a multi-target union is not a single override subject.
func (lc *lowerCtx) pumpObjectRef(obj lRef) bool {
	switch obj.Kind {
	case rSelf, rTriggered, rTriggeredNew, rTriggeredSource, rEquipped, rRemembered,
		rDelayedRemembered, rReplaced, rChosen, rEach, rSacrificed, rHost, rDiscarded:
		return true
	case rTarget:
		t := lc.ab.Targets[obj.N]
		return t.F != nil && t.F.Players == 0 && len(t.F.Clauses) != 0
	}
	return false
}

// pumpBody emits one Pump's effects for the already-resolved subject obj.
// extra is the keyword a KWChoice$ branch chose (nil for a plain Pump).
func (lc *lowerCtx) pumpBody(ps *params, obj lRef, zone int, extra []string) error {
	if v, ok := ps.get("AtEOT"); ok {
		// gorge pump.go -> scheduleAtEOT over the pumped battlefield objects,
		// after the pump
		act, err := atEOTAct(v)
		if err != nil {
			return err
		}
		defer lc.emit(lInstr{Op: "AtEOT", Obj: obj, C: act})
	}
	if done, err := lc.skipNextUntap(ps, obj, zone); done || err != nil {
		return err
	}
	if d, ok := ps.get("Duration"); ok {
		// Duration$ keyword grants only: a layer-6 permanent grant (gorge
		// AddContinuous Permanent) or one lasting until the controller's
		// next turn (the UntilTurn boundary, CR 611.2b), both the kernel's
		// override record; a P/T change or any other duration refuses
		_, na := ps.m["NumAtt"]
		_, nd := ps.m["NumDef"]
		kw, hasKW := ps.get("KW")
		if d == "UntilYourNextTurn" && lc.pumpObjectRef(obj) && !na && !nd && hasKW && zone == zBattlefield {
			// gorge's durationTiming gives a next-turn Pump the UntilTurn
			// boundary AddContinuous computes: it ends at the cleanup
			// before that controller's next turn, which the kernel's ovNT
			// record already models. The Undercity's Throne
			// (lower_initiative.go, "it gains hexproof until your next turn")
			// is one such grant; every keyword-only Pump line with this
			// duration lowers the same way. A P/T pump still refuses: it
			// has no layer-7c record.
			ks, err := pumpKeywords(kw)
			if err != nil {
				return err
			}
			lc.emitOverride(obj, &lOverride{AddKw: ks}, "UntilYourNextTurn")
			return nil
		}
		if d != "Permanent" || na || nd || !hasKW || zone != zBattlefield {
			return refuse("Pump Duration %q", d)
		}
		ks, err := pumpKeywords(kw)
		if err != nil {
			return err
		}
		lc.emitOverride(obj, &lOverride{AddKw: ks}, "Permanent")
		return nil
	}
	att, def := lValue{}, lValue{}
	if a, ok := ps.get("NumAtt"); ok {
		if a == "Double" {
			att = lValue{Kind: vPowerOf, Ref: obj}
		} else if a == "Triple" {
			// gorge statAddMultiplier: Triple adds twice the object's
			// current power (final 3x)
			att = lValue{Kind: vPowerOf, Ref: obj, Mul: 2}
		} else if v, err := lc.value(a); err != nil {
			return err
		} else {
			att = v
		}
	}
	if d, ok := ps.get("NumDef"); ok {
		if d == "Double" {
			// gorge NumForObject: the object's current toughness
			def = lValue{Kind: vToughnessOf, Ref: obj}
		} else if d == "Triple" {
			def = lValue{Kind: vToughnessOf, Ref: obj, Mul: 2}
		} else if v, err := lc.value(d); err != nil {
			return err
		} else {
			def = v
		}
	}
	if att.Kind != vNone || def.Kind != vNone {
		if att.Kind == vNone {
			att = cst(0)
		}
		if def.Kind == vNone {
			def = cst(0)
		}
		lc.emit(lInstr{Op: "PumpUntilEOT", Obj: obj, A: att, B: def})
	}
	kw, hasKW := ps.get("KW")
	mustBlock := false
	if hasKW {
		kw, mustBlock = splitMustBeBlocked(kw)
	}
	if hasKW && kw != "" {
		rest, fbs, err := splitFirebendingGrant(kw)
		if err != nil {
			return err
		}
		if len(fbs) > 0 && zone != zBattlefield {
			return refuse("granted Firebending to a non-battlefield object")
		}
		if rest != "" || len(fbs) == 0 {
			ks, err := pumpKeywords(rest)
			if err != nil {
				return err
			}
			lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: strings.Join(ks, "&")})
		}
		for _, n := range fbs {
			// CR 702.189a granted until end of turn: the latent Attacks
			// trigger rides an end-of-turn override record; identical
			// applications stack (lower_firegrant.go)
			idx, err := lc.firebendGrantTrigger(n)
			if err != nil {
				return err
			}
			lc.emitOverride(obj, &lOverride{GrantAbility: []int{idx}}, "EOT")
		}
	}
	if len(extra) > 0 {
		lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: strings.Join(extra, "&")})
	}
	if mustBlock {
		if zone != zBattlefield || !lc.pumpObjectRef(obj) {
			return refuse("Pump KW$ must be blocked if able on a non-permanent subject")
		}
		lc.emit(lInstr{Op: "MustBeBlocked", Obj: obj})
	}
	return nil
}

// mustBeBlockedKW is the one Pump / PumpAll keyword sentence that is a mark,
// not a keyword bit (CR 509.1c, ExtMustBeBlocked).
const mustBeBlockedKW = "HIDDEN CARDNAME must be blocked if able."

// splitMustBeBlocked takes mustBeBlockedKW out of a KW$ list: the rest of
// the list (possibly empty) and whether the sentence was there. Any other
// spelling stays in the list, where pumpKeywords refuses it.
func splitMustBeBlocked(kw string) (rest string, found bool) {
	var keep []string
	for _, k := range strings.Split(kw, "&") {
		if strings.TrimSpace(k) == mustBeBlockedKW {
			found = true
			continue
		}
		keep = append(keep, strings.TrimSpace(k))
	}
	return strings.Join(keep, " & "), found
}

// pumpKeywords lowers a KW$ list ("Flying & Double Strike").
func pumpKeywords(kw string) ([]string, error) {
	var out []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		switch {
		case simpleKeywords[k]:
			out = append(out, k)
		case k == "HIDDEN CARDNAME can't block.":
			out = append(out, "CantBlock")
		case k == "Prevent all combat damage that would be dealt to CARDNAME.":
			out = append(out, "PreventCombatDamageTo")
		case k == "Wither":
			out = append(out, "Wither") // CR 702.80 granted; not in simpleKeywords (that map also drives the printed K: switch)
		case k == "Flashback":
			out = append(out, "FlashbackOwnCost")
		case strings.HasPrefix(k, "Protection from "):
			c := strings.TrimPrefix(k, "Protection from ")
			if i := strings.Index("whiteblueblackredgreen", c); i < 0 || !(c == "white" || c == "blue" || c == "black" || c == "red" || c == "green") {
				return nil, refuse("keyword %q", k)
			}
			out = append(out, "ProtectionFrom"+strings.ToUpper(c[:1])+c[1:])
		default:
			return nil, refuse("keyword %q", k)
		}
	}
	return out, nil
}

func (lc *lowerCtx) pumpAll(ps *params) error {
	f, err := lc.filter(ps.str("ValidCards"))
	if err != nil {
		return err
	}
	if kw, ok := ps.m["KW"]; ok && strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(kw), "HIDDEN ")) == skipNextUntapKW {
		return lc.skipNextUntapAll(ps, f) // Sleep
	}
	if dur, ok := ps.get("Duration"); ok {
		return lc.pumpAllDuration(ps, f, dur)
	}
	var scope lRef
	if vt, ok := ps.get("ValidTgts"); ok {
		// "creatures target player controls" (gorge targetPlayerKindScope:
		// only the targeted player's permanents; an unresolvable scope
		// sweeps nothing): the player target slot rides Obj
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if len(pf.Clauses) != 0 || pf.Players == 0 {
			return refuse("PumpAll ValidTgts$ %q: not a player target", vt)
		}
		scope = lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf})}
	}
	if err := lc.pumpAllRemember(ps, f, scope); err != nil { // lower_rememberwritersmass.go
		return err
	}
	att, def := cst(0), cst(0)
	if a, ok := ps.get("NumAtt"); ok {
		if att, err = lc.value(a); err != nil {
			return err
		}
	}
	if d, ok := ps.get("NumDef"); ok {
		if def, err = lc.value(d); err != nil {
			return err
		}
	}
	in := lInstr{Op: "PumpAll", F: f, A: att, B: def, Obj: scope}
	if scope.Kind != rNone {
		in.Note = "only the target player's permanents"
	}
	mustBlock := false
	if kw, ok := ps.get("KW"); ok {
		kw, mustBlock = splitMustBeBlocked(kw)
		if mustBlock && scope.Kind != rNone {
			return refuse("PumpAll KW$ must be blocked if able with a ValidTgts$ player scope")
		}
		if kw != "" {
			ks, err := pumpKeywords(kw)
			if err != nil {
				return err
			}
			in.CS = strings.Join(ks, "&")
		}
	}
	_, hasAtt := ps.m["NumAtt"]
	_, hasDef := ps.m["NumDef"]
	if !mustBlock || in.CS != "" || hasAtt || hasDef {
		lc.emit(in)
	}
	if mustBlock {
		lc.emit(lInstr{Op: "MustBeBlocked", F: f}) // every permanent matching ValidCards$
	}
	return nil
}

// pumpAllDuration lowers PumpAll Duration$ UntilYourNextTurn as an
// OpOverrideAll over every permanent matching ValidCards$, as gorge does
// (CR 611.2c): AddKw for a keyword grant, PTDelta for a NumAtt$/NumDef$
// pump (a constant, read once; the kernel adds it after counters until the
// effect owner's next turn). Any other Duration$ refuses.
func (lc *lowerCtx) pumpAllDuration(ps *params, f *lFilter, dur string) error {
	if dur != "UntilYourNextTurn" {
		return refuse("PumpAll Duration$ %q", dur)
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("PumpAll Duration$ UntilYourNextTurn with ValidTgts$")
	}
	o := &lOverride{}
	a, hasA := ps.get("NumAtt")
	d, hasD := ps.get("NumDef")
	if hasA || hasD {
		att, def := cst(0), cst(0)
		var err error
		if hasA {
			if att, err = lc.value(a); err != nil {
				return err
			}
		}
		if hasD {
			if def, err = lc.value(d); err != nil {
				return err
			}
		}
		if !att.isConst() || !def.isConst() {
			return refuse("PumpAll Duration$ UntilYourNextTurn with a computed P/T pump")
		}
		o.PTDelta, o.PVal, o.TVal = true, att, def
	}
	if kw, ok := ps.get("KW"); ok {
		ks, err := pumpKeywords(kw)
		if err != nil {
			return err
		}
		o.AddKw = ks
	} else if !o.PTDelta {
		return refuse("PumpAll Duration$ UntilYourNextTurn without KW$")
	}
	in := lInstr{Op: "OverrideAll", F: f, CS: "UntilYourNextTurn", Note: overrideString(o)}
	lc.out.overrides = append(lc.out.overrides, o)
	in.C = len(lc.out.overrides) - 1
	lc.emit(in)
	return nil
}

// earthbend lowers Earthbend N (CR 701.66; gorge effEarthbend). gorge's
// parse step writes ValidTgts$ Land.YouCtrl into every Earthbend without
// one (normalizeImplicitTarget), which the IR already carries; any other
// ValidTgts$ is not that shape.
func (lc *lowerCtx) earthbend(ps *params) error {
	if v, ok := ps.m["ValidTgts"]; !ok {
		ps.m["ValidTgts"] = "Land.YouCtrl"
	} else if v != "Land.YouCtrl" {
		return refuse("Earthbend ValidTgts$ %q", v)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Num", 0)
	if err != nil {
		return err
	}
	ks, err := pumpKeywords("Haste")
	if err != nil {
		return err
	}
	// a 0/0 Creature land with haste for as long as it stays (gorge's
	// animateGrant, DurationPermanent, endOnLeave)
	o := &lOverride{AddTypes: typeBits["Creature"], HasSetPT: true, AddKw: ks, Dedup: true}
	lc.out.overrides = append(lc.out.overrides, o)
	lc.emit(lInstr{Op: "Earthbend", Obj: obj, A: n, C: len(lc.out.overrides) - 1})
	return nil
}

func (lc *lowerCtx) putCounter(ps *params) error {
	if lc.digImprintFoundMode {
		return lc.digImprintFoundPut(ps) // lower_digimprint.go
	}
	if _, ok := ps.m["DividedAsYouChoose"]; ok {
		if ok, err := lc.dividedCountersN(ps); ok || err != nil {
			return err
		}
		if ok, err := lc.dividedCountersAnyX(ps); ok || err != nil {
			return err
		}
		if ok, err := lc.dividedCountersPick(ps); ok || err != nil { // (Vastwood Hydra)
			return err
		}
		return lc.dividedCountersLKI(ps) // (Quirion Beastcaller)
	}
	if ps.is("ETB", "True") {
		return refuse("ETB counters outside a replacement")
	}
	if _, ok := ps.m["Bolster"]; ok {
		return lc.bolster(ps)
	}
	if _, ok := ps.m["Support"]; ok {
		return lc.support(ps)
	}
	if _, ok := ps.m["Choices"]; ok {
		return lc.putCounterChoices(ps)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if _, ok := ps.m["Adapt"]; ok {
		return lc.adapt(ps, obj)
	}
	if mono, ok := ps.get("Monstrosity"); ok {
		return lc.monstrosity(ps, obj, mono)
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	ct := ps.str("CounterType")
	cts := ps.str("CounterTypes")
	if cts != "" {
		if ct != "" {
			return refuse("PutCounter with both CounterType$ %q and CounterTypes$ %q", ct, cts)
		}
		return lc.putCounterKinds(obj, n, cts)
	}
	if ct == "" {
		return refuse("PutCounter without CounterType")
	}
	if ct == "ENERGY" {
		return lc.gainEnergy(obj, n)
	}
	if ct == "EachFromSource" {
		return lc.putCounterEachFromSource(ps, obj)
	}
	if ct == "ExistingCounter" {
		return lc.putCounterExisting(ps, obj, n)
	}
	in := lInstr{Op: "PutCounter", Obj: obj, A: n, CS: ct}
	if err := putCounterRememberPut(ps, &in); err != nil { // lower_putcounterparams.go
		return err
	}
	if err := putCounterRememberAmount(ps, &in); err != nil { // lower_remembered.go
		return err
	}
	lc.emit(in)
	return nil
}

// putCounterKinds lowers PutCounter's CounterTypes$ (plural) -- a DIFFERENT
// key from CounterType$ (gorge putcounter_params.go): one counter of each
// listed kind, repeats accumulating, each of CounterNum$ (default 1). Gorge
// emits one CounterChange per listed kind, in list order, per recipient
// (engine/effects/counters_put.go's OneOfEach arm); each kernel PutCounter
// instruction already applies its own replacement and counter-added
// triggers, so kgen emits one PutCounter per entry in list order. An entry
// gorge would keep in SpecialTypes (contains '_', ChosenFromList) or a kind
// the kernel's counter table does not know refuses, loudly.
func (lc *lowerCtx) putCounterKinds(obj lRef, n lValue, cts string) error {
	if obj.Kind == rRemembered {
		return refuse("PutCounter CounterTypes$ on the remembered set")
	}
	var kinds []string
	for _, k := range strings.Split(cts, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue // gorge splitCounterKinds drops empty entries
		}
		if strings.Contains(k, "_") {
			return refuse("PutCounter CounterTypes$ special entry %q", k)
		}
		if strings.EqualFold(k, "ChosenFromList") {
			return refuse("PutCounter CounterTypes$ ChosenFromList")
		}
		if _, err := counterKind(k); err != nil {
			return refuse("PutCounter CounterTypes$ kind %q: %v", k, err)
		}
		kinds = append(kinds, k)
	}
	for i, k := range kinds {
		in := lInstr{Op: "PutCounter", Obj: obj, A: n, CS: k}
		// one put action, one CounterPlayerAddedAll trigger per recipient
		// (gorge's counterBatch latch; kop.PutCtrCellFirst)
		if len(kinds) > 1 {
			in.B = cst(kop.PutCtrCellNext)
			if i == 0 {
				in.B = cst(kop.PutCtrCellFirst)
			}
		}
		lc.emit(in)
	}
	return nil
}

// adapt lowers Adapt$ N (CR 702.35a, gorge effPutCounter's AdaptSet arm):
// N +1/+1 counters on the source if it has none, read at resolution; an AB$
// root also gets gorge adaptGateOK's offer gate (activate only while the
// source has no +1/+1 counter). Only the source itself, the corpus's only
// recipient: a Defined$/ValidTgts$ recipient or a CounterNum$/CounterType$
// beside it refuses.
func (lc *lowerCtx) adapt(ps *params, obj lRef) error {
	if obj.Kind != rSelf {
		return refuse("Adapt$ recipient %v", obj)
	}
	if _, ok := ps.m["CounterNum"]; ok {
		return refuse("Adapt$ with CounterNum$")
	}
	if _, ok := ps.m["CounterType"]; ok {
		return refuse("Adapt$ with CounterType$")
	}
	n, err := lc.amount(ps, "Adapt", 1)
	if err != nil {
		return err
	}
	f, err := lc.filter("Card.Self+counters_GE1_P1P1")
	if err != nil {
		return err
	}
	none := lCond{Kind: cPresent, F: f, Op: opEQ, A: cst(0)}
	if lc.head && lc.depth == 1 && lc.ab.Kind == akActivated && lc.ab.Trigger == nil {
		if lc.ab.ActCond.Kind != cNone {
			return refuse("two activation conditions")
		}
		lc.ab.ActCond = none
	}
	return lc.guardBlock(none, func() error {
		lc.emit(lInstr{Op: "PutCounter", Obj: obj, A: n, CS: "P1P1"})
		return nil
	})
}

// monstrosity lowers Monstrosity$ N (CR 701.37a; gorge effPutCounter's
// MonstrositySet arm): N +1/+1 counters on the source if it is not
// monstrous, then the monstrous designation and the BecomeMonstrous event
// carrying N. The resolution guard lives in the op (gorge's resolve-time
// skip); an AB$ root also gets monstrosityGateOK's offer gate (activate
// only while the source is not monstrous). Only the source itself, the
// corpus's only recipient: a Defined$/ValidTgts$ recipient or a
// CounterNum$/CounterType$ beside it refuses.
func (lc *lowerCtx) monstrosity(ps *params, obj lRef, raw string) error {
	if obj.Kind != rSelf {
		return refuse("Monstrosity$ recipient %v", obj)
	}
	if _, ok := ps.m["CounterNum"]; ok {
		return refuse("Monstrosity$ with CounterNum$")
	}
	if _, ok := ps.m["CounterType"]; ok {
		return refuse("Monstrosity$ with CounterType$")
	}
	n, err := lc.value(raw)
	if err != nil {
		return err
	}
	f, err := lc.filter("Card.Self+IsMonstrous")
	if err != nil {
		return err
	}
	none := lCond{Kind: cPresent, F: f, Op: opEQ, A: cst(0)}
	if lc.head && lc.depth == 1 && lc.ab.Kind == akActivated && lc.ab.Trigger == nil {
		if lc.ab.ActCond.Kind != cNone {
			return refuse("two activation conditions")
		}
		lc.ab.ActCond = none
	}
	lc.emit(lInstr{Op: "Monstrosity", Obj: obj, A: n})
	return nil
}

func (lc *lowerCtx) token(ps *params) error {
	script := ps.str("TokenScript")
	xpt, err := lc.tokenXPTGate(ps, script) // lower_tokenxpt.go
	if err != nil {
		return err
	}
	if strings.Contains(script, ",") {
		return lc.tokenList(ps, script)
	}
	if script == "" {
		return refuse("TokenScript %q", script)
	}
	if _, ok := lc.p.reg.Token(script); !ok {
		return refuse("token script %q not in the corpus", script)
	}
	var n lValue
	if ps.str("TokenAmount") == "Excess" {
		n = lValue{Kind: vExcessDamage}
	} else if n, err = lc.amount(ps, "TokenAmount", 1); err != nil {
		return err
	}
	owner := lRef{Kind: rYou}
	if _, ok, err := lc.targetedAndYou(ps, "TokenOwner"); err != nil {
		return err
	} else if ok {
		// gorge's token path (tokenOwnerPlayers, token.go:144) reads
		// definedSpec, not DefinedRef, so the TargetedAndYou alias is not
		// expanded there: the TokenOwner falls back to the resolving
		// controller (with a Note). Match that one-token fallback; the
		// target is still asked.
		owner = lRef{Kind: rYou}
	} else if o, ok := ps.get("TokenOwner"); ok && o == "RememberedOwner" {
		// the owner of the card the enters trigger exiled (CR 108.3), only
		// in the remembered-persist shape (lower_rempersist.go)
		if !lc.remPersistShape() {
			return refuse("Token TokenOwner$ RememberedOwner outside the remembered-persist shape")
		}
		owner = lRef{Kind: rRememberedOwner}
	} else if o, ok := ps.get("TokenOwner"); ok && o == "RememberedController" {
		// gorge rememberedControllersLKI: the remembered object's
		// controller, its RememberLKI$ capture once it left the battlefield
		owner = lRef{Kind: rRememberedController}
	} else if ok {
		if (o == "Targeted" || o == "TargetedPlayer") && strings.TrimSpace(ps.str("ValidTgts")) != "" {
			// gorge's tokenOwnerPlayers (effects/token.go:144) reads
			// definedSpec for Targeted/TargetedPlayer: the Token SA's own
			// targets, so its own ValidTgts$ answers, not the chain's last
			// slot. Only a player-only, one-target declaration is exact; a
			// count rider would make the created token's owner ambiguous
			// here.
			for _, k := range []string{"TargetMin", "TargetMax", "TargetUnique", "TgtZone"} {
				if _, ok := ps.get(k); ok {
					return refuse("Token TokenOwner$ %q with %s$", o, k)
				}
			}
			vt := ps.str("ValidTgts")
			pf, err := lc.filter(vt)
			if err != nil {
				return err
			}
			if pf.Players == 0 || len(pf.Clauses) != 0 {
				return refuse("Token TokenOwner$ %q ValidTgts$ %q", o, vt)
			}
			owner = lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf, Zone: zNone})}
			ps.consume("ValidTgts", "TgtPrompt")
		} else if owner, err = lc.defined(o); err != nil {
			return err
		}
	}
	// LockTokenScript$ (a gift's Fish): Forge keeps the script from being
	// replaced; gorge reads no such parameter and creates the script as is
	ps.consume("LockTokenScript")
	in := lInstr{Op: "CreateToken", Obj: owner, A: n, CS: script}
	// AttachedTo$: a Role token enters attached (CR 111.10j-r, 303.4g). The
	// destination rides B.N's high byte (kop.TokAttach); Obj2 carries the
	// Ref. gorge resolves it once before the mint loop (effects/token.go
	// tokenAttachTargets): a Token SA's own ValidTgts$ answers with its own
	// picked target, everything else through DefinedSpec (the chain's
	// single target), and Self names the source.
	if v, ok := ps.get("AttachedTo"); ok {
		var dst lRef
		switch v {
		case "Targeted", "ThisTargetedCard":
			if vt, own := ps.get("ValidTgts"); own && strings.TrimSpace(vt) != "" {
				var err error
				dst, err = lc.objOrTarget(ps, "", zBattlefield)
				if err != nil {
					return err
				}
				if dst.Kind != rTarget {
					return refuse("Token AttachedTo %q", v)
				}
				if dst.N > 2 {
					return refuse("Token AttachedTo target slot %d", dst.N)
				}
			} else {
				if v == "ThisTargetedCard" {
					return refuse("Token AttachedTo ThisTargetedCard without its own ValidTgts")
				}
				// gorge's DefinedSpec(Targeted) is the chain's target; the
				// kernel's union ref is ordered differently, so only the
				// single-slot chain is read exactly.
				if len(lc.chainT) != 1 {
					return refuse("Token AttachedTo Targeted over %d chain target slots", len(lc.chainT))
				}
				if lc.chainT[0] > 2 {
					return refuse("Token AttachedTo target slot %d", lc.chainT[0])
				}
				dst = lRef{Kind: rTarget, N: lc.chainT[0]}
			}
		case "Self":
			dst = lRef{Kind: rSelf}
		case "Remembered":
			// the repeated card of a RepeatEach over cards (lower_repeatcards.go)
			if lc.repeatCard == 0 {
				return refuse("Token AttachedTo %q", v)
			}
			dst = lRef{Kind: rEach}
		default:
			return refuse("Token AttachedTo %q", v)
		}
		if a, ok := ps.get("TokenAmount"); ok && strings.TrimSpace(a) != "1" {
			return refuse("Token AttachedTo with TokenAmount %q", a)
		}
		if !tokenIsAura(lc.p, script) {
			return refuse("Token AttachedTo on a non-Aura token %q", script)
		}
		ps.consume("TgtPrompt")
		in.C |= 32
		in.Obj2 = dst
	} else if tokenIsAura(lc.p, script) {
		// an Aura token minted without a destination has gorge's CR
		// 303.4f gate / non-cast entry ask, which the kernel does not model.
		return refuse("Aura token %q without AttachedTo$", script)
	}
	if ps.is("TokenTapped", "True") {
		in.C |= 1
	}
	if v, ok := ps.get("TokenAttacking"); ok {
		c, err := lc.tokenAttacking(v)
		if err != nil {
			return err
		}
		in.C |= c
	}
	if ps.is("RememberOriginalTokens", "True") {
		// gorge effects/token_params.go: p.Remember = RememberTokens ||
		// RememberOriginalTokens, so the created tokens join Ctx.Remembered
		in.C |= 2 | 4 // kop.TokRemember | kop.TokRememberObj
	}
	if v, ok := ps.get("AtEOT"); ok {
		// gorge token.go -> scheduleAtEOT: one next-end-step promise per
		// minted token (the copy-token rider's dly entries)
		switch v {
		case "Sacrifice", "YourSacrifice":
			// gorge effects/ateot.go:26-31 deliberately reads the Your-prefixed
			// end-step values as their plain equivalents (atEOTRiders:
			// YourSacrifice = __kwEncoreSacrifice at StepEnd, as Sacrifice)
			in.C |= 8
		case "Exile":
			in.C |= 16
		case "Destroy", "SacrificeCombat":
			// the extended riders: both AtEOT bits (8|16) plus 512 (Destroy at
			// the end step, tracked, through destroy) or 1024 (sacrifice at
			// this turn's end of combat, untracked). The kind rides in
			// B.N's high byte (kop.TokAtEOTDestroy / TokAtEOTSacCombat).
			if in.C&32 != 0 {
				return refuse("Token AtEOT %q beside AttachedTo$", v)
			}
			if v == "Destroy" {
				in.C |= 8 | 16 | 512
				break
			}
			// gorge schedules SacrificeCombat as a thisTurn rider (|TT=turn)
			// at StepEndCombat. Inside an Attacks trigger the resolution is
			// always in this turn's combat, before its end-of-combat step
			// (CR 508.8, 511.1: attackers are declared, then the trigger
			// resolves in the declare-attackers step), so the thisTurn expiry
			// can never bite; elsewhere it could (a main-phase mint).
			if lc.ab.Trigger == nil || lc.ab.Trigger.Event != "Attacks" {
				return refuse("Token AtEOT SacrificeCombat outside an Attacks trigger")
			}
			in.C |= 8 | 16 | 1024
		default:
			return refuse("Token AtEOT %q", v)
		}
	}
	if ps.is("RememberTokens", "True") {
		// gorge token.go: each mint is appended to Ctx.Remembered. The
		// kernel's remembered set lives for one resolution while gorge's
		// persists on the source, so the tokens must be read later in
		// this chain, not by another ability.
		if !lc.restReadsRemembered() {
			return refuse("RememberTokens read outside its own resolution")
		}
		in.C |= 4
	}
	if err := lc.tokenImprint(ps, &in); err != nil { // lower_imprintzone.go
		return err
	}
	var pump []string
	if kw, ok := ps.get("PumpKeywords"); ok {
		// gorge token.go: each minted token gets one LAbilities effect
		// sourced to itself, until end of turn when PumpDuration$ is
		// absent or EOT (Sokenzan's "They gain haste until end of turn").
		// The kernel remembers the mints and grants the keywords over the
		// remembered set, so the set must hold only these mints when the
		// PumpAll reads it: an earlier instruction may have written it (a
		// preceding Destroy writes nothing, a RememberChanged Exile/Dig or
		// a previous token does), and ClearRemembered just before the mint
		// makes that irrelevant. Exact because nothing later in the chain
		// reads the set (restReadsRemembered) and an earlier delayed
		// trigger captured its object by value when it registered
		// (dirac/interp.go OpDelayedTrigger, interp_gen.go
		// OpDelayedProgram), so clearing now cannot change what it fires
		// with. RememberTokens$ on this Token is still refused: clearing
		// then would drop the tokens its own later reader expects.
		if d, ok := ps.get("PumpDuration"); ok && d != "EOT" {
			return refuse("Token PumpDuration$ %q", d)
		}
		if in.C&4 != 0 || lc.restReadsRemembered() {
			return refuse("Token PumpKeywords$ beside another remembered set")
		}
		ks, err := pumpKeywords(kw)
		if err != nil {
			return err
		}
		in.C |= 4
		pump = ks
	} else if _, ok := ps.get("PumpDuration"); ok {
		return refuse("Token PumpDuration$ without PumpKeywords$")
	}
	xptIdx := 0
	if xpt {
		// the Ex form counts the mint (tokMade) for ExtTokenPT
		if in.C&(64|128) != 0 {
			return refuse("Token TokenPower$ with TokenAttacking$")
		}
		in.C |= 256
		if xptIdx, err = lc.tokenXPTOverride(ps); err != nil {
			return err
		}
	}
	lc.out.Tokens = append(lc.out.Tokens, script)
	if pump != nil && len(lc.ab.Prog) != 0 {
		// Forget whatever an earlier instruction remembered (see the guard
		// above): the PumpAll that follows the mint must see only it.
		lc.emit(lInstr{Op: "ClearRemembered"})
	}
	if xpt {
		// the Ext reads the X/X, then runs the CreateToken that follows
		lc.emit(lInstr{Op: "TokenPT", C: xptIdx})
	}
	lc.emit(in)
	if pump != nil {
		f, err := lc.filter("Card.IsRemembered")
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "PumpAll", F: f, A: cst(0), B: cst(0), CS: strings.Join(pump, "&")})
	}
	return nil
}

// tokenIsAura reports whether the token script's face is an Aura with an
// Enchant keyword line (gorge's CR 303.4f/g entry path keys on the same).
func tokenIsAura(p *pool, script string) bool {
	c, ok := p.reg.Token(script)
	if !ok || len(c.Faces) == 0 {
		return false
	}
	f := c.Faces[0]
	aura, enchant := false, false
	for _, t := range f.Types {
		if t == "Aura" {
			aura = true
		}
	}
	for _, k := range f.Keywords {
		if strings.HasPrefix(k, "Enchant:") {
			enchant = true
		}
	}
	return aura && enchant
}

func (lc *lowerCtx) fight(ps *params) error {
	var a lRef
	var err error
	d, hasD := ps.get("Defined")
	if hasD {
		if d == "DelayTriggerRemembered" && lc.ab != nil && lc.ab.ReflexObj {
			a = lRef{Kind: rDelayedRemembered} // the reflexive trigger's captured target (lower_remembered.go)
		} else if a, err = lc.defined(d); err != nil {
			return err
		}
	}
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	lt := lTarget{F: f, Zone: zBattlefield}
	// "fights up to one target creature": gorge fightPairs (engine/effects/
	// damage.go) pairs Defined$ with each chosen target, and the placement
	// ask honours TargetMin$ 0 / TargetMax$ 1 (engine/rules/fight_test.go:235,
	// fight_cr70114_test.go:38). Only that spelling is read; any other
	// TargetMin/TargetMax shape is refused.
	tmin, hasMin := ps.get("TargetMin")
	tmax, hasMax := ps.get("TargetMax")
	if hasMin || hasMax {
		if !hasD || tmin != "0" || tmax != "1" {
			return refuse("Fight TargetMin$ %q TargetMax$ %q: only Defined$ with 0/1 is read", tmin, tmax)
		}
		lt.Min, lt.Max = cst(0), cst(1)
	}
	t := lc.addTarget(lt)
	if !hasD {
		return refuse("Fight without Defined$")
	}
	if a.Kind == rTriggered {
		// Affectionate Indrik's ETB "it fights": the source itself, only
		// when the trigger is a self-trigger (ValidCard$ Card.Self). A
		// non-self trigger (Tolsimir's entering Wolf) keeps the trigger
		// object.
		if tr := lc.ab.Trigger; tr != nil && tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Players == 0 &&
			(tr.F.Clauses[0] == lClause{Flags: fSelf} || tr.F.Clauses[0] == lClause{Types: tCreature, Flags: fSelf}) {
			a = lRef{Kind: rSelf}
		}
	}
	track, err := lc.fightExcess(ps)
	if err != nil {
		return err
	}
	cs := "with=Target" + strconv.Itoa(t)
	if track {
		cs += "|TrackExcess" // the hit dealt to the C creature records ExExcessDamage
	}
	lc.emit(lInstr{Op: "Fight", Obj: a, C: t, CS: cs})
	return lc.replaceDyingMark(ps, lRef{Kind: rTarget, N: t}, a) // only the fought target (Targeted), not Defined$
}

func (lc *lowerCtx) attach(ps *params) error {
	if o, ok := ps.get("Object"); ok && o != "Self" {
		return lc.attachObject(ps, o)
	}
	if _, ok := ps.get("Choices"); ok {
		if _, ok := ps.get("Object"); !ok {
			return lc.attachChoices(ps)
		}
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if lc.ab.Kind == akActivated && ps.is("Keyword", "Equip") {
		lc.ab.Name = "Equip"
	}
	if err := lc.mayGate(ps, kop.MayGuardAttach, obj); err != nil {
		return err
	}
	in := lInstr{Op: "Attach", Obj: obj}
	if v, ok := ps.m["RememberAttached"]; ok {
		// gorge attachTo: the attaching object joins the remembered set
		// when the attach happens
		if v = strings.TrimSpace(v); v != "True" {
			return refuse("Attach RememberAttached$ %q", v)
		}
		ps.consume("RememberAttached")
		in.CS = "Remember"
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) charm(ps *params) error {
	choices := strings.Split(ps.str("Choices"), ",")
	if len(choices) < 2 {
		return refuse("charm with %d choices", len(choices))
	}
	if lc.mode >= 0 {
		return refuse("nested charm")
	}
	if lc.ab.Modes != 0 {
		return refuse("two charms in one ability")
	}
	// Defined$ names who chooses the mode (gorge CharmParams.Defined);
	// "You" is the controller, the kernel's only chooser.
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("charm chooser Defined$ %q", d)
	}
	twPick, plainPick, err := lc.teamworkCharmNum(ps, len(choices))
	if err != nil {
		return err
	}
	if dyn, ok, err := lc.charmDynNum(ps, len(choices)); err != nil {
		return err
	} else if ok {
		plainPick = dyn // CharmNum$ over Count$Compare (lower_charmcmp.go)
	} else if lc.ab.Kind == akTriggered {
		if lc.out.ModesPickV.Kind != vNone {
			return refuse("a fixed triggered charm count beside a dynamic one")
		}
		lc.out.TrigCharmFixed = true
	}
	pick, minPick, err := charmCounts(ps, len(choices), plainPick)
	if err != nil {
		return err
	}
	if max(pick, twPick) > 1 && lc.ab.Kind != akSpell && lc.ab.Kind != akTriggered && !(lc.ab.Kind == akLatent && lc.reflexMulti) {
		return refuse("multi-pick charm on a %s ability", abilityKindNames[lc.ab.Kind])
	}
	if lo := lc.out.ModesMinCount; lo != 0 {
		minPick = min(lo, pick) // the offer gate's minimum (CardDef.ModesMinCount)
	}
	rp, err := lc.charmRepeatPlan(ps, choices, pick, twPick, lc.out.ModesPickV.Kind != vNone) // lower_charmrepeat.go
	if err != nil {
		return err
	}
	lc.ab.Modes = len(choices)
	lc.ab.ModesPick, lc.ab.ModesMin = pick, minPick
	if rp != nil {
		lc.ab.RepMax, lc.ab.RepCost, lc.ab.RepBudget = rp.max, rp.cost, rp.budget
	}
	rootTargeted := make([]bool, len(choices)) // gorge Targeted(): the mode root's ValidTgts$ is non-empty
	for i, name := range choices {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		if sa == nil {
			return refuse("charm choice %q is not an ability", name)
		}
		rootTargeted[i] = strings.TrimSpace(sa.Params["ValidTgts"]) != ""
		lc.mode = i
		mps, err := lc.modeCost(sa, i) // lower_modecost.go
		if err != nil {
			return err
		}
		c := i
		if rp != nil {
			if mps == nil {
				mps = newParams(sa.Params)
			}
			mps.consume("Pawprint") // the mode's price, read by charmRepeatPlan
			c = rp.c(i, 0)
		}
		saved := lc.chainT
		lc.chainT = append([]int(nil), lc.chainT...)
		at := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Charm", C: c})
		if err := lc.chain(sa, mps); err != nil {
			return err
		}
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
		if rp != nil {
			rp.replicate(lc, at, i)
		}
		lc.chainT = saved
	}
	lc.mode = -1
	if err := lc.charmRestrict(ps); err != nil {
		return err
	}
	if pick > 1 && len(choices) > 4 {
		return refuse("charm with more than 4 modes") // mode bits 0..3 only
	}
	if pick > len(choices) && rp == nil {
		// gorge clamps the maximum to the eligible modes; no carrier reads it
		return refuse("charm count above its modes (%d of %d)", pick, len(choices))
	}
	if max(pick, twPick) > 1 {
		if err := lc.charmMultiTargets(len(choices), rootTargeted); err != nil {
			return err
		}
		if rp != nil {
			return lc.charmRepeatTargets(rp, len(choices))
		}
	}
	return nil
}

// teamworkCharmNum reads a Teamwork card's CharmNum$ over an SVar whose
// body is Count$Teamwork.<a>.<b> (single digits >= 1; CR 702.194b: "choose
// both" when teamworked): <a> modes teamworked (CardDef.ModesPickTW), <b>
// otherwise. Any other non-literal CharmNum$ is left to charmCounts, which
// refuses it. It returns 0, 0 for a literal count.
func (lc *lowerCtx) teamworkCharmNum(ps *params, nModes int) (tw, plain int, err error) {
	v, ok := ps.m["CharmNum"]
	if !ok {
		return 0, 0, nil
	}
	body, ok := lc.face.SVars[strings.TrimSpace(v)]
	rest, isTW := strings.CutPrefix(strings.TrimSpace(body), "Count$Teamwork.")
	if !ok || !isTW {
		return 0, 0, nil
	}
	if lc.out.Teamwork == 0 {
		return 0, 0, refuse("charm count Count$Teamwork without Teamwork")
	}
	if _, has := ps.m["MinCharmNum"]; has {
		return 0, 0, refuse("teamwork charm count beside MinCharmNum$")
	}
	if lc.out.ModesPickTW != 0 {
		return 0, 0, refuse("two teamwork charms")
	}
	arms := strings.Split(rest, ".")
	if len(arms) != 2 {
		return 0, 0, refuse("teamwork charm count %q", rest)
	}
	var n [2]int
	for i, a := range arms {
		if len(a) != 1 || a[0] < '1' || a[0] > '9' {
			return 0, 0, refuse("teamwork charm count arm %q", a)
		}
		n[i] = int(a[0] - '0')
	}
	if n[0] > nModes || n[0] > 4 {
		return 0, 0, refuse("teamwork charm count %d of %d modes", n[0], nModes)
	}
	lc.out.ModesPickTW = n[0]
	return n[0], n[1], nil
}

// charmCounts reads CharmNum$/MinCharmNum$ (gorge CharmModeBounds): literal
// single digits only; the maximum defaults to 1 and the minimum to the
// maximum. AdditionalDescription$ is display text.
func charmCounts(ps *params, nModes int, num int) (pick, minPick int, err error) {
	ps.consume("AdditionalDescription")
	pick, minPick = 1, -1
	if num > 0 {
		// a Teamwork card's Count$Teamwork.<a>.<b>: <b> is the plain count
		// (lc.teamworkCharmNum read and checked it)
		ps.consume("CharmNum")
		pick = num
	}
	lit := func(key string) (int, bool, error) {
		v, ok := ps.get(key)
		if !ok {
			return 0, false, nil
		}
		v = strings.TrimSpace(v)
		if len(v) != 1 || v[0] < '0' || v[0] > '9' {
			return 0, false, refuse("charm count %s$ %q", key, v)
		}
		return int(v[0] - '0'), true, nil
	}
	if num == 0 {
		if n, ok, err := lit("CharmNum"); err != nil {
			return 0, 0, err
		} else if ok {
			pick = n
		}
	}
	if m, ok, err := lit("MinCharmNum"); err != nil {
		return 0, 0, err
	} else if ok {
		minPick = m
	}
	if pick < 1 {
		return 0, 0, refuse("charm count CharmNum$ %d", pick)
	}
	if minPick < 0 {
		minPick = pick
	}
	if minPick > pick {
		return 0, 0, refuse("charm count MinCharmNum$ %d above CharmNum$ %d", minPick, pick)
	}
	// pick 1, min 0: "choose up to one" (Hullbreaker Horror, Ertai
	// Resurrected), the kernel's multi-pick mode step
	return pick, minPick, nil
}

// charmMultiTargets refuses the target shapes of a multi-pick charm the
// kernel's combined target step does not read (gorge target_modes.go
// askCharmModeTargets: one KTarget when every slot is 1..1; a slot outside
// 1..1 takes the sequential askCharmSeqSlot, and TargetUnique another path).
// A mode's first declaration must be its root's own (rootTargeted, gorge
// Targeted()); later ones are chain links, announced after the combined ask
// (castSubAskLinks, target_unique_chain.go), which only a cast spell does.
func (lc *lowerCtx) charmMultiTargets(nModes int, rootTargeted []bool) error {
	for _, t := range lc.ab.Targets {
		if t.Mode < 0 {
			return refuse("multi-pick charm with a non-mode target")
		}
		if t.Mode >= nModes {
			return refuse("multi-pick charm with a target outside its modes")
		}
	}
	plural, chained := false, false
	for m := 0; m < nModes; m++ {
		n := 0
		for _, t := range lc.ab.Targets {
			if t.Mode != m {
				continue
			}
			n++
			if t.Unique {
				return refuse("multi-pick charm with TargetUnique")
			}
			if t.SameCtrl || t.DiffCtrl || t.SharedCrType || t.DistinctName || t.MaxMV != 0 || t.Divided != 0 || t.ParentPlus1 != 0 || t.AtRes ||
				!t.Min.isConst() || t.Min.N < 1 || t.Min.N > 2 || !t.Max.isConst() || t.Max.N < t.Min.N || t.Max.N > kop.MaxTargets {
				return refuse("multi-pick charm mode target bounds %s..%s", t.Min, t.Max)
			}
			// a slot of more than one target takes gorge's sequential per-mode asks
			// (askCharmSeqSlot), which the kernel derives from the slot
			// bounds (modal_charmseq.go); the chain links' place among
			// those asks is not read
			plural = plural || t.Max.N > 1
		}
		if n > 0 && !rootTargeted[m] {
			return refuse("multi-pick charm mode whose root declares no target")
		}
		if n > 1 && lc.ab.Kind != akSpell {
			return refuse("multi-pick charm mode chain targets on a triggered ability")
		}
		chained = chained || n > 1
	}
	if plural && chained {
		return refuse("multi-pick charm 1..N target slot beside a mode's chain targets")
	}
	return nil
}

// repeat lowers a gate-governed api:Repeat (gorge effRepeat): the body
// (RepeatSubAbility$) runs at most MaxRepeat$ times, again only while the
// RepeatCheckSVar$ gate compares true after a pass. Only the remembered-set
// count gate is lowered (Sphinx's Tutelage: `Remembered$Valid <filter>`).
func (lc *lowerCtx) repeat(ps *params) error {
	for _, k := range []string{"RepeatDefined", "RepeatPresent", "RepeatOptional", "RepeatNum", "RepeatCompare"} {
		if _, ok := ps.get(k); ok {
			return refuse("Repeat %s$", k)
		}
	}
	if _, ok := ps.get("ValidTgts"); ok {
		// the targeted player the body's Defined$ Targeted names
		if _, err := lc.objOrTarget(ps, "", zNone); err != nil {
			return err
		}
	}
	mx, err := lc.value(ps.str("MaxRepeat"))
	if err != nil {
		return err
	}
	gate := lc.face.SVars[ps.str("RepeatCheckSVar")]
	head, rest, _ := strings.Cut(strings.TrimSpace(gate), "$")
	if head != "Remembered" || !strings.HasPrefix(rest, "Valid ") {
		return refuse("Repeat gate %q", gate)
	}
	f, err := lc.filter(strings.TrimPrefix(rest, "Valid "))
	if err != nil {
		return err
	}
	op, v, err := lc.cmpSpec(ps.str("RepeatSVarCompare"))
	if err != nil {
		return err
	}
	body := cards.ResolveSVar(lc.face.SVars, ps.str("RepeatSubAbility"))
	if body == nil {
		return refuse("Repeat body %q", ps.str("RepeatSubAbility"))
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatWhile", A: mx, Cond: lCond{Kind: cRefMatches, Ref: lRef{Kind: rRemembered}, F: f, Op: op, A: v}})
	if err := lc.chain(body, nil); err != nil {
		return err
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// copySpell lowers a SpellCast trigger's "copy it" (gorge
// effCopySpellAbility, Defined$ TriggeredSpellAbility): Amount$ copies of
// the triggering instant or sorcery spell; MayChooseTarget$ True lets the
// copier choose new targets for each as it resolves (CR 707.10c).
func (lc *lowerCtx) copySpell(ps *params) error {
	if ps.str("Defined") == "Parent" {
		return lc.copyParentSpell(ps) // lower_choosetype.go
	}
	if _, ok := ps.get("Defined"); !ok {
		if tt := ps.str("TargetType"); tt == "Activated.YouCtrl,Triggered.YouCtrl" || tt == "Triggered.YouCtrl" {
			return lc.copyAbility(ps) // lower_copyab.go
		}
		if _, ok := ps.get("ValidTgts"); ok {
			return lc.copyTargetSpell(ps)
		}
	}
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "SpellCast" || ps.str("Defined") != "TriggeredSpellAbility" {
		return refuse("CopySpellAbility outside a SpellCast trigger's Defined$ TriggeredSpellAbility")
	}
	if tr.F == nil || len(tr.F.Clauses) == 0 || tr.F.Players != 0 {
		return refuse("CopySpellAbility of an unfiltered spell")
	}
	perm := false
	for _, c := range tr.F.Clauses {
		if c.Types != 0 && c.Types&^(tInstant|tSorcery) == 0 {
			continue // an instant or sorcery clause
		}
		if c == (lClause{Flags: fSelf}) && lc.out.Types&^(tInstant|tSorcery) == 0 {
			continue // the spell's own cast trigger on an instant or sorcery
		}
		perm = true // a permanent (or untyped) clause: the copy may be a permanent spell
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "CopySpell", Obj: lRef{Kind: rTriggered}, A: n}
	if ps.is("MayChooseTarget", "True") {
		in.C = 1
	}
	if perm {
		in.C |= 2 // may copy a permanent spell: the copy becomes a token (CR 707.10f)
	}
	// Controller$ You is the copy controller gorge defaults to; any other
	// selector stays unread (and refuses) until a carrier needs it.
	if v, ok := ps.m["Controller"]; ok && strings.TrimSpace(v) == "You" {
		ps.consume("Controller")
	}
	lc.emit(in)
	return nil
}

// copyTargetSpell lowers "copy target instant or sorcery spell you
// control" (Kitsa, Otterball Elite: gorge effCopySpellAbility's targeted
// arm copies the first object target): one copy (Amount$) of the target
// spell, MayChooseTarget$ True as for the trigger form. A filter that can
// match a permanent spell sets C&2 (kernel anyPermCopy).
func (lc *lowerCtx) copyTargetSpell(ps *params) error {
	if tt := ps.str("TargetType"); tt != "Spell" {
		return refuse("CopySpellAbility TargetType %q", tt)
	}
	f, err := lc.filter(ps.str("ValidTgts"))
	if err != nil {
		return err
	}
	if f == nil || len(f.Clauses) == 0 || f.Players != 0 {
		return refuse("CopySpellAbility of an unfiltered spell")
	}
	perm := false
	for _, c := range f.Clauses {
		if c.Types == 0 || c.Types&^(tInstant|tSorcery) != 0 {
			perm = true
		}
	}
	if _, ok := ps.get("TargetMin"); ok {
		return refuse("CopySpellAbility TargetMin")
	}
	if _, ok := ps.get("TargetMax"); ok {
		return refuse("CopySpellAbility TargetMax")
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true})
	in := lInstr{Op: "CopySpell", Obj: lRef{Kind: rTarget, N: t}, A: n}
	if ps.is("MayChooseTarget", "True") {
		in.C = 1
	}
	if perm {
		in.C |= 2 // may copy a permanent spell: the copy becomes a token (CR 707.10f)
	}
	lc.emit(in)
	return nil
}

// demonstrate lowers the body of the trigger gorge's Demonstrate keyword
// expansion appends (cards/kw_demonstrate.go): "when you cast this spell,
// copy it and you may choose a new target for the copy; an opponent copies
// it too" (CR 702.144a). The trigger must be the card's own SpellCast
// (ValidCard$ Card.Self) and the spell an instant or sorcery. The election
// and the two copies are the kernel's (OpDemonstrate).
func (lc *lowerCtx) demonstrate(ps *params) error {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "SpellCast" || ps.str("Defined") != "TriggeredSpellAbility" {
		return refuse("Demonstrate outside a SpellCast trigger's Defined$ TriggeredSpellAbility")
	}
	if tr.F == nil || len(tr.F.Clauses) == 0 || tr.F.Players != 0 {
		return refuse("Demonstrate of an unfiltered spell")
	}
	for _, c := range tr.F.Clauses {
		if c.Flags != fSelf || c.Types != 0 {
			return refuse("Demonstrate of a spell other than the card itself")
		}
	}
	if lc.out.Types&(tInstant|tSorcery) == 0 {
		return refuse("Demonstrate on a permanent spell")
	}
	lc.emit(lInstr{Op: "Demonstrate", Obj: lRef{Kind: rTriggered}})
	return nil
}

// giftCondition reads "if the gift was (wasn't) promised" (CR 702.174):
// ConditionPresent$ Card.Self+PromisedGift on the spell itself
// (ConditionZone$ Stack), compared EQ1 or EQ0. Any other PromisedGift
// condition shape refuses.
func (lc *lowerCtx) giftCondition(ps *params) (lCond, bool, error) {
	cp := strings.TrimSpace(ps.m["ConditionPresent"])
	if !strings.Contains(cp, "PromisedGift") {
		return lCond{}, false, nil
	}
	ps.consume("ConditionPresent")
	if cp != "Card.Self+PromisedGift" || !lc.out.Gift || ps.str("ConditionZone") != "Stack" {
		return lCond{}, false, refuse("gift condition %q", cp)
	}
	if _, ok := ps.get("ConditionDefined"); ok {
		return lCond{}, false, refuse("gift condition with ConditionDefined$")
	}
	switch ps.str("ConditionCompare") {
	case "EQ1", "GE1":
		return lCond{Kind: cPromisedGift}, true, nil
	case "EQ0":
		return lCond{Kind: cPromisedGift, Neg: true}, true, nil
	}
	return lCond{}, false, refuse("gift condition compare %q", ps.str("ConditionCompare"))
}

// teamworkCondition reads "if this spell was cast using teamwork" (CR
// 702.194b): ConditionPresent$ Card.Self+Teamwork on the spell itself
// (ConditionZone$ Stack), the resolving entry's paid provenance (CondKicked
// reads sfKicked, which a Teamwork card sets only for a paid tap). Any other
// Teamwork condition shape refuses.
func (lc *lowerCtx) teamworkCondition(ps *params) (lCond, bool, error) {
	cp := strings.TrimSpace(ps.m["ConditionPresent"])
	if !strings.Contains(cp, "Teamwork") {
		return lCond{}, false, nil
	}
	ps.consume("ConditionPresent")
	if cp != "Card.Self+Teamwork" || lc.out.Teamwork == 0 || ps.str("ConditionZone") != "Stack" ||
		lc.ab == nil || lc.ab.Kind != akSpell {
		return lCond{}, false, refuse("teamwork condition %q", cp)
	}
	if _, ok := ps.get("ConditionDefined"); ok {
		return lCond{}, false, refuse("teamwork condition with ConditionDefined$")
	}
	switch ps.str("ConditionCompare") {
	case "", "EQ1", "GE1":
		return lCond{Kind: cKicked}, true, nil
	}
	return lCond{}, false, refuse("teamwork condition compare %q", ps.str("ConditionCompare"))
}

// skipNextUntapAll lowers a PumpAll whose KW$ is exactly CR 611.2b's
// next-untap-step sentence (gorge registerPumpEffects stamps
// Object.CantUntapNextStep on each swept permanent at grant, whatever the
// Duration$) as OpSkipNextUntap over the filter. Defined$ Targeted scopes
// the sweep to the chain's player target (gorge cr-sleep-defined-targeted:
// targetPlayerKindScope inherits the parent's player target; Sleep). A P/T
// change, another keyword or any other scope refuses.
func (lc *lowerCtx) skipNextUntapAll(ps *params, f *lFilter) error {
	ps.consume("KW")
	_, na := ps.m["NumAtt"]
	_, nd := ps.m["NumDef"]
	if na || nd {
		return refuse("next-untap-step sweep beside a P/T change")
	}
	if d, ok := ps.get("Duration"); ok && d != "Permanent" {
		return refuse("next-untap-step sweep Duration %q", d)
	}
	if _, ok := ps.get("ValidTgts"); ok {
		return refuse("next-untap-step sweep with ValidTgts$")
	}
	var scope lRef
	if d, ok := ps.get("Defined"); ok {
		t := lc.lastTarget()
		if d != "Targeted" || t < 0 {
			return refuse("next-untap-step sweep Defined$ %q", d)
		}
		if pf := lc.ab.Targets[t].F; pf == nil || len(pf.Clauses) != 0 || pf.Players == 0 {
			return refuse("next-untap-step sweep Defined$ Targeted of an object target")
		}
		scope = lRef{Kind: rTarget, N: t}
	}
	lc.emit(lInstr{Op: "SkipNextUntap", Obj: scope, F: f, Note: "each matching permanent (the target player's)"})
	return nil
}

// chooseColorPrevent lowers a mid-resolution ChooseColor (Defined$ You)
// whose one sub-ability is an Effect registering only a bodyless DamageDone
// Prevent$ True replacement over sources of the chosen colour (Prismatic
// Strands: gorge effChooseColor stores the answer on the source card, whose
// Effect replacement reads it as ValidSource$ Card.ChosenColor for the turn,
// effect.go's bodyless prevent arm). Both nodes lower to one PreventColor
// (the choice is the op's own ask); any other ChooseColor chain refuses.
func (lc *lowerCtx) chooseColorPrevent(ps *params) error {
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("ChooseColor outside an as-enters replacement (Defined$ %q)", d)
	}
	ps.consume("AILogic", "SubAbility")
	n := lc.curNode.Sub
	if n != nil && n.API == "Pump" && n.Params["DefinedKW"] == "ChosenColor" {
		// (Skrelv) the colour rides the resolution (res.chosen)
		// for the chosen-colour grants that follow
		lc.colGrant = true
		lc.emit(lInstr{Op: "ChooseColorRes", Note: "the controller chooses a colour for this resolution"})
		return nil
	}
	if n != nil && n.API == "Animate" && strings.EqualFold(strings.TrimSpace(n.Params["Colors"]), "ChosenColor") {
		// (Puca's Eye, Mondo Gecko) the Animate's
		// Colors$ ChosenColor reads the resolution's colour (res.chosen)
		// when the five-record override applies
		lc.colGrant = true
		lc.emit(lInstr{Op: "ChooseColorRes", Note: "the controller chooses a colour for this resolution"})
		return nil
	}
	if n == nil || n.API != "Effect" || n.Sub != nil || lc.preventColorShape(n.Params) != "" {
		return refuse("ChooseColor outside an as-enters replacement")
	}
	lc.prevColor = true
	return nil
}

// preventColorShape names what keeps an Effect from being the
// prevent-the-chosen-colour shape ("" when it is).
func (lc *lowerCtx) preventColorShape(p map[string]string) string {
	for k := range p {
		switch k {
		case "ReplacementEffects", "SpellDescription", "StackDescription", "AILogic":
		default:
			return "Effect param " + k
		}
	}
	name := strings.TrimSpace(p["ReplacementEffects"])
	line, ok := lc.face.SVars[name]
	if !ok {
		return "replacement " + name
	}
	want := map[string]string{"Event": "DamageDone", "Prevent": "True", "ActiveZones": "Command", "ValidSource": "Card.ChosenColor"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "Description" {
			continue
		}
		if want[k] != v {
			return "replacement param " + k
		}
		seen++
	}
	if seen != len(want) {
		return "replacement shape"
	}
	return ""
}

// preventColorEffect is the Effect half of chooseColorPrevent: OpExt ExtPreventColor
// asks the controller for a colour and prevents all damage sources of that
// colour would deal this turn (kernel cast_altfree.go).
func (lc *lowerCtx) preventColorEffect(ps *params) error {
	lc.prevColor = false
	ps.consume("ReplacementEffects", "AILogic")
	lc.emit(lInstr{Op: "PreventColor", Note: "choose a colour; prevent all damage its sources would deal this turn"})
	return nil
}

// eachDamage lowers api:EachDamage as OpExt ExtEachDamage
// (Case of the Gateway Express; gorge effEachDamage). The damagers are
// either DefinedDamagers$ Valid <filter> (every battlefield permanent
// matching it) or a chain referent (Common Goal, Polukranos):
// ParentTarget (the nearest earlier link's target slot, gorge
// parentLinkTargets), Targeted (the union of the chain's earlier slots,
// refTargetUnion) or Targeted.YouCtrl (that union narrowed to the
// controller's permanents, gorge eachDamagerTargets). Each deals NumDmg$
// (a constant, or Count$CardPower: its own power) to the recipient: the
// node's own single object target, or Defined$ Self. EachToItself$,
// ToEachOther$, ValidCards$ damagers, DamageSource$, division and player
// recipients refuse.
func (lc *lowerCtx) eachDamage(ps *params) error {
	for _, k := range [...]string{"EachToItself", "ToEachOther", "ValidCards", "DamageSource", "DividedAsYouChoose"} {
		if _, ok := ps.get(k); ok {
			return refuse("EachDamage %s$", k)
		}
	}
	dd := strings.TrimSpace(ps.str("DefinedDamagers"))
	var f *lFilter
	mask := 0 // damager target slots (bit per slot); 0 = the Valid sweep
	if spec, ok := strings.CutPrefix(dd, "Valid "); ok {
		var err error
		if f, err = lc.filter(strings.TrimSpace(spec)); err != nil {
			return err
		}
		if f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("EachDamage damagers %q: a player alternative", dd)
		}
	} else {
		// read before the node's own target slot is added: gorge's
		// parent record and union are the links resolved before this one
		var slots []int
		switch dd {
		case "ParentTarget":
			if t := lc.lastTarget(); t >= 0 {
				slots = []int{t}
			}
		case "Targeted", "Targeted.YouCtrl":
			if !(lc.depth == 1 && lc.mode < 0 || lc.depth == 2 && lc.mode >= 0) {
				return refuse("EachDamage DefinedDamagers$ %s in a nested chain", dd)
			}
			if len(lc.chainT) > 1 {
				// gorge's union over two earlier slots names only the
				// root's (Friendly Rivalry's legendary second target never
				// deals damage: eachdmgtargeted seeds 432, 696; reported to gorge)
				return refuse("EachDamage DefinedDamagers$ %s over %d slots (gorge reads only the root's)", dd, len(lc.chainT))
			}
			slots = append(slots, lc.chainT...)
		case "Remembered":
			var err error
			if slots, err = lc.eachDamageRemembered(); err != nil { // lower_remembered.go
				return err
			}
		default:
			return refuse("EachDamage DefinedDamagers$ %q", dd)
		}
		if len(slots) == 0 {
			return refuse("EachDamage DefinedDamagers$ %s with no earlier target slot", dd)
		}
		for _, t := range slots {
			pt := lc.ab.Targets[t]
			if pt.F == nil || pt.F.Players != 0 || len(pt.F.Clauses) == 0 || pt.AtRes || pt.Zone != zBattlefield {
				return refuse("EachDamage damager slot %d: not a battlefield object target", t)
			}
			mask |= 1 << t
		}
		if dd == "Targeted.YouCtrl" {
			var err error
			if f, err = lc.filter("Card.YouCtrl"); err != nil {
				return err
			}
		}
	}
	n, err := lc.amount(ps, "NumDmg", 1)
	if err != nil {
		return err
	}
	switch {
	case n.Kind == vPowerOf && n.Ref.Kind == rSelf && n.Mul == 0 && n.Add == 0 && !n.Neg:
		// Count$CardPower read per damager (gorge's per-damager Ctx anchors
		// Source on it): B = 0, the kernel's "its power"
		n = cst(0)
	case !n.isConst() || n.N < 1:
		return refuse("EachDamage NumDmg$ %v: not a positive constant or the damager's power", n)
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rTarget:
		if t := lc.ab.Targets[obj.N]; t.F == nil || t.F.Players != 0 || !t.Max.isConst() || t.Max.N != 1 {
			return refuse("EachDamage target slot %d: not one object", obj.N)
		}
		if mask&(1<<obj.N) != 0 {
			return refuse("EachDamage recipient slot %d is a damager slot", obj.N)
		}
	case rSelf:
		if mask == 0 {
			return refuse("EachDamage Valid damagers to Self")
		}
	default:
		return refuse("EachDamage to %s", obj)
	}
	lc.emit(lInstr{Op: "EachDamage", F: f, B: n, Obj2: obj, C: mask})
	return nil
}

// colorGrantPump lowers a Pump DefinedKW$ ChosenColor after ChooseColorRes
// (Skrelv, Defector Mite; Mondo Gecko, Defined$ Self; gorge definedKWGrant,
// cr-pump-definedkw-chosencolor): the object -- the one object target, or
// the source itself (Defined$ Self) -- gains, until end of turn, hexproof
// from the chosen colour (Hexproof:ChosenColor) and/or toxic N (Toxic:N):
// OpExt ExtColorGrant, C bit 0 hexproof, A the toxic amount. No P/T
// change, no other keyword, no other duration.
func (lc *lowerCtx) colorGrantPump(ps *params) error {
	if !lc.colGrant || ps.str("DefinedKW") != "ChosenColor" {
		return refuse("Pump DefinedKW$ %q outside a chosen-colour grant", ps.str("DefinedKW"))
	}
	for _, k := range [...]string{"NumAtt", "NumDef", "Duration", "KWChoice", "PumpZone"} {
		if _, ok := ps.get(k); ok {
			return refuse("chosen-colour Pump %s$", k)
		}
	}
	if d, ok := ps.get("Defined"); ok && d != "Targeted" && d != "Self" {
		return refuse("chosen-colour Pump Defined$ %q", d)
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget && obj.Kind != rSelf {
		return refuse("chosen-colour Pump without its own target")
	}
	in := lInstr{Op: "ColorGrant", Obj: obj, A: cst(0), Note: "until end of turn: the chosen colour's grants"}
	for _, kw := range strings.Split(ps.str("KW"), "&") {
		kw = strings.TrimSpace(kw)
		switch {
		case kw == "Hexproof:ChosenColor":
			in.C |= 1
		case strings.HasPrefix(kw, "Toxic:"):
			n, err := strconv.Atoi(kw[len("Toxic:"):])
			if err != nil || n < 1 || n > 7 || in.A.N != 0 {
				return refuse("chosen-colour Pump %q", kw)
			}
			in.A = cst(n)
		default:
			return refuse("chosen-colour Pump keyword %q", kw)
		}
	}
	lc.colTgt = obj
	lc.emit(in)
	return nil
}

// colorGrantEffect lowers the Effect after a chosen-colour Pump: one
// CantBlockBy static, ValidAttacker$ Creature.IsRemembered (the pumped
// target, RememberObjects$ Targeted) and ValidBlocker$ Creature.ChosenColor,
// for the turn (gorge effEffect + freezeChosenColor): ExtColorGrant C bit 1.
func (lc *lowerCtx) colorGrantEffect(ps *params) error {
	if ps.str("RememberObjects") != "Targeted" || lc.colTgt.Kind != rTarget {
		return refuse("chosen-colour Effect RememberObjects$ %q", ps.str("RememberObjects"))
	}
	for _, k := range [...]string{"Duration", "Triggers", "ReplacementEffects", "ExileOnMoved", "ForgetOnMoved"} {
		if _, ok := ps.get(k); ok {
			return refuse("chosen-colour Effect %s$", k)
		}
	}
	name := strings.TrimSpace(ps.str("StaticAbilities"))
	line, ok := lc.face.SVars[name]
	if !ok || strings.Contains(name, ",") {
		return refuse("chosen-colour Effect statics %q", name)
	}
	want := map[string]string{"Mode": "CantBlockBy", "ValidAttacker": "Creature.IsRemembered", "ValidBlocker": "Creature.ChosenColor"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "Description" {
			continue
		}
		if want[k] != v {
			return refuse("chosen-colour Effect static param %s", k)
		}
		seen++
	}
	if seen != len(want) {
		return refuse("chosen-colour Effect static shape")
	}
	lc.emit(lInstr{Op: "ColorGrant", Obj: lc.colTgt, A: cst(0), C: 2, Note: "this turn: can't be blocked by creatures of the chosen colour"})
	return nil
}

// dividedCountersN lowers "distribute N +1/+1 counters among one to N target
// creatures" (N a literal 2..8; gorge counters_put.go AnnouncedDivision): a
// 1..N (TargetMin$ 0: 0..N) target slot with Divided N. ok=false leaves any other shape to
// dividedCountersLKI.
func (lc *lowerCtx) dividedCountersN(ps *params) (bool, error) {
	name := ps.str("DividedAsYouChoose")
	n, err := strconv.Atoi(name)
	if err != nil || n < 2 || n > 8 || strconv.Itoa(n) != name || ps.str("CounterNum") != name || ps.str("TargetMax") != name ||
		(ps.str("TargetMin") != "1" && ps.str("TargetMin") != "0") || ps.str("CounterType") != "P1P1" {
		return false, nil
	}
	// gorge announces a divided primitive's shares only when it is the
	// resolving object's own root target declaration (effects/division.go
	// AnnouncedDivision matches Ctx.OfferedSA by Line, the same SA
	// rules/cast_division.go castDivisionAPI reads). A SubAbility$ link (Rhino,
	// Terrible Trampler) divides at resolution, so lowering it as an announced
	// division diverges. The root is the ability's chain head, or a chosen
	// charm mode's head (offeredTargetSA).
	if !lc.head || !(lc.depth == 1 || (lc.depth == 2 && lc.mode >= 0)) {
		return true, refuse("divided PutCounter outside the ability's root target declaration")
	}
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return true, refuse("divided PutCounter without ValidTgts$")
	}
	for _, k := range []string{"TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers", "TgtZone", "Defined", "Choices"} {
		if _, ok := ps.get(k); ok {
			return true, refuse("divided PutCounter with %s$", k)
		}
	}
	f, err := lc.filter(vt)
	if err != nil {
		return true, err
	}
	if f.Players != 0 {
		return true, refuse("divided PutCounter target filter %q names players", vt)
	}
	ps.consume("TgtPrompt")
	minT := 1
	if ps.str("TargetMin") == "0" {
		minT = 0 // castable with no target, which then puts nothing
	}
	t := lc.addTarget(lTarget{F: f, Zone: zBattlefield, Min: cst(minT), Max: cst(n), Divided: n})
	lc.emit(lInstr{Op: "DividedCounters", Obj: lRef{Kind: rTarget, N: t}, Note: "distribute N +1/+1 counters among min..N targets"})
	return true, nil
}

// dividedCountersLKI lowers the dies trigger "distribute X +1/+1 counters
// among any number of target creatures you control, where X is the number of
// +1/+1 counters on CARDNAME" (Quirion Beastcaller; gorge
// cr-dies-trigger-divided-counters-lki): a Dies Self trigger whose PutCounter
// P1P1 reads CounterNum$, DividedAsYouChoose$ and TargetMax$ as one SVar
// TriggeredCard$CardCounters.P1P1 (the last-known counters), TargetMin$ 0.
// OpExt ExtDividedCounters; any other divided counter shape refuses.
func (lc *lowerCtx) dividedCountersLKI(ps *params) error {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "Dies" || lc.depth != 1 || tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Players != 0 ||
		tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
		return refuse("DividedAsYouChoose$ PutCounter outside a Dies Self trigger")
	}
	name := strings.TrimSpace(ps.str("DividedAsYouChoose"))
	if name == "" || ps.str("CounterNum") != name || ps.str("TargetMax") != name || ps.str("TargetMin") != "0" || ps.str("CounterType") != "P1P1" {
		return refuse("divided PutCounter shape (CounterNum$ %q, TargetMax$ %q, TargetMin$ %q, CounterType$ %q)", ps.str("CounterNum"), ps.str("TargetMax"), ps.str("TargetMin"), ps.str("CounterType"))
	}
	if lc.face.SVars[name] != "TriggeredCard$CardCounters.P1P1" {
		return refuse("divided PutCounter total SVar %s=%q", name, lc.face.SVars[name])
	}
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return refuse("divided PutCounter without ValidTgts$")
	}
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	for _, k := range []string{"TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers", "TgtZone", "Defined"} {
		if _, ok := ps.get(k); ok {
			return refuse("divided PutCounter with %s$", k)
		}
	}
	if f.Players != 0 {
		return refuse("divided PutCounter target filter %q names players", vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zBattlefield, Min: cst(0), Max: cst(1), LKIDiv: true})
	lc.emit(lInstr{Op: "DividedCounters", Obj: lRef{Kind: rTarget, N: t}, Note: "distribute the source's last-known +1/+1 counters among 0..X targets"})
	return nil
}

// counterLoseEffect lowers the Effect after a Counter RememberCountered$:
// one Continuous static, Affected$ Card.IsRemembered, RemoveAllAbilities$
// True, kept for as long as the host stays (Duration$ AsLongAsInPlay,
// ForgetOnMoved$ Battlefield) and applied only when the remembered source
// is still an artifact, creature or planeswalker on the battlefield
// (ConditionPresent$). The CounterLose instruction carries it as a
// host-bound LoseAbilities override (Tishana's Tidebinder).
func (lc *lowerCtx) counterLoseEffect(ps *params) error {
	at := lc.clAt - 1
	lc.clAt = -1 // nodeCondition skips this node's Condition*$ once
	// an ordered list, not a map: the first mismatch names the refusal, and
	// map order would make the census nondeterministic
	for _, kv := range [...][2]string{
		{"RememberObjects", "Remembered"},
		{"ForgetOnMoved", "Battlefield"},
		{"ConditionDefined", "Remembered"},
		{"ConditionPresent", "Artifact.inZoneBattlefield,Creature.inZoneBattlefield,Planeswalker.inZoneBattlefield"},
		{"Duration", "AsLongAsInPlay"},
	} {
		if ps.str(kv[0]) != kv[1] {
			return refuse("Counter-lose Effect %s$ %q", kv[0], ps.str(kv[0]))
		}
	}
	name := strings.TrimSpace(ps.str("StaticAbilities"))
	line, ok := lc.face.SVars[name]
	if !ok || strings.Contains(name, ",") {
		return refuse("Counter-lose Effect statics %q", name)
	}
	sw := map[string]string{"Mode": "Continuous", "Affected": "Card.IsRemembered", "RemoveAllAbilities": "True", "AffectedZone": "Battlefield"}
	seen := 0
	for _, part := range strings.Split(line, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "$")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "Description" {
			continue
		}
		if sw[k] != v {
			return refuse("Counter-lose Effect static param %s", k)
		}
		seen++
	}
	if seen != len(sw) {
		return refuse("Counter-lose Effect static shape")
	}
	lc.out.overrides = append(lc.out.overrides, &lOverride{LoseAbilities: true, WhileHost: true})
	lc.ab.Prog[at].C = len(lc.out.overrides) - 1
	return nil
}

// putCounterExisting lowers CounterType$ ExistingCounter (gorge
// putCounterExisting): the controller chooses a kind of counter already on
// the recipient and puts CounterNum$ more of it (Dathon, His Heart Open).
// Only a target or Self recipient and a constant amount of at least 1; the
// each-kind, remember, placer and remembered-set forms refuse by name.
func (lc *lowerCtx) putCounterExisting(ps *params, obj lRef, n lValue) error {
	if ps.is("EachExistingCounter", "True") {
		return refuse("PutCounter ExistingCounter with EachExistingCounter$")
	}
	for _, k := range []string{"RememberPut", "RememberAmount", "Placer"} {
		if _, ok := ps.m[k]; ok {
			return refuse("PutCounter ExistingCounter with %s$", k)
		}
	}
	if obj.Kind == rRemembered {
		return refuse("PutCounter ExistingCounter on the remembered set")
	}
	if !n.isConst() || n.N < 1 {
		return refuse("PutCounter ExistingCounter CounterNum %v", n)
	}
	lc.emit(lInstr{Op: "PutCounterExisting", Obj: obj, B: n})
	return nil
}
