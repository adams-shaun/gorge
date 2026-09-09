// trigger_queue.go owns the pending-trigger queue that checkTriggers
// (trigger_match.go) fills and putTriggersOnStack drains onto the stack in
// APNAP order, one group of same-controller triggers at a time. The
// ask/handle pairs below (askTriggerOrder/handleTriggerOrder,
// askTriggerOptional/handleTriggerOptional) are how a player orders or
// accepts/declines the triggers this file offers them; nothing here decides
// whether a trigger matches, only how an already-matched one gets resolved.
package rules

import (
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// putTriggersOnStack drains pendingTriggers onto the stack, asking each
// controller who has two or more of them for the order (CR 603.3b, user
// requirement R1) and asking each optional trigger's decider whether it goes
// on the stack at all (Forge's OptionalDecider$, user requirement R2).
//
// It reports whether it asked a decision and so has NOT finished. A true
// return means e.pending is set: the caller must return immediately without
// granting priority, and the drain is resumed by handleTriggerOrder /
// handleTriggerOptional below, both of which end in resumeTriggerDrain
// (turn.go). It is entered from exactly one place, turn.go's priorityRound,
// and re-entered only through resumeTriggerDrain -- which is itself the
// continuation of that same priorityRound. That is what makes "finish the
// priority round" the one correct continuation for every decision it asks,
// and it is why the resumption goes to grantPriority rather than back through
// priorityRound -- see resumeTriggerDrain's own header (turn.go) for the
// reason that still holds: a second call on priorityRound's path would give
// a replacement-blocked state-based action a second attempt per step and
// move sba.go's measured firing counts (Ruling T22-p). (Ruling T28-b: this
// used to say re-entering priorityRound "would run the draw step's draw a
// second time" -- true before Task 28 moved the draw out of priorityRound
// entirely; the reason above is what actually survives that move.)
//
// ORDERING DIRECTION (Ruling U2, and the one thing here that is silent if it
// is backwards): the ordering decision's choice[0] is the trigger put on the
// stack FIRST, and therefore the trigger that resolves LAST. That is the same
// direction as the between-player rule either side of it -- APNAP puts the
// active player's triggers on the stack first and resolves them last -- so
// one sentence describes the whole placement. The Prompt says so in the
// player's own words, and TestTriggerOrderChoiceDecidesResolutionOrder
// asserts it by the order the two effects actually RUN, not by the order they
// were pushed.
//
// The between-player order is not the player's (Ruling U1): it is CR 603.3b's
// APNAP, kept here as the same sort.SliceStable by controller rank the
// pre-Task-27 version used. Ties within one controller keep checkTriggers'
// discovery order (seat, then zone, then zone position); that is the order the
// options are OFFERED in, and which of them actually goes first is then the
// controller's answer rather than the engine's.
//
// RESUMABILITY (Ruling U3). The queue this walks can grow underneath it.
// Submit runs handle, then checkStateBased, then Advance, and checkStateBased
// (sba.go) is a bounded fixed-point loop that emits PlayerLost, MoveZone and
// GameOver events -- every one of which runs checkTriggers. So a creature
// dying to a state-based action while a controller is being asked to order
// their triggers really does append to e.pendingTriggers between the ask and
// the answer. Three properties make that safe:
//
//   - Appends only ever land at the END of e.pendingTriggers, and a decision
//     is always about entries at the FRONT. Nothing but popFrontTrigger and
//     dropDepartedTriggers ever removes an entry.
//   - e.orderedTriggers counts the leading entries whose order is already
//     settled. While it is non-zero the queue is not re-sorted, so a trigger
//     arriving mid-drain can neither be shuffled into a group the controller
//     has already ordered nor make them order the same triggers twice.
//   - A trigger leaves the queue in the same call that pushes it (or declines
//     it), so nothing can be pushed twice or dropped on the floor.
//
// A trigger that arrives after the group it would have joined was settled
// simply forms the next group, and is still placed before any player receives
// priority -- which is all CR 117.5 asks of it.
//
// CR 800.4a: an ability controlled by a player who has left the game ceases to
// exist, so dropDepartedTriggers discards those rather than (as the
// pre-Task-27 version did) ranking them after every living seat and putting
// them on the stack regardless.
func (e *Engine) putTriggersOnStack() bool {
	for {
		if e.G.Over {
			e.pendingTriggers, e.orderedTriggers = nil, 0
			return false
		}
		e.dropDepartedTriggers()
		if len(e.pendingTriggers) == 0 {
			e.pendingTriggers, e.orderedTriggers = nil, 0
			return false
		}
		if e.orderedTriggers == 0 {
			n := e.sortPendingTriggers()
			if n >= 2 {
				// R1. Exactly one trigger is never asked about: there is no
				// choice to make, and a decision with a single legal answer is
				// noise on the wire (definition of done, item 2).
				e.askTriggerOrder(e.pendingTriggers[0].Controller, n)
				return true
			}
			e.orderedTriggers = n
		}
		pt := e.pendingTriggers[0]
		// CR 603.5: an optional triggered ability goes on the stack
		// REGARDLESS of whether its controller wants to apply the effect; the
		// choice is made as it resolves (resolveTop's ability branch poses
		// askOptionalAtResolution). So an OptionalDecider$ trigger is pushed
		// unconditionally here, exactly like a mandatory one. Only a Miracle
		// offer (Task 18) keeps asking at placement -- it is a keyword CAST
		// offer, not a 603.5 optional triggered ability, and answer is what
		// decides whether the card is cast for its miracle cost at all.
		if who, optional, askable := e.optionalDecider(pt); optional && pt.Miracle {
			if !askable {
				// The decider left the game between this trigger matching and
				// its turn to be placed. R2 forbids assuming the answer, so
				// the trigger is declined rather than silently placed.
				e.popFrontTrigger()
				continue
			}
			e.askTriggerOptional(who, pt)
			return true
		}
		e.popFrontTrigger()
		e.pushTrigger(pt)
		if e.Pending() != nil {
			// Task 7: the trigger asked for its targets right after its
			// TriggerPush. Return immediately without granting priority, exactly
			// as for the ordering and optional asks above; the drain resumes
			// through handleTarget -> resumeTriggerDrain.
			return true
		}
	}
}

// sortPendingTriggers puts the whole queue in CR 603.3b APNAP order by
// controller and reports how many leading entries share the first entry's
// controller -- the size of the group whose internal order is that
// controller's to choose.
//
// Called only when e.orderedTriggers is zero, i.e. when no group's order has
// been settled yet, so it can never re-order an answer a player already gave.
// dropDepartedTriggers runs first on every pass, so every controller left here
// is alive and therefore present in rank; the map is indexed by key and never
// ranged over, so no map iteration order can reach an event or the order of a
// decision's options.
func (e *Engine) sortPendingTriggers() int {
	seats := e.G.AliveFrom(e.G.Active)
	rank := make(map[state.PlayerID]int, len(seats))
	for i, p := range seats {
		rank[p] = i
	}
	sort.SliceStable(e.pendingTriggers, func(i, j int) bool {
		return rank[e.pendingTriggers[i].Controller] < rank[e.pendingTriggers[j].Controller]
	})
	first := e.pendingTriggers[0].Controller
	n := 0
	for _, pt := range e.pendingTriggers {
		if pt.Controller != first {
			break
		}
		n++
	}
	return n
}

// dropDepartedTriggers discards every pending trigger controlled by a player
// who has left the game (CR 800.4a) and leaves e.orderedTriggers counting the
// same surviving entries it counted before.
func (e *Engine) dropDepartedTriggers() {
	kept := e.pendingTriggers[:0]
	ordered := 0
	for i, pt := range e.pendingTriggers {
		if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
			continue
		}
		if i < e.orderedTriggers {
			ordered++
		}
		kept = append(kept, pt)
	}
	e.pendingTriggers = kept
	e.orderedTriggers = ordered
}

