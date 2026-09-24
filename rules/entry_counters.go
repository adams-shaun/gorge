// entry_counters.go places the entry-characteristic counters a permanent
// enters the battlefield with (starting loyalty, Riot/Unleash's election, a
// Saga's lore counter, a Battle's defense counters) through REAL
// CounterChange events, so the CR 614 AddCounter replacement class and the
// CantPutCounter prohibition see them exactly like any other placement. The
// arithmetic lives in events.EntryCounterGrants. The final placements are
// prepared against the would-enter board, then folded with the move itself.
//
// Task agent-20260923T084704Z-b2386c25 adds the last leg: when a grant's
// AddCounter competition does not commute and the affected player must make
// CR 616.1's order choice, the ENTRY itself is staged behind the answer
// (entryCounterStage) instead of folding first and adjusting after. The
// staging is detected in Engine.emit's pre-pass, before any of the fold's
// observers run, so no observer -- an ETB trigger, a state-based action, a
// chapter queue -- ever sees the un-replaced entry. The completed stage is
// consumed by foldEntryMove, which folds the move with the finalized amounts
// in its Pairs payload exactly as an uncontested entry does.
package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// entryCounterStage parks a battlefield entry whose entry-characteristic
// counter grant is awaiting a CR 616.1 replacement-order answer. The move
// has NOT folded: no observer can see the un-replaced entry while the
// competition is outstanding. Resuming applies the answered bodies first (on
// an isolated preview of the post-entry board, the only state the zone
// filters match on), settles the remaining grants, then re-emits the move
// through the ordinary emit -- whose fold consumes the completed stage via
// entryStageDone.
type entryCounterStage struct {
	move     events.Event               // the staged entry as the pose received it
	grants   []events.EntryCounterGrant // the origin-zone snapshot
	placed   []events.EntryCounterGrant // grants finalized so far, in grant order
	counter  events.Event               // the parked grant's counter event
	cands    []replMatch                // its competition
	applied  []replMatch                // bodies already applied, in answer order
	player   state.PlayerID             // the asked player
	inRes    bool                       // the pose's in-resolution provenance
	idx      int                        // index into grants of the parked grant
	complete bool                       // every grant finalized; the fold may consume
}

// sameEntryMove reports whether ev is (a re-drive of) the staged move. The
// identifying fields are the ones a re-driven entry keeps: a re-emit may
// recompute markers, never origin, destination or object.
func (st *entryCounterStage) sameEntryMove(ev events.Event) bool {
	return st.move.Kind == ev.Kind && st.move.Obj == ev.Obj &&
		st.move.From == ev.From && st.move.To == ev.To
}

// entryCounterGrants snapshots the intrinsic counters of a battlefield
// entry, including tokens minted directly onto the battlefield. A MoveZone
// reads its object in the origin zone (elections and compleated payment);
// token mints read the card/face their Apply case will instantiate. A
// battlefield->battlefield stay is not a new object and grants nothing.
func (e *Engine) entryCounterGrants(ev events.Event) []events.EntryCounterGrant {
	switch ev.Kind {
	case events.MoveZone:
		if ev.To != state.ZBattlefield {
			return nil
		}
		o := e.G.Obj(ev.Obj)
		if o == nil || o.Zone == state.ZBattlefield {
			return nil
		}
		return events.EntryCounterGrants(o, events.IsFaceDownEntry(ev.Counter))
	case events.TokenCreate:
		return events.EntryCounterGrants(e.tokenSnapshot(ev), false)
	case events.CardToken:
		if src := e.G.Obj(ev.Obj); src != nil {
			// CardToken copies the card and face, not the source object's
			// counters, elections or compleated payment.
			return events.EntryCounterGrants(&state.Object{Card: src.Card, FaceIdx: src.FaceIdx}, false)
		}
	}
	return nil
}

