package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// setupPlacementDropsTrigger reports whether a trigger queued while an
// xmageFixture scenario's setup placed a permanent on the battlefield must be
// discarded before the drive loop answers it.
//
// XMage builds a generated scenario's starting position with
// addCard(Zone.BATTLEFIELD, ...), which puts the permanent onto the
// battlefield without "entering": no enters-the-battlefield trigger is put on
// the stack, so gorge must not resolve the ones its own MoveZone queued. The
// discard is deliberately narrow, because a single placement's emit also
// queues triggers the entry causes indirectly and that XMage DOES see fire:
//
//   - TriggerChangesZone (Destination$ Battlefield, a list admitting it, or
//     no Destination$ at all) and TriggerChangesZoneAll are the entry shapes
//     ("when this enters, ..."; "whenever one or more creatures enter").
//     Both are XMage-addCard-silent and are dropped.
//   - A Saga's synthesized chapter ability (pendingTrigger.Chapter) is dropped
//     only for a back-face placement whose ENTERING face is not itself a Saga
//     (backFaceChapter, computed by setupBackFaceDropsChapter): XMage's
//     transformed permanent then carries a face with no Saga ability, so no
//     lore counter and no chapter I (a transformed Saga's creature; the entry
//     fold queues nothing there, so the drop only guards it). A Saga placed
//     with addCard DOES enter with its lore counter and fire chapter I there,
//     so it stays queued -- a front-face Saga (Summon: Anima's life loss,
//     Summon: Titan's mill, Summon: Knights of Round's tokens, and Summon:
//     Leviathan bouncing Grizzly Bears before the setup checkpoint) and a
//     back-face Saga creature alike (Jecht's Braska's Final Aeon discarding
//     and drawing, Joshua's Phoenix, Warden of Fire dealing 2 and gaining 2:
//     XMage's setup snapshots hold lore 2 and the chapter I effect already
//     resolved).
//   - Everything else stays. In particular a planeswalker's entry loyalty
//     counters are emitted as CounterChanges inside the same entry fold, so a
//     CounterAdded / CounterAddedOnce / CounterAddedAll trigger on a permanent
//     already on the battlefield (Inspired Tethermage watching Ajani Goldmane
//     enter) is queued in the same window and MUST fire; the counted
//     compliance test TestGeneratedSetupKeepsCounterAddedTrigger pins it.
//
// A trigger whose line cannot be resolved is kept: the discard only ever
// removes a trigger it can positively classify as an entry shape, so an
// unrecognised synthetic trigger degrades toward firing rather than vanishing.
//
// dropAttached is set only by a placement that enters ATTACHED (setup's
// "attached", placeSetupCard's bearer): it also drops TriggerAttached, since
// XMage links the attachment without an attach event.
func setupPlacementDropsTrigger(t cards.Trigger, backFaceChapter, dropAttached bool) bool {
	if backFaceChapter {
		return true
	}
	switch t.ModeKind() {
	case cards.TriggerAttached:
		return dropAttached
	case cards.TriggerChangesZone:
		// Destination$ compiles once through the registered coder (the same
		// read the merge-base filter used), so this is a trip through the
		// compiled zone enum rather than a fresh string comparison. A
		// trigger queued in the placement window matched the placement's
		// own move to the battlefield, so an ABSENT Destination$ (Bullseye,
		// Death Dealer's "When NICKNAME enters", scripted with ValidCard$
		// Card.Self and no destination) is that entry too, as is a
		// destination list that admits the battlefield.
		d, ok := t.ParamCode(cards.PKDestination)
		return !ok || effects.Destination(d).Admits(state.ZBattlefield)
	case cards.TriggerChangesZoneAll:
		return true
	}
	return false
}

// setupBackFaceDropsChapter reports whether the chapter trigger a back-face
// battlefield placement queued must be dropped: the card is placed on its back
// face and that entering face (o.Face(), read after the FlipFace that rides
// ahead of the MoveZone) is not a Saga. A back-face Saga creature keeps its
// chapter I -- XMage's addCard resolves it from the entry lore counter.
func setupBackFaceDropsChapter(o *state.Object, backFace bool) bool {
	if !backFace || o == nil {
		return false
	}
	n, _ := cards.SagaChapters(o.Face())
	return n == 0
}