// popFrontTrigger removes the entry every pending decision is about. Entries
// are shifted down rather than the slice re-sliced so that index 0 always
// means the same thing to every reader, including one that ran between an ask
// and its answer.
func (e *Engine) popFrontTrigger() {
	if len(e.pendingTriggers) == 0 {
		return
	}
	e.removeTriggerAt(0)
}

// removeTriggerAt takes one entry out of the queue, keeping orderedTriggers
// counting the same settled entries it counted before. Entries are shifted
// down rather than the slice re-sliced so index 0 always means the same thing
// to every reader, including one that ran between an ask and its answer.
func (e *Engine) removeTriggerAt(i int) pendingTrigger {
	pt := e.pendingTriggers[i]
	e.pendingTriggers = append(e.pendingTriggers[:i], e.pendingTriggers[i+1:]...)
	if i < e.orderedTriggers {
		e.orderedTriggers--
	}
	return pt
}

// takeAnsweredTrigger removes and returns the pending trigger an optional
// decision was asked about.
//
// It is normally at the front: nothing on a reachable path removes from the
// front of the queue between an ask and its answer. Fix round 1, review
// finding F4: this used to be a bare front-only equality check whose failure
// branch did nothing at all, so the drain went straight on to ask the SAME
// trigger's question again -- the player's answer consumed, its Seq spent,
// and silently discarded. Searching forward instead honours the answer
// wherever the entry actually sits and never puts the same question twice,
// which is also how handleTriggerOrder's own defensive branch behaves (it
// forces progress rather than re-asking). A source carrying two pending
// triggers is unambiguous here because the search runs front-first, and the
// front one is the one that was asked about.
func (e *Engine) takeAnsweredTrigger(d *decision.Decision) (pendingTrigger, bool) {
	for i := range e.pendingTriggers {
		if e.pendingTriggers[i].Source == d.Source {
			return e.removeTriggerAt(i), true
		}
	}
	return pendingTrigger{}, false
}

