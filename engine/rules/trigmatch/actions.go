// The remaining single-event trigger modes.
//
// Mode$ CommitCrime, Vote, FlippedCoin, Attached, RingTemptsYou, BecomesTarget,
// LandPlayed and Phase.
//
// Split out of trigger_match.go so tickets touching different modes stop
// colliding on one file. Registration is at the bottom; a duplicate mode
// panics (registerTrigMatcher).

package trigmatch

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// commitCrimeMatches implements CR 700.13: targeting an opponent, a
// permanent they control, or a card in their graveyard commits one crime.
// A multi-target spell produces one TargetsChosen event per target. After
// Apply, append events can inspect the prior targets already on the stack;
// only the first criminal target may fire this trigger.
func commitCrimeMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	actor := e.ControllerOf(ev.Obj)
	if !TargetEventCommitsCrime(e, ev, actor) {
		return false
	}
	if o := e.Game().Obj(ev.Obj); o != nil && (ev.Amount == 2 || ev.Amount == 3) {
		// The targets already on the object before this one. A chain link's
		// target (events.SubTargetNotice, CR 601.2c) was appended to
		// SubTargets, after every root target; a root append to Targets.
		prior, priorSub := o.Targets, o.SubTargets
		if ev.Text == events.SubTargetNotice {
			if len(priorSub) > 0 {
				priorSub = priorSub[:len(priorSub)-1]
			}
		} else {
			priorSub = nil
			if len(prior) > 0 {
				prior = prior[:len(prior)-1]
			}
		}
		for _, target := range prior {
			if targetCommitsCrime(e, target, actor) {
				return false
			}
		}
		for _, target := range priorSub {
			if targetCommitsCrime(e, target, actor) {
				return false
			}
		}
	}
	if v := t.ParamStr(cards.PKValidPlayer); v != "" {
		return effects.MatchesPlayerSpec(e.Game(), v, actor, e.ControllerOf(source))
	}
	return true
}

func TargetEventCommitsCrime(e Board, ev events.Event, actor state.PlayerID) bool {
	if ev.Amount == 1 || ev.Amount == 3 {
		return targetCommitsCrime(e, state.Target{Player: ev.Player, IsPlayer: true}, actor)
	}
	return len(ev.IDs) == 1 && targetCommitsCrime(e, state.Target{Obj: ev.IDs[0]}, actor)
}

// targetCommitsCrime is CR 700.13's list, and only that list: an opponent; a
// permanent or a spell or ability on the stack an opponent controls; or a card
// in an opponent's graveyard, which is judged by its owner (CR 108.4a: a card
// that is not a permanent or spell has no controller). A card in exile, a
// hand or a library is none of these, whoever owns it.
func targetCommitsCrime(e Board, target state.Target, actor state.PlayerID) bool {
	if target.IsPlayer {
		return target.Player != actor && int(target.Player) < len(e.Game().Players)
	}
	o := e.Game().Obj(target.Obj)
	if o == nil {
		return false
	}
	switch o.Zone {
	case state.ZBattlefield, state.ZStack:
		return o.Controller != actor
	case state.ZGraveyard:
		return o.Owner != actor
	}
	return false
}

// eventCardAndPlayerMatch applies the shared ValidCard$/ValidPlayer$ clauses
// on action triggers. The player is the player who performed the action.
func eventCardAndPlayerMatch(e Board, t cards.Trigger, source, card state.ObjID, player state.PlayerID) bool {
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCard); v != "" && !e.MatchesSpec(v, card, source, ctrl, SpecOpts{}) {
		return false
	}
	if v := t.ParamStr(cards.PKValidPlayer); v != "" && !effects.MatchesPlayerSpec(e.Game(), v, player, ctrl) {
		return false
	}
	return true
}

// voteMatches implements Mode$ Vote (trig:Vote): the trigger fires on the
// canonical vote-finished Note both api:Vote shapes emit (engine/effects/vote.go),
// the same carrier-event shape trig:FlippedCoin fires on. List$ is the
// REFERENT SCOPE, not the firing condition: "Whenever players finish voting"
// (Erestor of the Council, Model of Unity, Grudge Keeper -- the whole corpus
// population) has no intervening-if, so the trigger fires whenever a vote
// finishes, with an empty List$ set simply meaning its referents resolve to
// nobody (Grudge Keeper stacks, its diff set is empty, and its body acts on
// nobody). The capture side (triggerReferents' Vote case) applies List$ when
// it binds the sets, so a body reading a spelling its own List$ does not name
// gets the empty set -- the parameter is read, never silently inert.
func voteMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if _, _, _, ok := effects.VoteFinishedResult(ev); !ok {
		return false
	}
	return true
}

func flippedCoinMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	flipper, win, ok := effects.FlipNoteResult(ev)
	if !ok {
		return false
	}
	// ValidResult$ gates the side: Win = heads (Amount 1), Lose = tails
	// (Amount 0). Every corpus FlippedCoin line carries one; an absent
	// ValidResult$ (no such line measured) would fire on both sides.
	if res := strings.TrimSpace(t.ParamStr(cards.PKValidResult)); res != "" {
		if strings.EqualFold(res, "Win") && !win {
			return false
		}
		if strings.EqualFold(res, "Lose") && win {
			return false
		}
	}
	// ValidPlayer$ names the FLIPPER ("whenever YOU win a coin flip"): the
	// Note's Player, through the shared player-spec grammar with the trigger's
	// own controller as You. The two "whenever a player wins" lines carry no
	// ValidPlayer$ and fire on any flipper.
	if v, ok := t.Param(cards.PKValidPlayer); ok {
		if !effects.MatchesPlayerSpecFrom(e.Game(), v, flipper, e.ControllerOf(source), source) {
			return false
		}
	}
	return true
}

// attachedMatches implements Mode$ Attached: the trigger fires when an Aura,
// Equipment or other attachment becomes attached to a permanent (CR
// 701.3a's "becomes attached" -- the event the engine's one shared attach
// emit site, engine/effects/attach.go's effAttach, publishes for the cast, equip
// and ETB-attached shapes alike). events.Attach with len(ev.IDs) > 0 carries
// the attachment in ev.Obj and the bearer in ev.IDs[0]; the no-IDs emits are
// the detach state-based actions (engine/rules/attach.go), which are NOT "becomes
// attached" and never match, and an emit with no attachment object (Obj == 0)
// has nothing to bind ValidSource$ against, so it never matches either.
// Forge's ValidSource$ names the ATTACHING
// object (Siona's Aura.YouCtrl, Enormous Energy Blade's Card.Self) and
// ValidTarget$ names the BEARER (Brood Keeper's Card.Self reads Self as the
// trigger's source through the same specCtx becomesTargetMatches uses).
// A trigger with NO ValidTarget$ fires only through Eriette's measured
// TargetRelativeToSource$ bearer filter (attachedRelativeBearerSpec); any
// other bearer-less line would over-fire on every Aura attach, so it never
// fires. The Static$ True guard keeps Forge's "static effect expressed as
// a trigger" lines (Metamorphic Alteration, Paleontologist's Pick-Axe --
// Execute$ DBClone continuous shapes with no static-trigger machinery here)
// from firing a clone on every attach.
func attachedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Attach || len(ev.IDs) == 0 || ev.Obj == 0 {
		return false
	}
	if t.ParamStr(cards.PKStatic) == "True" {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v, ok := t.Param(cards.PKValidSource); ok {
		if !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
			return false
		}
	}
	if v, ok := t.Param(cards.PKValidTarget); ok {
		return e.MatchesSpec(v, ev.IDs[0], source, ctrl, SpecOpts{})
	}
	if spec, ok := attachedRelativeBearerSpec(e, t, source, ev); ok {
		return e.MatchesSpec(spec, ev.IDs[0], source, ctrl, SpecOpts{})
	}
	return false
}

// attachedRelativeBearerSpec reads Eriette, the Beguiler's
// TargetRelativeToSource$ bearer filter ("a nonland permanent an opponent
// controls with mana value less than or equal to that Aura's mana value":
// Permanent.nonLand+OppCtrl+cmcLEX with SVar:X:Count$CardManaCost). The X
// is the ATTACHING object's mana value (CR 202.3), so the comparison is
// bound to that number before the ordinary filter reads the bearer. Only
// that measured shape is read; any other TargetRelativeToSource$ value
// fails closed as before.
func attachedRelativeBearerSpec(e Board, t cards.Trigger, source state.ObjID, ev events.Event) (string, bool) {
	v, ok := t.Params["TargetRelativeToSource"]
	if !ok {
		return "", false
	}
	src, att := e.Game().Obj(source), e.Game().Obj(ev.Obj)
	if src == nil || src.Face() == nil || att == nil || att.Face() == nil ||
		strings.TrimSpace(src.Face().SVars["X"]) != "Count$CardManaCost" || !strings.HasSuffix(v, "+cmcLEX") {
		return "", false
	}
	return strings.TrimSuffix(v, "X") + strconv.Itoa(int(att.Face().ManaValue())), true
}

// ringTemptsMatches implements Mode$ RingTemptsYou (CR 701.54d): the trigger
// fires when the Ring tempts its controller, "when the actions complete,
// even if some were impossible" — an event with Obj 0 (no creature was
// designated) still counts as a temptation. The tempted player is ev.Player;
// ValidPlayer$ gates on the tempted player against the source's controller
// (the corpus's only spelling, `ValidPlayer$ You`). ValidCard$ gates on the
// chosen Ring-bearer with the source's controller as "you" and the source as
// Other — the corpus's `Creature.YouCtrl+Other` shape ("a creature other than
// CARDNAME") and the plain `Creature.YouCtrl` shape both resolve through it;
// with no designated bearer a ValidCard$ trigger never fires.
func ringTemptsMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.RingTemptsYou {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v, ok := t.Param(cards.PKValidPlayer); ok {
		if !effects.MatchesPlayerSpec(e.Game(), v, ev.Player, ctrl) {
			return false
		}
	}
	if v := t.ParamStr(cards.PKValidCard); v != "" {
		if ev.Obj == 0 {
			return false // no creature became the Ring-bearer
		}
		if !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
			return false
		}
	}
	return true
}

