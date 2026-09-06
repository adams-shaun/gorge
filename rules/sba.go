// Package rules: state-based actions (CR 704) and game end (CR 104.4).
//
// checkStateBased, checkGameOver and destroyLethalDamage all predate this
// file: Task 21 added destroyLethalDamage (combat depends on lethal-damage
// and zero-toughness destruction, and nothing in this codebase checked
// either before then), and stubs.go carried checkStateBased/checkGameOver
// as a placeholder ever since Task 3 -- a checkGameOver call and nothing
// else. This task deletes stubs.go outright and relocates all three here
// rather than losing them with it, and fills in the state-based actions M1
// actually specifies: a player at 0 or less life loses (CR 704.5a); an
// eliminated player's permanents leave the battlefield; the game ends in a
// win when exactly one seat remains and a draw (CR 104.4a) when none do --
// not, as stubs.go's own checkGameOver used to, an unconditional win for
// seat 0 when nobody survived. Drawing from an empty library (CR 704.5c) is
// already a loss as of Task 14 (effects.DrawFor emits PlayerLost directly at
// the moment of the failed draw, which checkStateBased's own permanent-
// removal pass below picks up on its next run regardless of how a player
// came to be Lost); this build has no poison-counter mechanic, so CR
// 704.5b/704.5h(poison) is not modelled and not a gap this task needs to
// close.
package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// maxSBAPasses bounds the fixed-point loop below. A game that needs more
// passes than this has a rules bug (a static-effect cycle where destroying
// one creature always makes another lethal, forever), and reporting a stuck
// game via a bounded loop is better than hanging the one goroutine running
// the whole match.
const maxSBAPasses = 32

// sbaAttempts is one checkStateBased call's memory of what its
// state-based-action passes have already tried, together with the size of
// the alive-player set they tried it under.
//
// objs is destroyLethalDamage's (Ruling T22-j), players is
// checkLoseConditions' removal sweep's (Ruling T22-n), and tokens is
// exileDeadTokens' own (Task 13 fix round 1, review finding "minor 1" --
// added after the fact, once the same failure shape checkLoseConditions and
// destroyLethalDamage document at length -- a replacement permanently
// blocking the move -- was pointed out as reachable here too, even though
// nothing in today's corpus is known to actually do it: with no memory of
// its own, a blocked attempt would re-match every single pass and burn the
// entire maxSBAPasses budget on one checkStateBased call, forever, rather
// than the one attempt per call every other pass here gets). All three are
// membership-only -- nothing ever ranges over any of them, so none can
// reach an event or the order of a decision's options. tokens is
// deliberately its OWN map, not shared with objs: a token that is ALSO a
// creature can be found lethal by destroyLethalDamage (which marks it in
// objs) and then, in the very same pass, need exileDeadTokens to move it
// again (graveyard -> exile) -- sharing one map would have objs's mark
// wrongly block exileDeadTokens from ever attempting an object destroy
// LethalDamage had already touched. alive is Ruling T22-p's re-arm
// watermark, described on checkStateBased below.
type sbaAttempts struct {
	objs    map[state.ObjID]bool
	tokens  map[state.ObjID]bool
	players map[state.PlayerID]bool
	alive   int
}