// pushTrigger mints one triggered ability's stack object.
//
// Ruling T20-a: the object itself is created inside events.Apply's TriggerPush
// case, not here -- a direct, unlogged Game.AddObject call (as this used to
// be) names an ObjID a log-only replay never learns about, so its own
// zone-placement Move silently no-ops and the replayed stack permanently
// diverges from the live one. Player/Obj/Amount/IDs are all Apply needs to
// recreate the same object: the permanent (pt.Source), its Triggers index
// (pt.Idx, so Apply re-derives the *cards.SA rather than needing a logged
// pointer), the controller, and the Remembered object(s).
//
// Ruling U4: the ORDER of these events is the only place the ordering choice
// is recorded, and it is the whole of what a log-only replay needs. No event
// kind and no Event field was added for Task 27.
func (e *Engine) pushTrigger(pt pendingTrigger) {
	// Task 18: a Miracle offer is placed by casting the card for its miracle
	// cost, not by minting a triggered-ability stack object. castMiracle
	// verifies the card is still in the owner's hand, emits the reveal Note,
	// and begins the ordinary cast flow; it may pause on an X/target decision
	// of its own, which is exactly Task 7's drainAwaitsTarget continuation (see
	// castMiracle's doc).
	// Task 18: a Miracle offer is placed by casting the card for its miracle
	// cost, not by minting a triggered-ability stack object. castMiracle
	// verifies the card is still in the owner's hand, emits the reveal Note,
	// and begins the ordinary cast flow; it may pause on an X/target decision
	// of its own, which is exactly Task 7's drainAwaitsTarget continuation
	// (see castMiracle's doc). A Miracle trigger is NOT delayed, so the flag
	// order matters: a Miracle offer must reach castMiracle, never the
	// DelayedPush path below.
	if pt.Miracle {
		e.castMiracle(pt)
		return
	}
	// A Mode$ Phase delayed trigger (CR 603.7): its stack object is minted by
	// a DelayedPush event rather than a TriggerPush. The difference is the
	// Ability: TriggerPush re-derives it from a face Triggers index, while a
	// delayed trigger's Effect is the Execute$ SVar-named sub-ability on the
	// source's face, so the fired event carries the Execute$ name (Counter)
	// for events.Apply to resolve. Everything else -- the controller guard
	// below, the CR 800.4a check shared with the ordinary path, and the
	// Remembered/PlayerRef encoding -- is the same. A delayed trigger's
	// Execute in the Mode$ Phase shape this build implements declares no
	// ValidTgts$ (Flickerwisp's TrigBounce re-derives its referent from
	// Defined$ DelayTriggerRememberedLKI), so no target ask follows the push.
	if pt.Delayed {
		if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
			return
		}
		ids := make([]state.ObjID, 0, len(pt.Ctx.Remembered))
		for _, tgt := range pt.Ctx.Remembered {
			if tgt.IsPlayer {
				ids = append(ids, state.PlayerRef(tgt.Player))
				continue
			}
			ids = append(ids, tgt.Obj)
		}
		e.emit(events.Event{Kind: events.DelayedPush, Player: pt.Controller,
			Obj: pt.Source, Amount: int32(pt.DelayedID), Counter: pt.Execute,
			IDs: ids, Text: "delayed trigger"})
		e.drainAwaitsTarget = e.Pending() != nil
		return
	}
	// CR 800.4a / Ruling U6, fix round 2 (re-review N2): an ability
	// controlled by a player who has left the game ceases to exist, so it is
	// never minted. dropDepartedTriggers enforces this for the queue, but it
	// only runs inside putTriggersOnStack -- it cannot run while a decision
	// is pending, and an optional trigger's DECIDER may be a different,
	// living seat (OptionalDecider$ TriggeredCardController, 40 T: lines in
	// the corpus). So a decider could answer yes for a controller who had
	// been eliminated in the meantime and resurrect their ability. This is
	// the check placed at the one point a triggered ability's stack object is
	// created, rather than at each of the paths that reach it.
	if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
		return
	}
	// FL-41: Remembered can hold a player entry now (triggerRemembered's
	// DeclareAttackers case appends the defending player), but IDs is an
	// []ObjID -- there is no field here for a bare PlayerID. Encoding it
	// with state.PlayerRef instead of just writing tgt.Obj (always 0 for a
	// player target) is what lets events.Apply's TriggerPush case below
	// reconstruct {Player: p, IsPlayer: true} rather than {Obj: 0} -- which
	// playersOf (effects/context.go) would filter out, so Defined$
	// TriggeredDefendingPlayer would resolve to nothing and the effect
	// silently no-op (PlayerOf is never reached).
	ids := make([]state.ObjID, 0, len(pt.Ctx.Remembered))
	for _, tgt := range pt.Ctx.Remembered {
		if tgt.IsPlayer {
			ids = append(ids, state.PlayerRef(tgt.Player))
			continue
		}
		ids = append(ids, tgt.Obj)
	}
	e.emit(events.Event{Kind: events.TriggerPush, Player: pt.Controller,
		Obj: pt.Source, Amount: int32(pt.Idx), IDs: ids, Text: "triggered ability"})
	// Task 7: a trigger that declares ValidTgts$ asks its controller for
	// targets RIGHT AFTER its TriggerPush -- the ability object is now top of
	// stack, and the choice is asked before any player receives priority. The
	// drain records drainAwaitsTarget (Clone copies it) so handleTarget knows
	// to resume the drain through the SAME continuation handleTriggerOrder
	// uses rather than granting priority; putTriggersOnStack sees the new
	// pending decision and returns true. If the TriggerPush no-opped (Apply's
	// own guard), the stack never grew and there is no ability object to
	// target for -- skip the ask. askTarget itself may decline to ask (its
	// TargetMin$ 0 / fizzle paths), which is why the flag is derived from
	// e.Pending(), not from "we wanted to ask". SA is nil only for a
	// hand-seeded queue entry (clone_test's seeded fake), which never has
	// ValidTgts$ -- mirror the nil-tolerance the TriggerPush out-of-range
	// guard already provides.
	//
	// CR 603.3c: a modal triggered ability announces its mode choice when it
	// is put on the stack, not at resolution. That ask is posed here too,
	// in preference to the target ask for a trigger whose effect carries
	// both a Choices$ clause and a ValidTgts$ (the modal shape a Charm
	// commonly pairs with target selection INSIDE its modes, not on the
	// ability itself -- a top-level ValidTgts$ alongside Choices$ is not
	// exercised by the corpus). The answered modes are recorded on the
	// stack object (ChosenModes) and the drain resumes through handleModes,
	// the same continuation shape.
	if pt.SA != nil && len(e.G.Stack) > 0 &&
		e.G.Obj(e.G.Stack[len(e.G.Stack)-1]) != nil {
		id := e.G.Stack[len(e.G.Stack)-1]
		if pt.SA.Params["Choices"] != "" {
			e.askTriggerModes(pt.Controller, id, pt.SA)
			e.drainAwaitsModes = true
		} else if pt.SA.Params["ValidTgts"] != "" {
			e.askTarget(pt.Controller, id, pt.SA)
		}
	}
	// Fix round 1 (reviewer minor, cheap): derive drainAwaitsTarget from
	// e.Pending() rather than clearing it first. The old form set it false
	// up front and recomputed it only inside the ask branch, so a flag an
	// OUTER caller had already set and not yet consumed was discarded the
	// moment pushTrigger reached here without asking a target of its own.
	// Deriving it from Pending() after the optional ask preserves a
	// pre-existing true and recomputes the one decision the drain was paused
	// on otherwise; askTarget may decline to ask (its TargetMin$ 0 / fizzle
	// paths), which is exactly why the flag must come from the resulting
	// pending state rather than from "we wanted to ask".
	e.drainAwaitsTarget = e.Pending() != nil && !e.drainAwaitsModes
}

