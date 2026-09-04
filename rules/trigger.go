// Triggered abilities (T: lines) and replacement effects (R: lines). Every
// state mutation still goes through events.Emit -- engine.go's emit wraps it
// with applyReplacements ahead of logging and checkTriggers behind it, so
// this file's job is entirely about *deciding* what fires and *ordering*
// what goes on the stack, never about writing state directly.
package rules

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// pendingTrigger is a matched trigger waiting to be placed on the stack.
//
// Idx is Source's own Face().Triggers index for the matched T: line. SA is
// kept alongside it (the brief's own interface names both Source and SA)
// even though putTriggersOnStack ends up re-deriving the ability from
// Source+Idx rather than using SA directly: Ruling T20-a means the eventual
// stack object is created inside events.Apply, which cannot carry a raw
// *cards.SA pointer through the log -- only data (an ObjID and a small
// index) that lets Apply find the same *cards.SA a live engine already
// holds.
type pendingTrigger struct {
	Source     state.ObjID
	Controller state.PlayerID
	Idx        int
	SA         *cards.SA
	Ctx        effects.Ctx
}

// triggerKey identifies one T: line: the object that carries it, plus that
// object's own Triggers index (a card can have more than one). Used both for
// the cascade bound and for DamageDealtOnce's once-per-turn gate.
type triggerKey struct {
	Source state.ObjID
	Idx    int
}

// maxTriggerFires bounds how many times a single (source, trigger index)
// pair may queue a pending trigger over the life of a match. Ordinary play
// stays far below this -- even a trigger that fires every turn for a hundred
// turns is two orders of magnitude under it. The pathological case this
// guards is "a trigger that fires in response to its own effect": nothing in
// this build can hang or overflow the stack resolving any *one* ability
// (Resolve's own maxChain bounds a sub-ability chain, and every stack
// resolution requires a fresh external Submit round-trip -- resolveTop is
// only ever called from handlePriority's "pass" case), but nothing stops a
// naive auto-pass driver from sustaining pop-one/push-one-again forever
// across many such round-trips, tying up the match's one goroutine
// indefinitely. This cap is what makes that terminate: once a specific
// trigger has fired this many times, it simply stops matching, so the loop
// runs dry instead of running forever.
const maxTriggerFires = 256

// forEachObject walks every object currently in the game exactly once, in a
// fixed, deterministic order: living seats in ascending order from seat 0,
// then zone in Zone's own declared order (library, hand, battlefield,
// graveyard, exile, stack), then position within that zone's slice.
// checkTriggers and applyReplacements both need this same walk for their own
// discovery to be deterministic, so it is factored out here rather than
// duplicated. Game.Zone ignores its player argument for ZStack (the stack is
// shared across controllers, not per-seat), so that zone is visited only
// once, on the first living seat, rather than once per living player.
func (e *Engine) forEachObject(fn func(id state.ObjID)) {
	for si, p := range e.G.AliveFrom(0) {
		for z := state.ZLibrary; z <= state.ZStack; z++ {
			if z == state.ZStack && si != 0 {
				continue
			}
			for _, id := range append([]state.ObjID(nil), e.G.Zone(z, p)...) {
				fn(id)
			}
		}
	}
}

// controllerOf is a nil-safe Object.Controller read: a nonexistent ObjID
// (stale data, a malformed trigger source) degrades to seat 0 rather than
// panicking.
func (e *Engine) controllerOf(id state.ObjID) state.PlayerID {
	if o := e.G.Obj(id); o != nil {
		return o.Controller
	}
	return 0
}

