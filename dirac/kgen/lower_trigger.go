package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
)

var phaseSteps = map[string]string{"Upkeep": "Upkeep", "End of Turn": "End", "BeginCombat": "BeginCombat",
	"Draw": "Draw", "Main1": "Main1", "Main2": "Main2",
	// gorge state.ParsePhases is case-insensitive (the corpus writes
	// "End Of Turn"); EndCombat = COMBAT_END, which the kernel fires on
	// entering StepEndCombat (also after a no-attack declare, CR 508.8)
	"End Of Turn": "End", "EndCombat": "EndCombat"}

// whoOfPlayer maps a ValidPlayer-style value to a selector.
func whoOfPlayer(v string) (uint8, bool) {
	switch v {
	case "You":
		return whoYou, true
	case "Opponent", "Player.Opponent", "Player.Other":
		// gorge playerSpecQualifier: Opponent and Other are one qualifier
		return whoOpp, true
	case "Player", "":
		return whoAny, true
	}
	return 0, false
}

// allYouCtrl reports whether every alternative of f is an object you
// control (no player alternatives).
func allYouCtrl(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoYou {
			return false
		}
	}
	return true
}

// lowerTriggered lowers a T: line (or a granted/emblem trigger) as a new
// ability. zone overrides TriggerZones (the emblem's command zone).
// graveTriggerModes are the trigger modes whose kernel walk reads the graveyards.
var graveTriggerModes = map[string]bool{
	"Phase": true, "SpellCast": true, "AbilityCast": true, "CommitCrime": true, "AttackersDeclared": true,
	"FullyUnlock": true, "ChangesZone": true, "ChangesZoneAll": true, "Drawn": true, "Cycled": true, "Discarded": true, "Sacrificed": true, "FacesDilemma": true,
	"RingTemptsYou": true, // the Ring tempts (lower_ring.go; dirac/act_ring.go)
}

// triggerZoneHalves reports a TriggerZones$ comma list that is exactly the
// two-zone Eminence spelling, in either order, as the zone names to lower the
// line once per.
func triggerZoneHalves(z string) ([]string, bool) {
	parts := strings.Split(z, ",")
	if len(parts) != 2 {
		return nil, false
	}
	seen := map[string]bool{}
	for _, p := range parts {
		seen[strings.TrimSpace(p)] = true
	}
	if len(seen) != 2 || !seen["Command"] || !seen["Battlefield"] {
		return nil, false
	}
	return parts, true
}