// rearm forgets every memory when the alive-player set has shrunk since it
// was last refreshed, so an attempt that a now-eliminated player's
// permanent blocked gets exactly one more chance under the smaller alive
// set -- and none at all while nobody dies. Ruling T22-p (fix round 4):
// this is called at each of the points that consult the memories, not once
// per pass, because the sweep loop, destroyLethalDamage and exileDeadTokens
// all run AFTER the pass's own eliminations are marked and must not be
// re-armed out from under an attempt they made under the very same alive
// set.
//
// Player.Lost is monotone (events/apply.go's PlayerLost case is its only
// non-test writer and only ever sets it true), so alive never grows and
// this can fire at most len(Players) times per checkStateBased call: the
// loop stays bounded by work, not just by maxSBAPasses.
//
// Ruling T22-q (fix round 5, comment only -- no behaviour changed): this
// key closes ONE of the two ways a blocker can go inert mid-call, and the
// round-4 wording of this comment claimed both. It does not. Read it as a
// partial fix with a known open half, because that is what it is.
//
//   - CLOSED, the elimination sub-case: applyReplacements discovers
//     replacements through forEachObject, which walks AliveFrom(0), so
//     every replacement an eliminated player owns stops being consulted the
//     instant they are marked Lost. That is what alive tracks, and the
//     reason a blocked attempt is worth exactly one retry when it changes.
//
//   - OPEN, the predicate-flip sub-case (N9, booked for the whole-branch
//     review rather than patched here): a replacement also goes inert by
//     ceasing to MATCH. replacementMatches evaluates ValidCard$ through
//     effects.MatchesSpecFrom against the MOVING object, reading its Tapped
//     flag, its P1P1 counters, its zone, its controller and its combat
//     flags -- and every one of those is writable by a registered
//     ReplaceWith$ primitive (effects' Tap, PutCounter, RemoveCounterAll).
//     "The predicate is matched against the moving object" is therefore not
//     a safety property, which is how round 4 read it; it is precisely what
//     lets the substitute effect that blocked an attempt flip the predicate
//     that blocked it. Measured by a probe re-implementing
//     replacementMatches from outside the package: 0 outstanding-SBA
//     decision points at 29fa00d, 6 at bd3c730 and 4 here, out of ~2 261 --
//     0.18%, self-healing on the next checkStateBased call, and never
//     touching the CR 704.5a life invariant.
//
// What a future author most needs from this comment: widening the key --
// which is what a zone gate on replacements, or a ValidCard$ predicate a
// static effect can flip, would force -- must NOT be done by re-arming on
// "some other state-based action actually succeeded", nor on any signal a
// removal produces. A death chain then buys one re-arm per link, and the
// blocked removal sweep goes from a flat 2 firings per Submit back to
// 2/3/7/22/61 for chain lengths 0/1/5/20/60 -- 29fa00d's pre-round-3
// amplification. That variant has now been built and measured twice, once
// in fix round 4 and once independently by the re-review, agreeing to the
// digit. TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain is the
// assertion that catches it being made again.
//
// One more thing this file has repeatedly got wrong in prose: the residue
// above is NOT bounded by maxSBAPasses and is NOT announced by the Note
// below. A replacement that blocks an attempt forever does not exhaust the
// budget at all -- the attempted sets make the pass report no new work, the
// loop goes stable, and it returns silently. That is correct behaviour (a
// permanent block is a legitimate rules-level fixed point; there is nothing
// to warn about), and it is measured: zero Note events over five Submits
// for both the blocked destruction and the blocked removal sweep. The Note
// covers the OTHER case, an unbounded cycle of genuinely new work, and has
// never seen this one.
func (a *sbaAttempts) rearm(alive int) {
	if alive >= a.alive {
		return
	}
	a.alive = alive
	clear(a.objs)
	clear(a.tokens)
	clear(a.players)
}