// triggerOf re-reads the T: line a pending trigger came from, so nothing has
// to be cached on pendingTrigger for it.
//
// Fix round 1, review finding F5: the guarantee is narrower than the previous
// wording claimed. A parsed cards.Trigger is static text, but this reads
// Obj.Face(), which is NOT static -- it follows the object's ACTIVE face. A
// permanent whose face changed between the trigger matching and the drain
// would be read against the new face's Triggers slice, giving the wrong
// optionality flag and label. Nothing in M1 can reach that (effects.SetState
// is the only FlipFace source and no card in play uses it), but this is
// "correct because nothing flips faces", not "correct because the data cannot
// change". A source that has ceased to exist, or an index the current face is
// too short for, reports false rather than panicking.
func (e *Engine) triggerOf(pt pendingTrigger) (cards.Trigger, bool) {
	o := e.G.Obj(pt.Source)
	if o == nil {
		return cards.Trigger{}, false
	}
	f := o.Face()
	if f == nil || pt.Idx < 0 || pt.Idx >= len(f.Triggers) {
		return cards.Trigger{}, false
	}
	return f.Triggers[pt.Idx], true
}

// findTriggerForAbility returns the face trigger on source whose Effect is
// exactly the given *cards.SA -- the T: line a triggered-ability stack object
// came from. A triggered ability's stack object carries the same compiled SA
// pointer its source's Triggers entry holds (events.Apply re-derives it from
// the TriggerPush's Idx), so pointer equality identifies the line. This is
// what lets resolution (resolveTop's ability branch) re-read the trigger's
// CR 603.4 intervening-if condition and optionality (CR 603.5) from the
// source's static text rather than caching them on the stack object.
//
// It returns false for an activated ability, whose o.Ability comes from the
// source's Abilities slice rather than a Triggers entry, and for any source
// whose face has since changed or gone -- so a caller must treat false as
// "this is not a face trigger, apply no trigger-only rule" rather than as an
// error. A source that has ceased to exist entirely (a token or copy gone
// from the board) degrades the same way.
func (e *Engine) findTriggerForAbility(source state.ObjID, sa *cards.SA) (cards.Trigger, bool) {
	if sa == nil {
		return cards.Trigger{}, false
	}
	o := e.G.Obj(source)
	if o == nil {
		return cards.Trigger{}, false
	}
	f := o.Face()
	if f == nil {
		return cards.Trigger{}, false
	}
	for _, t := range f.Triggers {
		if t.Effect == sa {
			return t, true
		}
	}
	return cards.Trigger{}, false
}

