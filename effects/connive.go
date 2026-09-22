package effects

import (
	"strconv"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Connive is Forge's ConniveEffect (api:Connive, CR 702.59 — 53 raw corpus
// files carry a `DB$ Connive` body or an `AB$/SP$ Connive` action: the
// New Capenna connive mechanic — Ledger Shredder, Raffine's Silencer, Lethal
// Scheme's convoked sub, Mask of the Schemer's X-trigger, ...).
//
//	"Connive N" means "Draw N cards, then discard N cards. Put a +1/+1
//	counter on this permanent for each nonland card discarded this way."
//	(702.59a; a connive with no N connives 1 — 702.59b.)
//
// Per conniver and per ConniveNum$ (default 1):
//
//   - The draws come first (all N, before any discard), through the ordinary
//     DrawFor path, so a Dredge replacement over a connive draw is a real
//     choice exactly as for any other draw.
//   - The discards follow: N cards from the conniver's controller's hand.
//     Connive has no discard filter ("discard a card"), so the eligible set
//     is the whole hand. The strict-supersets rule (the effDiscard
//     TgtChoose discipline) applies: a hand with FEWER OR EXACTLY N cards
//     discards everything it owns in hand order and asks nothing — a
//     decision nobody could answer differently is never emitted — and a
//     hand with more than N poses a real KModes ask (Min = Max = N, options
//     in hand order), whose answer re-enters through rules' "connive" arm
//     and Ctx.ConniveDiscard. A host that cannot ask and botpolicy's KModes
//     clamp both take the first N options (the front N cards), so the
//     no-host stand-in and the bot answer are byte-identical by
//     construction and no R-9 Note is owed (the Explore discipline).
//   - The counters come last: one +1/+1 counter per NONLAND card discarded
//     (CR 702.59a's "for each nonland card discarded this way"), one
//     CounterChange carrying the whole count.
//   - The action is recorded by one events.Connive marker per completed
//     connive (IDs the discarded cards in discard order, Amount the nonland
//     count) — what trig:Connives matches. One marker per connive, never
//     one per discarded card.
//
// Defined$ selectors (TriggeredSourceLKICopy, Targeted, TriggeredCardLKICopy,
// Convoked, ReplacedCard, TriggeredAttackerLKICopy, ...) and the ValidTgts$
// target ask resolve through the ordinary machinery (Defined falls back to
// the source for the bare `DB$ Connive` self-connive trigger shape, the 27
// SVar:TrigConnive carriers). ConniveNum$ resolves through the ordinary Num
// evaluator (a literal is the common carrier; an X body resolves at
// resolution time, a body whose X never resolved connives 0 and does
// nothing).
func init() {
	Register("Connive", effConnive)
}

// effConnive resolves every conniving creature through its connive process.
// SubAbility$ chains are resolved by the ordinary Resolve walk, not here.
func effConnive(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "ConniveNum", 1)
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		// The resume cursor: skip targets before the one whose discard
		// suspended (the effExplore discipline — re-entry replays the loop
		// from its head, so earlier targets must not connive a second time).
		if c.ConniveObj != 0 && t.Obj != c.ConniveObj {
			continue
		}
		// The answered discard for THIS conniver: apply it (fx42 scoping —
		// captured into a local and cleared before application, so the
		// remaining targets pose their own fresh asks), then continue.
		if c.ConniveDone && c.ConniveObj == t.Obj {
			picks := c.ConniveDiscard
			c.ConniveDone, c.ConniveObj, c.ConniveDiscard = false, 0, nil
			applyConniveDiscard(h, t.Obj, picks)
			continue
		}
		if conniveOnce(h, c, sa, t.Obj, n) {
			// The ask was posted; the resolution is suspended. Nothing after
			// the ask may run on this pass — the answer re-enters through
			// rules' "connive" resume arm (the effExplore discipline).
			return
		}
	}
	// Leftover pending state no target consumed (the pending conniver left
	// play, a malformed resume): consumed and cleared, never inherited.
	c.ConniveDone, c.ConniveObj, c.ConniveDiscard = false, 0, nil
}