// checkStateBased applies state-based actions until none apply, which is
// what CR 704.3 requires: they are checked and rechecked, not just once.
// TestSBALoopsUntilStable is the regression test for the "not just once"
// half -- a lord dying can make a creature its own static was keeping alive
// newly lethal, and that second creature must not survive to see a second,
// separate checkStateBased call before it is caught.
//
// Ruling T22-h (fix round 1): "changed" must come from checkLoseConditions
// and destroyLethalDamage observing actual state after they emit, never
// from the fact that they emitted something. Engine.emit runs replacement
// effects first (rules/trigger.go), and a matching replacement can discard
// the very MoveZone this file proposes entirely -- a permanent this pass
// thinks it destroyed can simply still be sitting on the battlefield once
// the emit returns. The previous version of both functions reported
// "changed" from the event it built, not from where the object ended up,
// so a replacement that keeps a lethally-damaged permanent in play (a
// regeneration shield, "sacrifice a Clue instead", or the reviewer's own
// gain-1-life-instead reproduction) made every single pass "find" the same
// permanent lethal again: not a one-time miscount but the full
// maxSBAPasses budget, spent and reported as "changed", on every
// checkStateBased call for the rest of the match. See checkLoseConditions'
// removal sweep below for where that half of the fix lives; if
// maxSBAPasses is nonetheless exhausted (a genuine cycle, not a
// replacement -- e.g. a static that keeps making a different creature
// lethal forever), a Note event says so rather than the game quietly
// carrying on with state-based actions still outstanding.
//
// Ruling T22-j (fix round 2): destroyLethalDamage's own half of T22-h can
// UNDER-report. Reading "did it actually leave" is right for whether THAT
// object needs re-examining, but it threw away the fact that an attempt
// happened at all -- and checkLoseConditions runs before destroyLethalDamage
// within one pass, so a replacement whose substitute effect changes
// SBA-relevant state elsewhere (the reviewer's reproduction: `ReplaceWith$`
// a life-loss instead of a move) only becomes visible to checkLoseConditions
// on a LATER pass. Reporting "the shield didn't move, so nothing changed"
// denied the loop that later pass, so a player it drove to 0 life could sit
// there, un-Lost, handed a decision. attempted (below) is the fix: a set of
// object IDs destroyLethalDamage has already tried this call, threaded
// through every pass of this loop rather than rebuilt per pass, so an
// object already attempted is skipped on later passes (preserving T22-h's
// bound: one attempt per object per checkStateBased call, not thirty-two),
// while "changed" now means "attempted something NEW this pass" -- which is
// true on pass 1 regardless of outcome, giving pass 2 the chance to see
// whatever the replacement's own effect changed.
//
// Ruling T22-n (fix round 3): checkLoseConditions' own removal sweep had
// the identical shape T22-j fixed, one loop over -- "changed" came from
// whether a player's battlefield zone actually shrank, so a replacement
// that blocks the exile but changes SBA-relevant state elsewhere (some
// other player's life total) was invisible to this sweep and denied the
// later pass that would have caught it, same as T22-j's shield. swept
// (below) is destroyLethalDamage's attempted, one loop over: a set of
// PlayerIDs already swept this call, so "changed" means "swept someone new
// this pass" rather than "someone's battlefield actually shrank" -- the
// same discipline, applied to the same function's second loop, so the two
// no longer disagree about what "changed" means.
//
// Ruling T22-p (fix round 4): the discipline T22-j and T22-n share can
// under-COMPLETE, which is the mirror image of the under-report T22-j
// fixed. Whether an attempt succeeds is decided by the replacement effects
// that apply to it, and applyReplacements only ever looks at objects
// controlled by a player who is still alive (trigger.go's forEachObject
// walks AliveFrom(0)) -- so a blocker stops blocking the instant its
// controller is eliminated, and the blocked attempt's own substitute
// effect is routinely what eliminates them: a creature with lethal damage
// whose destruction a Creature.Other guardian replaces with "the
// guardian's controller loses 1 life", with that controller one life from
// zero. The attempt that failed under the old alive set would succeed
// under the new one, but the object was already in objs, so nothing
// retried it and the decision went out with a state-based action
// outstanding and nothing left on the board able to prevent it.
//
// tried.rearm is the fix, and the alive-player count is deliberately the
// ONLY thing that re-arms. It is the input that decides whether a
// replacement is consulted at all, and it is monotone, so both sets are
// cleared at most len(Players) times per call. It is NOT the only way a
// blocker can go stale -- a replacement that stops matching goes inert just
// as thoroughly as one whose controller died, and that half is still open;
// see Ruling T22-q on rearm above before widening this key. Re-arming on the
// wider "some other state-based action actually succeeded" instead would
// re-arm a blocked attempt once per link of an entirely unrelated death
// chain, which is the per-pass amplification T22-h exists to prevent. Built
// and measured rather than assumed: on
// TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain's own board,
// re-arming whenever the battlefield population actually shrank takes the
// blocked sweep from a flat 2 firings per Submit back to 2/3/7/22/61 for
// chain lengths 0/1/5/20/60 -- 29fa00d's pre-round-3 figures exactly. A
// repeatedly-blocked attempt with nobody dying is therefore still tried
// exactly once per checkStateBased call, as T22-h requires.
func (e *Engine) checkStateBased() {
	stable := false
	tried := &sbaAttempts{
		objs:    map[state.ObjID]bool{},
		tokens:  map[state.ObjID]bool{},
		players: map[state.PlayerID]bool{},
		alive:   e.G.AliveCount(),
	}
	for pass := 0; pass < maxSBAPasses; pass++ {
		changed := e.checkLoseConditions(tried)
		if e.destroyLethalDamage(tried) {
			changed = true
		}
		if e.exileDeadTokens(tried) {
			changed = true
		}
		if e.attachmentSBAs() {
			changed = true
		}
		if !changed {
			stable = true
			break
		}
	}
	if !stable {
		e.emit(events.Event{Kind: events.Note,
			Text: "state-based actions did not reach a fixed point within the pass budget"})
	}
	e.checkGameOver()
	e.releasePendingDecisionOfDepartedPlayer()
}

