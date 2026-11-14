package kgen

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
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
}

func (lc *lowerCtx) lowerTriggered(t cards.Trigger, zone string) error {
	var atkMostLife, descended, dethrone bool // fire-time player gates
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
	var err error
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
		return lc.filter(v)
	}
	giftSelf := false
	switch t.Mode {
	case "ChangesZone":
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
			tr.Event = "Dies"
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
		if tr.Origin == zBattlefield && filterReadsSuspected(tr.F) {
			// a dies / leaves trigger reads the LKI, and the LKI record carries
			// no arena designation bit
			return refuse("IsSuspected in a battlefield-origin ChangesZone ValidCard$")
		}
		if tr.Origin == zBattlefield && filterReadsNotBlocking(tr.F) {
			// gorge matches a leave on the LKI, where isBlocking reads the
			// live object's record and the attackers' BlockedBy lists; the
			// kernel's off-battlefield W7NotBlocking always holds
			return refuse("!blocking in a battlefield-origin ChangesZone ValidCard$")
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
		if v, ok := tp.get("Dethrone"); ok {
			// K:Dethrone's expansion (lower_dethrone.go)
			if dethrone, err = dethroneGate(v, tp, tr); err != nil {
				return err
			}
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
			// seat: only the player / your-planeswalker alternatives the
			// two-player kernel answers exactly; the rest refuse
			// (seat-exact but for Player.withMostLife, marked below)
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
				case "Player.withMostLife":
					// gorge attackedDefenderMatches over MatchesPlayerSpec:
					// the attacked player has the most life or is tied
					// (playerHasMost Life), read at the declaration only;
					// two-player, the defending player is the opponent
					// (Preacher of the Schism)
					if strings.Contains(v, ",") {
						return refuse("Attacks Attacked %q", v)
					}
					tr.Attacked |= atkOpp
					atkMostLife = true
					lc.twoSeat("Attacked$ Player.withMostLife read as the one opponent")
				default:
					return refuse("Attacks Attacked %q", v)
				}
			}
		}
	case "AttackerBlocked":
		// gorge checkAttackerBlockedTriggers: once per distinct blocked
		// attacker matching ValidCard$; ValidBlocker$ Card.Self ("whenever
		// this blocks a creature"), only the attackers the source blocks;
		// any other ValidBlocker$ (and ValidBlockerAmount$) refuses
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if vb, ok := tp.get("ValidBlocker"); ok {
			if vb != "Card.Self" {
				return refuse("AttackerBlocked ValidBlocker %q", vb)
			}
			tr.BlockedBySelf = true
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
		// and its owner (ValidPlayer$ You); Other/FirstTime$ refuse
		vc := tp.str("ValidCard")
		self := vc == "Card.Self" && tr.Zone == 0
		if !self && (tr.Zone != zBattlefield || strings.Contains(vc, "Self") || strings.Contains(vc, "Other")) {
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
		//     Cryoshatter's Card.AttachedBy when its body defers the tap.
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
				// (filter.go): the enchanted-permanent watcher. Its deferring
				// bodies (Cryoshatter's destroy) lower, as gorge's planner
				// defers any tap a non-damage watcher sees; a damage body
				// (Psychic Venom's payer damage) has a payment tiering the
				// kernel does not mirror yet, so it refuses.
				tp8, pn := tapsPlanClass(t.Effect)
				att := strings.Contains(vc, "AttachedBy")
				for _, c := range tr.F.Clauses {
					ok := c.Owner == whoAny && c.CmpStat == cmpNone && c.Special == 0
					switch {
					case ok && c.Ctrl == whoYou && c.Flags == 0 && !att:
					case ok && c.Ctrl == whoAny && c.Flags == fEquippedBy && (!att || tp8 == kop.TapPlanDefer):
					default:
						return refuse("Taps ValidCard %q", vc)
					}
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
		if tp.str("ValidPlayer") == "Player.Chosen" && lc.granted == 0 && tr.Zone == zBattlefield && entersChoosingPlayer(lc.face) {
			// the opponent the source chose as it entered (lower_chooseplayerenter.go)
			w, ok = whoChosen, true
		}
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
				return refuse("ValidSA %q", v)
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
		// target. With two players ValidTarget$ Opponent or You names exactly
		// one possible referent, so the per-referent latch of DamageDoneOnce
		// picks the same batches and the kernel's existing per-referent queue
		// is exact. Any other target (an object filter can match several
		// referents, which the line key collapses) stays refused.
		// ExcessDamageAll is not this mode.
		switch vt := tp.str("ValidTarget"); vt {
		case "Opponent":
			lc.twoSeat("DamageAll over the opponents read as one referent")
		case "You":
		default:
			return refuse("DamageAll ValidTarget %q", vt)
		}
		if err := lc.damageDoneOnce(tp, tr, t); err != nil {
			return err
		}
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
		if tp.str("ValidTarget") == atkLastTurnSpec {
			if err := atkYouLastTurn(tp, tr, t); err != nil { // lower_atklastturn.go
				return err
			}
		} else if tr.F2, err = readF("ValidTarget"); err != nil {
			return err
		}
		if cd, ok := tp.get("CombatDamage"); ok {
			if cd == "True" {
				tr.Combat = 1
			} else {
				tr.Combat = -1
			}
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
			if len(alts) != 1 {
				return refuse("BecomesTarget shape")
			}
			if tr.F, err = lc.filter(alts[0]); err != nil {
				return err
			}
			tr.Whose = whoOpp
			tr.Watch = true
		case tp.str("ValidSource") == "" && tp.str("ValidTarget") == "Card.Self" &&
			!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True"):
			// "Whenever this becomes the target of a spell or ability" with
			// no source restriction (CR 603.2; gorge becomesTargetMatches
			// with ValidTarget$ Card.Self and no ValidSource$): every
			// TargetsChosen naming the permanent fires it, its controller's
			// own included. A TfSelf line with no Whose gate: the kernel's
			// becameTarget fires it through the plain per-object walk
			// (fireObjGen), no opponent gate.
			tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		case tp.str("ValidSource") == "" && tp.str("ValidTarget") != "" &&
			!strings.Contains(tp.str("ValidTarget"), "Card.Self") && !strings.Contains(tp.str("ValidTarget"), ",") &&
			!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True"):
			// the watcher with no source restriction (Daru Spiritualist:
			// "whenever a Cleric creature you control becomes the target of
			// a spell or ability"): the opponent-source watcher's arm
			// without its Whose gate; one trigger per matching chosen
			// target, the targeted permanent as the trigger object
			// (gorge becomesTargetReferent)
			if tr.F, err = lc.filter(tp.str("ValidTarget")); err != nil {
				return err
			}
			tr.Watch = true
		default:
			return refuse("BecomesTarget shape")
		}
	case "Drawn":
		vc := tp.str("ValidCard")
		switch vc {
		case "Card.YouCtrl", "Card.YouOwn":
			tr.Whose = whoYou
		case "Card.OppOwn", "Card.OppCtrl":
			// a card being drawn is in its owner's hand, whose controller
			// is its owner (CR 108.4a; gorge matches OppCtrl on the drawn
			// object's Controller, its owner's seat): Sheoldred
			tr.Whose = whoOpp
		case "":
			// ValidPlayer$-scoped draws (gorge drawnMatches, trigmatch/cards.go:474):
			// the drawing player relative to the controller; Number$ counts the
			// drawer's own draws, as nthDrawOK does
			switch vp := tp.str("ValidPlayer"); vp {
			case "Opponent":
				tr.Whose = whoOpp
			case "Player":
				tr.Whose = whoAny
			default:
				return refuse("Drawn ValidCard %q ValidPlayer %q", vc, vp)
			}
		default:
			return refuse("Drawn ValidCard %q", vc)
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
	case "CounterPlayerAddedAll":
		if err := lc.ctrPlayerAddedAll(tp, tr); err != nil {
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
			return refuse("AbilityCast ValidSA$ %q", vs)
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
		// permanent; ValidCause$ and Static$ are other shapes.
		if _, ok := tp.get("ValidCause"); ok {
			return refuse("TurnFaceUp ValidCause$")
		}
		if _, ok := tp.get("Static"); ok {
			return refuse("TurnFaceUp Static$")
		}
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
		// CR 603.8 state trigger, the counter-threshold shape (Mazemind
		// Tome: TriggerZones$ Battlefield, IsPresent$
		// Card.Self+counters_GE<N>_<KIND>, N a literal 1..127:
		// kop.PredCountersGE packs the threshold in the high byte of an
		// int16 (PredN), so 128 would wrap negative). gorge
		// matches every event -- the Always matcher is vacuously true
		// (trigmatch/registry.go) -- and gates on the condition, which only
		// a counter put can newly satisfy: counters change only through
		// counter-put events, and a zone move only empties the count. The
		// kernel rides the counter-put event of ANY kind on the source
		// itself (the kind and the threshold ride the condition, not the
		// event), checked as the event happens and again when an instance
		// leaves the stack (dirac trig_statectr.go). gorge holds one
		// instance at a time (stateTriggerOutstanding, trigger_condition.go):
		// Outstanding marks the latch; noResolvingCheck exempts Mode$ Always
		// from the resolution recheck (trigger_condition.go), the kernel's
		// AfCheckOnTrigger. Every other Always shape keeps the refusal: its
		// condition can newly hold on events the kernel's re-check sites
		// never see (a life total, a hand size, another object's counters,
		// a PresentCompare$ or an X threshold).
		if zone != "" {
			return refuse("Always trigger on an Effect")
		}
		if tr.Zone != zBattlefield {
			return refuse("Always TriggerZones %q", tp.str("TriggerZones"))
		}
		ip := strings.TrimSpace(tp.str("IsPresent"))
		rest, ok := strings.CutPrefix(ip, "Card.Self+counters_GE")
		if !ok {
			return refuse("Always IsPresent$ %q", ip)
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
		for _, k := range sortedParamKeys(t.Params) {
			switch k {
			case "Mode", "Execute", "TriggerZones", "IsPresent", "TriggerDescription":
			default:
				return refuse("Always %s$", k)
			}
		}
		tr.Event, tr.Step = "CounterAdded", "*"
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
	if tp.is("PlayerTurn", "True") {
		tr.OnlyYourTurn = true
	}
	// "This ability triggers only once each turn" (CR 603.2h; gorge
	// trigger ActivationLimit$ 1): kernel TfOncePerTurn per source object.
	if al, ok := tp.get("ActivationLimit"); ok {
		if al != "1" {
			return refuse("ActivationLimit %q", al)
		}
		tr.OncePer = true
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
	fireOnly, err := lc.kvfTriggerConds(tp, &conds, atkMostLife, descended)
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
		if pd, ok := tp.get("PresentDefined"); ok {
			// gorge presentClauseHolds: PresentDefined$ Self counts the
			// source alone, 1 when it is on the battlefield and matches
			// the spec (PresentZone$ unread on that path), so each
			// alternative is the spec narrowed to Self. Any other
			// Defined group refuses.
			if strings.TrimSpace(pd) != "Self" {
				return refuse("trigger PresentDefined$ %q", pd)
			}
			if _, ok := tp.get("PresentZone"); ok {
				return refuse("trigger PresentDefined$ beside PresentZone$")
			}
			alts := strings.Split(ip, ",")
			for i, a := range alts {
				a = strings.TrimSpace(a)
				if strings.Contains(a, ".") {
					alts[i] = a + "+Self"
				} else {
					alts[i] = a + ".Self"
				}
			}
			ip = strings.Join(alts, ",")
		}
		if strings.Contains(ip, "blockingSource") || strings.Contains(ip, "blockedBySource") {
			// the block relation read when the trigger fires: Lairwatch
			// Giant's Blocks trigger (IsPresent$ Creature.blockedBySource,
			// GE2) diverges from gorge in a lockstep game (the damage
			// division asked of another creature), so it stays refused
			return refuse("trigger IsPresent$ %q reads the block relation at trigger time", ip)
		}
		lc.atkPresent = tr.Event == "Attacks"
		f, err := lc.filter(ip)
		lc.atkPresent = false
		if err != nil {
			return err
		}
		if tr.Event == "ETB" && filterSelfOnly(tr.F) {
			// a self-enters intervening-if that reads the source's host
			// (Historian's Wisdom: IsPresent$ Permanent.EnchantedBy+greatestPower)
			// is checked by gorge with the Aura already attached (CR 303.4f);
			// the kernel queues the ETB trigger inside enter(), before
			// resolve.go attaches the Aura, so EnchantedBy matches nothing
			// and the trigger never fires.
			for _, c := range f.Clauses {
				if c.Flags&fEnchantedBy != 0 {
					return refuse("self-enters IsPresent$ %q reads EnchantedBy before the Aura attaches", ip)
				}
			}
		}
		c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
		if pc, ok := tp.get("PresentCompare"); ok {
			op, v, err := lc.cmpSpec(pc)
			if err != nil {
				return err
			}
			c.Op, c.A = op, v
		}
		if c, err = lc.triggeredPresentZoneCond(c, tp, ip); err != nil {
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
		if strings.TrimSpace(ip2) != "Card.Self+!IsSolved" && strings.TrimSpace(ip2) != "Card.Self+IsSolved" {
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
	patchIfPaidSkips(ab)
	return nil
}

// lowerStatic lowers an S: line.
func (lc *lowerCtx) lowerStatic(s cards.Static) error {
	if s.Mode == "FlipCoinMod" {
		return lc.flipCoinModStatic(s) // lower_turnfacts.go
	}
	if s.Mode == "TapPowerValue" {
		return lc.tapPowerValueStatic(s) // lower_hiddenpick.go
	}
	if s.Mode == "OptionalCost" {
		return lc.optCostStatic(s) // lower_optcost.go
	}
	if s.Mode == "Continuous" && s.Params["CanBlockAmount"] != "" {
		if ok, err := lc.extraBlockStatic(s); ok || err != nil {
			return err // lower_extrablock.go
		}
	}
	if s.Mode == "MinMaxBlocker" && s.Params["Max"] == "" {
		return lc.minMaxBlockerStatic(s) // lower_choosetype.go
	}
	if s.Params["Condition"] == "MaxSpeed" {
		return lc.maxSpeedStatic(s) // lower_speed.go
	}
	if isChosenModeStatic(s) {
		return lc.chosenModeStatic(s) // lower_siegemode.go
	}
	if s.Mode == "CantAttackUnless" {
		if err := attackTaxReadable(s.Params); err != nil {
			return err // lower_attacktax.go: before the EffectZone$ strip below
		}
	}
	if s.Mode == "CantBlockUnless" {
		if err := blockTaxReadable(s.Params); err != nil {
			return err // lower_blocktax.go: before the EffectZone$ strip below
		}
	}
	if s.Params["EffectZone"] == "Battlefield" {
		// a printed static's default zone spelled out (gorge
		// EffectZoneOK: the battlefield, as with no EffectZone$)
		ps := make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			if k != "EffectZone" {
				ps[k] = v
			}
		}
		s.Params = ps
	}
	// a printed Mode$ Continuous static reads ExiledWithSource against its own
	// source (kernel ctxStatic); an emblem's or an Effect's static has another
	// source object, so only this entry sets it
	lc.exwStatic = s.Mode == "Continuous"
	sts, err := lc.staticBody(s)
	lc.exwStatic = false
	if err != nil {
		return err
	}
	for _, st := range sts {
		if st.Kind == "" {
			continue // a view-only MayLookAt lowers to no static
		}
		lc.out.Statics = append(lc.out.Statics, st)
	}
	return nil
}

// hasFlash reports whether the face prints Flash.
func (lc *lowerCtx) hasFlash() bool {
	for _, kw := range lc.face.Keywords {
		if kw == "Flash" {
			return true
		}
	}
	return false
}

// selfFlashPresent is the IsPresent$ spec shape a self flash grant may read:
// a type or subtype word and controller / owner / simple card-type
// qualifiers. The gate is evaluated for the caster with the carrier still in
// hand (no battlefield source), so a source-relative term (Self, Other,
// Enchanted, ...) is refused.
var selfFlashPresent = regexp.MustCompile(`^[A-Z][A-Za-z]*(\.(YouCtrl|YouOwn|OppCtrl|OppOwn|nonCreature|nonLand))*$`)

// selfFlashShape checks a CastWithFlash static that names EffectZone$ is the
// self grant kernel selfFlash evaluates (gorge castWithFlashTargets over
// withSelfStatics): EffectZone$ All, ValidCard$ Card.Self, and only the
// IsPresent$ / PresentZone$ / PresentCompare$ gate, read against the caster.
func (lc *lowerCtx) selfFlashShape(s cards.Static, ez string) error {
	if strings.TrimSpace(ez) != "All" {
		return refuse("CastWithFlash EffectZone %q", ez)
	}
	if strings.TrimSpace(s.Params["ValidCard"]) != "Card.Self" {
		return refuse("CastWithFlash EffectZone$ All beside ValidCard %q", s.Params["ValidCard"])
	}
	if lc.hasFlash() {
		return refuse("CastWithFlash self grant on a card with flash")
	}
	for k := range s.Params {
		switch k {
		case "Mode", "ValidCard", "ValidSA", "Caster", "EffectZone", "IsPresent", "PresentZone", "PresentCompare", "Description":
		default:
			return refuse("CastWithFlash self grant with %s$ (the gate is read with no source on the battlefield)", k)
		}
	}
	if ip, ok := s.Params["IsPresent"]; ok && !selfFlashPresent.MatchString(strings.TrimSpace(ip)) {
		return refuse("CastWithFlash self grant IsPresent %q", ip)
	}
	if _, ok := s.Params["IsPresent"]; !ok {
		if _, ok := s.Params["PresentZone"]; ok {
			return refuse("CastWithFlash self grant PresentZone$ without IsPresent$")
		}
	}
	return nil
}

// staticBody lowers a static's parameters.
func (lc *lowerCtx) staticBody(s cards.Static) ([]lStatic, error) {
	sp := newParams(s.Params)
	sp.consume("Mode")
	if lc.aiHintSVars(s.Params["AddSVar"]) || s.Mode == "Continuous" && lc.inertAddSVar(s.Params["AddSVar"]) {
		sp.consume("AddSVar") // a variable grant gorge's layer walk drops or no rules reader reads
	}
	st := lStatic{Kind: s.Mode}
	var err error
	readF := func(k string) (*lFilter, error) {
		v, ok := sp.get(k)
		if !ok {
			return nil, nil
		}
		return lc.filter(v)
	}
	readCond := func() error {
		if c, ok := sp.get("Condition"); ok {
			switch c {
			case "Threshold":
				st.Cond = lCond{Kind: cThreshold}
			case "PlayerTurn":
				st.Cond = lCond{Kind: cYourTurn}
			default:
				// gorge continuousConditionHolds (Continuous) and
				// costConditionHolds (a self cost static: the payer is
				// the controller) read these against the controller. The
				// self-carried AlternativeCost (gorge
				// alternativeCostScopeOK's costConditionHolds read, the
				// Force cycle: the caster is the static's controller, the
				// printed you) and the combat restriction statics (gorge
				// combatBoard.StaticGateHolds' continuousGateHolds,
				// Monstrous Hound) read theirs against the static's
				// controller, which statCond's and offerAltFreeHand's
				// Cond2 contexts name too.
				selfCost := (s.Mode == "ReduceCost" || s.Mode == "RaiseCost") && s.Params["ValidCard"] == "Card.Self"
				turnGate := s.Mode == "Continuous" || selfCost || reduceActivatorYou(s.Mode, s.Params) ||
					s.Mode == "AlternativeCost" || s.Mode == "CantBlock" || s.Mode == "CantAttack"
				switch {
				case c == "NotPlayerTurn" && turnGate:
					st.Cond = lCond{Kind: cYourTurn, Neg: true}
				case c == "Metalcraft" && (s.Mode == "Continuous" || selfCost):
					f, err := lc.filter("Artifact.YouCtrl")
					if err != nil {
						return err
					}
					st.Cond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(3)}
				case c == "Delirium" && (s.Mode == "Continuous" || selfCost):
					st.Cond = deliriumCond() // gorge graveyardCardTypeCount >= 4
				case c == "Hellbent" && s.Mode == "Continuous":
					st.Cond = lCond{Kind: cValue, A: lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rYou}}, Op: opEQ, B: cst(0)}
				default:
					return refuse("static Condition %q", c)
				}
			}
		}
		if cs, ok := sp.get("CheckSVar"); ok {
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			if cmp, ok := sp.get("SVarCompare"); ok {
				op, b, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				c.Op, c.B = op, b
			}
			st.Cond = andCond(st.Cond, c)
		}
		if ip, ok := sp.get("IsPresent"); ok {
			f, err := lc.filter(ip)
			if err != nil {
				return err
			}
			c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
			if pc, ok := sp.get("PresentCompare"); ok {
				op, v, err := lc.cmpSpec(pc)
				if err != nil {
					return err
				}
				c.Op, c.A = op, v
			}
			if pz, ok := sp.get("PresentZone"); ok {
				if c, err = presentZoneCond(c, pz, ip, lc); err != nil {
					return err
				}
			}
			st.Cond = andCond(st.Cond, c)
		}
		return nil
	}
	if err := readCond(); err != nil {
		return nil, err
	}
	if b, ok := sp.get("ClassBand"); ok {
		if s.Mode != "Continuous" {
			return nil, refuse("ClassBand on a %s static", s.Mode) // only the Continuous condition is lockstep-verified
		}
		c, err := classBandCond(b) // lower_delayed.go
		if err != nil {
			return nil, err
		}
		st.Cond = andCond(st.Cond, c)
	}
	var extra []lStatic
	switch s.Mode {
	case "Continuous":
		lc.grantInner = nil
		if err := extraBlockRefusal(sp); err != nil {
			return nil, err // lower_extrablock.go
		}
		goad := sp.is("Goad", "True")
		sp.consume("Goad")
		lure, lureOnly, err := lc.lureGrant(sp, &st) // lower_lure.go
		if err != nil {
			return nil, err
		}
		var sts []lStatic
		if lureOnly {
			st = lure
		} else if sts, err = lc.continuous(sp, &st); err != nil {
			return nil, err
		}
		if err := lc.selfCondCheck(&st); err != nil {
			return nil, err // lower_selfcondition.go
		}
		if err := affectedKwCheck(&st); err != nil {
			return nil, err // lower_selfkwaffected.go
		}
		extra = sts
		if lure.Kind != "" && !lureOnly {
			// the lure rides beside the other parts (Gift of the Deity's
			// +1/+1): a second static over the same filter and condition
			if extra == nil {
				extra = []lStatic{st} // extra replaces the static itself
			}
			extra = append(extra, lure)
		}
		if goad {
			// Goad$ True: the affected creatures are goaded while the
			// static applies (gorge's static goad route; kop.StatGoad), a
			// second static over the same filter and condition
			if st.Kind != "Continuous" || st.F == nil || st.F.Players != 0 {
				return nil, refuse("Goad$ on a %s static", st.Kind)
			}
			if extra == nil {
				extra = []lStatic{st} // extra replaces the static itself
			}
			extra = append(extra, lStatic{Kind: "Goad", F: st.F, Cond: st.Cond})
		}
	case "ReduceCost":
		if sp.str("Type") == "Ability" || sp.str("Type") == "" && (strings.HasPrefix(sp.str("ValidSpell"), "Activated.") || sp.str("ValidSpell") == "Static.Unlock") {
			if err := lc.reduceAbility(sp, &st, readF); err != nil {
				return nil, err
			}
			break // lower_reduceab.go
		}
		if sp.str("Type") != "Spell" {
			return nil, refuse("ReduceCost Type %q", sp.str("Type"))
		}
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if opt, err := lc.reduceOptPaid(sp, &st); err != nil {
			return nil, err // lower_optcost.go
		} else if !opt {
			if sp.str("ValidCard") == "Card.Self" {
				sp.consume("Relative") // gorge lists Relative$ as readable and never reads it (as reduceOptPaid)
			}
			if st.Amount, err = lc.value(sp.str("Amount")); err != nil {
				return nil, err
			}
		}
		if a, ok := sp.get("Activator"); ok && a != "You" {
			return nil, refuse("ReduceCost Activator %q", a)
		} else if !ok && sp.str("ValidCard") != "Card.Self" {
			// no Activator$: either player's spells (CR 601.2f; Forge
			// StaticAbilityCostChange applies it to every payer)
			st.Flags = append(st.Flags, "AnyCaster")
		}
		if ez, ok := sp.get("EffectZone"); ok {
			if ez != "All" {
				return nil, refuse("EffectZone %q", ez)
			}
			st.Zone = zAny
		}
		lc.spellBaseOK = true
		st.F2, err = readF("ValidTarget")
		lc.spellBaseOK = false
		if err != nil {
			return nil, err
		}
		if st.F == nil {
			// OnlyFirstSpell$ without ValidCard$ (Melek): the in-flight
			// gate is the ValidSpell$ filter, which keeps its Spell head
			// (build_ab builds st.F at ZoneStack); the first-spell ledger
			// spec is the stripped card spec (reduceGates).
			lc.spellBaseOK = true
			st.F, err = readF("ValidSpell")
			lc.spellBaseOK = false
			if err != nil {
				return nil, err
			}
		}
		if err := lc.reduceGates(sp, s.Params, &st); err != nil {
			return nil, err // lower_reducegates.go
		}
		if err := lc.reduceSpellShape(sp, &st); err != nil {
			return nil, err // lower_reducespell.go
		}
	case "RaiseCost":
		done, err := lc.raiseSelfWaterbend(sp, &st) // lower_raisewaterbend.go
		if err == nil && !done {
			done, err = lc.raiseSelfBehold(sp, &st)
		}
		if err != nil {
			return nil, err
		} else if !done {
			if err := lc.raiseCost(sp, &st); err != nil {
				return nil, err
			}
		}
	case "CantBeCast":
		if err := lc.cantBeCast(sp, &st); err != nil {
			return nil, err
		}
	case "CantBeActivated":
		if err := lc.cantActivate(sp, &st); err != nil {
			return nil, err // lower_cantact.go
		}
	case "CantTarget":
		if err := lc.cantTarget(sp, &st); err != nil {
			return nil, err // lower_canttarget.go
		}
	case "CantBlock":
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
	case "CantBeBeamedUp":
		sp.consume("Secondary")
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if v, ok := sp.get("ValidSA"); ok {
			if v != "Spell.KeywordSource" {
				return nil, refuse("CantBeBeamedUp ValidSA %q", v)
			}
			st.Flags = append(st.Flags, "BeamGranted")
		}
	case "MinMaxBlocker":
		// Max$ 1 only (gorge MinMaxBlockerBounds reads the
		// literal smallest Max$; the corpus's only value is 1)
		if sp.str("Min") != "" {
			return nil, refuse("MinMaxBlocker with both Min and Max")
		}
		if v := sp.str("Max"); v != "1" {
			return nil, refuse("MinMaxBlocker Max %q", v)
		}
		sp.consume("Max", "Secondary")
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
	case "CantBlockBy":
		if st.F, err = readF("ValidAttacker"); err != nil {
			return nil, err
		}
		if vb, ok := sp.get("ValidBlocker"); ok {
			rest, mode := pwVsSrcSplit(vb, lc.face.SVars) // lower_powercompare.go
			if st.F2, err = lc.filter(rest); err != nil {
				return nil, err
			}
			st.PwVsSrc = mode
		}
	case "CantAttack":
		if err := lc.cantAttack(sp, &st); err != nil {
			return nil, err
		}
	case "CantAttackUnless":
		if err := lc.cantAttackUnless(sp, &st); err != nil {
			return nil, err // lower_attacktax.go
		}
	case "CantBlockUnless":
		if err := lc.cantBlockUnless(sp, &st); err != nil {
			return nil, err // lower_blocktax.go
		}
	case "CanAttackIfHaste":
		// gorge canAttackIfHaste (rules/combat/attack.go): ValidCard$ (absent:
		// every creature) scopes the exempt creatures; a present ValidTarget$
		// is tried as a player spec on the defender, then as the attacked
		// permanent. Only the Frenzied Saddlebrute spelling lowers: it names
		// the defender player (an opponent of the static's controller) and the
		// planeswalkers that player controls, so the exemption depends on the
		// defender seat alone. gorge skips the line (inert) when it carries
		// a param outside its whitelist, so any other param refuses here
		// rather than lower a condition gorge never reads.
		sp.consume("Description", "Secondary")
		for k := range s.Params {
			switch k {
			case "Mode", "ValidCard", "ValidTarget", "Description", "Secondary":
			default:
				return nil, refuse("CanAttackIfHaste %s$ (gorge reads the line only over ValidCard$ and ValidTarget$)", k)
			}
		}
		if vc, ok := sp.get("ValidCard"); ok {
			if st.F, err = readF("ValidCard"); err != nil {
				return nil, err
			}
			if st.F == nil || len(st.F.Clauses) == 0 || st.F.Players != 0 {
				return nil, refuse("CanAttackIfHaste ValidCard %q", vc)
			}
		}
		if tv, ok := sp.get("ValidTarget"); ok {
			if strings.TrimSpace(tv) != "Opponent,Planeswalker.OppCtrl" {
				return nil, refuse("CanAttackIfHaste ValidTarget %q", tv)
			}
			st.Flags = append(st.Flags, "OppDef")
		}
	case "CanAttackDefender":
		// gorge combat.AttackAllowedThroughDefender: ValidCard$ (or
		// ValidCards$) must match the creature, ValidAttacked$ must admit
		// the defender. ValidAttacked$ stays unread (the unread-param check
		// refuses it); a player-only ValidCard$ filter refuses.
		if _, ok := sp.get("ValidCard"); !ok {
			return nil, refuse("CanAttackDefender without ValidCard$")
		}
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if st.F == nil || len(st.F.Clauses) == 0 || st.F.Players != 0 {
			return nil, refuse("CanAttackDefender ValidCard %q", s.Params["ValidCard"])
		}
	case "MustAttack":
		if st.F, err = readF("ValidCreature"); err != nil {
			return nil, err
		}
	case "CastWithFlash":
		if sp.str("Caster") != "You" || sp.str("ValidSA") != "Spell" {
			return nil, refuse("CastWithFlash shape")
		}
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if ez, ok := sp.get("EffectZone"); ok {
			// gorge withSelfStatics merges a card's OWN
			// face statics of the mode ONLY when they name an EffectZone$
			// (CR 113.6), so this is the card's own "you may cast this
			// spell as though it had flash if ..." grant from every zone.
			if err := lc.selfFlashShape(s, ez); err != nil {
				return nil, err
			}
			lc.out.SelfFlash = true
		}
		// no EffectZone$: withSelfStatics skips the static in hand, so it
		// grants only from the battlefield and never its own card flash
		// (Captain Mar-Vell): an ordinary battlefield static
	case "AlternativeCost":
		if sp.str("ValidSA") == "Spell" && sp.str("ValidCard") == "Card.Self" && sp.str("ValidPlayer") == "You" && sp.str("EffectZone") == "All" {
			// (Land Grant) gorge alternativeCosts' face walk
			// prices a card's own static; bare ValidSA$ Spell, its own
			// ValidCard$ and ValidPlayer$ You is the Spell.Self grant
			// (ValidCard$ is not read on that walk).
			sp.consume("ValidCard", "ValidPlayer")
			sp.m["ValidSA"] = "Spell.Self"
		}
		if sp.str("ValidSA") != "Spell.Self" || sp.str("EffectZone") != "All" {
			return nil, refuse("AlternativeCost shape")
		}
		if sp.str("Cost") == "Reveal<1/Hand>" {
			// "reveal your hand" (gorge cost.go
			// IsWholeHandRevealSpec: payable with any hand, no choice) is
			// a free alternative cost (kernel CardDef.AltFree)
			st.Cost = lCost{}
			sp.consume("Cost")
		} else if st.Cost, err = lc.altCostCost(sp.str("Cost")); err != nil {
			return nil, err
		}
		// a Return<N/Spec> part lowers alone (Gush, Thwart, Daze: the
		// kernel's return alternative, build_ab.go altReturnOK); beside any
		// other part build_ab.go refuses it as a non-mana alternative
		if st.Cost.Forage != nil || st.Cost.ExileG != nil && !st.Cost.ExileHand || st.Cost.Energy != 0 {
			return nil, refuse("alternative cost with a forage / exile-from-graveyard part")
		}
	case "Activations":
		if err := activationLimitStatic(s, sp, &st); err != nil {
			return nil, err
		}
	case "Panharmonicon":
		// gorge panharmoniconEchoes: a battlefield static whose ValidCard$
		// matches the triggering ability's source adds one trigger
		// placement. The one other shape read is the entering-permanent
		// family: ValidMode$ ChangesZone,ChangesZoneAll
		// with Destination$ Battlefield and a ValidCause$ filter, which also
		// needs the cause (the entering permanent or token) to match F2.
		// A ValidMode$ of one player-event mode alone (LifeGained, Drawn),
		// with an optional ValidPlayer$ You, echoes only that mode's triggers
		// of that player (flags "LifeGained" / "Drawn" and "You").
		// Every other gate (Origin$, Condition$, ...) is unread, so the shared
		// unread check refuses it by name.
		v, ok := sp.get("ValidCard")
		if !ok {
			return nil, refuse("Panharmonicon without ValidCard$")
		}
		if st.Cond.Kind != cNone {
			return nil, refuse("Panharmonicon with a Condition$ / IsPresent$ / CheckSVar$ gate")
		}
		if st.F, err = lc.filter(v); err != nil {
			return nil, err
		}
		vm, hasMode := sp.get("ValidMode")
		cause, hasCause := sp.get("ValidCause")
		dest, hasDest := sp.get("Destination")
		switch {
		case !hasMode && !hasCause && !hasDest:
		case hasMode && !hasCause && !hasDest && (vm == "LifeGained" || vm == "Drawn"):
			st.Flags = append(st.Flags, vm)
			switch vp, ok := sp.get("ValidPlayer"); {
			case !ok:
			case strings.TrimSpace(vp) == "You":
				st.Flags = append(st.Flags, "You")
			default:
				return nil, refuse("Panharmonicon ValidPlayer$ %s", vp)
			}
		case hasMode && hasCause && hasDest:
			if vm != "ChangesZone,ChangesZoneAll" {
				return nil, refuse("Panharmonicon ValidMode$ %s", vm)
			}
			if dest != "Battlefield" {
				return nil, refuse("Panharmonicon Destination$ %s", dest)
			}
			if st.F2, err = lc.filter(cause); err != nil {
				return nil, err
			}
		default:
			return nil, refuse("Panharmonicon with a partial ValidMode$ / ValidCause$ / Destination$ set")
		}
	case "UntapOtherPlayer":
		// ValidCard$ over the non-active player's tapped permanents, matched
		// from the static's source and controller (gorge
		// untapOtherStaticsMatch); absent means Card.Self. A player-only
		// filter reads nothing the permanent scan can evaluate.
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if st.F == nil {
			st.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		} else if st.F.Players != 0 {
			return nil, refuse("UntapOtherPlayer ValidCard %q", s.Params["ValidCard"])
		}
	case "CombatDamageToughness":
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if st.F == nil {
			return nil, refuse("CombatDamageToughness without ValidCard$")
		}
	case "CantGainLife", "CantDraw":
		switch v := sp.str("ValidPlayer"); v {
		case "", "Player":
		case "Opponent", "Player.Opponent":
			// gorge lifeGainForbidden / drawForbidden: the spec is matched from
			// the static's controller, so Opponent is every other seat
			st.Flags = append(st.Flags, "BanOpp")
		default:
			return nil, refuse("%s ValidPlayer %q", s.Mode, v)
		}
		sp.consume("ValidPlayer", "Secondary")
	case "CantPreventDamage": // the blanket form, or IsCombat$ True (combat damage only: Frenzied Baloth); ValidSource$, EffectZone$, Affected$ stay unread (gorge cantPreventDamage)
		if c, ok := sp.get("IsCombat"); ok {
			if !strings.EqualFold(strings.TrimSpace(c), "True") {
				return nil, refuse("CantPreventDamage IsCombat %q", c)
			}
			sp.consume("IsCombat")
			st.Flags = append(st.Flags, "NoPrevCombat")
		}
		sp.consume("Secondary")
		// Banefire's "the damage can't be prevented" (X is 5 or more, CheckSVar$ read
		// above): a stack-zone static of the spell itself. Only the pair lowers.
		if vs, ok := sp.get("ValidSource"); ok {
			if strings.TrimSpace(vs) != "Spell.Self" || sp.str("EffectZone") != "Stack" {
				return nil, refuse("CantPreventDamage ValidSource$ %q", vs)
			}
			st.Flags = append(st.Flags, "NoPrevSpellSelf")
		}
	case "CantPutCounter":
		// gorge PutCounterBlocked's object form (rules/layers_restrict.go):
		// ValidCard$ (or ValidObject$) scopes the permanent, CounterType$
		// the kind (absent: every kind). The player form is a separate
		// ValidPlayer$ reader the kernel does not have; refuse it rather
		// than turn it into a no-op object static.
		if vp := sp.str("ValidPlayer"); vp != "" {
			return nil, refuse("CantPutCounter ValidPlayer %q (player counters are not lowered)", vp)
		}
		for _, k := range []string{"ActiveZones", "AffectedZone"} {
			if z, ok := sp.get(k); ok && strings.TrimSpace(z) != "Battlefield" {
				return nil, refuse("CantPutCounter %s$ %q", k, z)
			}
		}
		if vc := sp.str("ValidCard"); vc != "" {
			if st.F, err = readF("ValidCard"); err != nil {
				return nil, err
			}
		} else if vo := sp.str("ValidObject"); vo != "" {
			if st.F, err = readF("ValidObject"); err != nil {
				return nil, err
			}
		} else {
			// an unscoped line forbids every permanent (match-all filter)
			if st.F, err = lc.filter("Card"); err != nil {
				return nil, err
			}
		}
		st.Ctr = -1 // every kind until CounterType$ names one
		if ct, ok := sp.get("CounterType"); ok {
			if ct = strings.TrimSpace(ct); ct != "" && ct != "Any" {
				ck, err := counterKind(ct)
				if err != nil {
					return nil, refuse("CantPutCounter CounterType$ %q", ct)
				}
				st.Ctr = int16(ck)
			}
		}
	case "CantPhaseIn":
		// gorge phaseBlocked (rules/cantphase.go): only the blanket form,
		// every phased-out permanent (The City on the Edge of Forever's
		// token); a narrower ValidCard$ (Teferi's remembered set) needs the
		// kernel's remembered-set filter and is refused.
		if v := strings.TrimSpace(sp.str("ValidCard")); v != "Card.phasedOutPermanent" {
			return nil, refuse("CantPhaseIn ValidCard$ %q", v)
		}
	case "CantSacrifice":
		// gorge sacrificeBlocked (rules/layers_restrict.go): ValidCard$ the
		// permanents, ActiveZones$ Battlefield the only zone it reads. The
		// cause-scoped lines (ValidCause$, ForCost$: Sigarda, Host of Herons)
		// need the sacrifice's cause and cost-or-effect provenance at every
		// choke point, which the kernel does not carry.
		for _, key := range []string{"ValidCause", "ForCost"} {
			if v, ok := sp.get(key); ok {
				return nil, refuse("CantSacrifice %s$ %q", key, v)
			}
		}
		if z, ok := sp.get("ActiveZones"); ok && strings.TrimSpace(z) != "Battlefield" {
			return nil, refuse("CantSacrifice ActiveZones$ %q", z)
		}
		sp.consume("Secondary")
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
	case "ManaConvert":
		if err := manaConvertStatic(sp); err != nil {
			return nil, err // lower_manaconvert.go
		}
	case "DisableTriggers":
		if err := lc.disableTriggersStatic(sp, &st); err != nil {
			return nil, err // lower_disabletriggers.go
		}
	case "ActivateAbilityAsIfHaste", "CantBeSuspected":
		// gorge activatesAsIfHaste (rules/activateasifhaste.go) skips an
		// unscoped static (fail closed); cantBeSuspected
		// (rules/cantbesuspected.go) reads an unscoped one as blanket. The
		// corpus always scopes both: refuse the bare form rather than pick.
		sp.consume("Secondary")
		if st.F, err = readF("ValidCard"); err != nil {
			return nil, err
		}
		if st.F == nil || len(st.F.Clauses) == 0 || st.F.Players != 0 {
			return nil, refuse("%s ValidCard %q", s.Mode, s.Params["ValidCard"])
		}
	default:
		return nil, refuse("static mode %s", s.Mode)
	}
	if left := sp.unread(); len(left) > 0 {
		return nil, refuse("static unread params %v", left)
	}
	if inner := lc.grantInner; len(inner) > 0 {
		// a station self grant's inner statics follow the outer static
		lc.grantInner = nil
		if extra == nil {
			extra = []lStatic{st}
		}
		return append(extra, inner...), nil
	}
	if extra != nil {
		return extra, nil
	}
	return []lStatic{st}, nil
}

// hexproofOwnColorsKW is the granted keyword lowered to its own static kind
// (kop.StatHexproofOwnColors): the kernel has no free keyword bit, and the
// quality is the recipient's own current colours, not the source's.
const hexproofOwnColorsKW = "Hexproof:CardColors"

// continuous lowers a Mode$ Continuous static.
func (lc *lowerCtx) continuous(sp *params, st *lStatic) ([]lStatic, error) {
	aff := sp.str("Affected")
	if err := lc.grantedLookOnly(sp); err != nil {
		return nil, err
	}
	if aff == "" {
		if !sp.is("CharacteristicDefining", "True") {
			return nil, refuse("Continuous without Affected")
		}
		aff = "Card.Self"
	}
	if kw := sp.str("AddKeyword"); kw == hexproofOwnColorsKW {
		// Tam, Mindful First-Year's "each other creature you control has
		// hexproof from each of its colors": the exact Affected$ shape only,
		// and nothing beside it (a condition or any unread key refuses).
		if aff != "Creature.Other+YouCtrl" {
			return nil, refuse("Hexproof:CardColors Affected %q", aff)
		}
		if st.Cond.Kind != cNone {
			return nil, refuse("Hexproof:CardColors static with a condition")
		}
		f, err := lc.filter(aff)
		if err != nil {
			return nil, err
		}
		st.F = f
		st.Kind = "HexproofOwnColors"
		return nil, nil
	}
	// the view-only top-of-library look. gorge reads
	// MayLookAt$ You/True/Player into a look-only continuous effect
	// (engine/rules/layers.go:766-785) whose only consumers are
	// Engine.MayLookAtLibraryTop (engine/rules/layers_static.go:253-330)
	// and the seat view (engine/view/view.go:66-72, :715): no event,
	// decision or legality reader at the pin. Lower it to nothing, and only
	// for the exact library-top shape -- every other MayLookAt$ (hand, a
	// library+battlefield mix, or the MayPlay permission beside it) stays
	// refused.
	//
	// and the view-only face-down battlefield look
	// (DSK Found Footage's printed static, MKM Lumbering Laundry's Effect
	// static). Its readers are library-only (MayLookAtLibraryTop,
	// layers_static.go:255-300), exile-only (MayLookAtFaceDown,
	// layers_static.go:302-340: Zone != ZExile returns false), the seat view
	// (engine/view/card.go:353-374, :522-538; view/view.go:66-72, :719) or
	// the oracle runner's expectation read (engine/rules/oracle_run.go:
	// 213-255); no event, decision or legality reader at the pin, so it too
	// lowers to no static. faceDownLookOnly is the exact shape: any
	// other MayLookAt$ spelling -- in particular one beside MayPlay$ --
	// keeps the named refusal below.
	if ml, ok := sp.get("MayLookAt"); ok {
		az, _ := sp.get("AffectedZone")
		exactAffected := aff == "Card.TopLibrary+YouCtrl" || aff == "Card.TopLibrary+YouOwn"
		libLook := exactAffected && az == "Library" && (ml == "You" || ml == "True" || ml == "Player")
		if libLook && !sp.is("MayPlay", "True") {
			if ez, ok := sp.get("EffectZone"); ok && ez != "Battlefield" {
				return nil, refuse("MayLookAt EffectZone %q", ez)
			}
			sp.consume("AffectedZone")
			st.Kind = "" // view-only: lower to nothing
			return nil, nil
		}
		if faceDownLookOnly(sp.m) || handRevealOnly(sp.m) { // lower_handreveal.go
			sp.consume("AffectedZone")
			st.Kind = "" // view-only: lower to nothing
			return nil, nil
		}
		// Future Sight / Magus of the Future: the library-top look beside
		// MayPlay$ True on the same line. gorge's static scan emits the look
		// as its own view-only ContinuousEffect (layers.go's MayLookAt
		// branch copies base, which carries no MayPlay) next to the MayPlay
		// grant, so the line is the Courser of Kruphix pair on one line: the
		// look lowers to nothing and the permission takes the MayPlay path
		// below (mayPlayStatic, the same library-top rules).
		if !libLook {
			return nil, refuse("MayLookAt shape")
		}
	}
	if kw, _ := sp.get("AddKeyword"); kw == "Beam me up" {
		return nil, lc.beamGrant(sp, st, aff) // lower_beammeup.go
	}
	if az, ok := sp.get("AffectedZone"); ok {
		if az == "Battlefield" {
			// The default.
		} else if az == "Stack" && !sp.is("MayPlay", "True") {
			return nil, lc.stackKwGrant(sp, st, aff) // lower_stackkw.go
		} else if az == "Graveyard" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Flashback") {
			return nil, lc.graveFbGrant(sp, st, aff) // lower_fbgrant.go
		} else if az == "Graveyard" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Escape") {
			return nil, lc.graveEscapeGrant(sp, st, aff) // lower_escapegrant.go
		} else if az == "Hand" && !sp.is("MayPlay", "True") && strings.HasPrefix(sp.str("AddKeyword"), "Warp") {
			return nil, lc.handWarpGrant(sp, st, aff) // lower_warpgrant.go
		} else if az == "Graveyard" && !sp.is("MayPlay", "True") && inertCountAs(sp.str("AddHiddenKeyword"), aff) {
			return nil, graveCountAsStatic(sp, st) // lower_escapegrant.go
		} else if az == "Graveyard" && !sp.is("MayPlay", "True") && sp.str("AddKeyword") != "" {
			return nil, refuseGraveKwGrant(sp, az) // lower_escapegrant.go
		} else if az == "All" && offZonePTShape(sp, aff) {
			return nil, offZonePTStatic(sp, st) // lower_offzonept.go
		} else if !sp.is("MayPlay", "True") {
			return nil, refuse("AffectedZone %q without MayPlay", az)
		}
	}
	if sp.is("MayPlay", "True") {
		return nil, lc.mayPlayStatic(sp, st, aff)
	}
	if _, ok := sp.get("SetMaxHandSize"); ok && aff == "Opponent" {
		return nil, lc.oppMaxHand(sp, st) // lower_playerstatic.go
	}
	if aff == "You" {
		switch {
		case sp.is("SetMaxHandSize", "Unlimited"):
			st.Kind = "NoMaxHandSize"
		case sp.m["AdjustLandPlays"] != "":
			n, err := strconv.Atoi(sp.str("AdjustLandPlays"))
			if err != nil || n < 1 || n > 3 {
				return nil, refuse("AdjustLandPlays %q", sp.str("AdjustLandPlays"))
			}
			st.Kind = "ExtraLandPlay"
			st.Amount = cst(n)
		case sp.is("AddKeyword", "Hexproof"):
			st.Kind = "PlayerHexproof"
		default:
			ok, err := lc.playerStatic(sp, st)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, refuse("player static")
			}
		}
		return nil, nil
	}
	if hk, ok := sp.m["AddHiddenKeyword"]; ok && !sp.used["AddHiddenKeyword"] { // a lure grant lureGrant took is lowered beside
		sp.used["AddHiddenKeyword"] = true
		if strings.TrimSpace(hk) != "CARDNAME can't block." {
			return nil, refuse("AddHiddenKeyword %q", hk)
		}
		f, err := lc.filter(aff)
		if err != nil {
			return nil, err
		}
		st.F = f
		st.Kind = "CantBlock"
		return nil, nil
	}
	f, err := lc.filter(aff)
	if err != nil {
		return nil, err
	}
	st.F = f
	if gc, ok := sp.get("GainControl"); ok {
		// layer 2 (CR 613.1b): "You control enchanted creature";
		// the kernel settles it on the source's attached object, so the
		// affected set must be that object and the static unconditional
		// (any further effect param stays unread and refuses)
		if gc != "You" {
			return nil, refuse("GainControl static %q", gc)
		}
		if !attachedOnly(f) {
			return nil, refuse("GainControl static affected %s", f)
		}
		if st.Cond.Kind != cNone {
			return nil, refuse("GainControl static with a condition")
		}
		st.Kind = "Control"
		return nil, nil
	}
	if sp.is("CharacteristicDefining", "True") {
		// gorge CDASetPT: SetPower$/SetToughness$ each set its half; a
		// missing half keeps the printed value (the "*/4" cards)
		pw, tg, _ := strings.Cut(lc.face.PT, "/")
		half := func(k, printed string) (lValue, error) {
			if e, ok := sp.get(k); ok {
				return lc.value(e)
			}
			n, err := strconv.Atoi(printed)
			if err != nil {
				return lValue{}, refuse("CDA shape")
			}
			return cst(n), nil
		}
		sp2, ok1 := sp.m["SetPower"]
		st2, ok2 := sp.m["SetToughness"]
		if !ok1 && !ok2 {
			return nil, refuse("CDA shape")
		}
		if ok1 && ok2 && sp2 == st2 {
			v, err := lc.value(sp.str("SetPower"))
			if err != nil {
				return nil, err
			}
			sp.consume("SetToughness")
			if ptAggIn(v) {
				return nil, refuse("CDA reads a P/T aggregate") // the layer walk would recurse
			}
			st.Kind = "CDAPT"
			st.Power, st.Tough = v, v
			return nil, nil
		}
		p, err := half("SetPower", pw)
		if err != nil {
			return nil, err
		}
		t, err := half("SetToughness", tg)
		if err != nil {
			return nil, err
		}
		if ptAggIn(p) || ptAggIn(t) {
			return nil, refuse("CDA reads a P/T aggregate") // the layer walk would recurse
		}
		st.Kind = "CDAPT"
		st.Power, st.Tough = p, t
		return nil, nil
	}
	// a Station threshold's self grant lowers here; the rest of the static
	// (AddType, AddKeyword) lowers as any Station threshold static
	if err := lc.stationSelfGrant(sp, st); err != nil {
		return nil, err
	}
	hard := false
	for _, k := range []string{"SetPower", "SetToughness", "SetColor", "RemoveAllAbilities", "RemoveCardTypes", "RemoveCreatureTypes", "RemoveType", "SetName", "AddAllCreatureTypes"} {
		if _, ok := sp.m[k]; ok {
			hard = true
		}
	}
	if t, ok := sp.get("AddType"); ok && !hard {
		// a card type added by a static (Ygra's "are Food artifacts in
		// addition to their other types") is a layer-4 override (CR 613);
		// subtypes alone stay a Continuous subtype grant
		for _, w := range strings.Split(t, "&") {
			if _, ok := typeAddBit(strings.TrimSpace(w)); ok {
				hard = true
			}
		}
	}
	if hard {
		return lc.overrideStatic(sp, st)
	}
	if r, ok := sp.get("RemoveKeyword"); ok {
		// a layer-6 removal beside the Continuous parts (Starforged
		// Sword's "gets +3/+3 and loses flying"): an Override carrying it
		ks, err := removeKeywords(r)
		if err != nil {
			return nil, err
		}
		st.Override = &lOverride{RemoveKw: ks}
		lc.out.overrides = append(lc.out.overrides, st.Override)
	}
	if p, ok := sp.get("AddPower"); ok {
		if st.Power, err = lc.value(p); err != nil {
			return nil, err
		}
	}
	if t, ok := sp.get("AddToughness"); ok {
		if st.Tough, err = lc.value(t); err != nil {
			return nil, err
		}
	}
	if k, ok := sp.get("AddKeyword"); ok {
		if aff == "You" {
			return nil, refuse("player keyword")
		}
		rest, ward, wardLife, err := splitWardGrant(k)
		if err != nil {
			return nil, err
		}
		rest, fbs, err := splitFirebendingGrant(rest)
		if err != nil {
			return nil, err
		}
		rest, melee := splitMeleeGrant(rest)
		rest, exalted := splitExaltedGrant(rest)
		rest, err = lc.grantEvolve(rest, st) // lower_evolvegrant.go
		if err != nil {
			return nil, err
		}
		rest, extort := splitExtortGrant(rest)  // lower_extortgrant.go
		rest, myriad := splitMyriadGrant(rest)  // lower_myriad.go
		rest, err = lc.splitProtGrant(rest, st) // lower_protgrant.go
		if err != nil {
			return nil, err
		}
		rest, st.Landwalk, err = lc.splitLandwalkGrant(rest, aff) // lower_landwalkgrant.go
		if err != nil {
			return nil, err
		}
		rest, st.Evasion, st.Infect = splitStaticKwMarks(rest) // lower_kwmark.go
		if rest != "" {
			if st.Kw, err = pumpKeywords(rest); err != nil {
				return nil, err
			}
		}
		if len(fbs) > 0 && lc.out.Types&tArtifact != 0 && filterSelfOnly(st.F) {
			return nil, refuse("DB$ Mana of an artifact source (typed mana)") // the attacker is this artifact
		}
		for _, n := range fbs {
			// CR 702.189a granted: one latent Attacks trigger per grant
			// (lower_firegrant.go; gorge trigger_granted.go)
			idx, err := lc.firebendGrantTrigger(n)
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; melee > 0; melee-- {
			// CR 702.121a/b granted: one latent Attacks trigger, listed
			// once per instance (lower_meleegrant.go; gorge rules/melee.go)
			idx, err := lc.meleeGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; exalted > 0; exalted-- {
			// CR 702.83a-b granted: one latent Attacks trigger, listed
			// once per instance (lower_exaltedgrant.go; gorge
			// rules/melee.go)
			idx, err := lc.exaltedGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; extort > 0; extort-- {
			// CR 702.101a/b granted: one latent SpellCast trigger, listed
			// once per instance (lower_extortgrant.go; gorge
			// rules/scriptfacts)
			idx, err := lc.extortGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		for ; myriad > 0; myriad-- {
			// CR 702.116a-b granted: one latent Attacks Self trigger,
			// listed once per instance (lower_myriad.go; gorge
			// rules/trigger_granted.go)
			idx, err := lc.myriadGrantTrigger()
			if err != nil {
				return nil, err
			}
			st.Grant = append(st.Grant, idx)
		}
		if ward > 0 || wardLife > 0 {
			// CR 702.21a: a granted ward {N} or ward—pay N life (Sheltered by
			// Ghosts' Aura, Coppercoat Vanguard's anthem): the KwWard bit plus
			// the static's own amount (kop.Static.Ward / WardLife; kernel
			// kw_ward.go)
			st.Kw = append(st.Kw, "Ward")
			st.Ward = ward
			st.WardLife = wardLife
		}
	}
	if t, ok := sp.get("AddType"); ok {
		for _, w := range strings.Split(t, "&") {
			w = strings.TrimSpace(w)
			if w == "ChosenType" {
				// gorge resolveChosenTypes: the host's recorded choice, live;
				// none recorded grants nothing
				st.AddChosen = true
				continue
			}
			if w == "AllBasicLandType" {
				// gorge appendLandTypes: the five basic land types in order,
				// each once (appendTypeOnce); the kernel's mana walk adds the
				// CR 305.6 intrinsic ability per granted type
				for _, b := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
					if !slices.Contains(st.AddSub, b) {
						st.AddSub = append(st.AddSub, b)
					}
				}
				continue
			}
			if !lc.known(w) {
				return nil, refuse("AddType %q", w)
			}
			if !slices.Contains(st.AddSub, w) {
				st.AddSub = append(st.AddSub, w)
			}
		}
	}
	if ab, ok := sp.m["AddAbility"]; ok && !sp.used["AddAbility"] { // not already lowered by stationSelfGrant
		sp.used["AddAbility"] = true
		idx, err := lc.grantedAbility(ab)
		if err != nil {
			return nil, err
		}
		st.Grant = append(st.Grant, idx)
	}
	if tr, ok := sp.m["AddTrigger"]; ok && !sp.used["AddTrigger"] { // not already lowered by stationSelfGrant
		sp.used["AddTrigger"] = true
		idx, err := lc.grantedTrigger(sp, st, tr)
		if err != nil {
			return nil, err
		}
		st.Grant = append(st.Grant, idx)
	}
	if readsFlying(st.F) {
		// a keyword-gated layer-6 effect is ordered after the effects it
		// depends on (gorge abilityDependencyOrder); one that changes the
		// keyword it reads could form a dependency cycle
		for _, k := range st.Kw {
			if k == "Flying" {
				return nil, refuse("keyword-gated static grants the keyword it reads")
			}
		}
		if st.Override != nil {
			for _, k := range st.Override.RemoveKw {
				if k == "Flying" {
					return nil, refuse("keyword-gated static removes the keyword it reads")
				}
			}
		}
	}
	return nil, nil
}

// removeKeywords lowers a RemoveKeyword$ list: plain keywords only (a
// parameterised one, Landwalk:Island, has no kernel bit).
func removeKeywords(kw string) ([]string, error) {
	var out []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if !simpleKeywords[k] {
			return nil, refuse("RemoveKeyword %q", k)
		}
		out = append(out, k)
	}
	return out, nil
}

// readsFlying reports whether a filter reads keywords (withFlying /
// withoutFlying, the only keyword reads kgen lowers).
func readsFlying(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&(fWithFlying|fWithoutFlying) != 0 {
			return true
		}
	}
	return false
}

// selfOnlyCond reports whether a static's condition reads nothing a
// characteristic derivation produces: none, the turn, or the source's own
// counters (a level-up or station static: IsPresent$ Card.Self+counters).
func selfOnlyCond(c lCond) bool {
	switch c.Kind {
	case cNone, cYourTurn:
		return true
	case cPresent:
		if c.F == nil || c.F.Players != 0 || len(c.F.Clauses) == 0 {
			return false
		}
		for _, cl := range c.F.Clauses {
			if cl.Flags&^fSelf != 0 || cl.Types != 0 || cl.TypesAll != 0 || cl.NotTypes != 0 || cl.Subtype != "" || cl.NotSubtype != "" ||
				cl.Colors != "" || cl.NotColors != "" || cl.CmpStat != cmpCounters {
				return false
			}
			if cl.Flags&fSelf == 0 {
				return false
			}
		}
		return true
	}
	return false
}

// creatureOnly reports whether every clause of f requires a creature.
func creatureOnly(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Types != tCreature {
			return false
		}
	}
	return true
}

// grantedTrigger lowers a static's AddTrigger$ (gorge layer 6: the
// affected permanent has the triggered ability; its own controller
// controls the trigger and Self is that permanent). The kernel's static
// grant walk (grantedAbilitiesN) reports the grant for an attached host (an
// Equipment's or Aura's) and, for any other affected filter, for every
// permanent the filter matches (globalGrantsGen: "artifact creatures you
// control have ..."), one instance per granting static in timestamp order.
// AddSVar$
// names SVars of this face the granted body reads (consumed).
func (lc *lowerCtx) grantedTrigger(sp *params, st *lStatic, name string) (int, error) {
	lc.granted++
	defer func() { lc.granted-- }()
	// An attached grant names its host: the kernel finds it as the card's own
	// permanent attached to the carrier (cost_sachost.go).
	aff := sp.str("Affected") // AffectedDefined$ Equipped/Enchanted arrives as .EquippedBy/.EnchantedBy
	lc.grantAttached = !strings.Contains(aff, ",") && (strings.Contains(aff, ".EquippedBy") || strings.Contains(aff, ".EnchantedBy"))
	defer func() { lc.grantAttached = false }()
	if st.F == nil || st.F.Players != 0 || len(st.F.Clauses) == 0 {
		return 0, refuse("AddTrigger affected %s", st.F)
	}
	if strings.Contains(name, "&") || strings.Contains(name, ",") {
		return 0, refuse("AddTrigger %q: more than one trigger", name)
	}
	if sv, ok := sp.get("AddSVar"); ok {
		for _, n := range strings.FieldsFunc(sv, func(r rune) bool { return r == '&' || r == ',' }) {
			if _, ok := lc.face.SVars[strings.TrimSpace(n)]; !ok {
				return 0, refuse("AddSVar %q not on the face", n)
			}
		}
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok {
		return 0, refuse("AddTrigger %q", name)
	}
	if t.Effect == nil {
		t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
	}
	if t.Mode == "Blocks" || t.Mode == "AttackerBlocked" || t.Mode == "AttackerBlockedByCreature" {
		return 0, refuse("AddTrigger of mode %s", t.Mode)
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) < idx+1 {
		return 0, refuse("AddTrigger lowering")
	}
	// (an ImmediateTrigger body appends its reflexive ability after the
	// granted one; it is reached through the body's own spawn, not granted)
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = "Granted"
	return idx, nil
}

// mayPlayStatic lowers "you may play/cast cards matching Affected from
// AffectedZone" (Muldrotha, Tinybones, Quilled Greatwurm, Omniscience).
func (lc *lowerCtx) mayPlayStatic(sp *params, st *lStatic, aff string) error {
	if sp.str("AffectedZone") == "Library" {
		if err := libTopRefusals(sp); err != nil {
			return err
		}
		var err error
		if aff, err = libTopAffected(aff); err != nil {
			return err
		}
	}
	if err := laterPermanentAlternative(aff); err != nil {
		return err
	}
	f, err := lc.filter(aff)
	if err != nil {
		return err
	}
	if err := ownGraveLandsOnly(sp, f); err != nil {
		return err
	}
	st.Kind = "MayPlayFrom"
	st.F = f
	zRaw := sp.str("AffectedZone")
	z := zRaw
	if z == "All" {
		// Weftwalking's first-spell free cast: the exact shape only
		if !sp.is("MayPlayDontGrantZonePermissions", "True") ||
			sp.str("MayPlayPlayer") != "Player.Active" ||
			!sp.is("MayPlayWithoutManaCost", "True") {
			return refuse("MayPlay zone %q outside the first-spell free shape", zRaw)
		}
		z = "Hand"
		st.Flags = append(st.Flags, "FreeOnly")
	}
	// "from outside the game" (The Holodeck's AffectedZone$ Sideboard,Graveyard,
	// CR 400.11): the sideboard rides beside the one real zone as a flag, so a
	// two-zone permission stays ONE static with one once-per-turn count
	// (gorge keys the limit on the static, not the zone)
	if base, ok := strings.CutPrefix(z, "Sideboard,"); ok && !strings.Contains(base, ",") {
		z = base
		st.Flags = append(st.Flags, "AlsoSideboard")
	}
	st.Zone = zoneNames[z]
	if st.Zone != zGraveyard && st.Zone != zExile && st.Zone != zHand && st.Zone != zLibrary {
		return refuse("MayPlay zone %q", z)
	}
	if dg, ok := sp.get("MayPlayDontGrantZonePermissions"); ok && (dg != "True" || zRaw != "All") {
		return refuse("MayPlayDontGrantZonePermissions$ %q on zone %q", dg, zRaw)
	}
	if mp, ok := sp.get("MayPlayPlayer"); ok {
		if mp != "Player.Active" || zRaw != "All" {
			return refuse("MayPlayPlayer$ %q", mp)
		}
		st.Flags = append(st.Flags, "SeatActive")
	}
	if ez, ok := sp.get("EffectZone"); ok {
		switch ez {
		case "Battlefield":
		case "Graveyard":
			st.Flags = append(st.Flags, "FunctionsInGraveyard")
		case "Exile":
			st.Flags = append(st.Flags, "FunctionsInExile")
		default:
			return refuse("MayPlay EffectZone %q", ez)
		}
	}
	if l, ok := sp.get("MayPlayLimit"); ok {
		if l != "1" {
			return refuse("MayPlayLimit %q", l)
		}
		// once per turn PER STATIC, a library permission's too (gorge
		// engine/rules/mayplay.go mayPlayKeyLimitReached)
		st.Flags = append(st.Flags, "OncePerTurn")
	}
	if sp.is("MayPlayIgnoreType", "True") {
		st.Flags = append(st.Flags, "AnyManaType")
	}
	if sp.is("MayPlayWithoutManaCost", "True") {
		st.Flags = append(st.Flags, "WithoutManaCost")
	}
	if rc, ok := sp.get("RaiseCost"); ok {
		// RemoveAnyCounter<N/Any/Creature>: remove N counters from among
		// creatures you control (Quilled Greatwurm 6, Dawnhand Dissident 3);
		// the count rides in Static.A (the flag keeps its spelling)
		n, ok := removeAnyCounterN(rc)
		if !ok {
			return lc.mayRaiseStatic(rc, st) // PayLife / Discard / PayLife + Sac (lower_mayraise.go)
		}
		st.Flags = append(st.Flags, "RaiseCostRemove6CountersFromCreatures")
		st.Amount = cst(n)
	}
	return nil
}

// overrideStatic lowers the layer-hard statics (Witness Protection,
// Imprisoned in the Moon) to an override record applied while the static
// applies.
func (lc *lowerCtx) overrideStatic(sp *params, st *lStatic) ([]lStatic, error) {
	var pump lStatic
	havePump := false
	if _, okP := sp.m["AddPower"]; okP {
		havePump = true
	} else if _, okT := sp.m["AddToughness"]; okT {
		havePump = true
	}
	if havePump {
		pm := map[string]string{}
		for _, k := range []string{"Affected", "AffectedZone", "AddPower", "AddToughness"} {
			if v, ok := sp.m[k]; ok {
				pm[k] = v
			}
		}
		pump = lStatic{Kind: "Continuous", Cond: st.Cond}
		if _, err := lc.continuous(newParams(pm), &pump); err != nil {
			return nil, err
		}
		if pump.Kind != "Continuous" {
			return nil, refuse("override pump shape")
		}
		sp.consume("AddPower", "AddToughness")
	}
	o := &lOverride{}
	pS, okP := sp.get("SetPower")
	tS, okT := sp.get("SetToughness")
	if okP || okT {
		// a static may set just one of them (Maha's base toughness 1);
		// the other keeps its earlier-layer value (gorge StaticSet)
		pn, e1 := strconv.Atoi(pS)
		tn, e2 := strconv.Atoi(tS)
		if okP && e1 != nil || okT && e2 != nil {
			// a non-integer value is read live at every layer-7b check,
			// anchored on the static's source (an Equipment's "your life
			// total" is its controller's). AffectedX is the per-affected
			// word, a different anchor, and stays refused by name. Only an
			// attached static is lowered here: the walk would apply a global
			// one too, but that shape (Porcelain Gallery's creature count)
			// has no lockstep repro yet, so it stays refused by policy.
			if pS == "AffectedX" || tS == "AffectedX" {
				return nil, refuse("set P/T %q/%q: the per-affected word", pS, tS)
			}
			if !attachedOnly(st.F) {
				return nil, refuse("set P/T %q/%q: a live value on a non-attached static", pS, tS)
			}
			pv, tv := cst(0), cst(0)
			var err error
			if okP {
				if pv, err = lc.value(pS); err != nil {
					return nil, refuse("set P/T %q/%q: %v", pS, tS, err)
				}
			}
			if okT {
				if tv, err = lc.value(tS); err != nil {
					return nil, refuse("set P/T %q/%q: %v", pS, tS, err)
				}
			}
			o.HasSetPT, o.LivePT, o.KeepP, o.KeepT = true, true, !okP, !okT
			o.PVal, o.TVal = pv, tv
		} else {
			o.HasSetPT, o.SetP, o.SetT, o.KeepP, o.KeepT = true, pn, tn, !okP, !okT
		}
	}
	if c, ok := sp.get("SetColor"); ok {
		o.HasSetColors = true
		cols, ok := colorWordSet(c)
		if !ok {
			return nil, refuse("SetColor %q", c)
		}
		o.SetColors = cols
	}
	if sp.is("RemoveAllAbilities", "True") {
		o.LoseAbilities = true
	}
	removeCard := sp.is("RemoveCardTypes", "True")
	removeCreature := sp.is("RemoveCreatureTypes", "True")
	if v, ok := sp.get("RemoveType"); ok {
		// layer 4 (CR 613.1d, 205.1b): the devotion gods' "isn't a creature".
		// Only the exact Creature word is lowered (gorge applyTypeEffect
		// RemoveTypes strips the named word from the type list, before this
		// effect's AddType$ words); any other value stays refused by name.
		if strings.TrimSpace(v) != "Creature" {
			return nil, refuse("RemoveType %q", v)
		}
		o.RemoveTypes = uint16(typeBits["Creature"])
	}
	if v, ok := sp.get("AddAllCreatureTypes"); ok {
		// layer 4 (CR 205.3m / 702.73a): the object has every creature
		// type. Only the exact True shape is lowered; any other value
		// (a False, an expression) stays refused by name.
		if !sp.is("AddAllCreatureTypes", "True") {
			return nil, refuse("AddAllCreatureTypes %q", v)
		}
		o.AllCreatureTypes = true
	}
	if t, ok := sp.get("AddType"); ok {
		for _, w := range strings.Split(t, "&") {
			w = strings.TrimSpace(w)
			if b, ok := typeAddBit(w); ok {
				o.SetTypes |= b
				continue
			}
			if !lc.known(w) {
				return nil, refuse("AddType %q", w)
			}
			o.SetSubtypes = append(o.SetSubtypes, w)
		}
	}
	if k, ok := sp.get("AddKeyword"); ok {
		ks, err := pumpKeywords(k)
		if err != nil {
			return nil, err
		}
		o.AddKw = ks
	}
	if removeCard {
		o.HasSetTypes = true
		o.HasSetSubtypes = true
		if err := o.keepSubtypes(strings.Split(sp.str("AddType"), "&")); err != nil {
			return nil, err
		}
	} else {
		// gorge continuous layers: without RemoveCardTypes$ the AddType$
		// words are added to the object's types (layer 4), the rest of the
		// record (P/T, colour, abilities) applies as with it
		if removeCreature {
			// gorge applyTypeEffect strips the creature subtypes and the
			// all-creature-types mark, then adds the effect's AddType$
			// words; the kernel's strip is a per-override mask step walked
			// for attached overrides with subtype-only additions
			if !attachedOnly(st.F) {
				return nil, refuse("global override RemoveCreatureTypes without RemoveCardTypes")
			}
			if o.SetTypes != 0 {
				return nil, refuse("override RemoveCreatureTypes adds a card type")
			}
			o.RemoveCreatureSub = true
		}
		for _, w := range o.SetSubtypes {
			switch w {
			case "Plains", "Island", "Swamp", "Mountain", "Forest":
				// CR 305.6: a basic land type carries its mana ability. gorge
				// appends the intrinsic "{T}: Add [c]" for every derived basic
				// type word (mana_activation.go's CR 305.6 block), so the
				// kernel's grant is lowerable exactly when the same AddType$
				// list also makes the object a LAND (Ashaya, Soul of the Wild's
				// "nontoken creatures you control are Forest lands", Life and
				// Limb's Saprolings): the mana walk's blg views (static_landtype.go)
				// cover any plain-shaped card then. Without the Land type the
				// intrinsic still applies in gorge but the kernel's blgMask
				// reads only lands, so that shape stays refused.
				if o.SetTypes&typeBits["Land"] == 0 {
					return nil, refuse("override adds basic land type %q", w)
				}
			}
		}
		o.AddTypes, o.SetTypes = o.SetTypes, 0
		o.AddSubtypes, o.SetSubtypes = o.SetSubtypes, nil
	}
	if r, ok := sp.get("RemoveKeyword"); ok {
		ks, err := removeKeywords(r)
		if err != nil {
			return nil, err
		}
		o.RemoveKw = ks
	}
	if !attachedOnly(st.F) {
		// a global override static (CR 613: Maha, Ygra) is walked
		// in layers 4-7 (its layer-6 parts too); its layer-3 name is
		// not (the naming path reads attached overrides only)
		if _, ok := sp.get("SetName"); ok {
			return nil, refuse("global override with SetName")
		}
		l6 := o.LoseAbilities || len(o.AddKw) > 0 || len(o.RemoveKw) > 0
		if l6 && !selfOnlyCond(st.Cond) {
			// a counting condition read while every permanent's
			// abilities are derived would re-enter the derivation
			return nil, refuse("global layer-6 override with condition %d", st.Cond.Kind)
		}
		if o.LoseAbilities && !creatureOnly(st.F) {
			// a land's mana abilities (CR 305.6) are not walked: the
			// mana planner reads them directly
			return nil, refuse("global lose-all override over non-creatures")
		}
	}
	if err := walkFilterOK(st.F); err != nil {
		return nil, err
	}
	if err := lc.finishOverrideStatic(sp, st, o); err != nil {
		return nil, err
	}
	if havePump {
		// CR 613.6: an override part before layer 6 keeps the whole effect,
		// the pump beside it too, applying once its source loses all abilities
		pump.OutlivesLoss = o.beginsBeforeLayer6()
		return []lStatic{pump, *st}, nil
	}
	return []lStatic{*st}, nil
}

// beginsBeforeLayer6 reports whether o has a part in layers 1 to 5 (the
// reading of ovrKeptOnLoss in the kernel, for a lowered override).
func (o *lOverride) beginsBeforeLayer6() bool {
	return o.HasSetTypes || o.SetTypes != 0 || o.AddTypes != 0 || o.HasSetSubtypes || len(o.SetSubtypes) > 0 ||
		len(o.AddSubtypes) > 0 || o.AllCreatureTypes || o.RemoveCreatureSub || o.RemoveTypes != 0 || o.HasSetColors || o.AddColors != "" || o.Name != ""
}

// keepSubtypes records what removing o's card types leaves, from the words the
// effect adds: gorge's own reading of CR 205.1a, so the two cannot differ on a
// word. A kept creature set is refused: nothing records a kept all-creature-types
// mark, and RemoveCreatureTypes$ would have to take it back.
func (o *lOverride) keepSubtypes(added []string) error {
	for i, w := range added {
		added[i] = strings.TrimSpace(w)
	}
	o.Kept = effects.SubtypesKeptBy(added)
	if o.Kept.Creature() {
		return refuse("RemoveCardTypes$ keeps creature subtypes")
	}
	return nil
}

// finishOverrideStatic reads an override static's name and granted ability
// and records it.
func (lc *lowerCtx) finishOverrideStatic(sp *params, st *lStatic, o *lOverride) error {
	if n, ok := sp.get("SetName"); ok {
		// The name filters compare CardIDs (Hare Apparent), so the rename
		// only names the object: lowerPool emits a name card carrying it
		// (dirac/feat reads the renamed object's name rows from it).
		o.Name = n
		lc.out.Tokens = append(lc.out.Tokens, nameCardKey+n)
	}
	if ab, ok := sp.get("AddAbility"); ok {
		idx, err := lc.grantedAbility(ab)
		if err != nil {
			return err
		}
		o.GrantAbility = append(o.GrantAbility, idx)
	}
	st.Kind = "Override"
	st.Override = o
	lc.out.overrides = append(lc.out.overrides, o)
	return nil
}

// lowerRepl lowers an R: line to a replacement-lite record.
func (lc *lowerCtx) lowerRepl(r cards.Repl) error {
	if printedPreventRepl(r) {
		// the printed K:Prevent keyword already became a CardDef fact; this
		// minted bodyless prevention line must not also lower (lower_preventkw.go)
		return nil
	}
	if lc.selfPreventRepl(r) { // lower_preventkw.go
		return nil
	}
	rp := newParams(r.Params)
	rp.consume("Event", "ReplaceWith", "ReplacementResult")
	rl := lRepl{}
	if b, ok := rp.get("ClassBand"); ok {
		c, err := classBandCond(b) // lower_delayed.go
		if err != nil {
			return err
		}
		rl.Cond = c
	}
	if _, kw := r.Params["Keyword"]; !kw {
		// a plain R: line (Banner of Kinship): gorge runs its choice on the
		// entered object; the ETBReplacement keyword's line (Heraldic
		// Banner) asks while the spell is on the stack
		rl.S = "Updated"
	}
	var err error
	switch r.Event {
	case "Moved":
		if rp.str("Destination") == "Graveyard" {
			if ok, err := lc.exileOnLeave(rp, r.With, &rl); ok || err != nil {
				if err != nil {
					return err
				}
				break
			}
			if err := lc.graveToLibrary(rp, r.With); err != nil {
				return err
			}
			rl.Kind = "GraveToLibrary"
			break
		}
		if ok, err := lc.discardOppToBattlefield(rp, r.With, &rl); ok || err != nil { // lower_discardetb.go
			if err != nil {
				return err
			}
			break
		}
		if ok, err := lc.payBeforeETB(rp, r.With, &rl); ok || err != nil { // lower_paybeforeetb.go
			if err != nil {
				return err
			}
			break
		}
		if rp.str("Destination") != "Battlefield" {
			return refuse("repl-hard: Moved to %s", rp.str("Destination"))
		}
		vc := rp.str("ValidCard")
		other := vc != "Card.Self" && !strings.HasPrefix(vc, "Card.Self+")
		// a self ETB replacement's condition and body read the
		// entering spell (Count$Converge)
		lc.replSelf = !other
		defer func() { lc.replSelf = false }()
		if other {
			if rp.str("ActiveZones") != "Battlefield" {
				return refuse("other-object ETB replacement without ActiveZones Battlefield")
			}
			var err error
			if rl.F, err = lc.filter(vc); err != nil {
				return err
			}
		} else if vc != "Card.Self" {
			// Card.Self+wasCastByYou: a condition on the entering object.
			if rl.Cond.Kind != cNone {
				return refuse("two replacement conditions")
			}
			f, err := lc.filter(vc)
			if err != nil {
				return err
			}
			rl.Cond = lCond{Kind: cRefMatches, Ref: lRef{Kind: rSelf}, F: f, Op: opGE, A: cst(1)}
		}
		wc, wcOK, err := lc.wasCastHandCond(rp, other) // lower_wascast.go
		if err != nil {
			return err
		}
		if wcOK {
			if rl.Cond.Kind != cNone {
				return refuse("two replacement conditions")
			}
			rl.Cond = wc
		} else if cs, ok := rp.get("CheckSVar"); ok {
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			if rl.Cond.Kind != cNone {
				return refuse("two replacement conditions")
			}
			rl.Cond = lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			// gorge replacementMatches: splitCompare/applyCompare on the
			// value; read only for the converge count (Steel Exemplar),
			// every other SVarCompare$ stays unread; the
			// spend too (Freestrider Commando's EQ0), and bloodthirst's GT0
			// (lower_bloodthirst.go)
			if cmp, ok := rp.m["SVarCompare"]; ok && (v.Kind == vConverge || v.Kind == vCastManaSpent || v.Kind == vDmgOppsTaken) {
				rp.used["SVarCompare"] = true
				op, b, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				rl.Cond.Op, rl.Cond.B = op, b
			}
		}
		if err := lc.replIsPresent(rp, &rl); err != nil { // lower_handmove.go
			return err
		}
		if r.With == nil {
			return refuse("ETB replacement without ReplaceWith")
		}
		ab := &lAbility{Kind: akLatent}
		lc.replOptional = strings.HasSuffix(strings.ToLower(rp.str("KeywordLine")), ":optional")
		if r.With.API == "Clone" && (rl.S == "Updated" || other || rl.Cond.Kind != cNone) {
			return refuse("ETB copy replacement shape")
		}
		savedRE := lc.replEnter
		lc.replEnter = true
		err = lc.withAbility(ab, func() error { return lc.replChain(r.With, other, &rl.Cond) })
		lc.replEnter = savedRE
		if err != nil {
			return err
		}
		rl.Prog = ab.Prog
		rl.Kind = "EntersWith"
		if spyglassChain(r.With) || handNameChain(r.With) {
			// gorge runs this chain (not a bare NameCard body) once the permanent
			// is on the battlefield: the queued ask, not the stack pre-ask
			rl.S = "Updated"
		}
		if other {
			rl.Kind = "OthersEnterWith"
		}
	case "Counter":
		if _, ok := rp.m["ValidCard"]; !ok && rp.str("ActiveZones") == "Battlefield" {
			if err := lc.spellsUncounterable(rp, &rl); err != nil { // lower_uncounterable.go
				return err
			}
			break
		}
		if rp.str("Layer") != "CantHappen" || rp.str("ValidCard") != "Card.Self" || rp.str("ValidSA") != "Spell" {
			return refuse("Counter replacement shape")
		}
		rl.Kind = "CantBeCountered"
		if cs, ok := rp.get("CheckSVar"); ok {
			// Banefire's "if X is 5 or more": the gate reads the spell's X
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			op, b, err := lc.cmpSpec(rp.str("SVarCompare"))
			if err != nil {
				return err
			}
			rl.Cond = lCond{Kind: cValue, A: v, Op: op, B: b}
		}
	case "Untap":
		if rp.str("Layer") != "CantHappen" {
			return refuse("Untap replacement shape")
		}
		vc := rp.str("ValidCard")
		if vc == "" {
			return refuse("Untap replacement without ValidCard$")
		}
		if _, ok := rp.get("ValidStepTurnToController"); !ok {
			// no step key: gorge cards.ReplUntap matches every Untap event
			// (rules/replacement_match.go), the "can't become untapped" class
			if r.With != nil {
				return refuse("Untap replacement ReplaceWith$ without ValidStepTurnToController$")
			}
			if err := lc.untapReplFilter(rp, &rl, vc, "AnyUntap"); err != nil {
				return err
			}
			break
		}
		if rp.str("ValidStepTurnToController") != "You" {
			return refuse("Untap replacement shape")
		}
		rl.Kind = "DoesntUntap"
		if vc != "Card.Self" {
			if err := lc.othersDoesntUntap(rp, &rl, vc); err != nil {
				return err
			}
		}
	case "GameLoss", "GameWin":
		if rp.str("Layer") != "CantHappen" || rp.str("ActiveZones") != "Battlefield" {
			return refuse("%s replacement shape", r.Event)
		}
		vp := rp.str("ValidPlayer")
		switch {
		case r.Event == "GameLoss" && vp == "You":
			rl.Kind = "YouCantLose"
			// ValidLoseReason$ LifeReachedZero narrows the replacement to the
			// loss for 0 or less life (CR 704.5a): poison, a drawn-from-empty
			// library and "loses the game" effects still lose (gorge
			// replacement_match.go compares the reason to the loss event's).
			// Any other reason refuses.
			if reason, ok := rp.get("ValidLoseReason"); ok {
				if strings.TrimSpace(reason) != "LifeReachedZero" {
					return refuse("GameLoss ValidLoseReason %q", reason)
				}
				rl.Bits = kop.CantLoseLife
			}
		case r.Event == "GameWin" && vp == "Opponent":
			rl.Kind = "OppsCantWin"
		default:
			return refuse("%s ValidPlayer %q", r.Event, vp)
		}
	case "DamageDone":
		if ok, err := lc.damageRaise(rp, r.With, &rl); ok || err != nil { // lower_damageraise.go
			if err != nil {
				return err
			}
			break
		}
		if ok, err := lc.damagePlus(rp, r.With, &rl); ok || err != nil {
			if err != nil {
				return err
			}
			break
		}
		if err = lc.damageRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "GainLife":
		if err := lc.lifeGainRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "Draw":
		if err := lc.drawRepl(rp, r.With, &rl); err != nil { // lower_drawrepl.go
			return err
		}
	case "LifeReduced":
		if err := lc.lifeLossRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "CreateToken":
		if err := lc.tokenRepl(rp, r.With, &rl); err != nil { // lower_replmult.go
			return err
		}
	case "AddCounter":
		if err := lc.addCounterRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "TurnFaceUp":
		if err := lc.turnUpRepl(rp, r.With, &rl); err != nil { // lower_turnfaceuprepl.go
			return err
		}
	case "Destroy":
		if err := lc.destroyHealRepl(rp, r.With, &rl); err != nil {
			return err
		}
	default:
		return refuse("repl-hard: %s replacement", r.Event)
	}
	if err := lc.replCondKeys(rp, &rl, rl.Kind); err != nil { // lower_condkeys.go
		return err
	}
	if left := rp.unread(); len(left) > 0 {
		return refuse("replacement unread params %v", left)
	}
	if _, ok := r.Params["ClassBand"]; ok && rl.Kind != "DoubleCountersYou" {
		// the kernel reads Repl.Cond only where lower_delayed.go wired it
		return refuse("ClassBand on a %s replacement", rl.Kind)
	}
	if rl.Kind == "ExileOnLeave" {
		// State.exileOnLeave holds one filter per card, so two dies-to-exile
		// abilities on one face would both use the last one's filter.
		for i := range lc.out.Repls {
			if lc.out.Repls[i].Kind == "ExileOnLeave" {
				return refuse("repl-hard: two dies-to-exile replacements on one face")
			}
		}
	}
	lc.out.Repls = append(lc.out.Repls, rl)
	return nil
}

// withIs: the replacement body is exactly DB$ api with the given params
// (and the always-present descriptive ones), no sub-ability.
func withIs(w *cards.SA, api string, want map[string]string) bool {
	if w == nil || w.API != api || w.Sub != nil {
		return false
	}
	n := 0
	for k, v := range w.Params {
		if ignoredParams[k] {
			continue
		}
		if strings.TrimSpace(want[k]) != strings.TrimSpace(v) || want[k] == "" {
			return false
		}
		n++
	}
	return n == len(want)
}

// destroyHealRepl lowers Dr. McCoy, Surly Surgeon's destroy replacement: an
// other creature its controller controls with a +1/+1 counter would be
// destroyed; instead remove all +1/+1 counters from it and heal all damage
// on it. The body is a chain, so withIs cannot read it.
func (lc *lowerCtx) destroyHealRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidCard") != "Creature.Other+YouCtrl+counters_GE1_P1P1" {
		return refuse("repl-hard: Destroy replacement shape")
	}
	if w == nil || w.API != "RemoveCounter" || w.Sub == nil {
		return refuse("repl-hard: Destroy replacement body")
	}
	bp := newParams(w.Params)
	bp.consume("Defined", "CounterType", "CounterNum", "SubAbility")
	if bp.str("Defined") != "ReplacedCard" || bp.str("CounterType") != "P1P1" || bp.str("CounterNum") != "All" || len(bp.unread()) != 0 {
		return refuse("repl-hard: Destroy replacement body")
	}
	hp := w.Sub
	if hp.API != "HealDamage" || hp.Sub != nil {
		return refuse("repl-hard: Destroy replacement heal body")
	}
	subp := newParams(hp.Params)
	subp.consume("Defined")
	if subp.str("Defined") != "ReplacedCard" || len(subp.unread()) != 0 {
		return refuse("repl-hard: Destroy replacement heal body")
	}
	f, err := lc.filter(rp.str("ValidCard"))
	if err != nil {
		return err
	}
	rl.Kind, rl.F = "DestroyHeal", f
	return nil
}