// becomesTargetMatches implements Mode$ BecomesTarget: the trigger fires
// when one of the chosen targets recorded by a TargetsChosen event
// (rules.handleTarget -- "the target decision being answered") matches its
// ValidTarget$ -- or, with no ValidTarget$, when its own source is among the
// targets. Forge's ValidTarget$ names the TARGETED object: the self-shapes
// (ValidTarget$ Card.Self, the ward family, Reality Smasher) match their own
// source that way, and the "a Dragon you control becomes the target" shapes
// (Thunderbreak Regent) match a target their source merely watches -- the
// 50 non-self corpus lines of the 132-line mode.
func becomesTargetMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	firstTime := strings.EqualFold(t.ParamStr(cards.PKFirstTime), "True")
	// Valiant$ True (CR 603.2's BLB ability word): the trigger's controller
	// counts only the targetings of a spell or ability THEY control, so the
	// once-each-turn gate needs the source's controller at hand.
	valiant := strings.EqualFold(t.ParamStr(cards.PKValiant), "True")
	if v, ok := t.Param(cards.PKValidSource); ok {
		// ValidSource$ names the spell or ability doing the targeting: the
		// TargetsChosen event's Obj, the stack object whose target decision
		// this event answers (commitCrimeMatches reads the same field as the
		// targeting actor). Reality Smasher's "spell an opponent controls"
		// (Spell.OppCtrl) and Thunderbreak Regent's "spell or ability"
		// (SpellAbility.OppCtrl) both resolve against that stack object; the
		// Ability* spellings (Loki, God of Mischief's Ability.YouCtrl) are
		// ability-shape predicates the object grammar has no base word for
		// and are answered by becomesTargetSourceMatches alongside the bare
		// `Activated` token. An event carrying no targeting object can never
		// match.
		if !becomesTargetSourceMatches(e, v, ev.Obj, source, e.ControllerOf(source)) {
			return false
		}
	}
	if v, ok := t.Param(cards.PKValidTarget); ok {
		for _, id := range ev.IDs {
			if e.MatchesSpec(v, id, source, e.ControllerOf(source), SpecOpts{}) {
				// CR 702.21a compares the Ward permanent's controller with the
				// controller of the targeting spell or ability ON THE STACK
				// (the same ev.Obj ValidSource$ reads above). For an ability,
				// protectionSource would unwrap ev.Obj to its source permanent,
				// whose controller may have changed since activation. Every
				// ward trigger targets only itself (the synthesized shape,
				// trigger_match.go's ward expansion), so this gate runs on the
				// self match.
				if t.ParamStr(cards.PKWard) == "True" &&
					(ev.Obj == 0 || e.ControllerOf(ev.Obj) == e.ControllerOf(source)) {
					return false
				}
				if firstTime && !firstTimeTargetedThisTurn(e, ev, id, false) {
					// "for the first time each turn" is per matched object: a
					// later target of this same event may still be its first.
					continue
				}
				if valiant && !valiantFirstTimeTargetedByYouThisTurn(e, ev, id, false, e.ControllerOf(source)) {
					continue
				}
				return true
			}
		}
		// A player target is not in ev.IDs: recordChosenTargets (T14-b)
		// carries it as Amount 1/3 with Player set, the shape
		// becomesTargetOnceMatches' ValidTarget$ read already answers with
		// the player-spec grammar. "Player,Permanent" (Loki, God of
		// Mischief) and the bare "You" (Leovold, Emissary of Trest) spellings
		// name the player half of their specs there; an object event (Amount
		// 0/2) never enters this branch, so the object half above keeps the
		// only say for it.
		if (ev.Amount == 1 || ev.Amount == 3) &&
			effects.MatchesPlayerSpecCtx(e.Game(), v, ev.Player, e.ControllerOf(source), e.PlayerSpecCtx(source)) {
			return (!firstTime || firstTimeTargetedThisTurn(e, ev, 0, true)) &&
				(!valiant || valiantFirstTimeTargetedByYouThisTurn(e, ev, 0, true, e.ControllerOf(source)))
		}
		return false
	}
	targeted := false
	for _, id := range ev.IDs {
		if id == source {
			targeted = true
			break
		}
	}
	if targeted && t.ParamStr(cards.PKWard) == "True" &&
		(ev.Obj == 0 || e.ControllerOf(ev.Obj) == e.ControllerOf(source)) {
		// The ValidTarget$ branch's ward gate, applied to the bare
		// self-targeted fallback: a ward trigger never fires for its own
		// controller's targeting (CR 702.21a compares the ward permanent's
		// controller with the targeting spell or ability's, the same ev.Obj
		// ValidSource$ reads above). Unreachable in the current corpus --
		// every ward trigger is keyword-synthesized with ValidTarget$
		// Card.Self (engine/cards/keywords.go, 0 raw Ward$ True lines) -- kept so
		// a future ward trigger without ValidTarget$ cannot fire for its
		// own controller.
		return false
	}
	return targeted && (!firstTime || firstTimeTargetedThisTurn(e, ev, source, false)) &&
		(!valiant || valiantFirstTimeTargetedByYouThisTurn(e, ev, source, false, e.ControllerOf(source)))
}