// entryPreview builds the isolated preview a grant settlement runs against:
// a cloned game with the entry move applied, a private log and a private
// replacement-choice queue, so no speculative event, ask or pose can leak
// into the real chain. The returned entrant is the object ID the entry will
// carry on the battlefield (a token mint's deterministic next ID).
func (e *Engine) entryPreview(ev events.Event) (*Engine, state.ObjID) {
	preview := *e
	preview.G = e.G.Clone()
	// A competing AddCounter choice can log an ask during the preview. Keep
	// both its log and its queue private; no speculative event may leak into
	// the real chain. Preserve prior events for log-backed counter predicates.
	shadow := *e.L
	shadow.Events = append([]events.Event(nil), e.L.Events...)
	preview.L = &shadow
	preview.replChoices = append([]replChoice(nil), e.replChoices...)
	// TokenCreate/CardToken mint in their own Apply fold rather than
	// emitting a MoveZone. The preview reveals their deterministic new ID;
	// use that ID for the same replacement and prohibition path as a card.
	entrant := ev.Obj
	if ev.Kind == events.TokenCreate || ev.Kind == events.CardToken {
		entrant = e.G.NextID
	}
	// Pin the live action cause before the preview's Apply takes the entrant
	// off the cloned stack: the entry's grants are put by whatever resolving
	// spell or ability moved the object, and the EffectOnly class (Doubling
	// Season) reads that cause. Live engines never carry a pin.
	preview.causePin = e.actionCause()
	events.Apply(preview.G, ev)
	return &preview, entrant
}

// settleEntryGrants walks the entry's grants in order against the previewed
// entry, returning the finalized amounts. A second return of true reports
// the first grant whose AddCounter competition parked a CR 616.1 order
// choice on the PREVIEW: the pose is appended to preview.replChoices for the
// caller to extract (everything from the length the caller snapshotted).
func (e *Engine) settleEntryGrants(entrant state.ObjID, grants []events.EntryCounterGrant) ([]events.EntryCounterGrant, int) {
	var placed []events.EntryCounterGrant
	for i, g := range grants {
		if g.Amount <= 0 || e.PutCounterBlocked(g.Kind, entrant, 0, false) {
			continue
		}
		counter := events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.Kind, Amount: g.Amount}
		rewritten, handled := e.applyReplacements(counter)
		if handled {
			// A noncommuting CR 616.1 choice must remain a real decision;
			// the pose sits at the tail of the preview's private queue.
			return placed, i
		}
		if rewritten.Amount > 0 {
			placed = append(placed, events.EntryCounterGrant{Kind: g.Kind, Amount: rewritten.Amount})
		}
	}
	return placed, -1
}

// extractEntryPose returns the AddCounter order pose the preview appended
// during one grant's settlement. Poses append at the tail of the preview's
// private queue; only poses appended after n0 belong to this settlement.
func extractEntryPose(preview *Engine, n0 int) *replChoice {
	for i := len(preview.replChoices) - 1; i >= n0; i-- {
		if preview.replChoices[i].kind == replChoiceAddCounter {
			rc := preview.replChoices[i]
			return &rc
		}
	}
	return nil
}

// stageEntryCounterOrder parks ev's entry behind the CR 616.1 order choice
// the preview parked for grant idx, and asks the affected player. inRes is
// the pose's in-resolution provenance, captured by the caller at the moment
// the entry was emitted. Returns true (the caller must not fold the move).
func (e *Engine) stageEntryCounterOrder(ev events.Event, preview *Engine, n0 int, grants []events.EntryCounterGrant, placed []events.EntryCounterGrant, idx int, inRes bool) bool {
	posed := extractEntryPose(preview, n0)
	if posed == nil {
		return false
	}
	st := &entryCounterStage{
		move: ev, grants: grants, placed: placed,
		counter: posed.ev, cands: posed.cands, player: posed.player,
		inRes: inRes, idx: idx,
	}
	e.replChoices = append(e.replChoices, replChoice{kind: replChoiceEntryOrder,
		ev: posed.ev, cands: posed.cands, player: posed.player,
		before: e.triggerBefore, inResolution: st.inRes,
		damaging: e.damaging, combatDamaging: e.combatDamaging, dmgSrcOverride: e.dmgSrcOverride,
		stage: st})
	if e.pending == nil {
		e.askReplacementChoice(st.player)
	}
	return true
}