// checkTriggers is called from emit after every event. It walks every
// object once (forEachObject) and, for each cards.Trigger on that object's
// face, asks triggerMatches whether this event satisfies it. A match
// appends to e.pendingTriggers with a context whose Remembered holds the
// triggering object; putTriggersOnStack later drains that queue onto the
// stack in APNAP order.
func (e *Engine) checkTriggers(ev events.Event) {
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			// Ruling F3: an ability or token object has no Face and
			// therefore no printed Triggers to check -- only real cards
			// carry triggered abilities.
			return
		}
		for ti, t := range f.Triggers {
			if !e.triggerMatches(t, id, ev) {
				continue
			}
			key := triggerKey{Source: id, Idx: ti}
			if e.triggerFireCount == nil {
				e.triggerFireCount = map[triggerKey]int32{}
			}
			if e.triggerFireCount[key] >= maxTriggerFires {
				continue // cascade bound: see maxTriggerFires.
			}
			if t.Mode == "DamageDealtOnce" {
				if e.damageOnceFired == nil {
					e.damageOnceFired = map[triggerKey]int32{}
				}
				if e.damageOnceFired[key] == e.G.Turn {
					continue // already fired this turn.
				}
				e.damageOnceFired[key] = e.G.Turn
			}
			e.triggerFireCount[key]++
			if t.Effect == nil {
				// Execute$ named an SVar this face never defined (or one
				// that failed to parse): the trigger matched, but there is
				// nothing to run.
				continue
			}
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
				Source:     id,
				Controller: o.Controller,
				Idx:        ti,
				SA:         t.Effect,
				Ctx: effects.Ctx{
					Source:     id,
					Controller: o.Controller,
					Remembered: triggerRemembered(ev, id),
				},
			})
		}
	})
}

// triggerRemembered is what a matched trigger's Ctx.Remembered holds: the
// object the triggering event was actually about (the card that changed
// zones, was cast, or is being targeted), or -- for an event with no object
// of its own, such as a step change, or a player-only Damage event -- the
// trigger's own source. None of the eight M1 modes' Execute$ abilities in
// the acceptance deck actually reads Remembered (they use Defined$
// Self/You), so this is deliberately one simple, general rule rather than a
// mode-specific one.
func triggerRemembered(ev events.Event, source state.ObjID) []state.Target {
	if ev.Obj != 0 {
		return []state.Target{{Obj: ev.Obj}}
	}
	return []state.Target{{Obj: source}}
}

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
// priorityRound, which would run the draw step's draw a second time.
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
	ids := make([]state.ObjID, 0, len(pt.Ctx.Remembered))
	for _, tgt := range pt.Ctx.Remembered {
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

// triggerMatches decides whether one cards.Trigger fires for ev. zoneGate
// (TriggerZones$) applies uniformly first; the rest is mode-specific.
func (e *Engine) triggerMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if !e.zoneGate(t, source, ev) {
		return false
	}
	switch t.Mode {
	case "ChangesZone":
		return e.zoneChangeMatches(t, source, ev)
	case "SpellCast":
		return e.spellCastMatches(t, source, ev)
	case "Attacks":
		return e.attacksMatches(t, source, ev)
	case "DamageDone", "DamageDealtOnce":
		return e.damageMatches(t, source, ev)
	case "BecomesTarget":
		return e.becomesTargetMatches(t, source, ev)
	case "LandPlayed":
		return e.landPlayedMatches(t, source, ev)
	case "Phase":
		return e.phaseMatches(t, source, ev)
	}
	// An unimplemented Mode$ never fires -- a malformed or unsupported
	// trigger doing nothing is the safe failure mode (Ruling: see
	// filter.go's own "unknown predicate never matches" precedent).
	return false
}

// zoneGate implements TriggerZones$: a trigger only fires while its source is
// in one of the listed zones. The default is the battlefield, which is why an
// enchantment's upkeep trigger stops when it is destroyed.
//
// checkTriggers calls this after the event has already been folded into
// state (emit logs before it checks triggers), so o.Zone alone only ever
// reflects the zone the object is in *now*. That is correct for an
// entering-the-zone trigger (Snapcaster's ETB: o.Zone is already
// Battlefield by the time this runs) but wrong for a leaving-the-zone one --
// a plain "dies" trigger (Origin$ Battlefield, Destination$ Graveyard,
// ValidCard$ Card.Self, default TriggerZones$ Battlefield) would never see
// its own source "in" the battlefield, because by the time checkTriggers
// runs the move has already happened and o.Zone reads Graveyard. CR 603.10's
// full "look back in time" is not modeled, but the one case Task 20's own
// ChangesZone mode needs it for is narrow and self-contained: when the event
// under test is itself the zone change of this trigger's own source (source
// == ev.Obj), the zone it was in immediately before (ev.From) counts as well
// as the zone it is in now, so both an ETB and a dies trigger with the
// ordinary default work from the same rule.
func (e *Engine) zoneGate(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	spec := t.Params["TriggerZones"]
	if spec == "" {
		spec = "Battlefield"
	}
	zones := [2]state.Zone{o.Zone, o.Zone}
	n := 1
	if source == ev.Obj && ev.Obj != 0 &&
		(ev.Kind == events.MoveZone || ev.Kind == events.Draw || ev.Kind == events.PutOnStack) {
		zones[1] = ev.From
		n = 2
	}
	for _, z := range strings.Split(spec, ",") {
		want := effects.ParseZone(strings.TrimSpace(z))
		for _, zone := range zones[:n] {
			if want == zone {
				return true
			}
		}
	}
	return false
}