// firstTimeTargetedThisTurn answers FirstTime$ True on Mode$ BecomesTarget
// ("becomes the target of a spell or ability for the first time each turn",
// Glyph Keeper, Kira, the Glasskites, Valiant creatures): true when no EARLIER
// TargetsChosen event this turn named the same object (or player). Same
// replay-stable backwards log scan as firstDamageToThisTurn -- no new event
// kind, no engine field, survives Clone -- stopping at TurnChange.
//
// Events recorded for the same stack object ev.Obj are the SAME targeting
// (a spell's later target groups, the event itself when the log already holds
// it) and never count as an earlier time. A TargetsChosen with no stack
// object is a hand-built fixture emit and counts like any other.
func firstTimeTargetedThisTurn(e Board, ev events.Event, id state.ObjID, player bool) bool {
	for i := len(e.Log().Events) - 1; i >= 0; i-- {
		le := e.Log().Events[i]
		if le.Kind == events.TurnChange {
			return true
		}
		if le.Kind != events.TargetsChosen || (ev.Obj != 0 && le.Obj == ev.Obj) {
			continue
		}
		if player {
			if (le.Amount == 1 || le.Amount == 3) && le.Player == ev.Player {
				return false
			}
			continue
		}
		for _, t := range le.IDs {
			if t == id {
				return false
			}
		}
	}
	return true
}

// valiantFirstTimeTargetedByYouThisTurn answers Valiant$ True on Mode$
// BecomesTarget (the BLB ability word: "becomes the target of a spell or
// ability you control for the first time each turn", Seedglaive Mentor, the
// Emberheart Challenger family): true when no EARLIER TargetsChosen event
// this turn named the same object (or player) with a targeting spell or
// ability the trigger's controller controls. Unlike firstTimeTargetedThisTurn,
// the count is per CONTROLLER (the ability-word condition reads "you control"
// inside the once-each-turn clause; Forge's TriggerBecomesTarget.Valiant /
// Card.isValiant is the per-controller reference -- XMage's
// BecomesTargetSourceFirstTimeTriggeredAbility is a different, count-all-
// targetings shape): an opponent's targeting neither fires nor disarms.
//
// Same replay-stable backwards log scan as firstTimeTargetedThisTurn -- no
// new event kind, no engine field, survives Clone -- stopping at
// TurnChange. The controller of the earlier targeting is read from
// replay-derived state (stackObjControllerAt), not from the stack object's
// live Controller or a spell card's Owner: the scan runs while the targeting
// object may already have left the stack (a resolution, counter or fizzle
// resets Controller to Owner, CR 400.7's control reset), a spell card's Owner
// is the card's owner and never the caster (a spell cast from a card its
// caster does not own -- Gonti's taken card -- would misread), and the fold
// mints an ability's stack object with the player who put it there as its
// Owner (PutOnStack the caster, AbilityPush the activator). An on-stack
// control change after the mint is the one shape the derivation cannot see
// for a spell; the current event's own source match still reads the live
// Controller, and no corpus Valiant line pairs with a spell-steal effect.
//
// Per-spell dedupe (Forge fires Valiant once per targeting spell, its
// distinct-target check): an EARLIER TargetsChosen of the SAME stack object
// that names the same target and was yours rejects this event too -- a
// spell's later target group or a second modal answer re-naming the creature
// is still the same spell targeting it once. firstTimeTargetedThisTurn keeps
// the plain skip instead: FirstTime$ is a per-OBJECT count and the corpus
// lines it answers all fire per targeting event (a multi-clause spell may
// fire it once per clause, the reading its landed cases pin). The event
// itself never counts: the matcher runs inside the event's own emit, so the
// log's last entry is this very event whenever it is already appended
// (emit's fold appends before checkTriggers; checkTriggers is only ever
// re-entered with a TargetsChosen synchronously, so no later event can
// precede the scan) and the skip matches it by its identifying fields.
// An event with no stack object (a hand-built fixture emit) names no
// controller and is skipped.
func valiantFirstTimeTargetedByYouThisTurn(e Board, ev events.Event, id state.ObjID, player bool, you state.PlayerID) bool {
	log := e.Log().Events
	skipLast := false
	if n := len(log); n > 0 {
		last := log[n-1]
		skipLast = last.Kind == ev.Kind && last.Obj == ev.Obj &&
			last.Amount == ev.Amount && last.Player == ev.Player &&
			slices.Equal(last.IDs, ev.IDs)
	}
	for i := len(log) - 1; i >= 0; i-- {
		if i == len(log)-1 && skipLast {
			continue
		}
		le := log[i]
		if le.Kind == events.TurnChange {
			return true
		}
		if le.Kind != events.TargetsChosen || le.Obj == 0 {
			continue
		}
		caster, ok := stackObjControllerAt(e, le.Obj, i)
		if !ok || caster != you {
			continue
		}
		if ev.Obj != 0 && le.Obj == ev.Obj {
			// The same spell or ability instance: only an earlier group
			// naming the SAME target rejects this one (the per-spell dedupe
			// above); a group naming another target is no first time for it
			// either way.
			if player {
				if (le.Amount == 1 || le.Amount == 3) && le.Player == ev.Player {
					return false
				}
			} else {
				for _, t := range le.IDs {
					if t == id {
						return false
					}
				}
			}
			continue
		}
		if player {
			if (le.Amount == 1 || le.Amount == 3) && le.Player == ev.Player {
				return false
			}
			continue
		}
		for _, t := range le.IDs {
			if t == id {
				return false
			}
		}
	}
	return true
}