// damageRepl lowers the DamageDone replacements that are not plus-N:
// Twinflame Tyrant's doubling (a source you control, to an opponent or a
// permanent an opponent controls), the filtered doublers (lower_replmult.go)
// and Crystal Barricade's prevention (noncombat damage to your other
// creatures).
func (lc *lowerCtx) damageRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse("repl-hard: DamageDone ActiveZones")
	}
	if rp.is("Prevent", "True") {
		if rp.str("ValidTarget") != "Creature.YouCtrl+Other" || !rp.is("IsCombat", "False") || w != nil {
			return refuse("repl-hard: DamageDone prevention shape")
		}
		rl.Kind = "PreventNoncombatToOthers"
		return nil
	}
	if rp.str("ValidSource") == "Card.YouCtrl,Emblem.YouCtrl" && rp.str("ValidTarget") == "Opponent,Permanent.OppCtrl" &&
		w != nil && withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) &&
		lc.face.SVars[w.Params["VarValue"]] == "ReplaceCount$DamageAmount/Twice" {
		rl.Kind = "DoubleDamageToOpps"
		return nil
	}
	if ok, err := lc.preventMillRepl(rp, w, rl); ok || err != nil { // lower_replmult.go
		return err
	}
	if ok, err := lc.selfDamageRepl(rp, w, rl); ok || err != nil { // lower_replmult.go
		return err
	}
	return lc.damageDoubler(rp, w, rl)
}