func (lc *lowerCtx) lowerTriggered(t cards.Trigger, zone string) error {
	if done, err := lc.lowerTriggerSplit(t, zone); done {
		return err
	}
	var descended, dethrone bool // fire-time player gates
	ab := &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities)}
	defer func(b bool) { lc.trigBody = b }(lc.trigBody)
	lc.trigBody = false
	defer func(b bool) { lc.batchPoolOK = b }(lc.batchPoolOK)
	lc.batchPoolOK = false
	// a Melee trigger's pump reads the declaration's capture (value.go)
	defer func(b bool) { lc.mlTrigger = b }(lc.mlTrigger)
	var svars map[string]string
	if lc.face != nil {
		svars = lc.face.SVars
	}
	lc.mlTrigger = isMeleeTrigger(t, svars)
	lc.out.Abilities = append(lc.out.Abilities, ab)
	lc.ab, lc.chainT, lc.mode = ab, nil, -1
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute")
	staticCand, err := staticTriggerCand(tp, t, zone)
	if err != nil {
		return err
	}
	tr := &lTrigger{Event: t.Mode}
	ab.Trigger = tr
	if z, ok := tp.get("TriggerZones"); ok {
		tr.Zone = zoneNames[z]
		if tr.Zone == 0 {
			return refuse("TriggerZones %q", z)
		}
		if tr.Zone == zGraveyard && !graveTriggerModes[t.Mode] && !graveDamageLine(t) {
			return refuse("TriggerZones$ Graveyard over Mode$ %s: the kernel's walk for it reads no graveyard", t.Mode)
		}
	}
	if zone != "" {
		// an Effect's trigger (zone "Command"): gorge registers it as an
		// event-matched delayed promise and never consults its
		// TriggerZones$ (rules/trigger_delayed.go); the registration is
		// the presence, so an absent or Battlefield spelling is the same
		if zoneNames[zone] != tr.Zone && !(zone == "Command" && (tr.Zone == 0 || tr.Zone == zBattlefield)) {
			return refuse("trigger zone %q, want %q", tr.Zone, zone)
		}
		tr.Zone = zoneNames[zone]
	}
	if od, ok := tp.get("OptionalDecider"); ok {
		if od != "You" && od != "TriggeredCardController" && !(od == "TriggeredPlayer" && t.Mode == "Phase") &&
			!(od == "TriggeredAttackingPlayer" && t.Mode == "AttackersDeclared") &&
			!(od == "TriggeredSourceController" && t.Mode == "DamageDone") {
			return refuse("OptionalDecider %q", od)
		}
		// gorge's one read: a "may" that is only a duration-bounded
		// play permission (Strongbox Raider) is no CR 603.5 election
		tr.Optional = scriptfacts.TriggerOptionalSpec(t) != ""
		// the Phase trigger's player is the active player: it answers
		// (gorge deciderFromSpecTriggeredPlayer; Peace in Our Time). An
		// AttackersDeclared trigger's TriggeredAttackingPlayer is the
		// declaring player — the active player too (CR 508.1), the
		// kernel's queue-time trigger player (gorge
		// deciderFromSpecTriggeredAttackingPlayer; the Commander curses)
		tr.OptTrigPlayer = tr.Optional && (od == "TriggeredPlayer" || od == "TriggeredAttackingPlayer")
		// a DamageDone line's "its controller may": the damage source's
		// controller answers (kop.Tf5OptSrcCtrl; checked against the
		// recipient shape once the filters are read, below)
		tr.OptSrcCtrl = tr.Optional && od == "TriggeredSourceController"
		// gorge folds the yes/no into the body's own pay/decline when the
		// controller decides and the body has a non-mandatory cost
		// (rules/trigger_queue.go optionalFoldsIntoCost): one question; a
		// CopySpellAbility body folds too when the trigger carries a
		// triggering card (a SpellCast: Mendicant Core's "pay {1}"). A source
		// triggering on itself makes TriggeredCardController the controller too
		selfDecides := od == "TriggeredCardController" && t.ParamStr(cards.PKValidCard) == "Card.Self"
		if e := t.Effect; tr.Optional && (od == "You" || selfDecides) && e != nil && e.API != "Mana" {
			if c := e.ParamStr(cards.PKCost); c != "" && !strings.HasPrefix(c, "Mandatory") {
				tr.Optional = false
			}
		}
	}
	readF := func(k string) (*lFilter, error) {
		v, ok := tp.get(k)
		if !ok {
			return nil, nil
		}
		if k == "ValidCard" && t.Mode == "SpellCast" && tr.Zone == zStack && strings.TrimSpace(v) == "Card" {
			// a stack-zone SpellCast trigger's bare Card is this card's own
			// cast (gorge trigmatch stackOnlyBareCardCast: Storied Veteran,
			// Hundred-Battle Veteran), the Card.Self carrier the kernel
			// fires from the cast spell itself
			v = "Card.Self"
		}
		v, scopeErr := scopeBareCastFromYourHand(t.Mode, k, tp, v)
		if k == "ValidCard" && (t.Mode == "ChangesZone" || t.Mode == "SpellCast") && !strings.Contains(v, ",") {
			// "if you cast it from your hand" / "a spell from your hand"
			// (Soulhunter Rakshasa, Jodah): read on the cast's stack entry
			// as the event happens, a trigger flag (kernel Tf2CastFromHand)
			if sv, had := stripCastToken(v, "wasCastFromYourHandByYou"); had {
				v, tr.CastHand = sv, true
			}
		}
		if k == "ValidCard" && t.Mode == "SpellCast" && strings.Contains(v, ",") {
			// every alternative requires the cast from your hand by you
			// (Dirgur Focusmage, Lilah): the shared requirement is the one
			// trigger flag, whichever alternative the spell matches
			// a partial spelling falls through to the
			// filter and refuses
			alts := strings.Split(v, ",")
			all := true
			for i, alt := range alts {
				var had bool
				if alts[i], had = stripCastToken(alt, "wasCastFromYourHandByYou"); !had {
					all = false
				}
			}
			if all {
				v, tr.CastHand = strings.Join(alts, ","), true
			}
		}
		return lc.filterUnless(scopeErr, v)
	}
	giftSelf := false
	switch t.Mode {
	case "ChangesZone":
		if raw := strings.TrimSpace(tp.str("Origin")); strings.Contains(raw, ",") {
			// an Origin$ set on a trigger: gorge matches the move's from-zone
			// against the set (trigmatch ZoneChangeMatchesWithCapture). Only
			// the "put into a graveyard from anywhere other than the
			// battlefield" shape lowers, onto the kernel's TrigToGraveyard
			// event fired by the placement itself (dirac/zones.go placeFromK).
			if err := lc.trigToGraveyardSet(tp, tr, t, raw, readF); err != nil {
				return err
			}
			break
		}
		if tr.Origin, err = zoneOf(tp, "Origin", zAny); err != nil {
			return err
		}
		destMask, destSet, err := czDestSet(tp)
		if err != nil {
			return err
		}
		if destSet {
			tr.Dest = zAny
		} else if tr.Dest, err = zoneOf(tp, "Destination", zAny); err != nil {
			return err
		}
		lc.wasCastOK = tr.Dest == zBattlefield // wasCast reads the entering spell
		tr.F, err = readF("ValidCard")
		lc.wasCastOK = false
		if err != nil {
			return err
		}
		if tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fPromisedGift != 0 {
			// "When this enters, if the gift was promised" (Scrapshooter's
			// ValidCard$ Card.Self+PromisedGift): read on the entering
			// permanent as the event happens (gorge matches ValidCard then)
			if tr.F.Players != 0 || tr.F.Clauses[0] != (lClause{Flags: fSelf | fPromisedGift}) || !lc.out.Gift {
				return refuse("gift trigger filter %s", tr.F)
			}
			tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
			giftSelf = true
		}
		switch {
		case destSet:
			ok, err := czPair(tr, destMask, zone, giftSelf, czCostBody(t))
			if err != nil {
				return err
			}
			if !ok {
				return refuse("ChangesZone %s->set", zoneLabel[tr.Origin])
			}
		case tr.Dest == zBattlefield && tr.Origin == zAny:
			tr.Event = "ETB"
		case tr.Origin == zBattlefield && tr.Dest == zGraveyard:
			lowerDies(tp, t, tr)
		case tr.Origin == zBattlefield && tr.Dest == zAny && !giftSelf:
			// leaves the battlefield to any zone (CR 603.6c, 603.10a): the
			// kernel's TrigLeaves fires in fireLeave, the permanent still on
			// the battlefield (gorge changesZoneMatches on the LKI)
			tr.Event = "Leaves"
		default:
			// a destination set on a battlefield leave, a graveyard leave and an
			// Exile origin on an enter (lower_changezonepairs.go)
			ok, err := czPair(tr, destMask, zone, giftSelf, czCostBody(t))
			if err != nil {
				return err
			}
			if !ok {
				return refuse("ChangesZone %s->%s", zoneLabel[tr.Origin], zoneLabel[tr.Dest])
			}
		}
		if tr.Origin == zBattlefield {
			if err := departedFilterRefusal(tr.F); err != nil {
				return err
			}
		}
		if tc, ok := tp.get("TriggerController"); ok && tc != "TriggeredCardController" {
			return refuse("TriggerController %q", tc)
		}
	case "ChangesZoneAll":
		if err := lc.changesZoneAll(tp, tr, t); err != nil {
			return err
		}
	case "Attacks":
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if v, ok := tp.get("Alone"); ok {
			// gorge trigmatch.AttacksMatches: Alone$ True (Exalted's
			// expansion) fires only when exactly one creature was declared
			// as an attacker (declaredAttackerCount, the whole declaration)
			if v != "True" {
				return refuse("Attacks Alone %q", v)
			}
			tr.Alone = true
		}
		if v, ok := tp.get("Training"); ok {
			// K:Training's expansion (gorge kwTraining; trigmatch
			// AttacksMatches): fires only when the source attacks with
			// another attacker of strictly greater power
			if v != "True" || tp.str("ValidCard") != "Card.Self" {
				return refuse("Attacks Training %q", v)
			}
			tr.Training = true
		}
		// K:Dethrone's expansion or Condition$ AttackedPlayerWithMostLife
		if dethrone, err = dethroneOrMostLifeGate(tp, tr); err != nil { // lower_dethrone.go
			return err
		}
		if v, ok := tp.get("Myriad"); ok {
			// K:Myriad's expansion (gorge kwMyriad; trigmatch
			// AttacksMatchedAttackers fires it only over its DB$ Myriad
			// Execute$ body; lower_myriad.go)
			if err := myriadMarker(v, tp.str("ValidCard"), cards.ResolveSVar(svars, t.Params["Execute"])); err != nil {
				return err
			}
		}
		if v, ok := tp.get("Attacked"); ok {
			// gorge attackedDefenderMatches, from the trigger controller's
			// seat: the player / your-planeswalker alternatives the kernel
			// answers exactly; the rest refuse
			for _, alt := range strings.Split(v, ",") {
				switch strings.TrimSpace(alt) {
				case "You":
					tr.Attacked |= atkYou
				case "Planeswalker.YouCtrl":
					tr.Attacked |= atkYourPW
				case "Opponent", "Player.Opponent":
					tr.Attacked |= atkOpp
				case "Player":
					tr.Attacked |= atkYou | atkOpp
				case "Player.EnchantedBy":
					// gorge attackedDefenderMatches over MatchesPlayerSpec:
					// the attacked player is enchanted by THIS listener's Aura
					// (playerSpecQualifierEnchantedBy), the Attacks twin of
					// AttackedTarget$ Player.EnchantedBy (Tf4DefEnch). Alone
					// only: a comma alternative would need a second gate.
					if strings.Contains(v, ",") {
						return refuse("Attacks Attacked %q", v)
					}
					tr.Attacked |= atkEnch
				case "Player.withMostLife":
					// gorge attackedDefenderMatches over MatchesPlayerSpec:
					// the attacked player has the most life or is tied
					// (playerHasMost Life), read at the declaration only.
					// seat-exact: the gate is dethroneCond's comparison over
					// the defended seat (kop.RefDefendingPlayer), not the one
					// opponent (Preacher of the Schism, Undercover Butler).
					if strings.Contains(v, ",") {
						return refuse("Attacks Attacked %q", v)
					}
					tr.Attacked |= atkOpp
					dethrone = true
				default:
					return refuse("Attacks Attacked %q", v)
				}
			}
		}
	case "AttackerBlocked":
		// gorge checkAttackerBlockedTriggers: once per distinct blocked
		// attacker matching ValidCard$; ValidBlocker$ Card.Self ("whenever
		// this blocks a creature"), only the attackers the source blocks;
		// any other ValidBlocker$ is a filter over the attacker's blockers
		// (lower_modeparams.go blockerScope); ValidBlockerAmount$ refuses
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if err := lc.blockerScope(tp, tr, readF); err != nil {
			return err
		}
	case "AttackerUnblocked":
		// gorge checkAttackerUnblockedTriggers: at the declare-blockers
		// round's completion, once per unblocked attacker matching
		// ValidCard$ (Tf2Unblocked on the become-blocked subscription);
		// ValidDefender$ stays unread, a zone other than the battlefield
		// refuses
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("AttackerUnblocked TriggerZones %d", tr.Zone)
		}
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		tr.Event = "AttackerBlocked"
		tr.Unblocked = true
	case "Cycled":
		// gorge cycledMatches on the cycling cost's discard: only "when you
		// cycle this card" (ValidCard$ Card.Self, no TriggerZones$: it
		// fires from wherever the discard put it, trigger_zone_gate.go)
		// cycle this card" (ValidCard$ Card.Self, no TriggerZones$: it
		// fires from wherever the discard put it), or a battlefield
		// "whenever you/a player cycle(s) a card" over the cycled card
		// and its owner (ValidPlayer$ You); FirstTime$ refuses. Other is no
		// restriction: the cycled card is in the graveyard it was discarded
		// to, never the battlefield listener itself (Drannith Stinger)
		vc := tp.str("ValidCard")
		self := vc == "Card.Self" && tr.Zone == 0
		if !self && (tr.Zone != zBattlefield || strings.Contains(vc, "Self")) {
			return refuse("Cycled ValidCard %q TriggerZones %d", vc, tr.Zone)
		}
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if vp, ok := tp.get("ValidPlayer"); ok {
			if vp != "You" {
				return refuse("Cycled ValidPlayer %q", vp)
			}
			tr.Whose = whoYou
		}
		tr.Event, tr.Cycled = "Discarded", true
		if self {
			tr.Zone = zGraveyard
		}
	case "AttackerBlockedByCreature":
		if err := lc.blockedByCreature(tp, tr); err != nil {
			return err
		}
	case "Taps":
		// gorge tapsMatches: every real Tap event (not an entry-tapped
		// state) of a permanent matching ValidCard$, mana taps included (a
		// mana dork's {T}). Only "whenever this becomes tapped" of a card
		// with no mana ability, and two non-Self shapes, are lowered: the
		// kernel fires TrigTaps where a permanent is tapped (attack, {T} and
		// tap-a-creature costs, Tap effects, regeneration, and a granted
		// mana ability's {T}: gen manaTaps). A non-Self ValidCard$ lowers as
		//   - "whenever you tap an untapped creature an opponent controls"
		//     (ValidPlayer$ You, every clause OppCtrl): ValidPlayer$ is the
		//     TAPPER (gorge TapActor), which the kernel carries in fireTapsBy,
		//     and a mana tap can never match it (you tap only your own
		//     permanents for mana), or
		//   - a watcher over your own creatures (every clause YouCtrl) or
		//     over the equipped creature (every clause EquippedBy), with no
		//     ValidPlayer$ (any tapper): Captain America, Living Legend's
		//     Creature.YouCtrl, Hawkeye's Bow's Creature.EquippedBy, and
		//     Cryoshatter's Card.AttachedBy (a destroy-self or poison-counter
		//     body is vetted ordinary, tapsAttachedClass; an unvetted body
		//     defers the tap).
		//     A FirstTime$ True line (Captain America, Living Legend) is the
		//     same watcher with the tapped-this-turn mark consulted first.
		// FirstTime$ True ("the first time it has become tapped this turn",
		// gorge Facts.TappedTurn) reads the arena's per-slot tapped-this-turn
		// mark (mechTappedTurn, filter_permmarks.go): every real tap sets it
		// after its Taps triggers, and a watcher entering later still sees an
		// earlier tap. FirstTime$ on self and non-Self lines both lower.
		if v, ok := tp.get("FirstTime"); ok {
			if v != "True" {
				return refuse("Taps FirstTime$ %q", v)
			}
			tr.First = true
		}
		if vc := tp.str("ValidCard"); vc != "Card.Self" {
			if tr.F, err = readF("ValidCard"); err != nil {
				return err
			}
			if tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) == 0 {
				return refuse("Taps ValidCard %q", vc)
			}
			if vp := tp.str("ValidPlayer"); vp != "You" {
				if vp != "" {
					return refuse("Taps ValidPlayer %q", vp)
				}
				// AttachedBy lowers to the same flag as EquippedBy
				// (filter.go): the enchanted-permanent watcher (tapsAttachedClass,
				// tapsAttachedClauses).
				tp8, pn, destroySelf := tapsAttachedClass(t)
				if !tapsAttachedClauses(tr.F, vc, tp8, destroySelf) {
					return refuse("Taps ValidCard %q", vc)
				}
				tr.TapPlan, tr.TapPainN = uint8(tp8), pn
				break
			}
			for _, c := range tr.F.Clauses {
				if c.Ctrl != whoOpp {
					return refuse("Taps ValidCard %q", vc)
				}
			}
			tr.Whose = whoYou
			break
		}
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if lc.granted != 0 {
			// A granted self Taps watcher (an AddTrigger$ grant: Tale of Katara
			// and Toph's "creatures you control have 'whenever this creature
			// becomes tapped for the first time during each of your turns, put
			// a +1/+1 counter on it'") fires for each recipient's own tap, not
			// the granting face's: the tapped source's class is the body's
			// (tapsPlanClass — gorge's planner classes the recipient,
			// pay.PaymentPlanAbilityTier), and the granting face's own mana
			// abilities read nothing for it.
			tp8, pn := tapsPlanClass(t.Effect)
			tr.TapPlan, tr.TapPainN = uint8(tp8), pn
			break
		}
		for _, a := range lc.face.Abilities {
			if a.API == "Mana" || a.API == "ManaReflected" {
				// The card's own {T} for mana fires its own Taps line (CR 603.2,
				// 605.1b). gorge's planner prices the source only for the vetted
				// "deals N damage to you" body (City of Brass, last resort at N;
				// pay.PaymentPlanSelfDamageTrigger): any other body defers the
				// source in gorge's census, a tier the kernel does not mirror.
				n, ok := selfTapsPain(t)
				if ok {
					for _, m := range lc.face.Abilities {
						ok = ok && (m.API != "Mana" && m.API != "ManaReflected" || selfPainPlainMana(m))
					}
				}
				if !ok {
					return refuse("Taps trigger on a card with a mana ability whose body is not a literal \"deals N damage to you\" or a plain {T} mana ability")
				}
				tr.TapPlan, tr.TapPainN = uint8(kop.TapPlanPain), n
				break
			}
		}
	case "TapsForMana":
		if err := lc.tapsForManaWatch(tp, tr, t, readF); err != nil {
			return err
		}
	case "Blocks":

		// gorge checkBlocksTriggers/blocksCandidates: one instance per
		// declared (attacker, blocker) pair, ValidCard$ read against the
		// blocker, ValidBlocked$ against the attacker; the trigger object
		// is the attacker (kernel fireBlocksGen)
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if tr.F2, err = readF("ValidBlocked"); err != nil {
			return err
		}
		if tr.F2 != nil && (tr.F2.Players != 0 || len(tr.F2.Clauses) == 0) {
			return refuse("Blocks ValidBlocked %s", tr.F2)
		}
	case "AttackersDeclared", "AttackersDeclaredOneTarget":
		if t.Mode == "AttackersDeclaredOneTarget" || tp.m["AttackedTarget"] != "" {
			// gorge: one DeclareAttackers event per defender, the line
			// matched per event (AttackersDeclaredMatchedAttackers'
			// AttackedTarget$ read): only the player forms lower
			// (Tf2DefPlayer, the player-defender group)
			tr.Event = "AttackersDeclared"
			switch at := tp.str("AttackedTarget"); at {
			case "Player":
				tr.DefPlayer = 1
			case "Opponent":
				tr.DefPlayer = 2
			case "You":
				tr.DefPlayer = 3 // the defending player is the listener's controller (gorge attackedDefenderMatches)
			case "Player.EnchantedBy":
				// gorge playerSpecQualifierEnchantedBy: the defender is
				// enchanted by THIS listener's Aura (Forge's
				// player.isEnchantedBy(source), not any enchantment). The
				// kernel's fireDefPlayer reads the listener's own atchSeat
				// (Tf4DefEnch, dirac/trig_defendingplayer.go).
				tr.DefPlayer = 4
			default:
				return refuse("AttackersDeclared AttackedTarget %q", at)
			}
		}
		if tr.F, err = readF("ValidAttackers"); err != nil {
			return err
		}
		// gorge AttackersDeclaredMatchedAttackers: AttackingPlayer$ is a
		// player spec on the attacking player (absent: any); the kernel
		// counts the attackers matching ValidAttackers$ (attackersMatching)
		ap := tp.str("AttackingPlayer")
		w, ok := whoOfPlayer(ap)
		if !ok {
			return refuse("AttackingPlayer %q", ap)
		}
		if ap == "" && allYouCtrl(tr.F) {
			w = whoYou // only the controller's own attack can match
		}
		tr.Whose = w
		if a, ok := tp.get("ValidAttackersAmount"); ok {
			// gorge ComparePresent over the matching attackers of one
			// defender's declaration: the six operators, a literal 1..127
			cmp, n, ok := trigCmp(a)
			if !ok {
				return refuse("ValidAttackersAmount %q", a)
			}
			if cmp != uint8(kop.DmgGE) {
				tr.AtkCmp = cmp
			}
			tr.MinAttackers = n
		}
	case "Phase":
		ph := tp.str("Phase")
		if pc, ok := tp.get("PhaseCount"); ok {
			// gorge phaseGate: PhaseCount$ N is the Nth member of the Phase$
			// set in turn order (Fireglass Mentor: Main, 2 = the second main)
			switch {
			case ph == "Main" && pc == "1":
				ph = "Main1"
			case ph == "Main" && pc == "2":
				ph = "Main2"
			default:
				return refuse("Phase %q PhaseCount %q", ph, pc)
			}
		}
		step, ok := phaseSteps[ph]
		if !ok {
			return refuse("phase %q", ph)
		}
		tr.Step = step
		w, ok := whoOfPlayer(tp.str("ValidPlayer"))
		if tp.str("ValidPlayer") == "Player.EnchantedController" {
			// gorge playerEnchantedController: the bearer's controller
			w, ok = whoEnchCtrl, true
		}
		if tp.str("ValidPlayer") == "Player.isMonarch" {
			w, ok = whoMonarch, true // lower_monarch.go: the monarch's turn
		}
		if tp.str("ValidPlayer") == "Player.Chosen" && lc.granted == 0 && tr.Zone == zBattlefield && entersChoosingPlayer(lc.face) {
			// the opponent the source chose as it entered (lower_chooseplayerenter.go)
			w, ok = whoChosen, true
		}
		w, ok = lc.phaseEnchantedBy(tp, tr, w, ok) // lower_enchantedby.go
		if tp.str("ValidPlayer") == "You.descended" {
			// gorge playerSpecQualifierDescended (CR 700.11): a permanent
			// card entered your graveyard this turn, read as the trigger
			// matches (Ruin-Lurker Bat)
			w, ok, descended = whoYou, true, true
		}
		if !ok {
			return refuse("ValidPlayer %q", tp.str("ValidPlayer"))
		}
		tr.Whose = w
	case "SpellCast", "SpellCastOrCopy":
		if t.Mode == "SpellCastOrCopy" {
			// gorge trigmatch: the cast (PutOnStack) or a spell copy
			// (StackCopy) matches; the cast-count gates read casts only
			tr.Event, tr.OrCopy = "SpellCast", true
			for _, k := range []string{"ActivatorThisTurnCast", "TargetsValid", "OpponentTurn"} {
				if _, ok := tp.get(k); ok {
					return refuse("SpellCastOrCopy with %s$", k)
				}
			}
		}
		if v, ok := tp.get("Static"); ok {
			// gorge resolves a Static$ True SpellCast trigger off the
			// stack by the trigger drain, before the CR 603.3b ordering
			// ask (engine/rules/static_trigger.go): no priority pass, no
			// order choice. Only a body PROVABLY ask-free may resolve off
			// the stack — gorge's staticTriggerAskFree refuses a line
			// with an optional/unless/cost rider, a target, a Charm or an
			// asking body, and so does the body check after the chain
			// lowers. Every other mode keeps refusing
			// Static$ as unread.
			if t.Mode != "SpellCast" || v != "True" {
				return refuse("%s Static$ %q", t.Mode, v)
			}
			for _, k := range [...]string{"OptionalDecider", "Optional", "UnlessCost", "UnlessSwitch", "UnlessResolveSubs", "UnlessPayer", "Cost"} {
				if strings.TrimSpace(t.Params[k]) != "" {
					return refuse("static SpellCast trigger %s$ %q (an off-stack resolution cannot ask)", k, t.Params[k])
				}
			}
			tr.Static = true
		}
		lc.spellProvOK = true // the cast-provenance words read the spell cast
		// a "Chosen" bound reads the number the source chose as it entered (lower_choosenumber.go)
		lc.chosenNumOK = lc.granted == 0 && tp.str("TriggerZones") == "Battlefield" && entersChoosingNumber(lc.face)
		tr.F, err = readF("ValidCard")
		lc.spellProvOK, lc.chosenNumOK = false, false
		if err != nil {
			return err
		}
		w, ok := whoOfPlayer(tp.str("ValidActivatingPlayer"))
		if !ok {
			w, ok = castNonActive(tr, t.Mode, tp.str("ValidActivatingPlayer"))
			if ok && tp.is("OpponentTurn", "True") {
				return refuse("ValidActivatingPlayer %q with OpponentTurn$", tp.str("ValidActivatingPlayer"))
			}
		}
		if !ok {
			return refuse("ValidActivatingPlayer %q", tp.str("ValidActivatingPlayer"))
		}
		tr.Whose = w
		if tp.is("OpponentTurn", "True") {
			tr.OnlyOppTurn = true
		}
		if v, ok := tp.get("ValidSA"); ok {
			// "whenever you cast a spell using mana produced by CARDNAME"
			// (gorge trigmatch validSAManaFromCardSelf): the cast's payment
			// tapped this very source. Only the exact spelling, on a plain
			// SpellCast of a permanent source (kernel Tf2ManaFromSelf).
			// "Spell.ManaSpent <op><n>" is the spend gate (spentGateOf).
			// "Spell.MayPlaySource" (gorge validSAMayPlaySourceClause) reads
			// the cast's FlagMayPlay: the may-play permission provenance,
			// the flag-only approximation of Forge's host-specific
			// MayPlaySource. It lowers as kernel
			// Tf3MayPlaySrc, a trigger flag read off the spell's stack
			// entry as the trigger fires — a filter clause would be
			// dropped by fireSelfCast's TfSelf filter stripping, so one
			// flag covers the Card.Self carriers (Undead Sprinter) too.
			if took, err := lc.validSAHead(v, t.Mode, tr); took {
				if err != nil {
					return err
				}
			} else if gate, ok := spentGateOf(v, false); ok && t.Mode == "SpellCast" {
				tr.SpentGate = gate
			} else if v == "Spell.MayPlaySource" && t.Mode == "SpellCast" {
				tr.MayPlaySrc = true
			} else if v != "Spell.ManaFromCard.StrictlySelf" || t.Mode != "SpellCast" {
				return refuse("spell-ability filter ValidSA %q", v) // the T12 spell-ability filter guard (T13c's words)
			} else {
				tr.ManaFromSelf = true
			}
		}
		if v, ok := tp.get("ValidSAonCard"); ok {
			// gorge spellValidSAonCardMatches: the same mana-spent
			// comparison over the cast spell, X naming the spell's printed
			// mana value. Only that spelling, on a plain SpellCast, as the
			// sole spend gate (kernel spentGateOK).
			gate, ok := spentGateOf(v, true)
			if !ok || t.Mode != "SpellCast" || tr.SpentGate != 0 {
				return refuse("ValidSAonCard %q on %s", v, t.Mode)
			}
			tr.SpentGate = gate
		}
		if v, ok := tp.get("ActivatorThisTurnCast"); ok {
			// gorge CompareIntCount over activatorCastCount (the caster's
			// spells this turn that match ValidCard$, this one included): the
			// six operators, a literal 1..127
			cmp, n, ok := trigCmp(v)
			if !ok {
				return refuse("ActivatorThisTurnCast %q", v)
			}
			switch vc := t.ParamStr(cards.PKValidCard); {
			case vc == "" || vc == "Card" || vc == "Card.YouCtrl":
			case tr.CastHand || strings.Contains(vc, "wasCast") || strings.Contains(vc, "CastSa"):
				return refuse("ActivatorThisTurnCast over cast provenance %q", vc)
			default:
				tr.NthFiltered = true
				for _, c := range tr.F.Clauses {
					if why := castRingClauseInexact(c); why != "" {
						return refuse("ActivatorThisTurnCast over ValidCard %q: %s", vc, why)
					}
				}
			}
			if cmp != uint8(kop.DmgEQ) {
				tr.NthCmp = cmp
			}
			tr.NthSpell = n
		}
		if v, ok := tp.get("TargetsValid"); ok {
			if tr.OrTargets {
				return refuse("TargetsValid beside an IsTargeting ValidSA")
			}
			// gorge TargetShapeMatches / targetMatchesTargetsValid: the
			// trigger fires if ANY target of the spell's chain matches ANY
			// comma alternative, a player target through the player spec,
			// an object target through the object spec. Lowered for the
			// object alternatives that are on the battlefield (Card.Self,
			// or an explicit inZoneBattlefield), a zone-free type
			// alternative (Danitha, Spear of Agony's Creature.OppCtrl, which
			// also matches a creature SPELL on the stack: effects
			// matchesBase reads the type in any zone), and one You or
			// Opponent player alternative (kernel castTargetsPlayer). The
			// zone-free alternatives carry kop.ZoneAny on Filter2, the marker
			// the kernel's castTargetsAll reads for an off-battlefield
			// target through the card matcher. A line mixing a battlefield
			// and a zone-free object alternative stays refused: the kernel
			// would read the battlefield one in any zone too.
			if err := lc.targetsValid(tr, v, false); err != nil {
				return err
			}
		}
		if v, ok := tp.get("Increment"); ok {
			// gorge kwIncrement's Increment$ marker (trigmatch
			// IncrementHolds): the source is a creature and the cast's spend
			// exceeds its current power/toughness, read as the trigger fires
			// and rechecked on resolution (kernel Tf2Increment). Only the exact spelling, on a
			// plain SpellCast (gorge's SpellCastOrCopy carries it on the cast
			// event only; the kernel's copy arm reads casts only).
			if t.Mode != "SpellCast" || v != "True" {
				return refuse("Increment %q on %s", v, t.Mode)
			}
			tr.Increment = true
		}
	case "LifeGained", "LifeLost":
		w, ok := whoOfPlayer(tp.str("ValidPlayer"))
		if !ok {
			return refuse("ValidPlayer %q", tp.str("ValidPlayer"))
		}
		tr.Whose = w
		if tp.is("FirstTime", "True") {
			tr.First = true
		}
	case "DamageDoneOnce":
		if err := lc.damageDoneOnce(tp, tr, t); err != nil {
			return err
		}
	case "DamageAll":
		// gorge damageBatchKey's DamageAll latch keys on the trigger line
		// alone (trigger_match.go all: true), so the line fires once per
		// damage batch in which a matching source dealt damage to a matching
		// target, no matter how many opponents the batch hit. The kernel
		// reads that as a batch-level DamageDone (Tf2Batch without
		// Tf2PerReferent): dmgBatchOnce/dmgBatchSeen latch the line for the
		// whole sweep. Any other target (an object filter can match several
		// referents) stays refused. ExcessDamageAll is not this mode.
		switch vt := tp.str("ValidTarget"); vt {
		case "Opponent", "You":
		default:
			return refuse("DamageAll ValidTarget %q", vt)
		}
		if err := lc.damageDoneOnce(tp, tr, t); err != nil {
			return err
		}
		// gorge's DamageAll latch is the batch-level "one or more" mode
		// (trigger_match.go damageBatchKey): one trigger per damage batch,
		// not one per damaged referent. Keep Tf2Batch, drop Tf2PerReferent.
		tr.PerRef = false
	case "DamageDone", "DamageDealtOnce":
		if tr.SrcInPlay, err = srcInPlay(tp.str("ValidSource")); err != nil {
			return err
		}
		lc.spellBaseOK = tr.Zone == zGraveyard // lower_gravedamage.go drops the head
		tr.F, err = readF("ValidSource")
		lc.spellBaseOK = false
		if err != nil {
			return err
		}
		if tr.Zone == zGraveyard {
			if err := dropSpellSourceHead(tr.F); err != nil {
				return err
			}
		}
		if err := lc.damageTargetSpec(tp, tr, t, readF); err != nil { // lower_enchantedby.go
			return err
		}
		if cd, ok := tp.get("CombatDamage"); ok {
			if cd == "True" {
				tr.Combat = 1
			} else {
				tr.Combat = -1
			}
		}
		if err := lc.damageAmountGate(tp, tr, t); err != nil { // lower_damageamount.go
			return err
		}
		if err := lc.damageDoneSelfObjRef(tp, tr, t); err != nil {
			return err
		}
		if tr.OptSrcCtrl && (tr.Event != "DamageDone" || !dmgSourceIsTriggerObj(tr)) {
			return refuse("OptionalDecider %q on a per-referent DamageDone line", "TriggeredSourceController")
		}
	case "BecomesTarget":
		switch {
		case tp.is("Ward", "True") && tp.str("ValidTarget") == "Card.Self":
			tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
			tr.Event = "BecomesTargetByOpp"
		case tp.is("Valiant", "True") && tp.str("ValidTarget") == "Card.Self" &&
			tp.str("ValidSource") == "SpellAbility.YouCtrl":
			// Valiant (CR 603.2 ability word; gorge
			// trigmatch.valiantFirstTimeTargetedByYouThisTurn): the first
			// targeting this turn by a spell or ability its controller
			// controls; the kernel's per-turn mark is in kw_valiant.go
			tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
			tr.Valiant = true
		case tp.str("ValidSource") == "SpellAbility.OppCtrl" &&
			tp.str("ValidTarget") == "Card.Self" &&
			!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True"):
			// "Whenever this becomes the target of a spell or ability an
			// opponent controls" (CR 603.2; gorge trigmatch
			// becomesTargetMatches with ValidTarget$ Card.Self and
			// ValidSource$ SpellAbility.OppCtrl). This is the watcher form
			// whose filter is Self: gorge matches ValidTarget$ against the
			// TargetsChosen event's chosen Ids, so the trigger object is the
			// targeted permanent itself (becomesTargetReferent). The kernel
			// fires it through btWatchers, whose objTrigGen self match
			// restricts a TfSelf line to the targeted permanent
			// (kernel trig_becomestarget.go). FirstTime$ would need
			// gorge's per-object once-a-turn gate, which the kernel does not
			// model here: refuse it rather than fire every time.
			tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
			tr.Whose = whoOpp
			tr.Watch = true
		case tp.str("ValidSource") == "SpellAbility.OppCtrl" &&
			tp.str("ValidTarget") != "" && !strings.Contains(tp.str("ValidTarget"), "Card.Self"):
			// a watcher (Pawpatch Recruit, Surrak, Elusive Hunter): each
			// object an opponent's spell or ability targets that matches
			// ValidTarget$ (gorge becomesTargetMatches, one TargetsChosen
			// event per chosen target); the trigger object is the targeted
			// one (gorge becomesTargetReferent). A creature-spell
			// alternative (Spell.Creature+YouCtrl) is a flag on the stack
			// target walk (kernel Tf2SpellToo).
			var alts []string
			for _, a := range strings.Split(tp.str("ValidTarget"), ",") {
				if a == "Spell.Creature+YouCtrl" {
					tr.SpellToo = true
					continue
				}
				alts = append(alts, a)
			}
			if len(alts) == 0 {
				return refuse("BecomesTarget shape")
			}
			if tr.F, err = lc.btTargetList(tr, alts); err != nil { // lower_bt_source.go
				return err
			}
			tr.Whose = whoOpp
			tr.Watch = true
		case isBTSpellSourceLine(tp):
			// the spell source forms: lower_bt_source.go
			if err := lc.lowerBTSpellSource(tp, tr); err != nil {
				return err
			}
		case tp.str("ValidSource") == "" && tp.str("ValidTarget") == "Card.Self" &&
			!tp.is("Ward", "True") && !tp.is("Valiant", "True"):
			// "Whenever this becomes the target of a spell or ability" with
			// no source restriction (CR 603.2; gorge becomesTargetMatches
			// with ValidTarget$ Card.Self and no ValidSource$): every
			// TargetsChosen naming the permanent fires it, its controller's
			// own included. A TfSelf line with no Whose gate: the kernel's
			// becameTarget fires it through the plain per-object walk
			// (fireObjGen), no opponent gate. FirstTime$ True is this
			// shape's once-a-turn rider (btSelfFirstTime); any other shape
			// carrying it falls to the default refuse.
			tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
			if err := btSelfFirstTime(tp, tr); err != nil { // lower_bt_source.go
				return err
			}
		case tp.str("ValidSource") == "" && tp.str("ValidTarget") != "" &&
			!strings.Contains(tp.str("ValidTarget"), "Card.Self") &&
			!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True"):
			// the watcher with no source restriction (Daru Spiritualist:
			// "whenever a Cleric creature you control becomes the target of
			// a spell or ability"): the opponent-source watcher's arm
			// without its Whose gate; one trigger per matching chosen
			// target, the targeted permanent as the trigger object
			// (gorge becomesTargetReferent). ValidTarget$ may be a comma
			// list of permanent filters: the OR of the alternatives (gorge
			// matches the whole ValidTarget$ against each chosen target).
			if tr.F, err = lc.btTargetList(tr, strings.Split(tp.str("ValidTarget"), ",")); err != nil { // lower_bt_source.go
				return err
			}
			tr.Watch = true
		default:
			return refuse("BecomesTarget shape")
		}
	case "Drawn":
		if err := lc.drawnScope(tp, tr); err != nil { // lower_modeparams.go
			return err
		}
		if n, ok := tp.get("Number"); ok {
			tr.NthDraw, _ = strconv.Atoi(n)
		}
		if err := lowerDrawnFirstInDrawStep(tp, tr); err != nil {
			return err
		}
	case "Discarded":
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if tp.m["ValidCard"] == "Card.Self" {
			// "When you discard this card" (gorge trigger_zone_gate.go): a
			// line without TriggerZones$ fires on its own card's discard
			// wherever it lands; the kernel fires it from the graveyard
			// (kernel fireSelfDisc)
			if zone != "" || tr.Zone != 0 && tr.Zone != zGraveyard {
				return refuse("Discarded Card.Self TriggerZones %q", tp.str("TriggerZones"))
			}
			tr.Zone = zGraveyard
		}
		if err := lowerDiscardedCause(tp, tr); err != nil { // lower_trigparams.go
			return err
		}
	case "DiscardedAll":
		// "whenever you discard one or more cards" (gorge trigmatch
		// discardedAllMatches, rules discard batch): once per discard
		// action, amt = the cards of the batch (kernel trig_discardedall.go).
		// ValidPlayer$ is the discarding player; ValidCard$ the card.
		for _, k := range []string{"FirstTime", "ValidCause", "Random"} {
			if _, ok := tp.get(k); ok {
				return refuse("DiscardedAll %s$", k)
			}
		}
		if zone != "" {
			return refuse("DiscardedAll trigger on an Effect")
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("DiscardedAll TriggerZones %q", tp.str("TriggerZones"))
		}
		switch vp := tp.str("ValidPlayer"); vp {
		case "You":
			tr.Whose = whoYou
		case "Player":
			tr.Whose = whoAny
		default:
			return refuse("DiscardedAll ValidPlayer$ %q", vp)
		}
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
	case "CounterAddedOnce", "CounterAdded":
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if t.Mode == "CounterAddedOnce" {
			// gorge counterAddedMatches: FirstTime$ True is the once-a-turn
			// latch over the same CounterChange batch event (kernel
			// Tf2FirstTime, trig_counterfirst.go). The latch is per SOURCE
			// (usedThisTurn), so it carries only the Card.Self reading
			// ("the first time each turn", U.S.S. Enterprise-D, Axgard
			// Artisan); Botanical Brawler's "the first time ... on that
			// permanent" is a per-RECIPIENT latch the kernel does not have,
			// so it refuses. The per-counter CounterAdded shape has no such
			// latch at all: leave the param unread there so the census
			// refuses it.
			if v, ok := tp.get("FirstTime"); ok {
				if v != "True" {
					return refuse("CounterAddedOnce FirstTime$ %q", v)
				}
				if vc := tp.str("ValidCard"); vc != "Card.Self" {
					return refuse("CounterAddedOnce FirstTime$ on ValidCard$ %q", vc)
				}
				tr.First = true
			}
		}
		ct := tp.str("CounterType")
		if ct == "" {
			// gorge counterAddedMatches: an absent CounterType$ admits every
			// counter kind. Only the per-counter ActivationLimit$ 1 shape
			// lifts (the kernel's any-kind TrigNAnyCounter event); the
			// crossing (kind-specific) and CounterAddedOnce stay refused.
			if t.Mode != "CounterAdded" {
				return refuse("%s without CounterType", t.Mode)
			}
			if tp.str("CounterAmount") != "" {
				return refuse("CounterAdded CounterAmount$ without CounterType")
			}
			ct = "*"
		}
		tr.Step = ct
		if t.Mode == "CounterAdded" {
			// gorge counterAddedMatches: a plain line is one trigger PER
			// counter (CR 603.2c), which this lowering does not carry; only
			// the crossing shape, "when the Nth counter is put on", fires
			// once, and the per-counter shape with ActivationLimit$ 1
			// ("... This ability triggers only once each turn"): since
			// gorge's per-counter loop re-checks both limit gates
			// for every extra instance, so the first counter queues the one
			// trigger of the turn and the rest are refused. That is the
			// kernel's one-per-batch event under TfOncePerTurn (set below),
			// provided the body does not read the per-counter Amount (the
			// first instance carries TriggerAmount 1, the batch event its
			// whole size).
			cmp := tp.str("CounterAmount")
			if cmp == "" {
				if al, ok := tp.get("ActivationLimit"); !ok {
					// a plain line: one trigger per counter of the batch
					// (CR 603.2c), each carrying amount 1 (gorge queues one
					// instance per counter), so a body reading the amount
					// reads 1 as on gorge. Only a named counter kind
					// (kernel Tf4PerCounter reads the event's one kind).
					if ct == "*" {
						return refuse("CounterAdded per-counter (no CounterAmount$) of any counter kind")
					}
					tr.PerCounter = true
				} else if al != "1" {
					return refuse("CounterAdded per-counter (no CounterAmount$)")
				} else if lc.saReadsBatch(t.Effect) {
					return refuse("CounterAdded per-counter body reads the trigger")
				}
			} else {
				n, perr := strconv.Atoi(strings.TrimPrefix(cmp, "EQ"))
				if !strings.HasPrefix(cmp, "EQ") || perr != nil || n < 1 || n > 255 {
					return refuse("CounterAdded CounterAmount$ %q", cmp)
				}
				if _, ok := tp.get("ActivationLimit"); ok {
					return refuse("CounterAdded crossing with ActivationLimit$")
				}
				tr.CrossAt = n
			}
		}
		if vs, ok := tp.get("ValidSource"); ok && vs != "You" {
			return refuse("CounterAddedOnce ValidSource %q", vs)
		} else if ok {
			tr.Whose = whoYou
		}
	case "CounterAddedAll", "CounterPlayerAddedAll", "CounterRemoved", "CounterRemovedOnce":
		if err := lc.ctrRemovedOrPlayerAddedAll(tp, tr, t, readF); err != nil { // lower_counterremoved.go
			return err
		}
	case "Untaps":
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
	case "ManaExpend":
		// gorge trigmatch.ManaExpendMatches: Player$ (omitted = You) is the
		// spender; Amount$ a literal (an SVar or Count$ amount refused)
		if p, ok := tp.get("Player"); ok && p != "You" {
			return refuse("ManaExpend Player %q", p)
		}
		n, err := strconv.Atoi(tp.str("Amount"))
		if err != nil || n <= 0 {
			return refuse("ManaExpend Amount %q", tp.str("Amount"))
		}
		tr.Whose, tr.Amount = whoYou, n
	case "Sacrificed", "SacrificedOnce":
		// gorge sacrificedMatches: ValidCard$ against the sacrificed
		// permanent's last known information (the kernel fires fireSac
		// while it is still on the battlefield), ValidPlayer$ against the
		// sacrificing player, its controller as it was sacrificed: the same
		// as a controller term on every ValidCard$ clause
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if tr.F != nil && tr.F.Players != 0 {
			return refuse("Sacrificed ValidCard %s", tr.F)
		}
		if vp, ok := tp.get("ValidPlayer"); ok {
			w, ok := whoOfPlayer(vp)
			if !ok || vp == "" {
				return refuse("Sacrificed ValidPlayer %q", vp)
			}
			self := tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Clauses[0] == lClause{Flags: fSelf}
			if w != whoAny && !(self && w == whoYou) {
				if tr.F == nil {
					tr.F = &lFilter{Clauses: []lClause{{}}}
				}
				for i := range tr.F.Clauses {
					c := &tr.F.Clauses[i]
					if w == whoYou && len(tr.F.Clauses) > 1 && *c == (lClause{Flags: fSelf}) {
						// gorge counts a trigger's per-turn limit per object
						// stint (trigger_match.go triggerTurnFiresNow), so a
						// source sacrificed itself after the limit was used
						// triggers again: Kingpin, Wilson Fisk diverges from
						// the rules and from the kernel
						for _, lim := range [...]string{"ActivationLimit", "GameActivationLimit", "ResolvedLimit"} {
							if _, ok := tp.get(lim); ok {
								return refuse("Sacrificed Self alternative with %s", lim)
							}
						}
						// a bare Self alternative beside ValidPlayer$ You: the
						// source's last known controller is the trigger's own
						// while it is on the battlefield to see itself
						// sacrificed, so only the other alternatives carry the
						// controller term
						continue
					}
					if c.Flags&fSelf != 0 || c.Ctrl != 0 && c.Ctrl != w {
						return refuse("Sacrificed ValidPlayer %q with ValidCard %s", vp, tr.F)
					}
					c.Ctrl = w
				}
			}
		}
		if t.Mode == "SacrificedOnce" {
			// gorge (trigger_match.go checkFaceTriggers): Mode$
			// SacrificedOnce is Sacrificed queued once per sacrifice batch
			// (CR 603.2c), latched by the engine's sacrifice brackets (an
			// effect's departure batch, emitSacrifices at the cost sites);
			// triggerTurnLimitFor adds no implicit per-turn limit, and an
			// ActivationLimit$ stays an extra per-turn cap (read below).
			// The kernel's batch latch (Tf2Batch, trig_oneormore.go) with its
			// cost-site sacrifice batches (act_proliferate.go). A body
			// reading the batch refuses: the queued instance carries only
			// its first member.
			tr.Event, tr.Batch = "Sacrificed", true
			if lc.saReadsBatch(t.Effect) {
				return refuse("SacrificedOnce body reads the batch")
			}
		}
	case "ClassLevelGained":
		if err := lc.classLevelGained(tp, tr); err != nil {
			return err
		}
	case "Proliferate":
		if err := lc.proliferateTrigger(tp, tr); err != nil {
			return err
		}
	case "FacesDilemma":
		if err := lc.facesDilemmaTrigger(tp, tr); err != nil {
			return err
		}
	case "BecomeMonarch":
		if err := lc.becomeMonarchTrigger(tp, tr); err != nil { // lower_monarch.go
			return err
		}
	case "RingTemptsYou":
		if err := lc.ringTemptsTrigger(tp, tr, readF); err != nil { // lower_ring.go
			return err
		}
	case "Scry", "Surveil":
		if err := lc.scrySurveilTrigger(tp, tr, t.Mode); err != nil {
			return err
		}
	case "Explores":
		if err := lc.exploresTrigger(tp, tr); err != nil {
			return err
		}
	case "CollectEvidence":
		if err := lc.collectEvidenceTrigger(tp, tr); err != nil {
			return err
		}
	case "Exploited":
		if err := lc.exploitedTrigger(tp, tr); err != nil { // lower_exploit.go
			return err
		}
	case "TokenCreatedOnce":
		if err := lc.tokenCreatedOnce(tp, tr, t); err != nil {
			return err
		}
	case "BecomeMonstrous":
		// "when <filter> becomes monstrous" (CR 701.37b; gorge trigmatch
		// monstrousMatches): the AlterAttribute event's object matched
		// against ValidCard$, read as the permanent is marked
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
	case "GiveGift":
		// "whenever you give a gift" (CR 702.174b; gorge trig:GiveGift)
		w, ok := whoOfPlayer(tp.str("ValidPlayer"))
		if !ok {
			return refuse("ValidPlayer %q", tp.str("ValidPlayer"))
		}
		tr.Whose = w
	case "AbilityCast":
		// CR 602.2 / 603.2 (gorge trigmatch abilityCastMatches): the
		// activation completes. Only the three ValidSA$ classes the kernel
		// reads exactly lower (kop.TrigAbCast*); ValidActivatingPlayer$ is
		// absent or You; ValidSA$ SpellAbility beside TargetsValid$ is the
		// target-shape class; ValidCard$, ValidSAonCard$ and the rest stay
		// unread and refuse. The kernel scans the battlefield and
		// graveyard cards only.
		if zone != "" {
			return refuse("AbilityCast trigger on an Effect")
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield && tr.Zone != zGraveyard {
			return refuse("AbilityCast TriggerZones %q", tp.str("TriggerZones"))
		}
		switch vp := tp.str("ValidActivatingPlayer"); vp {
		case "":
			tr.Whose = whoAny
		case "You":
			tr.Whose = whoYou
		default:
			return refuse("AbilityCast ValidActivatingPlayer$ %q", vp)
		}
		tv, hasTV := tp.get("TargetsValid")
		switch vs := tp.str("ValidSA"); vs {
		case "SpellAbility":
			// any activated ability whose chosen targets match TargetsValid$
			// (Ertha Jo, Frontier Mentor): the kernel judges it once the
			// targets are chosen (kop.TrigAbCastAny). Without TargetsValid$
			// it is every activation, which no corpus card asks for.
			if !hasTV {
				return refuse("AbilityCast ValidSA$ SpellAbility without TargetsValid$")
			}
			if err := lc.targetsValid(tr, tv, true); err != nil {
				return err
			}
			tr.AbClass = kop.TrigAbCastAny
		case "Activated.Exhaust":
			tr.AbClass = kop.TrigAbCastExhaust
		case "Activated.Loyalty":
			tr.AbClass = kop.TrigAbCastLoyalty
		case "Activated.Loyalty+OppCtrl":
			if tr.Whose != whoAny {
				return refuse("AbilityCast Activated.Loyalty+OppCtrl beside ValidActivatingPlayer$")
			}
			tr.AbClass = kop.TrigAbCastLoyaltyOpp
		default:
			return refuse("AbilityCast spell-ability filter ValidSA$ %q", vs) // the T12 spell-ability filter guard (T13c's words)
		}
		if hasTV && tr.AbClass != kop.TrigAbCastAny {
			return refuse("AbilityCast TargetsValid$ beside ValidSA$ %q", tp.str("ValidSA"))
		}
	case "SpellAbilityCast":
		if err := lc.namedCastTrigger(tp, tr, zone); err != nil { // lower_choosename.go
			return err
		}
	case "CommitCrime":
		// CR 700.13 (gorge trigmatch commitCrimeMatches): ValidPlayer$ is
		// matched against the criminal. The kernel scans the battlefield
		// and graveyard cards only, so an Effect's trigger, or another
		// TriggerZones$, refuses.
		w, ok := whoOfPlayer(tp.str("ValidPlayer"))
		if !ok {
			return refuse("CommitCrime ValidPlayer %q", tp.str("ValidPlayer"))
		}
		if zone != "" {
			return refuse("CommitCrime trigger on an Effect")
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield && tr.Zone != zGraveyard {
			return refuse("CommitCrime TriggerZones %q", tp.str("TriggerZones"))
		}
		tr.Whose = w
	case "TurnFaceUp":
		// CR 708.6: a permanent matching ValidCard$ is turned face up.
		// gorge trigmatch turnFaceUpMatches reads ValidCard$ against the
		// permanent and ValidCause$ against the turn-up's actor (the
		// special action's controller or the resolving effect's controller;
		// trigParams4 reads it). Static$ True is the ordinary off-stack
		// static-trigger candidate staticTriggerCand settled.
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
	case "Transformed":
		// CR 701.27a / 712.14a: a permanent matching ValidCard$ transforms
		// (gorge trigmatch transformedMatches reads ValidCard$ against the
		// object on the battlefield at match time). ValidPlayer$ is checked
		// against that object's controller; no corpus card uses it, so it
		// refuses rather than guess the trigger player.
		for _, k := range []string{"ValidCause", "Static", "ValidPlayer"} {
			if _, ok := tp.get(k); ok {
				return refuse("Transformed %s$", k)
			}
		}
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		// secondaryYields suppresses a Secondary trigger only when a
		// non-secondary sibling with the same Execute$ also matched the
		// same event. Only a Transformed trigger matches a FlipFace, so a
		// non-secondary Transformed sibling is the one shape that could
		// suppress this trigger; anything else is safe.
		if tp.is("Secondary", "True") && lc.transformedSecondaryConflict(t) {
			return refuse("Transformed Secondary$ True beside a non-secondary Transformed trigger with the same Execute$")
		}
	case "BecomesPlotted":
		// gorge BecomesPlottedMatches: "when this card
		// becomes plotted" only (ValidCard$ Card.Self, TriggerZones$
		// Exile: Aloe Alchemist); the kernel's plot action fires it
		if tp.str("ValidCard") != "Card.Self" || tr.Zone != zExile {
			return refuse("BecomesPlotted ValidCard %q TriggerZones %d", tp.str("ValidCard"), tr.Zone)
		}
		tp.consume("ValidCard")
	case "Attached":
		// gorge attachedMatches (Assimilation Aegis): the
		// attachment's own "becomes attached to a creature" (ValidSource$
		// Card.Self, ValidTarget$ Creature, on the battlefield); the kernel's
		// attachTo fires it on a move to a different bearer
		if tp.str("ValidSource") != "Card.Self" || tp.str("ValidTarget") != "Creature" {
			return refuse("Attached ValidSource$ %q ValidTarget$ %q", tp.str("ValidSource"), tp.str("ValidTarget"))
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("Attached TriggerZones %d", tr.Zone)
		}
		tp.consume("ValidSource", "ValidTarget")
	case "Unattached":
		// gorge trigmatch unattachedMatches (CR 701.3b/d): the attachment's
		// own "becomes unattached from a permanent" (ValidAttachment$
		// Card.Self, ValidObject$ Permanent, the Captain's Hook family); the
		// kernel's detach arms fire it (trig_unattached.go), RefTriggerObj
		// the former bearer. gorge evaluates ValidAttachment$ against the
		// event's Obj, the attachment the detach names, and Card.Self binds
		// to the trigger's own source: the kernel queues only the attachment's
		// own abilities, which is the same read for the corpus's Card.Self
		// spelling. ValidObject$ is the bearer, matched against it as it last
		// existed; the kernel's fire sites pass every bearer, so only the
		// vacuous spellings lower.
		if v := tp.str("ValidAttachment"); v != "Card.Self" {
			return refuse("Unattached ValidAttachment$ %q", v)
		}
		if v := tp.str("ValidObject"); v != "" && v != "Permanent" {
			return refuse("Unattached ValidObject$ %q", v)
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("Unattached TriggerZones %d", tr.Zone)
		}
		tp.consume("ValidAttachment", "ValidObject")
	case "BecomesSaddled":
		// gorge BecomesSaddledMatches / FirstTimeSaddled (trigmatch/
		// saddled.go): the Mount's own Saddled grant, each resolution of
		// its Saddle (Stubborn Burrowfiend); the kernel's saddle op fires it
		if v := tp.str("ValidSaddled"); v != "Card.Self" {
			return refuse("BecomesSaddled ValidSaddled$ %q", v)
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("BecomesSaddled TriggerZones %d", tr.Zone)
		}
		tp.consume("ValidSaddled")
		if v, ok := tp.get("FirstTimeSaddled"); ok {
			if v != "True" {
				return refuse("BecomesSaddled FirstTimeSaddled$ %q", v)
			}
			tr.First = true
		}
	case "Evolved":
		// gorge trigmatch evolvedMatches (CR 702.100b): the events.Evolved
		// marker rules' resolveTop emits once the Evolve keyword's counter
		// lands names the evolving permanent, which ValidCard$ Card.Self
		// matches; the kernel fires it as that trigger finishes resolving
		// (kw_evolve.go). ValidPlayer$ is left unread (refused).
		if v := tp.str("ValidCard"); v != "Card.Self" {
			return refuse("Evolved ValidCard$ %q", v)
		}
		if tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("Evolved TriggerZones %d", tr.Zone)
		}
		tp.consume("ValidCard")
	case "UnlockDoor":
		// CR 709.5h "when you unlock this door" (gorge rules face_rooms.go
		// queueUnlockTriggers): read only on a Room door, fired directly by
		// the kernel for the unlocked door (dirac/face_rooms.go). gorge
		// reads ValidPlayer$ (absent or You) and ThisDoor$ (absent or True)
		// and ignores ValidCard$; the corpus spells it Card.Self. Any other
		// parameter would be ignored by gorge's queue, so it refuses.
		if lc.out.FaceMode != kop.FaceRoom || zone != "" {
			return refuse("UnlockDoor trigger off a Room door")
		}
		if v := tp.str("ValidPlayer"); v != "" && v != "You" {
			return refuse("UnlockDoor ValidPlayer$ %q", v)
		}
		if v := tp.str("ValidCard"); v != "" && v != "Card.Self" {
			return refuse("UnlockDoor ValidCard$ %q", v)
		}
		if v := tp.str("ThisDoor"); v != "" && v != "True" {
			return refuse("UnlockDoor ThisDoor$ %q", v)
		}
		for k := range t.Params {
			switch k {
			case "Mode", "Execute", "ValidPlayer", "ValidCard", "ThisDoor", "TriggerDescription":
			default:
				return refuse("UnlockDoor %s$", k)
			}
		}
		if t.Params["Execute"] == "" || t.Effect == nil {
			return refuse("UnlockDoor without Execute$")
		}
	case "FullyUnlock":
		// CR 709.5i "whenever you fully unlock a Room" (gorge trigmatch
		// room.go fullyUnlockMatches): the unlock action's one DoorUnlock
		// on the last locked door, matched against the Room. The Room is on
		// the battlefield and the unlocker is its controller. The kernel's
		// unlock action scans battlefield and graveyard listeners
		// (face_fullyunlock.go). A Room watching its own unlock is
		// refused: the kernel fires FullyUnlock from the unlock action on
		// the Room, not from its own faces, so a Room-door carrier has no
		// kernel path.
		if lc.out.FaceMode == kop.FaceRoom {
			return refuse("FullyUnlock trigger on a Room door")
		}
		if zone != "" {
			return refuse("FullyUnlock trigger on an Effect")
		}
		if v := tp.str("ValidCard"); v != "" && v != "Card.Room" {
			return refuse("FullyUnlock ValidCard$ %q", v)
		}
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		switch v := tp.str("ValidPlayer"); v {
		case "":
			tr.Whose = whoAny
		case "You":
			tr.Whose = whoYou
		default:
			return refuse("FullyUnlock ValidPlayer$ %q", v)
		}
		if v, ok := tp.get("Secondary"); ok && v != "True" {
			return refuse("FullyUnlock Secondary$ %q", v)
		}
		if v, ok := tp.get("OptionalDecider"); ok && v != "You" {
			return refuse("FullyUnlock OptionalDecider$ %q", v)
		}
		switch tr.Zone {
		case 0, zBattlefield, zGraveyard:
		default:
			return refuse("FullyUnlock TriggerZones %q", tp.str("TriggerZones"))
		}
		if t.Params["Execute"] == "" || t.Effect == nil {
			return refuse("FullyUnlock without Execute$")
		}
		for _, k := range sortedParamKeys(t.Params) {
			switch k {
			case "Mode", "Execute", "ValidCard", "ValidPlayer", "Secondary", "OptionalDecider", "TriggerZones", "TriggerDescription":
			default:
				return refuse("FullyUnlock %s$", k)
			}
		}
	case "TapAll":
		// gorge tapAllMatches and the action-batch latch (trigger_match.go;
		// CR 603.2c): "whenever one or more <ValidCards$> you control become
		// tapped" queues once per tapping action (a Tap or TapAll
		// resolution, the declare-attackers taps, one cost's tap parts, one
		// mana ability's tapped costs) and once per tap outside one (a mana
		// ability's {T}, a cost's own {T}). The kernel's TrigTaps over a
		// filter of the tapped permanent, latched per tap batch
		// (dirac/trig_tapall.go). Activator$, ValidPlayer$ and Attacker$
		// stay unread and refuse; a body reading the batch refuses (the
		// queued instance carries only its first member).
		if tr.Zone != zBattlefield {
			return refuse("TapAll TriggerZones %d", tr.Zone)
		}
		vc := tp.str("ValidCards")
		if tr.F, err = readF("ValidCards"); err != nil {
			return err
		}
		if tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) == 0 {
			return refuse("TapAll ValidCards %q", vc)
		}
		for _, c := range tr.F.Clauses {
			if c.Ctrl != whoYou || c.Flags&fSelf != 0 {
				return refuse("TapAll ValidCards %q", vc)
			}
		}
		if lc.saReadsBatch(t.Effect) {
			return refuse("TapAll body reads the batch")
		}
		tr.Event, tr.Batch = "Taps", true
	case "Always":
		// CR 603.8 state trigger over a board condition: IsPresent$ with
		// PresentCompare$/PresentZone$, lowered by the generic IsPresent$
		// path below to the same Present(filter, op, n) condition a static
		// Condition$ uses (the counter threshold, Mazemind Tome:
		// Card.Self+counters_GE<N>_<KIND>, is one such filter). gorge matches
		// every event -- the Always matcher is vacuously true
		// (trigmatch/registry.go) -- and gates on the condition. The kernel
		// checks it at the counter-put event of ANY kind on the source itself,
		// as the state-based actions finish a clean pass (turn.go sba ->
		// sbaClean), and when an instance leaves the stack (dirac
		// trig_statectr.go), the points a battlefield or counter change
		// settles at. gorge holds one instance at a time
		// (stateTriggerOutstanding, trigger_condition.go): Outstanding marks
		// the latch; noResolvingCheck exempts Mode$ Always from the
		// resolution recheck (trigger_condition.go), the kernel's
		// AfCheckOnTrigger. A condition that is not an IsPresent$ stays
		// refused (a life total, a hand size or an X threshold has no
		// re-check site here); so is a PresentZone$ naming a hand, library
		// or graveyard -- the kernel has no re-check site at a move in or
		// out of those zones either, so only the default Battlefield zone
		// lowers.
		if zone != "" {
			return refuse("Always trigger on an Effect")
		}
		if tr.Zone != zBattlefield {
			return refuse("Always TriggerZones %q", tp.str("TriggerZones"))
		}
		ip := strings.TrimSpace(tp.str("IsPresent"))
		if ip == "" {
			return refuse("Always without IsPresent$")
		}
		if err := alwaysCounterThreshold(ip); err != nil {
			return err
		}
		if pz := tp.str("PresentZone"); pz != "" && pz != "Battlefield" {
			return refuse("Always PresentZone$ %q", pz)
		}
		for _, k := range sortedParamKeys(t.Params) {
			switch k {
			case "Mode", "Execute", "TriggerZones", "IsPresent", "PresentCompare", "PresentZone", "TriggerDescription":
			default:
				return refuse("Always %s$", k)
			}
		}
		tr.Event, tr.Step = "CounterAdded", "*"
		var err error
		if tr.F, err = lc.filter("Card.Self"); err != nil {
			return err
		}
		tr.Outstanding = true
		ab.Name = "CheckOnTriggerOnly"
	case "LandPlayed", "SearchedLibrary", "Forage", "Discover", "ManifestDread":
		if err := lc.trigModes2(tp, tr, t.Mode, zone, readF); err != nil {
			return err
		}
	default:
		return refuse("trigger mode %s", t.Mode)
	}
	playerTurnGate(tp, tr)
	if err := lc.trigOncePerTurn(tp, tr); err != nil {
		return err
	}
	if rl, ok := tp.get("ResolvedLimit"); ok {
		if err := lc.resolvedLimit(tr, rl, zone); err != nil {
			return err
		}
	}
	if err := lc.classBand(tp); err != nil {
		return err
	}
	// Intervening-if conditions.
	noResolve := tp.is("NoResolvingCheck", "True")
	var conds []lCond
	fireOnly, err := lc.kvfTriggerConds(tp, &conds, descended)
	if err != nil {
		return err
	}
	if dethrone {
		conds, fireOnly = append(conds, dethroneCond()), true // lower_dethrone.go
	}
	if fc, err := lc.firstAttackCombat(tp, tr, &conds, t.Mode); err != nil { // lower_addphase.go
		return err
	} else if fc {
		fireOnly = true
	}
	if cs, ok := tp.get("CheckSVar"); ok {
		if c, isCC, err := lc.castCollectedSVar(tp, cs); isCC { // lower_collectedevidence.go
			if err != nil {
				return err
			}
			conds = append(conds, c)
		} else {
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			if cmp, ok := tp.get("SVarCompare"); ok {
				op, b, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				c.Op, c.B = op, b
			}
			conds = append(conds, c)
		}
	}
	if cdp, ok := tp.get("CheckDefinedPlayer"); ok && cdp != "You.withMostLife" {
		// gorge trigger_condition.go checkDefinedPlayerHolds: the crime
		// spelling (CR 700.13) and the monarch's (CR 725, lower_monarch.go)
		// are read; every other one refuses
		if c, ok := monarchCheck(cdp); ok {
			conds = append(conds, c)
		} else if cdp != "You.committedCrimeThisTurn" {
			return refuse("CheckDefinedPlayer %q", cdp)
		} else {
			conds = append(conds, lCond{Kind: cValue, A: lValue{Kind: vCommittedCrime, Sub: []lValue{cst(1), cst(0)}}, Op: opGE, B: cst(1)})
		}
	}
	if ip, ok := tp.get("IsPresent"); ok {
		c, err := lc.triggerIsPresent(tp, tr, ip)
		if err != nil {
			return err
		}
		conds = append(conds, c)
	}
	if v, ok := tp.get("Revolt"); ok {
		// gorge trigger_condition.go: Revolt$ True, a permanent the
		// controller controlled left the battlefield this turn (CR 207.2c),
		// an intervening-if (CR 603.4) checked as it triggers and resolves
		if v != "True" {
			return refuse("Revolt$ %q", v)
		}
		conds = append(conds, lCond{Kind: cValue, A: lValue{Kind: vRevolt}, Op: opGE, B: cst(1)})
	}
	if ip2, ok := tp.get("IsPresent2"); ok {
		// a Case's solve gate as the second presence clause (gorge
		// trigger_condition.go: IsPresent2$ with its own compare, GE1 by
		// default): only the source's own Solved designation is read
		if !meldPresent2OK(lc, ip2) {
			return refuse("IsPresent2$ %q", ip2)
		}
		for _, k := range [...]string{"PresentCompare2", "PresentZone2", "PresentDefined2", "PresentPlayer2"} {
			if _, ok := tp.get(k); ok {
				return refuse("IsPresent2$ with %s", k)
			}
		}
		f, err := lc.filter(ip2)
		if err != nil {
			return err
		}
		conds = append(conds, lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)})
	}
	if tp.is("Threshold", "True") {
		conds = append(conds, lCond{Kind: cThreshold})
	}
	if v, ok := tp.get("Blessing"); ok {
		// gorge trigger_condition.go: Blessing$ True, the trigger's
		// controller's city's-blessing latch (CR 702.131), an intervening-if
		// checked as it triggers and again at resolution, as Threshold$ True
		// joins the conds
		if v != "True" {
			return refuse("Blessing$ %q", v)
		}
		conds = append(conds, lCond{Kind: cBlessing})
	}
	if v, ok := tp.get("Hellbent"); ok {
		// gorge trigger_condition_hellbent.go: "you have no cards in hand",
		// an intervening-if re-checked at resolution
		if v != "True" {
			return refuse("Hellbent %q", v)
		}
		conds = append(conds, lCond{Kind: cValue, A: lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rYou}}, Op: opEQ, B: cst(0)})
	}
	if v, ok := tp.get("Delirium"); ok {
		// gorge trigger_condition.go: Delirium$ True, four or more card
		// types in the controller's graveyard
		if v != "True" {
			return refuse("Delirium %q", v)
		}
		conds = append(conds, deliriumCond())
	}
	if err := lc.spendPhaseConds(tp, t.Mode, &conds); err != nil { // lower_trigphase.go
		return err
	}
	if err := echoGate(tp, tr, ab, &conds, noResolve || fireOnly || giftSelf); err != nil { // lower_echo.go
		return err
	}
	switch len(conds) {
	case 0:
	case 1:
		tr.Cond = conds[0]
	default:
		// gorge triggerConditionHoldsWithSVars: every clause is its own gate
		// that must hold, as the trigger fires and again at resolution, so
		// the result is the AND of all of them (a Case's solve gate beside
		// its condition, IsPresent$ beside CheckSVar$); the kernel's
		// CondAnd evaluates nested conjunctions with no arity cap
		tr.Cond = conds[0]
		for _, c := range conds[1:] {
			tr.Cond = andCond(tr.Cond, c)
		}
	}
	if giftSelf {
		if tr.Cond.Kind != cNone || noResolve {
			return refuse("gift trigger with another condition")
		}
		tr.Cond = lCond{Kind: cPromisedGift}
		ab.Name = "CheckOnTriggerOnly"
	}
	if fireOnly && !noResolve {
		// a trigger-matching gate (Attacked$, ValidPlayer$) is read as the
		// trigger fires only (gorge trigmatch), never on resolution
		if len(conds) != 1 || giftSelf {
			return refuse("a fire-time player gate beside another condition")
		}
		ab.Name = "CheckOnTriggerOnly"
	}
	if cs, _ := tp.get("CheckSVar"); cs == "Count$OffspringPaid" {
		ab.Name = "CheckOnTriggerOnly" // gorge captures the payment as it fires
	}
	if noResolve {
		if tr.Cond.Kind == cNone {
			return refuse("NoResolvingCheck without a condition")
		}
		tr.Cond.Neg = false
		ab.Name = "CheckOnTriggerOnly"
	}
	if err := lc.readEvolveCondition(tp, tr); err != nil { // lower_handmove.go
		return err
	}
	if err := lc.trigParams3(tp, tr, t.Mode); err != nil { // lower_trigparams.go
		return err
	}
	if err := lc.trigParams4(tp, tr, t.Mode); err != nil { // lower_trigparams.go
		return err
	}
	if left := tp.unread(); len(left) > 0 {
		return refuse("trigger unread params %v", left)
	}
	if t.Effect == nil {
		return refuse("trigger without an effect")
	}
	if tr.Event == "Blocks" && lc.saReadsRemembered(t.Effect) {
		// gorge seeds a Blocks instance's Remembered with the attacker
		// (checkBlocksTriggers); the kernel's remembered set is empty
		return refuse("Blocks trigger body reads Remembered")
	}
	if tr.ByCreature && lc.byCreatureReadsRemembered(t.Effect) {
		// gorge seeds the pair's other member as Remembered
		// (queueAttackerBlockedTrigger); the kernel's remembered set is empty
		return refuse("AttackerBlockedByCreature trigger body reads Remembered")
	}
	savedBody := lc.trigBody
	lc.trigBody = true
	defer func() { lc.trigBody = savedBody }()
	if err := lc.chain(t.Effect, nil); err != nil {
		return err
	}
	if tr.Static {
		// gorge's staticTriggerAskFree (engine/rules/static_trigger.go):
		// an off-stack resolution has no placement to announce targets, no
		// Charm to pick modes, no tape to serve a mid-resolution ask. The
		// body must be provably ask-free: the only shapes that lower to a
		// single no-ask rider instruction (the enters-with-counters
		// Effects, lower_effetbctr.go). Anything else stays on the stack
		// in gorge — it refuses here rather than resolve wrongly.
		if len(ab.Targets) != 0 || len(ab.Prog) != 1 || (ab.Prog[0].Op != "EffEtbCounters" && ab.Prog[0].Op != "EffFinalityCtr") {
			return refuse("static SpellCast trigger body is not the proven ask-free rider shape")
		}
	}
	if err := lc.staticTriggerBody(staticCand, ab, tr, t, svars); err != nil {
		return err
	}
	patchIfPaidSkips(ab)
	return nil
}