// stackObjControllerAt answers, off replay-derived state only, the player
// who controlled the stack object while it sat on the stack. PutOnStack is
// the one mint kind whose Obj IS the stack object (the card itself moved),
// and its Player is the caster, so the log answers for spells directly --
// including a spell cast from a card its caster does not own, whose object
// Owner is the card's owner and would misread (Gonti's taken card). Every
// other mint (AbilityPush/KeywordAbilityPush/GainedAbilityPush/GrantAbilityPush,
// the TriggerPush family, DelayedPush, StackCopy) names its SOURCE on Obj and
// the fold mints the stack object itself with the minting player as its
// Owner (foldAbilityPush's AddObject(nil, e.Player) shape); that Owner is
// the activator/trigger-controller/copier and, unlike a spell card's, is
// never re-pointed afterwards (a Move's CR 400.7 reset re-derives Controller
// from it), so the live object answers for those. No evidence at all (a
// hand-built fixture id the log never pushed) reports not found, and the
// caller skips the event.
func stackObjControllerAt(e Board, obj state.ObjID, before int) (state.PlayerID, bool) {
	log := e.Log().Events
	for i := before; i >= 0; i-- {
		if le := log[i]; le.Obj == obj && le.Kind == events.PutOnStack {
			return le.Player, true
		}
	}
	o := e.Game().Obj(obj)
	if o == nil {
		return 0, false
	}
	return o.Owner, true
}

// becomesTargetOnceMatches implements Mode$ BecomesTargetOnce (Forge's
// TriggerBecomesTargetOnce): the BATCH reading of Mode$ BecomesTarget, "whenever
// one or more creatures you control become the target ...". Forge fires it once
// per targeting ACTION -- after the whole spell/ability has chosen its targets
// -- carrying the target SET in AbilityKey.Targets and the causing card in
// AbilityKey.Cause. This engine emits one TargetsChosen event per chosen target,
// so the once-per-action cadence rides the target batch
// (openTargetBatch/closeTargetBatch around recordChosenTargets, and the
// queue-time gate in trigger_match.go's checkFaceTriggers); this matcher answers
// only whether ONE event's own target/source/cause clause holds.
//
// ValidTarget$: any one of the action's targets -- a permanent (ev.IDs) or a
// player (ev.Player, shapes 1/3) -- matching the spec (Professor Hojo's
// Creature.YouCtrl+inZoneBattlefield, Leyline's You,Permanent.YouCtrl...).
// ValidSource$: the targeting spell/ability; the corpus's `Activated` spelling
// is Forge's activated-ability predicate and is answered directly, every other
// spelling through the ordinary filter grammar (SpellAbility.OppCtrl). An
// event with no targeting object fails closed. ValidCause$: the host card of
// the targeting ability (Forge's sp.getHostCard()), which protectionSource
// resolves for a spell (itself) and an ability (its source permanent);
// Psychic Battle excludes itself by name that way.
func becomesTargetOnceMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	you := e.ControllerOf(source)
	if v, ok := t.Param(cards.PKValidSource); ok {
		if !becomesTargetSourceMatches(e, v, ev.Obj, source, you) {
			return false
		}
	}
	if v, ok := t.Param(cards.PKValidTarget); ok {
		matched := false
		for _, id := range ev.IDs {
			if e.MatchesSpec(v, id, source, you, SpecOpts{}) {
				matched = true
				break
			}
		}
		if !matched && (ev.Amount == 1 || ev.Amount == 3) {
			matched = effects.MatchesPlayerSpecCtx(e.Game(), v, ev.Player, you, e.PlayerSpecCtx(source))
		}
		if !matched {
			return false
		}
	}
	if v, ok := t.Param(cards.PKValidCause); ok {
		cause := e.ProtectionSource(ev.Obj)
		if cause == 0 || !e.MatchesSpec(v, cause, source, you, SpecOpts{}) {
			return false
		}
	}
	return true
}