// graveToLibrary checks Progenitus's "if this would be put into a
// graveyard from anywhere, shuffle it into its owner's library instead".
func (lc *lowerCtx) graveToLibrary(rp *params, w *cards.SA) error {
	if rp.str("ValidCard") != "Card.Self" || rp.str("ActiveZones") != "" ||
		!withIs(w, "ChangeZone", map[string]string{"Hidden": "True", "Origin": "All", "Destination": "Library",
			"Defined": "ReplacedCard", "Reveal": "True", "Shuffle": "True"}) {
		return refuse("repl-hard: Moved to Graveyard shape")
	}
	return nil
}

// exileOnLeave lowers the dies-to-exile shape: a non-self permanent
// matching ValidCard/ValidLKI is exiled instead of put into a graveyard.
// Only battlefield-origin shapes are exact: an explicit Origin$ Battlefield,
// a ValidLKI Creature.* (gorge's implicit battlefield gate), or a Permanent
// filter (permanents exist only on the battlefield). The body must be a plain
// ChangeZone of ReplacedCard to Exile; Hidden is accepted (it changes nothing
// for a Defined$ object and the kernel does not model face-down exile).
func (lc *lowerCtx) exileOnLeave(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	vc, vl := rp.str("ValidCard"), rp.str("ValidLKI")
	if vc == "" && vl == "" {
		return false, nil
	}
	if vc == "Card.Self" || strings.HasPrefix(vc, "Card.Self+") {
		return false, nil // Progenitus and other self replacements
	}
	if !exileBody(w) {
		return false, nil
	}
	if az := rp.str("ActiveZones"); az != "" && az != "Battlefield" {
		return true, refuse("repl-hard: dies-to-exile ActiveZones %q", az)
	}
	origin := rp.str("Origin")
	battlefield := origin == "Battlefield"
	if !battlefield {
		switch origin {
		case "", "All", "Any":
			battlefield = permanentSpec(vc) || strings.HasPrefix(vl, "Creature.")
		}
	}
	anyZone := false
	if !battlefield {
		// "from anywhere": no Origin$ (or All/Any) and a card spec. gorge's
		// replacementMatches then takes a move to a graveyard from every zone
		// an explicit other origin and ValidLKI stay refused.
		switch origin {
		case "", "All", "Any":
			anyZone = vl == ""
		}
		if !anyZone {
			return true, refuse("repl-hard: dies-to-exile from any zone")
		}
	}
	spec := vc
	if spec == "" {
		spec = vl
	} else if vl != "" {
		return true, refuse("repl-hard: dies-to-exile ValidCard and ValidLKI")
	}
	f, err := lc.filter(spec)
	if err != nil {
		return true, err
	}
	if anyZone {
		for _, c := range f.Clauses {
			// the kernel matches the moving card by what it is (types, colours,
			// subtypes, owner, token); a controller or a stat clause on a card
			// in a hidden zone is not read exactly
			if c.Ctrl != whoAny || c.CmpStat != cmpNone || c.BasePermanent || c.SpellBase || c.Unblocked ||
				c.Flags&^(fToken|fNonToken) != 0 {
				return true, refuse("repl-hard: dies-to-exile from any zone with a controller or stat clause")
			}
		}
		rl.AnyZone = true
	}
	rl.Kind = "ExileOnLeave"
	rl.F = f
	return true, nil
}