// optionalDecider reports whether pt is an optional trigger, which seat gets
// the yes/no question, and whether that seat can still answer it.
//
// The spelling is Forge's, established by grepping .cards/cardsfolder rather
// than guessed (the precedent for not guessing is Ruling T12-a). On a T: line
// optionality is spelled OptionalDecider$ and nothing else: 1496 T: lines
// carry it, and the bare Optional$ form -- 1143 occurrences (SVar 884, A 225,
// R 19, S 15) -- never once appears on a T: line, because it is a different
// thing (a "you may" inside an ability's own resolution, not a choice about
// whether the trigger is put on the stack). The parser needs no change to
// carry it: parseParams already collects every Key$ value on the line into
// Trigger.Params.
//
// "Bare" means the anchored pattern (^|[^A-Za-z])Optional$ and is load-
// bearing in the count: the unanchored substring gives 1199, because Forge
// also has RevealOptional$ (39), ChoiceOptional$ (10) and RepeatOptional$
// (7). Three separate counts were produced for this figure across two review
// rounds -- 1141, 1199 and 1143 -- before it was settled by anchoring the
// pattern. The load-bearing numbers above (1496, and zero on any T: line)
// were exact every time.
//
// The decider values that actually occur on T: lines, with counts:
//
//	You                        1441   the trigger's controller
//	TriggeredCardController      40   controller of the triggering card
//	TriggeredSourceController     5   controller of the triggering source
//	TriggeredPlayer               3   the player the event was about
//	EnchantedController           3   controller of the enchanted permanent
//	TriggeredAttackingPlayer      2   the attacking player
//	TriggeredActivator            2   who activated the triggering ability
//
// So the answer to "is the decider ever someone other than the controller" is
// yes, and it is honoured for the two forms this engine can actually resolve:
// TriggeredCardController and TriggeredSourceController both read the
// controller of the object the trigger remembered, which is the object the
// triggering event was about (triggerRemembered, above) -- the same object
// Forge means by both names in every T: line in the corpus that uses them.
//
// CAVEAT (Task 6, fix round 1): that equivalence assumes Remembered[0] is
// the object the FIRING trigger's own T: line is on, which triggerRemembered
// no longer guarantees for a DeclareAttackers-driven Attacks trigger --
// Remembered there is every attacker declared against one defending player
// (handleAttackers emits one event per defender since Task m34), in event
// order, so Remembered[0] is whichever creature attacking that defender was
// declared first, not necessarily pt.Source. No repo-deck T: line combines
// Mode$ Attacks with OptionalDecider$ TriggeredCardController/
// TriggeredSourceController today (measured against the same corpus the
// table above was), so this is a latent gap, not a live one -- but a future
// card that did would have this read the wrong creature's controller
// whenever it attacks alongside another creature that happened to be
// declared first.
//
// LIMITATION, stated rather than assumed: the remaining ten T: lines
// (TriggeredPlayer, EnchantedController, TriggeredAttackingPlayer,
// TriggeredActivator -- 0.7% of the corpus's optional triggers, and none of
// them in a mode this milestone implements) name a player this engine's
// pendingTrigger cannot derive, because events.Event carries no attacking
// player, no activator and no enchant link. Those fall back to asking the
// controller. That still satisfies R2 -- a human is asked, and no outcome is
// assumed -- but it can ask the wrong human, so it is a real gap and not a
// silent approximation. Closing it needs the trigger to capture the player at
// match time, which is a change to what checkTriggers records.
func (e *Engine) optionalDecider(pt pendingTrigger) (who state.PlayerID, optional, askable bool) {
	// Task 18: a Miracle offer is always optional and its decider is always the
	// owner (the controller of the drawn card). It has no T: line to read, so
	// this must be special-cased before triggerOf (which would fail for it).
	if pt.Miracle {
		who = pt.Controller
		if int(who) >= len(e.G.Players) || e.G.Players[who].Lost {
			return who, true, false
		}
		return who, true, true
	}
	t, ok := e.triggerOf(pt)
	if !ok {
		return 0, false, false
	}
	spec := t.Params["OptionalDecider"]
	if spec == "" {
		return 0, false, false
	}
	who, askable = e.deciderFromSpec(spec, pt.Controller, pt.Ctx.Remembered)
	return who, true, askable
}