// becomesTargetSourceMatches answers a Mode$ BecomesTarget / BecomesTargetOnce
// ValidSource$ clause over the targeting stack object. Forge's `Activated`
// predicate is an ability-shape test, not a card filter, so it is answered
// directly: an activated ability's stack object carries an Ability and is not a
// stored trigger (the same split counterValidSA makes for R:Event$ Counter).
// The `Ability` base (Loki, God of Mischief's `Ability.YouCtrl`) is the same
// ability-shape test -- the ordinary object-filter grammar has no Ability base
// word, so without the special case the alternative would fall to a card-type
// test an ability stack object can never satisfy and fail closed; its dotted
// qualifiers are answered by abilitySourceQualifiersHold. Every other comma
// alternative -- SpellAbility.OppCtrl, Spell.OppCtrl, a plain object spec --
// goes through the ordinary object-filter grammar. An absent targeting object
// fails closed.
func becomesTargetSourceMatches(e Board, spec string, stackObj, source state.ObjID, you state.PlayerID) bool {
	if stackObj == 0 {
		return false
	}
	o := e.Game().Obj(stackObj)
	if o == nil {
		return false
	}
	for alt := range strings.SplitSeq(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		if alt == "Activated" {
			if o.Ability != nil && !isTriggeredObject(e.Game(), o) {
				return true
			}
			continue
		}
		if quals, isAbility := abilitySourceAlt(alt); isAbility {
			if o.Ability != nil && !isTriggeredObject(e.Game(), o) &&
				abilitySourceQualifiersHold(quals, o, you) {
				return true
			}
			continue
		}
		if e.MatchesSpec(alt, stackObj, source, you, SpecOpts{}) {
			return true
		}
	}
	return false
}

// abilitySourceAlt splits Forge's `Ability` ValidSource$ base from its
// qualifiers: a bare `Ability`, or `Ability.<quals>` with the qualifiers
// `+`-joined (Ability.YouCtrl; Ability.Land+namedLabyrinth of Skophos+YouCtrl).
// isAbility is false for every other base word.
func abilitySourceAlt(alt string) (quals []string, isAbility bool) {
	rest, ok := strings.CutPrefix(alt, "Ability")
	if !ok {
		return nil, false
	}
	if rest == "" {
		return nil, true
	}
	if !strings.HasPrefix(rest, ".") {
		return nil, false
	}
	for q := range strings.SplitSeq(rest[1:], "+") {
		if q = strings.TrimSpace(q); q != "" {
			quals = append(quals, q)
		}
	}
	return quals, true
}