// conniveOnce runs one connive process for conniver and reports whether the
// discard ask was POSTED (the resolution suspended — the caller must stop
// its walk immediately). A conniver that has left the battlefield connives
// nothing. The draws all happen first; the discard ask (when the hand is
// larger than N) is the only suspension point.
func conniveOnce(h Host, c *Ctx, sa *cards.SA, conniver state.ObjID, n int32) bool {
	g := h.Game()
	o := g.Obj(conniver)
	if o == nil || o.Zone != state.ZBattlefield {
		return false
	}
	ctrl := o.Controller
	for i := int32(0); i < n; i++ {
		DrawFor(h, ctrl)
	}
	// The discard: N cards from the conniver's controller's hand, no filter.
	hand := zoneOf(g, state.ZHand, ctrl)
	n2 := n
	if n2 > int32(len(hand)) {
		n2 = int32(len(hand))
	}
	if n2 <= 0 {
		emitConniveRecord(h, conniver, ctrl, nil, 0)
		return false
	}
	if int32(len(hand)) > n {
		// A real choice: more cards in hand than to discard. Options in hand
		// order; a host that cannot ask and botpolicy's KModes clamp both
		// take the first n (the front n cards) — the deterministic stand-in,
		// byte-identical to the no-ask path's pick.
		opts := make([]decision.Option, 0, len(hand))
		for _, id := range hand {
			name := "a card"
			if co := g.Obj(id); co != nil && co.Face() != nil {
				name = co.Face().Name
			}
			opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
				Label: "Discard " + name, Obj: id, Player: ctrl})
		}
		d := &decision.Decision{Player: ctrl, Kind: decision.KModes, Min: int(n2), Max: int(n2),
			Source: c.Source, ResumeKind: "connive", ResumeSA: sa,
			ResumeTarget: int(conniver),
			Prompt:       "Choose " + strconv.Itoa(int(n2)) + " card(s) to discard",
			Options:      opts}
		if Ask(h, d) == AskAsked {
			// Park the mid-connive state (documenting; the resume arm is what
			// actually re-seeds it — the suspended Ctx is discarded).
			c.ConniveObj = conniver
			return true
		}
		applyConniveDiscard(h, conniver, hand[:n2])
		return false
	}
	// The whole hand is exactly the discard set: no choice to be made.
	applyConniveDiscard(h, conniver, hand)
	return false
}

// applyConniveDiscard applies one connive's discard outcome: each picked
// card still in the conniver's controller's hand is discarded in pick order,
// then one +1/+1 counter per NONLAND discard (CR 702.59a), then the one
// events.Connive record both shapes share.
func applyConniveDiscard(h Host, conniver state.ObjID, picks []state.ObjID) {
	g := h.Game()
	o := g.Obj(conniver)
	if o == nil {
		return
	}
	ctrl := o.Controller
	discarded := make([]state.ObjID, 0, len(picks))
	nonland := int32(0)
	for _, id := range picks {
		co := g.Obj(id)
		if co == nil || co.Zone != state.ZHand || co.Owner != ctrl {
			continue
		}
		h.Emit(events.Discard(id, ctrl))
		discarded = append(discarded, id)
		if co.Face() != nil && !co.Face().IsLand() {
			nonland++
		}
	}
	if nonland > 0 {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: conniver, Counter: "P1P1", Amount: nonland})
	}
	emitConniveRecord(h, conniver, ctrl, discarded, nonland)
}

// emitConniveRecord emits the one events.Connive marker per completed
// connive action (what trig:Connives matches).
func emitConniveRecord(h Host, conniver state.ObjID, ctrl state.PlayerID, discarded []state.ObjID, nonland int32) {
	h.Emit(events.Event{Kind: events.Connive, Obj: conniver, Player: ctrl,
		IDs: discarded, Amount: nonland})
}