// deciderFromSpec resolves an OptionalDecider$ spec string to the seat that
// answers the yes/no, plus whether that seat can still answer it. It is the
// one place the spec grammar lives, shared by the placement path (which has
// a pendingTrigger) and the CR 603.5 resolution path (rules/stack.go's
// resolveTop, which has the stack object's own controller and Remembered
// rather than a pendingTrigger -- the two carry the same controller and the
// same Remembered objects, so the decider is re-derived identically).
// controller is the ability's controller and remembered the objects the
// trigger captured; the returned askable is false when the decider has left
// the game.
func (e *Engine) deciderFromSpec(spec string, controller state.PlayerID, remembered []state.Target) (who state.PlayerID, askable bool) {
	who = controller
	switch spec {
	case "You":
		// The controller, which who already is.
	case "TriggeredCardController", "TriggeredSourceController":
		if len(remembered) > 0 {
			if o := e.G.Obj(remembered[0].Obj); o != nil {
				who = o.Controller
			}
		}
	}
	if int(who) >= len(e.G.Players) || e.G.Players[who].Lost {
		return who, false
	}
	return who, true
}

// PendingTriggers reports the triggers matched but not yet on the stack, in
// queue order (index 0 is placed first). Read-only; the slice is fresh, and
// so is each entry's own Label -- neither aliases e.pendingTriggers, so a
// caller (view.Project) mutating what it gets back cannot corrupt the drain.
//
// Ruling F2/R3: this is the whole of what lets view describe the trigger
// queue without importing rules -- state.PendingTrigger is the shared
// vocabulary, built here from the same triggerLabel and optionalDecider the
// drain itself (putTriggersOnStack, above) already uses, so a client's view
// of "what's about to hit the stack" can never disagree with what actually
// does.
func (e *Engine) PendingTriggers() []state.PendingTrigger {
	if len(e.pendingTriggers) == 0 {
		return nil
	}
	out := make([]state.PendingTrigger, 0, len(e.pendingTriggers))
	for _, pt := range e.pendingTriggers {
		who, optional, _ := e.optionalDecider(pt)
		out = append(out, state.PendingTrigger{
			Source:     pt.Source,
			Controller: pt.Controller,
			Label:      e.triggerLabel(pt),
			Optional:   optional,
			Decider:    who,
		})
	}
	return out
}

// StackOptional reports whether a stack object is an optional triggered
// ability whose resolution-time yes/no (CR 603.5) has not yet been answered,
// and which seat answers it. Ruling VW-1: the optionality belongs on the
// stack entry, because under CR 603.5 the ability is already on the stack
// when the question is posed, so the queue is empty at that moment. It
// reuses the same OptionalDecider$ read and deciderFromSpec derivation as
// the placement and resolution paths rather than re-deriving the spec
// grammar — the view asks for the answer through view.Chars.
//
// It reports not-optional for anything that is not a face trigger (an
// activated ability, or a source whose face has since changed or gone), and
// for a trigger whose decider has left the game — a decider who is nobody
// has already ceased to exist (CR 800.4a), so such an object is never
// observed awaiting its question on the stack.
func (e *Engine) StackOptional(id state.ObjID) (optional bool, decider state.PlayerID) {
	o := e.G.Obj(id)
	if o == nil || o.Ability == nil {
		return false, 0
	}
	t, ok := e.findTriggerForAbility(o.Source, o.Ability)
	if !ok {
		return false, 0
	}
	spec := t.Params["OptionalDecider"]
	if spec == "" {
		return false, 0
	}
	who, askable := e.deciderFromSpec(spec, o.Controller, o.Remembered)
	if !askable {
		return false, 0
	}
	return true, who
}

// triggerLabel is what a client shows for one pending trigger. The card's own
// TriggerDescription$ is the text a real player would recognise; the source's
// name disambiguates two copies of the same card.
func (e *Engine) triggerLabel(pt pendingTrigger) string {
	// Task 18: a Miracle offer is named by the card and its miracle cost, not by
	// a TriggerDescription$ (there is no T: line). This is the label the brief's
	// interface spells -- "Miracle — reveal <name> and cast it for <cost>?" --
	// and it is what askTriggerOptional shows inside its offer prompt.
	if pt.Miracle {
		name := "it"
		if o := e.G.Obj(pt.Source); o != nil {
			if f := o.Face(); f != nil && f.Name != "" {
				name = f.Name
			}
		}
		cost, ok := e.miracleCost(pt.Source)
		if !ok {
			cost = ""
		}
		return "Miracle — reveal " + name + " and cast it for " + cost + "?"
	}
	name := "Triggered ability"
	if o := e.G.Obj(pt.Source); o != nil {
		if f := o.Face(); f != nil && f.Name != "" {
			name = f.Name
		}
	}
	if t, ok := e.triggerOf(pt); ok {
		if d := t.Params["TriggerDescription"]; d != "" {
			return name + ": " + d
		}
	}
	return name
}