// checkLoseConditions applies CR 704.5a (life 0 or less) to every player not
// already marked Lost, in fixed seat order (Players is a slice walked by
// index, never a map, so simultaneous eliminations always emit in the same
// order run to run). It then separately sweeps every Lost player -- from
// life loss just above, from an empty-library draw (effects.DrawFor, which
// emits PlayerLost directly, independent of this loop, at the moment the
// draw fails), or from any future cause -- and removes whatever they still
// control from the battlefield, which is CR 800.4a's own cleanup for a
// player who has left the game. Doing this generically, keyed only on
// p.Lost rather than on which branch just set it, is what makes a decked-out
// player's board get cleaned up exactly the same way a player who hit 0
// life does, through the one path, rather than needing its own copy of the
// same logic.
//
// Reports whether anything changed, which is what checkStateBased's own
// pass loop uses to decide whether to run again.
//
// Ruling T22-h (fix round 1), superseded by T22-n (fix round 3, see
// checkStateBased): the first fix measured the removal sweep's own outcome
// (the zone's length before versus after removePermanents) rather than
// reporting "changed" just because the zone was non-empty going in -- right
// for bounding a replacement's amplification, but, like destroyLethal-
// Damage's own T22-h before T22-j, it threw away the fact that an attempt
// happened at all. tried.players is this loop's memory of that, exactly
// mirroring destroyLethalDamage's tried.objs: a player already swept this
// call is skipped entirely on later passes (one sweep attempt per player
// per checkStateBased call, never re-run for no reason), while "changed"
// now means "swept a player NOT already swept this pass" -- true the moment
// a new sweep is attempted, regardless of whether removePermanents' own
// MoveZone events actually moved anything, so a later pass still gets to
// see whatever a blocking replacement's own substitute effect changed
// elsewhere.
//
// Ruling T22-p (fix round 4): the rearm sits BETWEEN the two loops, not at
// the top of the function. The elimination this pass's own first loop just
// marked is exactly the one that can have made a previous pass's blocked
// sweep worth retrying -- and doing it here rather than at the top of the
// next pass also means a player whose ward has just gone inert is swept in
// the same checkStateBased call, not the one after. Sweeping in the second
// loop then records the attempt under the alive set it was actually made
// under, so this pass's own new sweeps are not re-armed by this pass's own
// eliminations.
func (e *Engine) checkLoseConditions(tried *sbaAttempts) bool {
	changed := false
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if !p.Lost && p.Life <= 0 {
			e.emit(events.Event{Kind: events.PlayerLost, Player: p.ID, Text: "life total is 0 or less"})
			changed = true
		}
	}
	tried.rearm(e.G.AliveCount())
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if !p.Lost || tried.players[p.ID] {
			continue
		}
		if len(e.G.Zone(state.ZBattlefield, p.ID)) == 0 {
			continue
		}
		tried.players[p.ID] = true
		e.removePermanents(p.ID)
		changed = true
	}
	return changed
}