// placeSetupCard moves a setup card from the zone it was dealt to into its
// scenario zone (the MoveZone of oracleRun.build's placement loop).
//
// A generated compliance scenario (fixture) mirrors XMage's setup, which
// places every card with addCard rather than by playing it:
//
//   - A BATTLEFIELD placement keeps the ordinary move, so the card's own
//     enters-with replacements still run, and discards exactly the entry-
//     shaped triggers the move queued (setupPlacementDropsTrigger).
//   - Any OTHER zone (graveyard, exile, hand, library, command) is a plain
//     placement: no "Moved" replacement of a battlefield card may rewrite it
//     (Rest in Peace's "exile instead", Leyline of the Void's) and no
//     trigger it queued may fire (Planar Void's "whenever another card is
//     put into a graveyard"). The replacement pass is skipped with
//     applyingReplacement for this one emit, restored by defer so a panic or
//     any early return of the caller cannot leak it past setup, and every
//     trigger the move queued is discarded.
//
// Only fixture scenarios take either branch; a hand-written scenario keeps the
// ordinary move and every trigger it queues.
//
// A nonzero bearer marks a battlefield move as an effect-named Aura entry
// (events.MarkNamedAttachEntry): the Aura enters attached to bearer with no
// choice (CR 303.4f), and a fixture placement also drops the "whenever an Aura
// becomes attached" trigger that entry queued.
func (r *oracleRun) placeSetupCard(id state.ObjID, from, to state.Zone, fixture, backFace bool, bearer state.ObjID) {
	e := r.e
	pendingBefore := len(e.pendingTriggers)
	move := events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to}
	if bearer != 0 {
		events.MarkNamedAttachEntry(&move, []state.ObjID{bearer})
	}
	if !fixture {
		e.emit(move)
		return
	}
	if to != state.ZBattlefield {
		func() {
			prior := e.applyingReplacement
			e.applyingReplacement = true
			defer func() { e.applyingReplacement = prior }()
			e.emit(move)
		}()
		if len(e.pendingTriggers) > pendingBefore {
			e.pendingTriggers = e.pendingTriggers[:pendingBefore]
		}
		e.orderedTriggers = min(e.orderedTriggers, len(e.pendingTriggers))
		return
	}
	e.emit(move)
	// A battlefield placement whose card carries an as-enters choice
	// (Gathering Stone's "As this artifact enters, choose a creature type")
	// is PARKED on that ask: the move is re-emitted only when the setup
	// drive answers it, after this window has closed, so its entry triggers
	// queue later. Hand the drop to putTriggersOnStack, which runs before
	// any of them can reach the stack (dropSetupEntryTriggers).
	if e.landEntryParked(id) {
		e.setupEntryDrops = append(e.setupEntryDrops, setupEntryDrop{
			obj: id, dropChapter: setupBackFaceDropsChapter(e.G.Obj(id), backFace)})
	}
	// XMage fixture setup places battlefield cards with addCard, which does
	// not queue entry triggers; drop exactly the entry-shaped triggers this
	// placement queued and keep every other trigger, including the
	// CounterAdded triggers a planeswalker's entry loyalty counters fire on
	// an already-placed permanent (setupPlacementDropsTrigger).
	kept := e.pendingTriggers[:pendingBefore]
	ordered := min(e.orderedTriggers, pendingBefore)
	// Copy the tail first: appending to kept writes back into
	// e.pendingTriggers[pendingBefore:], the very slice this loop reads, and a
	// dropped trigger would shift later entries down under the iteration.
	tail := append([]pendingTrigger(nil), e.pendingTriggers[pendingBefore:]...)
	// A back-face chapter is dropped only when the entering face is not
	// itself a Saga (setupBackFaceDropsChapter).
	dropChapter := setupBackFaceDropsChapter(e.G.Obj(id), backFace)
	for _, pt := range tail {
		t, _ := e.triggerOf(pt)
		if setupPlacementDropsTrigger(t, pt.Chapter && dropChapter, bearer != 0) {
			continue
		}
		kept = append(kept, pt)
	}
	e.pendingTriggers = kept
	e.orderedTriggers = ordered
}