// lowerDies lowers a Battlefield$->Graveyard$ ChangesZone line as a Dies
// trigger. A listener's "that creature's controller may" (Fecundity) asks the
// dying creature's last-known controller (CR 603.5; gorge's
// deciderFromSpecTriggeredCardController), so the entry's trigger player is
// that controller rather than the listener's.
func lowerDies(tp *params, t cards.Trigger, tr *lTrigger) {
	tr.Event = "Dies"
	if !tr.Optional {
		return
	}
	if od, _ := tp.get("OptionalDecider"); od == "TriggeredCardController" && t.ParamStr(cards.PKValidCard) != "Card.Self" {
		tr.OptTrigPlayer = true
	}
}

// lowerStatic lowers an S: line.
// trigToGraveyardSet lowers a ChangesZone trigger whose Origin$ is a comma
// set, the "put into a graveyard from anywhere other than the battlefield"
// family (Syr Konrad, the Grim; Ultron the Annihilator; Ultron's Auxiliary).
// gorge matches the move's from-zone against the Origin$ set and its to-zone
// against Destination$ (trigmatch ZoneChangeMatchesWithCapture) as the move
// completes, from the listener on the battlefield; the kernel is one
// TrigToGraveyard ability that placeFromK fires when a card is placed into a
// graveyard, Trigger.N carrying the origin zones as a 1<<kop.Zone mask
// (0 = any), TrigLandPlayed's origin-mask pattern. The battlefield origin is
// out of the family: a battlefield->graveyard move is a Dies line, its own
// event and its own firing site.
func (lc *lowerCtx) trigToGraveyardSet(tp *params, tr *lTrigger, t cards.Trigger, raw string, readF func(string) (*lFilter, error)) error {
	var mask int16
	for _, w := range strings.Split(raw, ",") {
		z, ok := zoneNames[strings.TrimSpace(w)]
		if !ok {
			return refuse("ChangesZone Origin$ %q", raw)
		}
		if z == zBattlefield {
			return refuse("ChangesZone Origin$ %q includes Battlefield", raw)
		}
		mask |= 1 << uint(zoneMap[z])
	}
	if dst := strings.TrimSpace(tp.str("Destination")); dst != "Graveyard" {
		return refuse("ChangesZone Origin$ %s to %s", raw, dst)
	}
	if czCostBody(t) {
		return refuse("ChangesZone Origin$ %s to Graveyard with a cost body", raw)
	}
	if tc, ok := tp.get("TriggerController"); ok && tc != "TriggeredCardController" {
		return refuse("TriggerController %q", tc)
	}
	tr.Origin, tr.Dest = zAny, zGraveyard
	tr.Event, tr.N = "ToGraveyard", mask
	lc.wasCastOK = false
	f, err := readF("ValidCard")
	if err != nil {
		return err
	}
	tr.F = f
	return nil
}