// removePermanents moves an eliminated player's permanents out of the game.
// Exile is the closest zone this build has to CR 800.4a's "leaves the
// game" -- the same approximation resolveTop already makes for a resolved
// ability with no printed card (CR 608.2m).
func (e *Engine) removePermanents(p state.PlayerID) {
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZBattlefield, p)...)
	for _, id := range ids {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZExile, Text: "controller left the game"})
	}
}

// casualty is one creature destroyLethalDamage has found lethal, together
// with which CR 704.5 clause is why -- Ruling T22-i (fix round 1): 704.5f
// (toughness <= 0) and 704.5g (lethal damage/deathtouch) are rules-distinct
// (only one of them is actually "destruction", so only one of them is
// something Indestructible or a replacement effect keyed on Destroy could
// ever apply to), and the log should be able to tell them apart even though
// Text carries no rules weight of its own.
type casualty struct {
	id   state.ObjID
	text string
}

// destroyLethalDamage performs the two CR 704.5 state-based actions combat
// damage depends on: a creature with toughness 0 or less (704.5f) is
// destroyed regardless of damage or Indestructible, and a creature with
// damage marked on it greater than or equal to its toughness, or with any
// damage at all from a source with Deathtouch (704.5g, via the Deathtouched
// counter damageStep marks), is destroyed unless Indestructible.
//
// This is general state-based-action logic, not combat-specific -- a
// creature killed by an ordinary DealDamage spell dies here exactly the same
// way one killed in combat does -- which is why it lives here rather than in
// combat.go, where Task 21 first added it.
//
// The Indestructible check reads the same way effDestroy/effDestroyAll
// (effects/zone.go) already do; it is not one of the eight keywords Task 21
// registers as implemented, so a card that actually carries it still reads
// as unsupported for deck-routing purposes, but a creature that is
// Indestructible for some other already-working reason should not
// spuriously die to lethal damage.
//
// Every candidate is found in one pass before any of them actually leaves
// (dead is collected, then destroyed), so one creature's departure this same
// pass can never retroactively change whether another one here counts as
// lethal, matching CR 704.3's "state-based actions are performed
// simultaneously" for one round of this check.
//
// Ruling T22-h (fix round 1), superseded by T22-j (fix round 2, see
// checkStateBased): the first fix reported whether a candidate actually
// left the battlefield, not whether a MoveZone was emitted for it --
// correct for bounding the amplification a replacement causes, but it threw
// away the fact that an attempt happened at all. tried.objs is this
// function's memory of that, shared across every pass of one
// checkStateBased call (constructed once there, passed down here each
// pass, never rebuilt per pass): an object already in it is skipped
// entirely -- not re-examined, not re-emitted -- and "changed" now means
// "found and attempted an object NOT already tried this pass", which is
// true exactly once per object per call regardless of whether the
// attempt actually moved it. That is what lets a later pass's
// checkLoseConditions see whatever a replacement's own substitute effect
// changed (T22-j's fix), while still bounding this function to one attempt
// per object per checkStateBased call (T22-h's fix, preserved): the object
// will not be found lethal-and-new again until the NEXT checkStateBased
// call, whether or not it actually left -- unless a player has been
// eliminated in the meantime, which is Ruling T22-p's one re-arm and the
// reason for the rearm call below (an elimination during THIS function's
// own emits, from a substitute effect that decks a player out, is picked up
// by the next pass's rearm rather than mid-loop).
func (e *Engine) destroyLethalDamage(tried *sbaAttempts) bool {
	tried.rearm(e.G.AliveCount())
	var dead []casualty
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if tried.objs[id] {
				continue
			}
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			f := o.Face()
			if f == nil || !f.IsCreature() {
				continue
			}
			if e.Toughness(id) <= 0 {
				dead = append(dead, casualty{id, "toughness <= 0"})
				continue
			}
			if o.Damage <= 0 {
				continue
			}
			if o.Damage < e.Toughness(id) && o.Counter("Deathtouched") == 0 {
				continue
			}
			if e.HasKeyword(id, "Indestructible") {
				continue
			}
			dead = append(dead, casualty{id, "lethal damage"})
		}
	}
	for _, c := range dead {
		tried.objs[c.id] = true
		e.emit(events.Event{Kind: events.MoveZone, Obj: c.id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: c.text})
	}
	return len(dead) > 0
}