// setupEntryDrop is one fixture battlefield placement whose entry an
// as-enters choice parked (placeSetupCard): obj is the placed card, and
// dropChapter carries the back-face chapter verdict the placement window
// would have applied (setupBackFaceDropsChapter, read at placement).
type setupEntryDrop struct {
	obj         state.ObjID
	dropChapter bool
}

// dropSetupEntryTriggers is placeSetupCard's discard for a PARKED fixture
// placement, run by putTriggersOnStack before the queue is ordered or pushed.
// XMage's addCard answers the as-enters choice but still places the card
// without "entering", so the entry-shaped triggers the completed move queued
// must not fire -- exactly setupPlacementDropsTrigger's verdict, applied to
// the pending queue once the parked card is on the battlefield and no longer
// parked. Between the answer that completes the entry and this first drain
// the engine only runs turn 1's untap and upkeep turn-based actions, which
// move nothing onto the battlefield, so every entry-shaped trigger then
// pending is the parked entry's own. A card still parked (a second as-enters
// ask) waits for the next drain; one that never reached the battlefield
// (its entry replaced away) is forgotten. The list is the harness's alone:
// nil in every host, replay and test outside a fixture setup, and cleared
// when the setup drive stops.
func (e *Engine) dropSetupEntryTriggers() {
	pending := e.setupEntryDrops[:0]
	for _, sd := range e.setupEntryDrops {
		if e.landEntryParked(sd.obj) {
			pending = append(pending, sd)
			continue
		}
		o := e.G.Obj(sd.obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		kept := e.pendingTriggers[:0]
		ordered := 0
		for i, pt := range e.pendingTriggers {
			t, _ := e.triggerOf(pt)
			if setupPlacementDropsTrigger(t, pt.Chapter && pt.Source == sd.obj && sd.dropChapter, false) {
				continue
			}
			if i < e.orderedTriggers {
				ordered++
			}
			kept = append(kept, pt)
		}
		if len(kept) < len(e.pendingTriggers) {
			e.noteTrigShrink()
		}
		e.pendingTriggers = kept
		e.orderedTriggers = ordered
	}
	e.setupEntryDrops = pending
	if len(pending) == 0 {
		e.setupEntryDrops = nil
	}
}

// setupPlacedBackFace reports whether the seat's setup places the named
// battlefield card on its back face (oracleSeat.BackFace).
func setupPlacedBackFace(s oracleSeat, name string) bool {
	for _, back := range s.BackFace {
		if cards.NormalizeName(back) == cards.NormalizeName(name) {
			return true
		}
	}
	return false
}

// setupBackFaceLoyalty is the starting loyalty a back-face placement of a
// transforming planeswalker enters with when its ENTERING face prints none:
// Forge spells the transformed face of an Oko-cycle walker "Loyalty: 0", a
// carry-over marker for the counters the permanent already held, never a real
// zero-loyalty entry (that face is reachable only by transforming). The
// harness grants the front face's printed starting loyalty instead. 0 when the
// entering face has its own positive loyalty, when either face is not a
// planeswalker, or when a printed value does not parse (the same fail-closed
// read events.EntryCounterGrants takes).
func setupBackFaceLoyalty(o *state.Object) int32 {
	if o == nil || o.Card == nil || len(o.Card.Faces) < 2 {
		return 0
	}
	back := o.Face()
	if back == nil || !back.IsPlaneswalker() {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(back.Loyalty)); err == nil && n > 0 {
		return 0
	}
	front := o.Card.Faces[0]
	if front == nil || !front.IsPlaneswalker() {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(front.Loyalty))
	if err != nil || n <= 0 {
		return 0
	}
	return int32(n)
}