// permanentSpec: a bare Permanent base matches only battlefield objects.
func permanentSpec(vc string) bool {
	if !strings.HasPrefix(vc, "Permanent") {
		return false
	}
	if len(vc) == len("Permanent") {
		return true
	}
	switch vc[len("Permanent")] {
	case '.', '+', ',':
		return true
	}
	return false
}

// exileBody: DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile
// (Origin All/Battlefield, Hidden optional), no sub-ability or rider.
func exileBody(w *cards.SA) bool {
	if w == nil || w.API != "ChangeZone" || w.Sub != nil {
		return false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" {
		return false
	}
	if o := bp.str("Origin"); o != "" && o != "All" && o != "Battlefield" {
		return false
	}
	if h, ok := bp.get("Hidden"); ok && !strings.EqualFold(h, "True") {
		return false
	}
	bp.consume("Defined", "Destination", "Origin", "Hidden")
	return len(bp.unread()) == 0
}

// addCounterRepl lowers the counter replacements: Doubling Season's
// unfiltered EffectOnly doubling, plus the filtered doublers and plus/minus
// one shapes (ValidCounterType, DB$ ReplaceCounter).
func (lc *lowerCtx) addCounterRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse("repl-hard: AddCounter ActiveZones")
	}
	if _, ok := rp.get("ValidPlayer"); ok {
		return lc.onePoisonRepl(rp, w, rl) // lower_delayed.go
	}
	if _, ok := rp.get("ValidSource"); ok {
		if rp.str("ValidObject") == youPlusObject {
			return lc.youPlusCountersRepl(rp, w, rl) // lower_counterrepl.go
		}
		if rp.str("ValidObject") == "Permanent.inZoneBattlefield,Player" {
			return lc.youDoubleCountersRepl(rp, w, rl) // lower_delayed.go
		}
		return lc.putterDoubleCountersRepl(rp, w, rl) // lower_counterrepl.go
	}
	vc := rp.str("ValidCard")
	if vc == "" {
		return refuse("repl-hard: AddCounter ValidCard")
	}
	if rp.str("ValidSource") != "" || rp.str("ValidObject") != "" {
		return refuse("repl-hard: AddCounter putter shape")
	}
	if rp.is("EffectOnly", "True") {
		// Doubling Season: ValidCard$ Permanent.YouCtrl+inZoneBattlefield,
		// DB$ ReplaceCounter | Amount$ Y, Y:ReplaceCount$CounterNum/Twice
		if vc != "Permanent.YouCtrl+inZoneBattlefield" || w == nil ||
			!withIs(w, "ReplaceCounter", map[string]string{"Amount": w.Params["Amount"]}) ||
			lc.face.SVars[w.Params["Amount"]] != "ReplaceCount$CounterNum/Twice" {
			return refuse("repl-hard: AddCounter replacement shape")
		}
		rl.Kind = "DoubleCounters"
		return nil
	}
	ck := rp.str("ValidCounterType")
	if ck == "" {
		return lc.anyKindCountersRepl(rp, w, rl, vc) // lower_counterrepl.go
	}
	if _, err := counterKind(ck); err != nil {
		return refuse("repl-hard: AddCounter counter type %q", ck)
	}
	if w == nil || w.API != "ReplaceCounter" || w.Sub != nil {
		return refuse("repl-hard: AddCounter body")
	}
	bp := newParams(w.Params)
	if bp.str("ValidCounterType") != ck {
		return refuse("repl-hard: AddCounter body counter type")
	}
	amt := bp.str("Amount")
	if cc, ok := bp.get("ChooseCounter"); ok && !strings.EqualFold(cc, "True") {
		return refuse("repl-hard: AddCounter ChooseCounter %q", cc)
	}
	bp.consume("ValidCounterType", "Amount", "ChooseCounter")
	if left := bp.unread(); len(left) > 0 {
		return refuse("repl-hard: AddCounter body params %v", left)
	}
	switch lc.face.SVars[amt] {
	case "ReplaceCount$CounterNum/Twice":
		rl.Kind, rl.Ctr = "DoubleCountersOn", ck
	case "ReplaceCount$CounterNum/Plus.1":
		rl.Kind, rl.N, rl.Ctr = "AddCountersOn", cst(1), ck
	case "ReplaceCount$CounterNum/Minus.1":
		rl.Kind, rl.N, rl.Ctr = "AddCountersOn", cst(-1), ck
	default:
		return refuse("repl-hard: AddCounter amount %q", lc.face.SVars[amt])
	}
	var err error
	if rl.F, err = lc.filter(vc); err != nil {
		return err
	}
	return nil
}