// byCreatureReadsRemembered is saReadsRemembered for an
// AttackerBlockedByCreature body, minus the spellings that name the pair
// member without reading the trigger's remembered set: a delayed trigger's
// RememberObjects$ Triggered{Attacker,Blocker}[LKICopy] (delayedStepRemember
// binds the object at registration, Deathgazer's destroy at end of combat)
// and the delayed body's Defined$ DelayTriggerRememberedLKI.
func (lc *lowerCtx) byCreatureReadsRemembered(sa *cards.SA) bool {
	strip := strings.NewReplacer(
		"RememberObjects$ TriggeredAttackerLKICopy", "", "RememberObjects$ TriggeredBlockerLKICopy", "",
		"RememberObjects$ TriggeredAttacker", "", "RememberObjects$ TriggeredBlocker", "",
		"DelayTriggerRememberedLKI", "")
	for ; sa != nil; sa = sa.Sub {
		if strings.Contains(strip.Replace(sa.Line), "Remember") {
			return true
		}
	}
	for _, v := range lc.face.SVars {
		if strings.Contains(strip.Replace(v), "Remember") {
			return true
		}
	}
	return false
}

// rememberRead reports whether s mentions a remembered object. The exact
// token Player.IsRemembered is the RepeatEach player binding (join forces'
// UnlessPayer$), not a remembered object, so it does not count.
func rememberRead(s string) bool {
	return strings.Contains(strings.ReplaceAll(s, "Player.IsRemembered", ""), "Remember")
}

