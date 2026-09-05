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
		if who, optional, askable := e.optionalDecider(pt); optional {
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
// Remembered there is every declared attacker in the whole combat, in
// event order, so Remembered[0] is whichever creature attacked first, not
// necessarily pt.Source. No repo-deck T: line combines Mode$ Attacks with
// OptionalDecider$ TriggeredCardController/TriggeredSourceController today
// (measured against the same corpus the table above was), so this is a
// latent gap, not a live one -- but a future card that did would have this
// read the wrong creature's controller whenever it attacks alongside
// another creature that happened to be declared first.
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
	t, ok := e.triggerOf(pt)
	if !ok {
		return 0, false, false
	}
	spec := t.Params["OptionalDecider"]
	if spec == "" {
		return 0, false, false
	}
	who = pt.Controller
	switch spec {
	case "You":
		// The controller, which who already is.
	case "TriggeredCardController", "TriggeredSourceController":
		if len(pt.Ctx.Remembered) > 0 {
			if o := e.G.Obj(pt.Ctx.Remembered[0].Obj); o != nil {
				who = o.Controller
			}
		}
	}
	if int(who) >= len(e.G.Players) || e.G.Players[who].Lost {
		return who, true, false
	}
	return who, true, true
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

// triggerLabel is what a client shows for one pending trigger. The card's own
// TriggerDescription$ is the text a real player would recognise; the source's
// name disambiguates two copies of the same card.
func (e *Engine) triggerLabel(pt pendingTrigger) string {
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

// handleTriggerOptional places the trigger on a yes and discards it on a no,
// then resumes the drain. A no emits nothing at all beyond the DecisionAsk and
// DecisionMade that every decision emits, which is what "declining leaves the
// game otherwise untouched" means (definition of done, item 4).
func (e *Engine) handleTriggerOptional(d *decision.Decision, in decision.Intent) {
	yes := false
	if opts := d.Chosen(in); len(opts) == 1 {
		yes = opts[0].Kind == "yes"
	}
	if pt, ok := e.takeAnsweredTrigger(d); ok && yes {
		e.pushTrigger(pt)
	}
	e.resumeTriggerDrain()
}