// replChain lowers an ETB replacement's ReplaceWith chain: enters tapped,
// enters with counters, as-enters choices.
// A chained Tap/PutCounter node may carry its own ConditionCheckSVar$ gate (a
// SkipIf over that node); an absent Defined$ names the
// entering object.
func (lc *lowerCtx) replChain(sa *cards.SA, other bool, cond *lCond) error {
	for node := sa; node != nil; node = node.Sub {
		ps := newParams(node.Params)
		ps.consume("SubAbility")
		want := "Self"
		if other {
			want = "ReplacedCard"
		}
		// A chained node's own Condition gate (gorge registry.go:1226: a
		// failed gate skips this node only; the chain continues). The
		// unchained single-node shapes keep the whole-replacement condition
		// below (*cond), unchanged.
		chained := sa.Sub != nil || node != sa
		var ncond lCond
		if chained && (node.API == "Tap" || node.API == "PutCounter") {
			if cs, ok := ps.get("ConditionCheckSVar"); ok {
				if other {
					return refuse("per-node ETB gate on an others-enter replacement")
				}
				v, err := lc.value(cs)
				if err != nil {
					return err
				}
				ncond = lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
				if cmp, ok := ps.get("ConditionSVarCompare"); ok {
					op, b, err := lc.cmpSpec(cmp)
					if err != nil {
						return err
					}
					ncond.Op, ncond.B = op, b
				}
			}
		}
		gAt := -1
		if ncond.Kind != cNone {
			gAt = len(lc.ab.Prog)
			lc.emit(lInstr{Op: "SkipIf", Cond: ncond})
		}
		switch node.API {
		case "Tap":
			if d, hasD := ps.get("Defined"); !ps.is("ETB", "True") || hasD && strings.TrimSpace(d) != want {
				return refuse("ETB tap shape")
			}
			// The shock lands' as-enters election: "As CARDNAME enters, you
			// may pay N life. If you don't, it enters tapped." gorge poses
			// it through Resolve's generic unless gate (effects/registry.go
			// unlessProceed -> poseUnlessAsk) to UnlessPayer$ You, the
			// entering permanent's controller, and offers pay only while
			// life >= N (CR 119.4, UnlessCostPayableFromCtx). Only the exact
			// shape lowers: the whole replacement is the one unswitched Tap
			// with no other condition, and the cost is a literal PayLife<N> or
			// a one-card Reveal or Behold (revealLandCost). Anything else
			// refuses.
			if uc, ok := ps.get("UnlessCost"); ok {
				if other || node != sa || sa.Sub != nil || cond.Kind != cNone {
					return refuse("ETB tap UnlessCost on a chained ETB replacement")
				}
				if p, ok := ps.get("UnlessPayer"); !ok || p != "You" {
					return refuse("ETB tap UnlessCost without UnlessPayer$ You")
				}
				if sw, ok := ps.get("UnlessSwitched"); ok {
					return refuse("ETB tap UnlessCost UnlessSwitched$ %s", sw)
				}
				if verb, spec, ok := revealLandCost(uc); ok {
					if err := lc.revealLandElection(verb, spec); err != nil {
						return err
					}
					break
				}
				n, literal := 0, false
				if rest, has := strings.CutPrefix(strings.TrimSpace(uc), "PayLife<"); has {
					if arg, hasArg := strings.CutSuffix(rest, ">"); hasArg {
						if v, err := strconv.Atoi(arg); err == nil && v >= 1 {
							n, literal = v, true
						}
					}
				}
				cost, err := lc.costUnless(uc)
				if !literal || err != nil || cost.PayLife != n {
					return refuse("ETB tap UnlessCost %q", uc)
				}
				// UnlessPaid(PayLife N, You) then the Tap body: gorge's
				// unswitched orientation skips the body when the life is
				// paid and runs it on a decline.
				if err := lc.unless(ps, func() error {
					lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
					return nil
				}); err != nil {
					return err
				}
				break
			}
			// ConditionPlayerTurn$ (gorge Resolve's Condition* gate): "enters
			// tapped if it's not your turn" -- the whole replacement is the
			// one Tap, so the gate becomes the replacement's condition
			// (CondYourTurn: the entering permanent's controller is active).
			if pt, ok := ps.get("ConditionPlayerTurn"); ok {
				if sa.Sub != nil || node != sa || cond.Kind != cNone || other {
					return refuse("ConditionPlayerTurn on a chained ETB replacement")
				}
				switch pt {
				case "True":
					*cond = lCond{Kind: cYourTurn}
				case "False":
					*cond = lCond{Kind: cYourTurn, Neg: true}
				default:
					return refuse("ConditionPlayerTurn %q", pt)
				}
			}
			// ConditionPresent$ + ConditionCompare$ (gorge
			// conditionMetBattlefield, conditions.go:1183): "enters tapped
			// unless you control <N> <spec>". The whole replacement is the
			// one Tap and gorge counts the battlefield for the entering
			// permanent's controller excluding the entering object itself
			// (kernel turn_gen.go clears the object from occ before the
			// replacement runs, so the count excludes it the same way).
			if cp, ok := ps.get("ConditionPresent"); ok {
				if other {
					return refuse("ConditionPresent on an others-enter replacement")
				}
				if sa.Sub != nil || node != sa || cond.Kind != cNone {
					return refuse("ConditionPresent on a chained ETB replacement")
				}
				cmp, ok := ps.get("ConditionCompare")
				if !ok {
					return refuse("ConditionPresent without ConditionCompare on an ETB replacement")
				}
				f, err := lc.filter(cp)
				if err != nil {
					return err
				}
				op, v, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				*cond = lCond{Kind: cPresent, F: f, Op: op, A: v}
			}
			// ConditionCheckSVar$ (+ ConditionSVarCompare$): the replacement
			// applies while the SVar's value passes the comparison (gorge
			// Resolve's Condition* gate on the one Tap; Starting Town's
			// "unless it's your first, second, or third turn")
			if cs, ok := ps.get("ConditionCheckSVar"); ok && ncond.Kind == cNone {
				if cond.Kind != cNone {
					return refuse("ConditionCheckSVar beside another ETB replacement condition")
				}
				if sa.Sub != nil || node != sa || other {
					return refuse("ConditionCheckSVar on a chained ETB replacement")
				}
				v, err := lc.value(cs)
				if err != nil {
					return err
				}
				c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
				if cmp, ok := ps.get("ConditionSVarCompare"); ok {
					op, b, err := lc.cmpSpec(cmp)
					if err != nil {
						return err
					}
					c.Op, c.B = op, b
				}
				*cond = c
			}
			lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
		case "Untap":
			// "Lands you control enter untapped" (The Wandering Minstrel,
			// Spelunking): the plain ETB untap of the entering
			// card, ordered against an enters-tapped body by the kernel's
			// CR 616.1 entry-order ask. Only the unconditional single-node
			// other-enter shape: enterGen does not evaluate a condition on
			// an other-enter program (Archelos' Card.Self+tapped/untapped
			// pair stays refused).
			if d, hasD := ps.get("Defined"); !ps.is("ETB", "True") || hasD && strings.TrimSpace(d) != want {
				return refuse("ETB untap shape")
			}
			if !other || node != sa || sa.Sub != nil || cond.Kind != cNone {
				return refuse("conditional/chained/self ETB untap shape")
			}
			lc.emit(lInstr{Op: "EnterUntapped", Obj: lRef{Kind: rReplaced}})
		case "ChangeZone":
			// "creatures you control enter tapped and attacking" (Kira Nerys,
			// Firebrand): the replaced card moves from Origin$ All to the
			// battlefield with Tapped$ True and Attacking$ True, nothing else
			// (gorge effects.EntryStateBody). Only the unchained other-enter shape.
			if !other || node != sa || sa.Sub != nil || ncond.Kind != cNone ||
				!ps.is("Origin", "All") || !ps.is("Destination", "Battlefield") || ps.str("Defined") != want ||
				!ps.is("Tapped", "True") || !ps.is("Attacking", "True") {
				return refuse("ETB replacement ChangeZone shape")
			}
			lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
			lc.emit(lInstr{Op: "EnterAttacking", Obj: lRef{Kind: rReplaced}})
		case "PutCounter":
			if d, hasD := ps.get("Defined"); !ps.is("ETB", "True") || hasD && strings.TrimSpace(d) != want {
				return refuse("ETB counter shape")
			}
			n, err := lc.amount(ps, "CounterNum", 1)
			if err != nil {
				return err
			}
			lc.emit(lInstr{Op: "EnterWithCounters", Obj: lRef{Kind: rReplaced}, A: n, CS: ps.str("CounterType")})
		case "ChooseColor":
			if ps.str("Defined") != "You" {
				return refuse("ChooseColor Defined")
			}
			ex, err := chooseColorExclude(ps) // lower_handmove.go
			if err != nil {
				return err
			}
			lc.emit(lInstr{Op: "ChooseColor", Obj: lRef{Kind: rYou}, C: ex})
		case "GenericChoice":
			if err := lc.etbSetChosenMode(node, sa, other, cond); err != nil { // lower_siegemode.go
				return err
			}
			ps.consume("Choices", "Defined", "SetChosenMode", "ShowChoice", "AILogic", "SpellDescription")
		case "Clone":
			if err := lc.cloneEnter(ps, node, sa); err != nil {
				return err
			}
			if node == sa && sa.Sub != nil {
				// cloneEnter consumed the exact counter-rider chain into the
				// BecomeCopy flag; its links are not separate instructions.
				if left := ps.unread(); len(left) > 0 {
					return refuse("Clone unread params %v", left)
				}
				return nil
			}
		case "AlterAttribute":
			// "enters prepared" (K:ETBReplacement:Other:<SVar> with
			// AlterAttribute Prepared; Updated)
			if other || ps.str("Attributes") != "Prepared" {
				return refuse("ETB replacement api AlterAttribute")
			}
			if err := lc.alterPrepared(ps, lRef{Kind: rReplaced}); err != nil {
				return err
			}
		case "ChooseNumber":
			if err := lc.etbChooseNumber(ps, node, sa, other, cond); err != nil { // lower_choosenumber.go
				return err
			}
		case "ChoosePlayer":
			if !other && node == sa && !spyglassChain(sa) {
				// the as-enters opponent (lower_chooseplayerenter.go)
				if err := lc.etbChoosePlayer(ps, node, sa, other, cond); err != nil {
					return err
				}
				break
			}
			// Sorcerous Spyglass's chain head (lower_choosename.go spyglassChain)
			if other || node != sa || !spyglassChain(sa) {
				return refuse("ETB replacement api ChoosePlayer")
			}
			if err := lc.choosePlayer(ps); err != nil {
				return err
			}
		case "RevealHand":
			// the chain's look: folded into the name ask's decisions (B = 2)
			if other || !spyglassChain(sa) && !handNameChain(sa) {
				return refuse("ETB replacement api RevealHand")
			}
			ps.consume("Defined", "Look", "RememberRevealed", "SpellDescription")
		case "Cleanup":
			if other || !spyglassChain(sa) && !handNameChain(sa) {
				return refuse("ETB replacement api Cleanup")
			}
			lc.clearChosenPlayer(ps)
			ps.consume("ClearRemembered") // Alhammarret's chain: the revealed set was never kept
		case "NameCard":
			if err := lc.etbChooseName(ps, node, sa, other, cond); err != nil { // lower_choosename.go
				return err
			}
		case "ChooseCard":
			if err := lc.etbChooseCard(ps, node, sa, other, cond); err != nil { // lower_choosecardenter.go
				return err
			}
		case "ChooseType":
			// gorge effChooseType: an absent Defined$ is the controller
			if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" || ps.str("Type") != "Creature" {
				return refuse("ChooseType shape")
			}
			in := lInstr{Op: "ChooseType", Obj: lRef{Kind: rYou}, Note: "choice among creature types"}
			if err := lc.chooseTypeValid(ps, &in); err != nil {
				return err
			}
			lc.emit(in)
		default:
			return refuse("ETB replacement api %s", node.API)
		}
		if gAt >= 0 {
			lc.ab.Prog[gAt].Skip = len(lc.ab.Prog) - gAt - 1
		}
		if left := ps.unread(); len(left) > 0 {
			return refuse("%s unread params %v", node.API, left)
		}
	}
	return nil
}

