package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// oracleSetupAttach places one of a seat's battlefield Auras already attached
// (the seat's "attached" list).
//
// CR 303.4f: an Aura entering other than by resolving enters attached, and the
// effect putting it onto the battlefield may name what it enchants, so no
// choice is made and nothing is targeted; setup is that effect. Card is a
// SEAT-RELATIVE ref of that seat's battlefield ("Name" or "Name#k", the part
// of a runner ref after "pN:") whose bare name is in Battlefield; To is a FULL
// ref of a battlefield object of any seat (a card or a setup token). The
// card's ref binds in its ordinary place, so "pN:Name#k" ordinals do not
// move; only its placement is deferred until every seat's cards and tokens
// are placed, and then runs in seat then list order. Placement fires no
// trigger: not the Aura's entry triggers and not "whenever an Aura becomes
// attached" (XMage links the attachment without an attach event). A player
// bearer is not supported.
type oracleSetupAttach struct {
	Card string `json:"card"`
	To   string `json:"to"`
}

// setupAttachedEntry is one deferred attached placement.
type setupAttachedEntry struct {
	seat   int
	id     state.ObjID
	name   string
	attach oracleSetupAttach
}

// setupAttachedMatch returns the attached entry of s naming the nth placement
// of battlefield card name, if any.
func setupAttachedMatch(p int, s oracleSeat, name string, nth int) (oracleSetupAttach, bool) {
	for _, a := range s.Attached {
		_, n, _, _, k, err := splitRef(oracleSeatRef(p, a.Card))
		if err == nil && k == nth && cards.NormalizeName(n) == cards.NormalizeName(name) {
			return a, true
		}
	}
	return oracleSetupAttach{}, false
}

// deferSetupAttached records a battlefield placement that s lists in
// "attached" for placeAttached, and reports whether it did. Its ref is
// already bound by the caller.
func (r *oracleRun) deferSetupAttached(s oracleSeat, p int, zone state.Zone, name string, nth int, id state.ObjID) bool {
	if zone != state.ZBattlefield {
		return false
	}
	a, ok := setupAttachedMatch(p, s, name, nth)
	if ok {
		r.attachedDeferred = append(r.attachedDeferred, setupAttachedEntry{seat: p, id: id, name: name, attach: a})
	}
	return ok
}

// validateSetupAttached rejects an "attached" entry the runner cannot place: a
// card that is empty, carries a "pN:" prefix, is not on the seat's battlefield
// or is listed twice, or a "to" that is not a runner ref.
func validateSetupAttached(p int, s oracleSeat) error {
	type key struct {
		name string
		nth  int
	}
	seen := map[key]bool{}
	for _, a := range s.Attached {
		if a.Card == "" {
			return harnessf("setup: p%d attached has an empty card", p)
		}
		if _, _, _, _, _, err := splitRef(a.Card); err == nil {
			return harnessf("setup: p%d attached card %q must be seat-relative (no pN: prefix)", p, a.Card)
		}
		_, name, token, _, nth, err := splitRef(oracleSeatRef(p, a.Card))
		if err != nil {
			return err
		}
		have := 0
		for _, n := range s.Battlefield {
			if cards.NormalizeName(n) == cards.NormalizeName(name) {
				have++
			}
		}
		if token || nth > have {
			return harnessf("setup: p%d attached card %q is not on its battlefield", p, a.Card)
		}
		k := key{cards.NormalizeName(name), nth}
		if seen[k] {
			return harnessf("setup: p%d attached lists %q twice", p, a.Card)
		}
		seen[k] = true
		if _, _, _, _, _, err := splitRef(a.To); err != nil {
			return harnessf("setup: p%d attached %q: %v", p, a.Card, err)
		}
	}
	return nil
}