// abilityLabel is the resolution-side sibling of triggerLabel: the label a
// client sees for a triggered-ability stack OBJECT (which carries no
// pendingTrigger to read a queue entry off), built from the source's name
// and the trigger's own TriggerDescription$ -- the same text triggerLabel
// shows for the queued trigger it came from.
func (e *Engine) abilityLabel(o *state.Object, t cards.Trigger) string {
	name := "Triggered ability"
	if src := e.G.Obj(o.Source); src != nil {
		if f := src.Face(); f != nil && f.Name != "" {
			name = f.Name
		}
	}
	if desc := t.Params["TriggerDescription"]; desc != "" {
		return name + ": " + desc
	}
	return name
}

// askTriggerModes is CR 603.3c: a modal triggered ability's controller
// announces the mode choice when putting the ability on the stack, not at
// resolution. It poses the same KModes decision effCharm would, but at
// placement, with ResumeKind/ResumeSA set so the answer's handler records
// the chosen SVar names onto the stack object (handleModes' placement
// branch) rather than re-entering a suspended resolution.
//
// CharmNum is read as a literal integer (default 1, the overwhelmingly
// common "choose one"), because the full Num/Qty grammar needs a resolving
// context this placement ask does not have; a trigger whose CharmNum is
// computed is rare and degrades to 1, same as the no-engine-host fallback.
// The option list mirrors effCharm's -- Choices$ order, SpellDescription$ as
// the label, resolved from the trigger's source SVar table -- so an index
// chosen here maps to the same SVar name modeChoiceNames produces at
// resolution.
func (e *Engine) askTriggerModes(p state.PlayerID, obj state.ObjID, sa *cards.SA) {
	charmNum := 1
	if v, ok := sa.Params["CharmNum"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 {
			charmNum = n
		}
	}
	var source state.ObjID
	var svars map[string]string
	if so := e.G.Obj(obj); so != nil {
		source = so.Source
	}
	if so := e.G.Obj(source); so != nil {
		if sf := so.Face(); sf != nil {
			svars = sf.SVars
		}
	}
	e.ask(modeDecision(p, source, sa, svars, charmNum))
}

// askTriggerOrder is R1: the controller of two or more simultaneous triggers
// chooses the order. Min == Max == n over exactly that controller's own n
// pending triggers, so Decision.Validate's existing rules (n choices, all
// distinct, all in range) already mean "a permutation of these n" and Ruling
// U2 needs no wire-format change at all.
//
// Option i is e.pendingTriggers[i]; that positional correspondence is the only
// binding between the decision and the queue, and handleTriggerOrder rechecks
// it before acting on the answer.
func (e *Engine) askTriggerOrder(p state.PlayerID, n int) {
	d := &decision.Decision{Player: p, Kind: decision.KTriggerOrder, Min: n, Max: n,
		Prompt: "Order your simultaneous triggered abilities: the one you choose " +
			"first is put on the stack first, and so resolves last"}
	for i := 0; i < n; i++ {
		pt := e.pendingTriggers[i]
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "trigger",
			Label: e.triggerLabel(pt), Obj: pt.Source, Player: pt.Controller})
	}
	e.ask(d)
}

// handleTriggerOrder applies an answered ordering decision, then resumes the
// drain. in.Choices is exactly a permutation of [0, n) -- Validate enforced
// that -- so the copy below is total: every entry of the group is written
// exactly once and none is lost or duplicated.
func (e *Engine) handleTriggerOrder(d *decision.Decision, in decision.Intent) {
	if e.frontIsTheOfferedGroup(d) {
		n := len(d.Options)
		perm := make([]pendingTrigger, 0, n)
		for _, c := range in.Choices {
			perm = append(perm, e.pendingTriggers[c])
		}
		copy(e.pendingTriggers, perm)
		e.orderedTriggers = n
	} else if len(e.pendingTriggers) > 0 {
		// Defensive, and believed unreachable: nothing between ask and answer
		// removes from the front of the queue, so the group is still there.
		// If it somehow is not, settle exactly one entry so the drain makes
		// forward progress in the queue's existing order rather than re-asking
		// a decision whose answer it cannot apply.
		e.orderedTriggers = 1
	}
	e.resumeTriggerDrain()
}

// frontIsTheOfferedGroup rechecks that the queue still starts with exactly the
// triggers d offered, in the order it offered them. Ruling U3's "cannot be
// reordered by that path", verified rather than assumed.
func (e *Engine) frontIsTheOfferedGroup(d *decision.Decision) bool {
	n := len(d.Options)
	if n == 0 || n > len(e.pendingTriggers) || e.orderedTriggers != 0 {
		return false
	}
	for i := 0; i < n; i++ {
		if e.pendingTriggers[i].Controller != d.Player ||
			e.pendingTriggers[i].Source != d.Options[i].Obj {
			return false
		}
	}
	return true
}