// cloneEnterCounterRider accepts exactly Dominion Saboteur's rider chain: a
// Clone body with RememberCloneOrigin$ True, then one PutCounter with Defined$
// Self, ETB$ True, CounterType$ EachFromSource and EachFromSource$ Remembered
// (the copy origin is the remembered choice), then optionally the trailing
// Cleanup ClearRemembered$ True. Everything else about the body stays
// cloneEnter's. The chain is consumed here rather than lowered as separate
// instructions: OpBecomeCopy's BecomeCopyCtr flag carries the counters during
// the entry, where gorge's rider runs before the enters triggers match.
func (lc *lowerCtx) cloneEnterCounterRider(ps *params, sa *cards.SA) error {
	if !ps.is("RememberCloneOrigin", "True") {
		return refuse("ETB copy chained rider without RememberCloneOrigin$")
	}
	link := sa.Sub
	if link.API != "PutCounter" {
		return refuse("ETB copy chained rider %s", link.API)
	}
	lp := newParams(link.Params)
	if !lp.is("ETB", "True") || lp.str("Defined") != "Self" ||
		!strings.EqualFold(lp.str("CounterType"), "EachFromSource") ||
		lp.str("EachFromSource") != "Remembered" {
		return refuse("ETB copy chained rider shape")
	}
	lp.consume("SubAbility")
	if left := lp.unread(); len(left) > 0 {
		return refuse("ETB copy chained rider unread params %v", left)
	}
	if link.Sub == nil {
		return nil
	}
	cleanup := link.Sub
	if cleanup.API != "Cleanup" || cleanup.Sub != nil {
		return refuse("ETB copy chained rider tail %s", cleanup.API)
	}
	cp := newParams(cleanup.Params)
	if !cp.is("ClearRemembered", "True") {
		return refuse("ETB copy chained rider cleanup shape")
	}
	if left := cp.unread(); len(left) > 0 {
		return refuse("ETB copy chained rider cleanup unread params %v", left)
	}
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

// cloneEnter lowers "you may have this enter as a copy of <Choices$>,
// except ..." (gorge's ETB clone: cast_etbchoice.go collectETBChoices
// kind "copy", effects/clone.go effClone with CloneEnter.ETB): the clone
// ask is posed while the spell is on the stack, over the battlefield
// permanents matching Choices$ (gorge etbOptions: seat 0's battlefield,
// then seat 1's), plus "enter as itself" when the keyword line is
// :Optional (or nothing matches). Exceptions: copyExceptions' list
// (AddTypes$, AddKeywords$, SetPower$/SetToughness$, colours...); NewName$
// and AddKeywords$ Changeling refuse by name.
func (lc *lowerCtx) cloneEnter(ps *params, node, sa *cards.SA) error {
	if node != sa || len(lc.ab.Prog) != 0 {
		return refuse("ETB copy in a chain")
	}
	// Dominion Saboteur's exact counter rider (remembered copy origin, then
	// PutCounter EachFromSource$ Remembered, then Cleanup): the whole chain is
	// folded into the BecomeCopy op's BecomeCopyCtr flag, so the kernel carries
	// the counters into the entering object before its enters triggers match.
	rememberCounters := false
	if node.Sub != nil {
		if err := lc.cloneEnterCounterRider(ps, sa); err != nil {
			return err
		}
		rememberCounters = true
	}
	if _, ok := ps.get("Defined"); ok {
		return refuse("ETB copy Defined$")
	}
	ch, ok := ps.get("Choices")
	if !ok {
		return refuse("ETB copy without Choices$")
	}
	lc.cloneRepl = true
	f, err := lc.filter(ch)
	lc.cloneRepl = false
	if err != nil {
		return err
	}
	for _, c := range f.Clauses {
		// a copied Aura must attach and a copied planeswalker enters with
		// its loyalty: neither is modelled, so the templates are creatures
		// and artifacts only
		if c.Types&^(tCreature|tArtifact) != 0 || c.Types == 0 && c.Subtype != "Equipment" {
			return refuse("ETB copy of %q (not a creature or artifact)", ch)
		}
	}
	// the copy exceptions (CR 707.9b), read through the shared
	// copyExceptions; a name change and the Changeling CDA are not copy
	// exceptions gorge applies the same way (clone.go L531), refused by name
	if _, ok := ps.get("NewName"); ok {
		return refuse("ETB copy NewName$")
	}
	if k, ok := ps.get("AddKeywords"); ok {
		for _, w := range strings.Split(k, "&") {
			if strings.TrimSpace(w) == "Changeling" {
				return refuse("ETB copy AddKeywords Changeling: a CDA, not an exception (gorge clone.go L531)")
			}
		}
	}
	exc, hasExc, myriads, err := lc.copyExceptionsMyriad(ps, true)
	if err != nil {
		return err
	}
	grant, err := lc.cloneEnterGrant(ps) // lower_cloneentergrant.go
	if err != nil {
		return err
	}
	if myriads > 0 {
		// "except it has myriad" (Auton Soldier; CR 707.9b, 702.116a-b):
		// one granted-Myriad trigger per instance rides in the grant record
		// beside any AddAbilities$ (lower_myriad.go); at most two grants
		if grant == nil {
			grant = &lOverride{}
		}
		for ; myriads > 0; myriads-- {
			idx, err := lc.myriadGrantTrigger()
			if err != nil {
				return err
			}
			grant.GrantAbility = append(grant.GrantAbility, idx)
		}
		if len(grant.GrantAbility) > 2 {
			return refuse("ETB copy grants %d abilities (Myriad included): more than two", len(grant.GrantAbility))
		}
	}
	in := lInstr{Op: "BecomeCopy", Obj: lRef{Kind: rReplaced}, F: f}
	if grant != nil {
		lc.out.overrides = append(lc.out.overrides, grant)
		in.A = cst(len(lc.out.overrides)) // index+1
	}
	if rememberCounters {
		in.B = cst(int(kop.BecomeCopyCtr))
		in.Note = "with the copy origin's counters"
	}
	if lc.replOptional {
		in.C = 1
	}
	if hasExc {
		lc.out.overrides = append(lc.out.overrides, exc)
		in.C |= len(lc.out.overrides) << 1 // index+1
		in.Note = "except " + overrideString(exc)
	}
	lc.emit(in)
	return nil
}

// attachedOnly reports whether every clause of f is limited to the
// source's host (EnchantedBy / EquippedBy).
func attachedOnly(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&(fEquippedBy|fEnchantedBy) == 0 {
			return false
		}
	}
	return true
}