// placeAttached places every deferred attached Aura, in seat then list order,
// once all cards and tokens are on the battlefield: the marked entry names the
// bearer, so the Aura enters attached with no choice (CR 303.4f), and a bearer
// it cannot legally enchant leaves it in the library (CR 303.4g), a harness
// error here.
func (r *oracleRun) placeAttached(sc oracleScenario) error {
	e := r.e
	for _, d := range r.attachedDeferred {
		bearer, err := r.resolve(d.attach.To)
		if err != nil {
			return err
		}
		if b := e.G.Obj(bearer); b == nil || b.Zone != state.ZBattlefield {
			return harnessf("setup: p%d attached %q names %q, which is not on the battlefield", d.seat, d.attach.Card, d.attach.To)
		}
		r.placeSetupEntry(sc, d.seat, d.id, d.name, state.ZBattlefield, bearer)
		if o := e.G.Obj(d.id); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bearer {
			return harnessf("setup: p%d %q could not enter attached to %q (CR 303.4g)", d.seat, d.attach.Card, d.attach.To)
		}
	}
	r.attachedDeferred = nil
	return nil
}

// placeSetupEntry places setup card id (named name) into zone for seat p and
// runs the per-placement tail: the back-face flip, tapped, counters and
// SetupSettled. A nonzero bearer places a battlefield Aura attached to it.
func (r *oracleRun) placeSetupEntry(sc oracleScenario, p int, id state.ObjID, name string, zone state.Zone, bearer state.ObjID) {
	seat := sc.Setup[fmt.Sprintf("p%d", p)]
	e := r.e
	backFace := setupPlacedBackFace(seat, name)
	if from := e.G.Obj(id).Zone; from != zone {
		// A back-face battlefield card enters ON its back face: the
		// FlipFace rides ahead of the MoveZone (the same sequence a
		// transformed ChangeZone emits, effects.applyTransformed), so
		// the entry fold reads the face XMage's addCard places --
		// a back-face Saga creature enters with its lore counter, a
		// back-face non-Saga (a transformed Saga's creature, the
		// crafted Braided Quipu) enters with none and no chapter
		// trigger. A flip after the entry would leave the FRONT
		// face's entry grants (The Legend of Kuruk's lore counter)
		// stranded on a permanent whose face never carried them.
		if backFace && zone == state.ZBattlefield {
			// Placement is not a transform (CR 701.28): the back
			// face's "as this transforms" replacement (Sephiroth's
			// emblem) must not run for a card the harness merely
			// seats on that face, as XMage's addCard does.
			prior := e.applyingReplacement
			e.applyingReplacement = true
			e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1, Text: "Transformed"})
			e.applyingReplacement = prior
		}
		r.placeSetupCard(id, from, zone, sc.xmageFixture, backFace, bearer)
	}
	if zone == state.ZBattlefield && backFace {
		// A back-face PLANESWALKER whose entering face prints no
		// positive loyalty (Forge's "Loyalty: 0" carry-over spelling,
		// Oko, Lorwyn Liege's Shadowmoor Scion) is not enterable as
		// printed: a real Oko reaches the back face by transforming,
		// carrying its loyalty. XMage's addCard places the back face
		// with the front face's starting loyalty, so the harness
		// grants that where the entry fold (reading the back face)
		// granted none; without it the walker sits at 0 loyalty and
		// the CR 704.5i state-based action removes it before its
		// first-main-phase trigger can fire.
		if n := setupBackFaceLoyalty(e.G.Obj(id)); n > 0 {
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "LOYALTY", Amount: n})
		}
	}
	for _, tapped := range seat.Tapped {
		if cards.NormalizeName(tapped) == cards.NormalizeName(name) {
			e.emit(events.Event{Kind: events.Tap, Obj: id})
			break
		}
	}
	if zone == state.ZBattlefield {
		for _, ev := range setupCounters(seat, name) {
			ev.Obj = id
			e.emit(ev)
		}
		// XMage's addCard places a permanent as if it had been there
		// since before the game: it is not summoning sick (CR 302.6)
		// on turn 1 whichever seat it sits with. The first TurnChange
		// clears sickness only for the starting seat, so a
		// non-starting seat's {T} ability (Trap Runner on p1) would
		// otherwise be unactivatable on p0's turn 1.
		e.emit(events.Event{Kind: events.SetupSettled, Obj: id})
	}
}