// tokenCasualty is a token exileDeadTokens found to have left the
// battlefield, together with the zone it left FROM -- captured before the
// move, mirroring casualty above and the same real-zone-over-claimed-zone
// discipline events.Move applies to the removal itself.
type tokenCasualty struct {
	id   state.ObjID
	from state.Zone
}

// exileDeadTokens is CR 111.7: a token ceases to exist the instant it
// leaves the battlefield. This build already parks every Ephemeral object
// in exile rather than deleting it outright (state.Object.Ephemeral's own
// doc), so the state-based action is a plain zone check -- any token whose
// CURRENT zone is neither the battlefield nor the stack nor already exile
// is moved to exile, Text "ceased to exist". The stack is excluded for the
// same reason Ephemeral's own IsCopy half exists: a token copy of a spell
// or ability legitimately sits there without being a permanent yet (Task
// 13 does not implement CopySpellAbility, so no card can produce this
// today, but the exclusion costs nothing and matches the brief exactly).
//
// Task 13 fix round 1 (review finding "minor 1"): this now takes the same
// tried memory checkLoseConditions and destroyLethalDamage do, in its own
// tried.tokens map (see sbaAttempts' own doc for why it cannot share
// tried.objs). The original version relied only on the zone update itself
// -- moving a token to exile changes its own zone to the value the check
// excludes, so it naturally stops matching -- which is correct for the
// ordinary case and remains exactly how a SUCCESSFUL move retires itself
// here. What it did not bound was a replacement PERMANENTLY blocking the
// move (some "cards can't leave your graveyard" effect intercepting the
// move FROM graveyard, say): with no memory of the attempt, that token
// would be rediscovered and re-attempted on every single one of the 32
// passes in the budget, every checkStateBased call, forever -- exactly the
// amplification T22-h/T22-j/T22-n exist to prevent for the other two
// passes. tried.tokens closes the same gap here: an object already
// attempted this call is skipped on later passes, "changed" means
// "attempted something NEW this pass" (true the moment a new attempt
// happens, regardless of whether the move actually lands, mirroring
// destroyLethalDamage's own convention), and tried.rearm at the top gives a
// blocked attempt one more chance the instant the alive-player set shrinks,
// the same as its siblings.
//
// Walks e.G.Objs by index -- the dense arena, never a map -- so multiple
// tokens dying at once are exiled in a fixed, reproducible order.
func (e *Engine) exileDeadTokens(tried *sbaAttempts) bool {
	tried.rearm(e.G.AliveCount())
	var dead []tokenCasualty
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if tried.tokens[o.ID] {
			continue
		}
		if o.IsToken && o.Zone != state.ZBattlefield && o.Zone != state.ZStack && o.Zone != state.ZExile {
			dead = append(dead, tokenCasualty{o.ID, o.Zone})
		}
	}
	for _, c := range dead {
		tried.tokens[c.id] = true
		e.emit(events.Event{Kind: events.MoveZone, Obj: c.id,
			From: c.from, To: state.ZExile, Text: "ceased to exist"})
	}
	return len(dead) > 0
}