// walkFilterOK refuses an override static whose affected filter reads a
// characteristic the layer walk cannot serve from the characteristics
// built so far (P/T, keywords): matching it would re-enter the derivation
// (CR 613, kernel matchPermCur).
func walkFilterOK(f *lFilter) error {
	if f == nil {
		return nil
	}
	for _, c := range f.Clauses {
		switch c.CmpStat {
		case cmpPower, cmpToughness, cmpPowerLTToughness, cmpPowerGTBase:
			return refuse("override static filter reads P/T")
		}
		if c.Flags&(fWithFlying|fWithoutFlying) != 0 {
			return refuse("override static filter reads keywords")
		}
	}
	return nil
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

// splitWardGrant takes a Continuous static's "Ward:N" or "Ward:PayLife<N>"
// term (one) out of an AddKeyword$ list: the rest of the list and the mana N
// or life N (0: none). A ward with another cost or two ward terms are
// refused.
func splitWardGrant(kw string) (string, int, int, error) {
	var rest []string
	ward, wardLife := 0, 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		v, ok := strings.CutPrefix(k, "Ward:")
		if !ok {
			rest = append(rest, k)
			continue
		}
		if life, ok := strings.CutPrefix(v, "PayLife<"); ok {
			body, ok := strings.CutSuffix(life, ">")
			if !ok {
				return "", 0, 0, refuse("keyword %q", k)
			}
			n, err := strconv.Atoi(body)
			if err != nil || n < 1 || n > 255 || ward != 0 || wardLife != 0 {
				return "", 0, 0, refuse("keyword %q", k)
			}
			wardLife = n
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 255 || ward != 0 || wardLife != 0 {
			return "", 0, 0, refuse("keyword %q", k)
		}
		ward = n
	}
	return strings.Join(rest, " & "), ward, wardLife, nil
}

// removeAnyCounterN parses `RemoveAnyCounter<N/Any/Creature>` for N in 1..255
// (no leading zero); any other counter kind or filter is not read.
func removeAnyCounterN(rc string) (int, bool) {
	body, ok := strings.CutPrefix(rc, "RemoveAnyCounter<")
	if !ok {
		return 0, false
	}
	body, ok = strings.CutSuffix(body, "/Any/Creature>")
	if !ok || body == "" || body[0] == '0' {
		return 0, false
	}
	n, err := strconv.Atoi(body)
	if err != nil || n < 1 || n > 255 {
		return 0, false
	}
	return n, true
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

// laterPermanentAlternative refuses a MayPlay$ Affected$ list whose second or
// later alternative has the Permanent base (Kagha, Shadow Archdruid's
// "Land.YouOwn+...,Permanent.YouOwn+..."). Off the battlefield Forge reads
// Permanent as a permanent card, which the kernel's filter does, but gorge's
// targetSpecForZone (engine/rules/stack.go) rewrites only a leading
// Permanent to PermanentCard, so gorge's later alternative matches no card
// in the affected zone and the two engines offer different casts.
func laterPermanentAlternative(aff string) error {
	alts := strings.Split(aff, ",")
	for _, a := range alts[1:] {
		a = strings.TrimSpace(a)
		if b, ok := strings.CutPrefix(a, "Permanent"); ok && (b == "" || b[0] == '.' || b[0] == '+') {
			return refuse("MayPlay Affected$ %q: a later Permanent alternative off the battlefield (gorge targetSpecForZone rewrites only the first)", aff)
		}
	}
	return nil
}

// ownGraveLandsOnly refuses a typed (MayPlayText$) graveyard MayPlay$ grant
// that admits a land its controller may not own (Coram, the Undertaker's
// "play a land ... from among cards in graveyards"). gorge registers no
// may-play effect for a MayPlayText$ static (layers.go mayPlayGrant), so its
// land offers come from the fallback walk alone, which reads only the
// player's own graveyard (legal_mayplay.go mayPlayLandIds walk(p, true));
// the kernel offers an opponent's land too.
func ownGraveLandsOnly(sp *params, f *lFilter) error {
	if strings.TrimSpace(sp.m["MayPlayText"]) == "" || !strings.Contains(sp.str("AffectedZone"), "Graveyard") {
		return nil
	}
	for _, c := range f.Clauses {
		if (c.Types == 0 || c.Types&tLand != 0) && c.NotTypes&tLand == 0 && c.Owner != whoYou && c.Ctrl != whoYou {
			return refuse("MayPlayText$ graveyard grant of a land you may not own: gorge's land walk reads only your graveyard")
		}
	}
	return nil
}