// entryETBChoiceOutstanding reports whether ev's entry still owes an
// as-enters election (the same entryETBChoice walk the replacement dispatch
// poses from, at the ordinal that entry's pose sequence has reached). Pure:
// it only builds the next choice's options.
func (e *Engine) entryETBChoiceOutstanding(ev events.Event) bool {
	if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield {
		return false
	}
	ordinal := 0
	if e.etbMove != nil {
		if e.etbMove.Obj != ev.Obj {
			return false
		}
		ordinal = e.etbNext
	}
	_, ok := e.entryETBChoice(ev, ordinal)
	return ok
}

// entryCounterOrderParks is Engine.emit's pre-pass: it reports whether ev's
// entry must stage behind an outstanding or newly posed CR 616.1 order
// choice instead of folding. A completed stage returns false -- the fold
// that re-drive reaches consumes it.
func (e *Engine) entryCounterOrderParks(ev events.Event) bool {
	for i := range e.replChoices {
		rc := &e.replChoices[i]
		if rc.kind == replChoiceEntryOrder && rc.stage != nil && rc.stage.sameEntryMove(ev) {
			return !rc.stage.complete
		}
	}
	if e.entryStageDone != nil && e.entryStageDone.sameEntryMove(ev) {
		return false
	}
	// An entry still owing an as-enters election stages nothing yet: the
	// election is part of defining the grant set (riot, then unleash, then a
	// name/type/colour election each append their grant on their own emit),
	// and the replacement dispatch poses it on this very emit. Staging before
	// the last election would snapshot an incomplete grant set and the
	// completed stage would fold the entry without the late grant.
	if e.entryETBChoiceOutstanding(ev) {
		return false
	}
	grants := e.entryCounterGrants(ev)
	if len(grants) == 0 {
		return false
	}
	preview, entrant := e.entryPreview(ev)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		return false
	}
	n0 := len(preview.replChoices)
	placed, park := preview.settleEntryGrants(entrant, grants)
	if park < 0 {
		return false
	}
	return e.stageEntryCounterOrder(ev, preview, n0, grants, placed, park, e.resolvingObj != 0)
}

// resumeEntryCounterOrder answers one staged competition: the chosen body
// applies FIRST (CR 616.1's answer), the rest re-drive on an isolated
// preview of the post-entry board -- the only state the zone filters match
// on -- and a live non-commuting remainder re-poses for real. A fully
// settled stage completes: the staged move re-emits through the ordinary
// emit and its fold consumes the finalized amounts.
func (e *Engine) resumeEntryCounterOrder(rc replChoice, idx int) {
	st := rc.stage
	preview, entrant := e.entryPreview(st.move)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		// The entry cannot happen; the competition is moot. Re-emit the move
		// so the ordinary path records whatever the move actually does (the
		// same verdict the fold's preview takes).
		e.emit(st.move)
		return
	}
	// The chosen body applies first, on the preview.
	m := rc.cands[idx]
	if n, ok := preview.applyAddCounterBody(st.counter, m, st.counter.Amount); ok {
		st.counter.Amount = n
	}
	st.applied = append(st.applied, m)
	preview.driveAddCounterCompetition(
		replChoice{ev: st.counter, cands: st.cands, appliedRepls: st.applied},
		func(ev events.Event, applied []replMatch) {
			st.counter, st.applied = ev, applied
		},
		func(rc2 replChoice, p state.PlayerID) {
			// A live non-commuting remainder: re-pose for real against the
			// same stage.
			st.counter, st.applied = rc2.ev, rc2.appliedRepls
			e.replChoices = append(e.replChoices, replChoice{kind: replChoiceEntryOrder,
				ev: rc2.ev, cands: rc2.cands, player: p,
				before: e.triggerBefore, inResolution: st.inRes,
				damaging: e.damaging, combatDamaging: e.combatDamaging, dmgSrcOverride: e.dmgSrcOverride,
				stage: st})
			if e.pending == nil {
				e.askReplacementChoice(p)
			}
		})
	if len(e.replChoices) > 0 {
		last := &e.replChoices[len(e.replChoices)-1]
		if last.kind == replChoiceEntryOrder && last.stage == st && !st.complete {
			// The drive re-posed: the stage stays outstanding for the next
			// answer; nothing is finalized yet.
			return
		}
	}
	// The parked grant finalized.
	if st.counter.Amount > 0 {
		st.placed = append(st.placed, events.EntryCounterGrant{Kind: st.counter.Counter, Amount: st.counter.Amount})
	}
	// The grants after the parked one settle against the same preview.
	for j := st.idx + 1; j < len(st.grants); j++ {
		n0 := len(preview.replChoices)
		more, park := preview.settleEntryGrants(entrant, st.grants[j:])
		st.placed = append(st.placed, more...)
		if park < 0 {
			break
		}
		if !e.stageEntryCounterOrder(st.move, preview, n0, st.grants, st.placed, j+park, st.inRes) {
			continue
		}
		// stageEntryCounterOrder built a fresh stage for the new park; graft
		// the accumulated placement onto it so the next resume continues
		// this entry rather than starting over.
		fresh := e.replChoices[len(e.replChoices)-1].stage
		fresh.placed = st.placed
		return
	}
	st.complete = true
	e.entryStageDone = st
	e.emit(st.move)
}