// abilitySourceQualifiersHold answers the qualifiers an `Ability` ValidSource$
// alternative carries. YouCtrl/OppCtrl compare the targeting ability's
// controller (TriggeredSourceSA's on-stack controller, o.Controller) with the
// trigger source's controller. Any other qualifier -- the host-card filters
// (Ability.Land+namedX, Ability.Backup's keyword provenance) and the value
// comparisons (Ability.numTargets EQ1) -- is unmodelled here and fails closed;
// the corpus carriers that need them (Skophos Maze Warden, Mirror Shield
// Hoplite, Agrus Kos, Eternal Soldier) stay unable to fire.
func abilitySourceQualifiersHold(quals []string, o *state.Object, you state.PlayerID) bool {
	for _, q := range quals {
		switch q {
		case "YouCtrl", "YouControl", "YourControl", "YouControlled":
			if o.Controller != you {
				return false
			}
		case "OppCtrl", "OpponentControls":
			if o.Controller == you {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// landPlayedMayPlaySourceClause is the LandPlayed spelling of the may-play
// provenance clause (validSAMayPlaySourceClause is the SpellCast one).
const landPlayedMayPlaySourceClause = "SpellAbility.MayPlaySource"

// landPlayedMatches implements Mode$ LandPlayed. This fires on the MoveZone
// that the land-play special action uses to move a land to the battlefield
// (from the hand or, under a play permission, another zone) -- not on the
// separate LandPlayed event legal.go's "play_land" case also emits, which
// carries only a Player (no Obj), and so has nothing ValidCard$ could ever
// match against.
//
// ValidSA$ SpellAbility.MayPlaySource (Share the Spoils, The Fourth / Eighth
// Doctor, Serra Paragon: "When they do ...") scopes the trigger to a land
// played through a may-play permission. A land play carries no provenance
// flag, so the clause is answered from the zone: a play from a non-hand zone
// can only be a play under a permission to play from there (CR 305.1). Known
// approximation: that is "played from a non-hand zone", not "played through
// THIS card's grant" (the stand-in scanMayPlaysThisTurn and the SpellCast
// arm's FlagMayPlay already make), so a land played from the graveyard under
// Crucible of Worlds would also fire the rider, and a hand land played under
// a hand-zone grant fails closed (no rider).
func landPlayedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield {
		return false
	}
	// The move counts only when it IS the land-play special action (CR 305.1,
	// also under a play permission: Crucible of Worlds' graveyard, Light Up
	// the Stage's exile), never a land merely put onto the battlefield -- from
	// hand included (CR 305.4: putting a land onto the battlefield is not
	// playing it).
	if ev.Obj == 0 || e.Facts().PlayingLand != ev.Obj {
		return false
	}
	// Origin$ scopes the zone the land was played from ("play a land from
	// exile", "from anywhere other than your hand"): a play from a zone the
	// list does not name is not this trigger's event. The list is a
	// whitelist, so a word naming a zone the engine never plays from (Shadow
	// of the Goblin's Ante) only drops that zone; it does not void the list.
	if o, ok := t.ParamCode(cards.PKOrigin); ok {
		if zl := effects.ZoneList(o); !zl.All() && !zl.Has(ev.From) {
			return false
		}
	}
	if v, ok := t.Param(cards.PKValidSA); ok {
		// Any other spelling is unmodelled: fail closed, as ValidSAMatches does.
		if v != landPlayedMayPlaySourceClause || ev.From == state.ZHand {
			return false
		}
	}
	obj := e.Game().Obj(ev.Obj)
	if obj == nil || obj.Face() == nil || !obj.Face().IsLand() {
		return false
	}
	if v, ok := t.Param(cards.PKValidCard); ok {
		return e.MatchesSpec(v, ev.Obj, source, e.ControllerOf(source), SpecOpts{})
	}
	return true
}

// PhaseMatches implements Mode$ Phase after phaseGate has already checked
// its Phase$ parameter. The mode itself is only a StepChange event plus its
// optional ValidPlayer$ restriction.
func PhaseMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StepChange {
		return false
	}
	if v, ok := t.Param(cards.PKValidPlayer); ok {
		// StepChange carries no Player of its own -- a step always belongs
		// to the current active player.
		if !effects.MatchesPlayerSpecCtx(e.Game(), v, e.Game().Active, e.ControllerOf(source), e.PlayerSpecCtx(source)) {
			return false
		}
	}
	return true
}

// rolledDieMatches implements Mode$ RolledDie (trig:RolledDie): the trigger
// fires on the canonical per-die roll Note effRollDice emits (engine/effects/dice.go's
// DieRollNote, decoded by DieRollResult), so every "whenever you roll a die /
// roll a 4 or higher" line fires exactly as the roll happens. The trigger
// fires ONCE PER DIE, each with that die's own result -- Forge's RolledDie
// cadence, and the reading Natural$ True ("a die's highest natural result")
// and Number$ 3 ("your third die each turn") require.
//
//   - ValidResult$ is the result filter: a literal ("4"), a comma list
//     ("1,2"), an <OP><N> comparison (GE4, EQ1, LE3, GT/LT/NE) over the
//     matched value, or "Highest" (the natural roll is the die's maximum,
//     "a die's highest natural result"). An unparseable value fails closed.
//   - Natural$ True matches against the UNMODIFIED die rather than the
//     Modifier$-adjusted result ("when you roll a natural 20").
//   - ValidSides$ scopes to a die size ("on a six-sided die").
//   - ValidPlayer$ names the roller ("whenever YOU roll"), through the shared
//     player-spec grammar with the trigger's own controller as You.
//   - Number$ N ("your third die each turn") fires only on the Nth die,
//     gated at the QUEUE site (dieRollNumberAllows, in trigger_match.go's
//     checkFaceTriggers beside ActivationLimit$) -- the count can only
//     advance when the trigger is really queued, never from a speculative
//     matcher call, and the per-turn counter lives per trigger line.
//   - Static$ True lines are Forge's continuous-effect-expressed-as-a-trigger
//     (the two Attraction static lines), which this build does not run as
//     triggers -- the attachedMatches guard.
//   - RolledToVisitAttractions$ True scopes to a roll made to visit
//     Attractions; no Attraction deck exists here, so no roll is one and the
//     line never fires (fail closed, never over-fires).
func rolledDieMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	roller, sides, natural, result, ok := effects.DieRollResult(ev)
	if !ok {
		return false
	}
	return rolledDieCommon(e, t, source, roller, sides, natural, result)
}

// rolledDieOnceMatches implements Mode$ RolledDieOnce (trig:RolledDieOnce),
// Forge's "whenever you roll one or more dice" mode: the trigger fires ONCE
// per DB$ RollDice resolution on the canonical batch roll Note effRollDice
// emits (engine/effects/dice.go's DieRollBatchNote, decoded by DieRollBatchResult),
// however many dice the action rolled. The per-die Mode$ RolledDie matcher
// cannot express this cadence -- it would fire once per die -- and the batch
// Note is the resolution boundary the per-die Notes do not carry.
//
// The result filter runs against the batch's HIGHEST modified result
// (Pairs[0][1]): a ValidResult$ line fires if any die of the batch satisfied
// it, which is what Farideh's "if any of those results was 10 or higher"
// means and the only multi-die Once reader in the corpus. Every other
// RolledDieOnce carrier (Vexing Puzzlebox, Brazen Dwarf, Feywild Trickster,
// Vrondiss, Wyll, Barbarian Class) rolls exactly one die, where the highest
// result IS the result. Natural$ True on a Once line is likewise read against
// the highest modified result (0 corpus carriers; the natural-max is not
// carried on the batch Note). No Number$ rides this mode.
func rolledDieOnceMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event) bool {
	roller, _, maxResult, _, ok := effects.DieRollBatchResult(ev)
	if !ok {
		return false
	}
	return rolledDieCommon(e, t, source, roller, 0, maxResult, maxResult)
}