// saReadsRemembered reports whether an ability chain (or any SVar of the
// face, which its values may name) mentions a remembered object.
func (lc *lowerCtx) saReadsRemembered(sa *cards.SA) bool {
	for ; sa != nil; sa = sa.Sub {
		if rememberRead(sa.Line) {
			return true
		}
	}
	for _, v := range lc.face.SVars {
		if rememberRead(v) {
			return true
		}
	}
	return false
}

// transformedSecondaryConflict reports whether another Transformed trigger on
// the same face shares this trigger's Execute$ without being Secondary.
// gorge's secondaryYields would suppress the Secondary one when both matched
// the same FlipFace; the kernel has no secondary mechanism, so that shape
// refuses. A sibling of any other mode can never match a FlipFace.
func (lc *lowerCtx) transformedSecondaryConflict(t cards.Trigger) bool {
	ex := t.Params["Execute"]
	for _, o := range lc.face.Triggers {
		if o.Mode != "Transformed" || o.Effect == t.Effect || o.Params["Execute"] != ex {
			continue
		}
		if o.Params["Secondary"] != "True" {
			return true
		}
	}
	return false
}

// spentGateOf reads a "Spell.ManaSpent <op><n>" clause (gorge
// trigmatch.ValidSAMatches case 2, spellValidSAonCardMatches) as a
// kop.DmgGate; <n> may be the X sentinel 0xff (the spell's printed mana
// value) only when allowX (ValidSAonCard$: ValidSAMatches fails X closed).
// A literal outside 0..254 or any other spelling is not read.
func spentGateOf(v string, allowX bool) (int16, bool) {
	rest, ok := strings.CutPrefix(v, "Spell.ManaSpent ")
	if !ok || len(rest) < 3 {
		return 0, false
	}
	var cmp kop.DmgCmp
	switch rest[:2] {
	case "GE":
		cmp = kop.DmgGE
	case "LE":
		cmp = kop.DmgLE
	case "EQ":
		cmp = kop.DmgEQ
	case "NE":
		cmp = kop.DmgNE
	case "GT":
		cmp = kop.DmgGT
	case "LT":
		cmp = kop.DmgLT
	default:
		return 0, false
	}
	arg := rest[2:]
	if arg == "X" {
		return kop.DmgGate(cmp, 0xff), allowX
	}
	for _, c := range arg {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n > 254 {
		return 0, false
	}
	return kop.DmgGate(cmp, n), true
}

// alwaysCounterThreshold bounds the Always counter-threshold shape
// (Card.Self+counters_GE<N>_<KIND>): kop.PredCountersGE packs the threshold in
// the high byte of an int16, so N is a literal 1..127, and the kind must
// exist. Any other IsPresent$ filter passes through to the generic path.
func alwaysCounterThreshold(ip string) error {
	rest, ok := strings.CutPrefix(ip, "Card.Self+counters_GE")
	if !ok {
		return nil
	}
	thr, kind, ok := strings.Cut(rest, "_")
	if !ok {
		return refuse("Always IsPresent$ %q", ip)
	}
	n, err := strconv.Atoi(thr)
	if err != nil || n < 1 || n > 127 {
		return refuse("Always IsPresent$ %q (threshold %q)", ip, thr)
	}
	if _, err := counterKind(kind); err != nil {
		return refuse("Always IsPresent$ %q (%v)", ip, err)
	}
	return nil
}

// activationLimit lowers "This ability triggers only once each turn"
// (CR 603.2h; gorge trigger ActivationLimit$ 1): kernel TfOncePerTurn per
// source object.
func (lc *lowerCtx) trigOncePerTurn(tp *params, tr *lTrigger) error {
	if al, ok := tp.get("ActivationLimit"); ok {
		if al != "1" {
			return refuse("ActivationLimit %q", al)
		}
		tr.OncePer = true
	}
	return nil
}

// scopeBareCastFromYourHand reads a SpellCast ValidCard$'s bare
// wasCastFromYourHand with no ValidActivatingPlayer$ as "your hand" of the
// trigger's controller (CR 109.5; gorge castFromHandAnyAdmitsWindow, Forge
// CardProperty), not any caster's: Baron Helmut Zemo. A positive word becomes
// the ByYou read; a negated one has no carrier and refuses by name.
func scopeBareCastFromYourHand(mode, k string, tp *params, v string) (string, error) {
	if k != "ValidCard" || mode != "SpellCast" || !strings.Contains(v, "wasCastFromYourHand") {
		return v, nil
	}
	if _, scoped := tp.get("ValidActivatingPlayer"); scoped {
		return v, nil
	}
	if strings.Contains(v, "!wasCastFromYourHand") {
		return v, refuse("SpellCast !wasCastFromYourHand without ValidActivatingPlayer$")
	}
	if !strings.Contains(v, "wasCastFromYourHandByYou") {
		v = strings.ReplaceAll(v, "wasCastFromYourHand", "wasCastFromYourHandByYou")
	}
	return v, nil
}

// filterUnless is lc.filter(v), or err when an earlier read of the same
// parameter already refused (scopeBareCastFromYourHand).
func (lc *lowerCtx) filterUnless(err error, v string) (*lFilter, error) {
	if err != nil {
		return nil, err
	}
	return lc.filter(v)
}