// foldEntryMove is shared by the ordinary emit tail and the Updated
// replacement paths. A preview fold on an isolated Game lets counter filters
// see the entering permanent in its destination zone without changing live
// state. The finalized counter amounts travel in the entry event's Pairs
// payload (MoveZone, TokenCreate or CardToken): events.Apply installs them IN
// the entry, before any observer runs. CounterChange records after the entry
// are notification-only: they do not place counters twice on replay.
func (e *Engine) foldEntryMove(ev events.Event) events.Event {
	if st := e.entryStageDone; st != nil && st.complete && st.sameEntryMove(ev) {
		e.entryStageDone = nil
		return e.foldEntryWithPlaced(ev, st.placed)
	}
	grants := e.entryCounterGrants(ev)
	if len(grants) == 0 {
		return events.Emit(e.G, e.L, ev)
	}
	preview, entrant := e.entryPreview(ev)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		return events.Emit(e.G, e.L, ev)
	}
	placed, park := preview.settleEntryGrants(entrant, grants)
	if park >= 0 {
		// Residual: only reachable where the emit pre-pass does not run --
		// an entry emitted inside a replacement body, or a TokenCreate/
		// CardToken mint. There the ordinary counter path owns its park and
		// resume, and the placements follow the move (the pre-staging
		// behaviour this task's pre-pass replaces for ordinary entries).
		stored := events.Emit(e.G, e.L, ev)
		for _, grant := range grants {
			e.emit(events.Event{Kind: events.CounterChange, Obj: entrant, Counter: grant.Kind, Amount: grant.Amount})
		}
		return stored
	}
	return e.foldEntryWithPlaced(ev, placed)
}

// foldEntryWithPlaced folds the entry move with the finalized grant amounts
// in its Pairs payload, then emits each grant's notification-only
// CounterChange (EntryCounterNotice) through the ordinary emit path so
// trig:CounterAdded and the per-turn ledger see the placement exactly once.
func (e *Engine) foldEntryWithPlaced(ev events.Event, placed []events.EntryCounterGrant) events.Event {
	for _, g := range placed {
		ev.Pairs = append(ev.Pairs, events.EntryCounterPair(g))
	}
	stored := events.Emit(e.G, e.L, ev)
	entrant := ev.Obj
	for _, g := range placed {
		// Replacement has already settled; the marker only notifies observers.
		savedApplying, savedFold := e.applyingReplacement, e.counterReplacementFold
		e.applyingReplacement, e.counterReplacementFold = true, true
		e.emit(events.Event{Kind: events.CounterChange, Obj: entrant,
			Counter: g.Kind, Amount: g.Amount, Text: events.EntryCounterNotice})
		e.applyingReplacement, e.counterReplacementFold = savedApplying, savedFold
	}
	return stored
}