// rolledDieCommon applies the parameters Mode$ RolledDie and Mode$ RolledDieOnce
// share, so the two cadences can never disagree about Static$/ValidSides$/
// ValidResult$/ValidPlayer$. sides is 0 when the carrier (the batch Note) does
// not name a die size, in which case a ValidSides$ line fails closed rather
// than matching any size.
func rolledDieCommon(e Board, t cards.Trigger, source state.ObjID, roller state.PlayerID, sides, natural, result int32) bool {
	if strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKStatic)), "True") {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKRolledToVisitAttractions)), "True") {
		return false
	}
	if v := strings.TrimSpace(t.ParamStr(cards.PKValidSides)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || int32(n) != sides {
			return false
		}
	}
	matched := result
	if strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKNatural)), "True") {
		matched = natural
	}
	if v, present := t.Param(cards.PKValidResult); present && !dieResultMatches(v, matched, natural, sides) {
		return false
	}
	if v, present := t.Param(cards.PKValidPlayer); present {
		if !effects.MatchesPlayerSpecFrom(e.Game(), v, roller, e.ControllerOf(source), source) {
			return false
		}
	}
	return true
}

// dieResultMatches reports whether a die roll's matched value (the modified
// result, or the natural roll when Natural$ True) satisfies a trigger's
// ValidResult$ spec: the comma-list of literals and <OP><N> comparisons the
// corpus carries, plus the special "Highest" (the natural roll is the die's
// maximum face). An unrecognised token fails closed.
func dieResultMatches(spec string, matched, natural, sides int32) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return true
	}
	if strings.EqualFold(spec, "Highest") {
		return sides > 0 && natural == sides
	}
	for part := range strings.SplitSeq(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			if int32(n) == matched {
				return true
			}
			continue
		}
		if CompareIntCount(matched, part) {
			return true
		}
	}
	return false
}

// phaseTriggerMatches is Mode$ Phase plus the echo gate.
//
// kw:Echo (CR 702.35a) rides the Echo$ True marker on its generated keyword
// trigger the way Annihilator$ rides its own: the intervening-if must suppress
// the trigger BEFORE it stacks (a stacked-but-owed-nothing echo is an
// observable divergence). The gate reads the object's control-acquisition
// tuple (engine/rules/echo.go) against the controller's most recent upkeep.
//
// Named rather than a closure in init() so the param census attributes the
// Echo$ read to a matcher it can reach from the registration, not to init.
func phaseTriggerMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if !PhaseMatches(e, t, source, ev) {
		return false
	}
	if t.ParamStr(cards.PKEcho) == "True" {
		return e.EchoGateHolds(source)
	}
	return true
}

func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return commitCrimeMatches(e, t, source, ev)
	}, "CommitCrime")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return rolledDieMatches(e, t, source, ev)
	}, "RolledDie")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return rolledDieOnceMatches(e, t, source, ev)
	}, "RolledDieOnce")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return voteMatches(e, t, source, ev)
	}, "Vote")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return flippedCoinMatches(e, t, source, ev)
	}, "FlippedCoin")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return attachedMatches(e, t, source, ev)
	}, "Attached")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return ringTemptsMatches(e, t, source, ev)
	}, "RingTemptsYou")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return becomesTargetMatches(e, t, source, ev)
	}, "BecomesTarget")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return becomesTargetOnceMatches(e, t, source, ev)
	}, "BecomesTargetOnce")
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return landPlayedMatches(e, t, source, ev)
	}, "LandPlayed")
	registerTrigMatcher(phaseTriggerMatches, "Phase")
}

// isTriggeredObject reports whether stack object o is a triggered ability
// (rules' isTriggered; state.TriggerOf owns the fact).
func isTriggeredObject(g *state.Game, o *state.Object) bool {
	_, ok := state.TriggerOf(g, o)
	return ok
}