// checkGameOver ends the game once at most one seat remains (CR 104.2a's
// "the last player left in the game wins", CR 104.4a's draw when nobody is).
// A concession is a way to be Lost (M2d-3 emits the same PlayerLost the
// life-loss path does), so a conceded game ends here exactly like one whose
// losers lost to life or the library.
// Ruling T22-b: this used to default the winner to seat 0 whenever nobody
// survived -- PlayerID's zero value is a real seat, so an unconditional
// `w := state.PlayerID(0)` silently crowned it every time elimination was
// simultaneous. There is no winner in that case; Amount: 1 tells Apply's
// GameOver case to record a draw (Game.Draw) instead of a winner.
func (e *Engine) checkGameOver() {
	if e.G.Over {
		return
	}
	alive := e.G.AliveFrom(0)
	if len(alive) > 1 {
		return
	}
	if len(alive) == 1 {
		w := alive[0]
		e.emit(events.Event{Kind: events.GameOver, Player: w, Text: e.G.Players[w].Name})
	} else {
		e.emit(events.Event{Kind: events.GameOver, Amount: 1, Text: "draw"})
	}
	e.pending = nil
}

// releasePendingDecisionOfDepartedPlayer keeps a decision asked of a player
// who has since left the game from stranding the match. Only that player may
// answer it, Advance does nothing at all while e.pending is set, and with
// three or more seats the elimination does not end the game -- so without
// this the one goroutine running the match waits forever on an answer that
// can never come. With two seats checkGameOver above has already cleared
// e.pending, so this is the three-or-more case.
//
// Fix round 1, review finding F2: this used to fire only for Task 27's two
// trigger kinds, on the stated grounds that they were the only decisions
// asked from inside handle -- Submit runs handle, then this, then Advance, so
// a decision asked from inside handle is the only one state-based actions run
// underneath. That premise was FALSE, and the review traced the
// counterexample: handlePriority (legal.go) -> castSpell (stack.go) ->
// askTarget asks a KTarget decision from inside handle too. It is now
// deliberately keyed on nothing but "the player cannot answer", because
// enumerating which decisions can reach this state is precisely the reasoning
// that was wrong the first time.
//
// The continuation is resumeTriggerDrain for every kind, and it is the right
// one for all of them: state-based actions to a fixed point, then any queued
// triggers onto the stack, then priority to the next player who can actually
// take it. Re-entering priorityRound instead of resumeTriggerDrain would put
// a second checkStateBased call on priorityRound's own path, where step() has
// already reached the same fixed point -- giving a replacement-blocked
// state-based action a second attempt per step and moving sba.go's own
// measured firing counts (Ruling T22-p; see resumeTriggerDrain's header in
// turn.go). (Ruling T28-b: this used to say re-entering priorityRound "would
// re-run the draw step's draw" -- true before Task 28 moved the draw out of
// priorityRound entirely; the reason above is what actually survives that
// move.)
//
// This cannot loop. dropDepartedTriggers has already discarded the departed
// player's own triggers (CR 800.4a); an optional trigger whose DECIDER
// departed but whose controller lives is declined rather than re-asked; and
// e.pending is cleared before the resume, so the nested checkStateBased that
// resume performs finds nothing left to release.
func (e *Engine) releasePendingDecisionOfDepartedPlayer() {
	d := e.pending
	if d == nil || e.G.Over {
		return
	}
	if int(d.Player) >= len(e.G.Players) || !e.G.Players[d.Player].Lost {
		return
	}
	e.pending = nil
	e.resumeTriggerDrain()
}
