// entry_counters.go places the entry-characteristic counters a permanent
// enters the battlefield with (starting loyalty, Riot/Unleash's election, a
// Saga's lore counter, a Battle's defense counters) through REAL
// CounterChange events, so the CR 614 AddCounter replacement class and the
// CantPutCounter prohibition see them exactly like any other placement. The
// arithmetic lives in events.EntryCounterGrants. The final placements are
// prepared against the would-enter board, then folded with the move itself.
package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

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

// foldEntryMove is shared by the ordinary emit tail and the Updated
// replacement paths. A preview fold on an isolated Game lets counter filters
// see the entering permanent in its destination zone without changing live
// state. The finalized counter amounts travel in the entry event's Pairs
// payload (MoveZone, TokenCreate or CardToken): events.Apply installs them IN
// the entry, before any observer runs. CounterChange records after the entry
// are notification-only: they do not place counters twice on replay.
func (e *Engine) foldEntryMove(ev events.Event) events.Event {
	grants := e.entryCounterGrants(ev)
	if len(grants) == 0 {
		return events.Emit(e.G, e.L, ev)
	}
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
	events.Apply(preview.G, ev)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		return events.Emit(e.G, e.L, ev)
	}
	var placed []events.EntryCounterGrant
	for _, g := range grants {
		if g.Amount <= 0 || preview.PutCounterBlocked(g.Kind, entrant, 0, false) {
			continue
		}
		counter := events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.Kind, Amount: g.Amount}
		rewritten, handled := preview.applyReplacements(counter)
		if handled {
			// A noncommuting CR 616.1 choice must remain a real decision;
			// the ordinary counter path owns its park and resume.
			stored := events.Emit(e.G, e.L, ev)
			for _, grant := range grants {
				e.emit(events.Event{Kind: events.CounterChange, Obj: entrant, Counter: grant.Kind, Amount: grant.Amount})
			}
			return stored
		}
		if rewritten.Amount > 0 {
			placed = append(placed, events.EntryCounterGrant{Kind: g.Kind, Amount: rewritten.Amount})
		}
	}
	for _, g := range placed {
		ev.Pairs = append(ev.Pairs, events.EntryCounterPair(g))
	}
	stored := events.Emit(e.G, e.L, ev)
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