// askTriggerOptional is R2: an optional trigger reaches the stack only on an
// explicit yes. Min == Max == 1 over two options, "yes" first.
func (e *Engine) askTriggerOptional(who state.PlayerID, pt pendingTrigger) {
	label := e.triggerLabel(pt)
	d := &decision.Decision{Player: who, Kind: decision.KTriggerOptional, Min: 1, Max: 1,
		Prompt: "Put this optional triggered ability on the stack? — " + label,
		Source: pt.Source,
		Options: []decision.Option{
			{Index: 0, Kind: "yes", Label: "Yes — " + label, Obj: pt.Source, Player: pt.Controller},
			{Index: 1, Kind: "no", Label: "No", Obj: pt.Source, Player: pt.Controller},
		}}
	e.ask(d)
}

// askOptionalAtResolution is CR 603.5's question at resolution time: the
// optional triggered ability is already ON the stack (putTriggersOnStack
// pushes it unconditionally), and its decider chooses whether to apply the
// effect as it resolves. It poses a KTriggerOptional decision with a resume
// point whose kind is "optional", so handleTriggerOptional can route the
// answer: a yes re-enters the suspended resolution (resumeResolution runs
// the ability's effect), a no lets the ability leave the stack having done
// nothing (finishResumption). The decider is derived from the stack object's
// own controller + Remembered (deciderFromSpec), not from a pendingTrigger,
// because the queued trigger has already been consumed by the drain.
func (e *Engine) askOptionalAtResolution(who state.PlayerID, o *state.Object, sa *cards.SA, label string) {
	d := &decision.Decision{Player: who, Kind: decision.KTriggerOptional, Min: 1, Max: 1,
		ResumeKind: "optional", ResumeSA: sa, Source: o.Source,
		Prompt: "Apply this triggered ability's effect? — " + label,
		Options: []decision.Option{
			{Index: 0, Kind: "yes", Label: "Yes — " + label, Obj: o.Source, Player: o.Controller},
			{Index: 1, Kind: "no", Label: "No", Obj: o.Source, Player: o.Controller},
		}}
	e.ask(d)
	// The resume point mirrors Engine.Ask's shape (rules/resolution.go): the
	// suspended object is the top of stack, the sub-ability to resume is the
	// ability's own effect, and the kind tags the switch in resumeResolution
	// so the yes answer does not overwrite ctx.Modes (which was already
	// seeded from ChosenModes for a modal trigger). A decision pending means
	// nothing else can resolve between ask and answer, so the object cannot
	// have moved. askOptionalAtResolution is only ever reached from a first-
	// pass resolution (resolveTop's ability branch, not already suspended),
	// so this is a fresh resume point, never stacked over an existing one.
	e.resume = &resumePoint{kind: "optional", obj: o.ID, sa: sa}
}

// handleTriggerOptional applies an answered optional-trigger decision. There
// are two shapes, told apart by whether a resolution is suspended:
//
//   - The CR 603.5 resolution ask (e.resume set with kind "optional"): the
//     ability is already on the stack. A yes re-enters the suspended
//     resolution through resumeResolution (which runs the ability's effect,
//     exactly as resolveTop's own tail would have, and then moves it off the
//     stack); a no lets the ability leave the stack having done nothing via
//     finishResumption. Either way the resolution completes and, mirroring
//     resumeResolution's own tail, priority returns to the active player.
//   - The placement ask (a Miracle offer, the one place an optional decision
//     is still handed out before the ability is on the stack): a yes places
//     the trigger (for a Miracle, that begins the cast flow), a no discards
//     it, then the drain resumes.
//
// A declined optional trigger on the resolution path still emits its
// TriggerPush (the ability did go on the stack); only a placement no emits
// nothing beyond every decision's own DecisionAsk/DecisionMade.
func (e *Engine) handleTriggerOptional(d *decision.Decision, in decision.Intent) {
	yes := false
	if opts := d.Chosen(in); len(opts) == 1 {
		yes = opts[0].Kind == "yes"
	}
	if e.resume != nil {
		rp := e.resume
		e.resume = nil
		if yes {
			e.resumeResolution(rp, d.Chosen(in))
		} else {
			e.finishResumption(rp.obj)
			// Mirror resumeResolution's own tail (CR 117.3b): a suspended
			// resolution that completes -- even by doing nothing -- resets
			// the pass count and returns priority to the active player.
			e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
		}
		return
	}
	if pt, ok := e.takeAnsweredTrigger(d); ok && yes {
		e.pushTrigger(pt)
	}
	e.resumeTriggerDrain()
}