// zoneChangeMatches implements Mode$ ChangesZone.
func (e *Engine) zoneChangeMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.MoveZone && ev.Kind != events.Draw && ev.Kind != events.PutOnStack {
		return false
	}
	if o, ok := t.Params["Origin"]; ok && o != "Any" && effects.ParseZone(o) != ev.From {
		return false
	}
	if d, ok := t.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		return effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source)
	}
	return true
}

// spellCastMatches implements Mode$ SpellCast: ValidCard$ and
// ValidActivatingPlayer$ against a PutOnStack event.
func (e *Engine) spellCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.PutOnStack {
		return false
	}
	// Casting a spell means an actual card entering the stack. This build
	// also uses PutOnStack-shaped Move()s for nothing else today (triggered
	// abilities go on the stack via a dedicated TriggerPush event --
	// putTriggersOnStack, above, and events.Apply's TriggerPush case), but a
	// Face()-less object could otherwise satisfy a bare "Any"/"Spell"
	// ValidCard$ regardless (matchesBase's Spell/Any cases don't consult
	// Face()), so this guard holds regardless of how a future ability-object
	// path might reach here. Ruling F3.
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidCard"]; ok {
		if !effects.MatchesSpecFrom(e.G, v, ev.Obj, ctrl, source) {
			return false
		}
	}
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	return true
}

// attacksMatches implements Mode$ Attacks against a DeclareAttackers event.
// DeclareAttackers carries every attacker declared this combat in one event
// (IDs), so -- like every other mode here -- this fires at most once per
// event rather than once per qualifying attacker: a documented M1
// simplification, not a missed multi-attacker case.
func (e *Engine) attacksMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.DeclareAttackers {
		return false
	}
	spec, ok := t.Params["ValidCard"]
	if !ok {
		for _, id := range ev.IDs {
			if id == source {
				return true
			}
		}
		return false
	}
	ctrl := e.controllerOf(source)
	for _, id := range ev.IDs {
		if effects.MatchesSpecFrom(e.G, spec, id, ctrl, source) {
			return true
		}
	}
	return false
}

// damageSource identifies who dealt a just-emitted Damage event, for
// ValidSource$ matching. events.Event carries no explicit source field for
// Damage -- every Damage event this build emits (effects/damage.go's
// DealDamage/DamageAll) comes from a primitive running inside Resolve,
// called only from resolveTop while the resolving spell or ability is still
// the top of the stack (resolveTop pops it only after Resolve returns), so
// the current stack top is that source for every code path this build has
// today. A future combat-damage implementation, or any Damage emission
// outside ability resolution, would need Event to carry an explicit source
// instead of relying on this.
func (e *Engine) damageSource() state.ObjID {
	if len(e.G.Stack) == 0 {
		return 0
	}
	return e.G.Stack[len(e.G.Stack)-1]
}

// damageMatches implements Mode$ DamageDone and DamageDealtOnce (the once-
// per-turn gate itself lives in checkTriggers, alongside the cascade bound;
// this is purely the per-event parameter match, shared by both modes).
func (e *Engine) damageMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Damage {
		return false
	}
	if strings.EqualFold(t.Params["CombatDamage"], "True") {
		// M1 has no combat-damage implementation (dealCombatDamage is a
		// no-op stub, Task 21/22's territory) and Event carries nothing to
		// distinguish combat from noncombat damage, so a trigger that
		// insists on CombatDamage$ True can never fire yet.
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidSource"]; ok {
		src := e.damageSource()
		if src == 0 || !effects.MatchesSpecFrom(e.G, v, src, ctrl, source) {
			return false
		}
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		if ev.Obj != 0 {
			if !effects.MatchesSpecFrom(e.G, v, ev.Obj, ctrl, source) {
				return false
			}
		} else if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	return true
}

// becomesTargetMatches implements Mode$ BecomesTarget: the trigger's own
// source must be among the chosen targets recorded by a TargetsChosen event
// (rules.handleTarget -- "the target decision being answered").
func (e *Engine) becomesTargetMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	targeted := false
	for _, id := range ev.IDs {
		if id == source {
			targeted = true
			break
		}
	}
	if !targeted {
		return false
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		return effects.MatchesSpecFrom(e.G, v, source, e.controllerOf(source), source)
	}
	return true
}

// landPlayedMatches implements Mode$ LandPlayed. This fires on the MoveZone
// hand->battlefield of a land specifically -- not on the separate LandPlayed
// event legal.go's "play_land" case also emits, which carries only a Player
// (no Obj), and so has nothing ValidCard$ could ever match against.
func (e *Engine) landPlayedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.MoveZone || ev.From != state.ZHand || ev.To != state.ZBattlefield {
		return false
	}
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil || !obj.Face().IsLand() {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		return effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source)
	}
	return true
}

// phaseMatches implements Mode$ Phase.
func (e *Engine) phaseMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StepChange {
		return false
	}
	want := strings.ToLower(t.Params["Phase"])
	if want != "" && !strings.Contains(ev.Step.String(), want) {
		return false
	}
	if v, ok := t.Params["ValidPlayer"]; ok {
		// StepChange carries no Player of its own -- a step always belongs
		// to the current active player.
		if !effects.MatchesPlayerSpec(e.G, v, e.G.Active, e.controllerOf(source)) {
			return false
		}
	}
	return true
}

// applyReplacements is called from emit before the event is logged. The
// single M1 replacement event is "Moved" (R:Event$ Moved), which applies to
// a MoveZone event and honours Origin$, Destination$, ValidCard$ and
// ReplaceWith$.
//
// At most one matching replacement applies, in forEachObject's own
// deterministic scan order: CR 616's full multi-replacement, player-chooses-
// order algorithm is not modeled, which is an M1 simplification, not an
// oversight. A match resolves ReplaceWith$'s ability (which itself emits
// whatever events its own primitives call for -- Ruling: applyingReplacement
// is already true by then, so those nested emits skip this check entirely
// rather than potentially replacing themselves forever) and reports true, so
// emit discards the original event instead of logging it.
func (e *Engine) applyReplacements(ev events.Event) (events.Event, bool) {
	if ev.Kind != events.MoveZone {
		return ev, false
	}
	var matchID state.ObjID
	var matchRepl *cards.Repl
	e.forEachObject(func(id state.ObjID) {
		if matchRepl != nil {
			return
		}
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			return
		}
		for i := range f.Repls {
			if e.replacementMatches(f.Repls[i], id, ev) {
				matchID, matchRepl = id, &f.Repls[i]
				return
			}
		}
	})
	if matchRepl == nil || matchRepl.With == nil {
		return ev, false
	}

	o := e.G.Obj(matchID)
	ctx := &effects.Ctx{Source: matchID, Controller: o.Controller,
		Remembered: []state.Target{{Obj: ev.Obj}}}
	if f := o.Face(); f != nil {
		effects.SetSVars(ctx, f.SVars)
	}
	e.applyingReplacement = true
	effects.Resolve(e, ctx, matchRepl.With)
	e.applyingReplacement = false
	return ev, true
}

// replacementMatches implements R:Event$ Moved's own Origin$/Destination$/
// ValidCard$ parameters -- the same shape as zoneChangeMatches, for a
// replacement instead of a trigger.
func (e *Engine) replacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	if r.Event != "Moved" || ev.Kind != events.MoveZone {
		return false
	}
	if o, ok := r.Params["Origin"]; ok && o != "Any" && effects.ParseZone(o) != ev.From {
		return false
	}
	if d, ok := r.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
		return false
	}
	if v, ok := r.Params["ValidCard"]; ok {
		return effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source)
	}
	return true
}

func init() {
	effects.RegisterNonAPI(
		"trig:ChangesZone", "trig:SpellCast", "trig:Attacks", "trig:DamageDone",
		"trig:DamageDealtOnce", "trig:BecomesTarget", "trig:LandPlayed", "trig:Phase",
		"repl:Moved",
	)
}
