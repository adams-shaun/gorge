package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// CR 708.6 / CR 116.2b: any time its controller has priority, a face-down
// permanent that was cast with Morph (CR 702.37a), Megamorph (CR 702.168a)
// or Disguise (CR 702.169a) may be turned face up. Turning it face up is a
// SPECIAL ACTION -- it does not use the stack and cannot be responded to --
// whose only cost is the keyword's own parameter (the {3} cast is the
// face-down one; the colon parameter is the turn-up one). Megamorph's rider
// adds a +1/+1 counter as the permanent turns face up.
//
// A manifested card (Choices$ Manifest) and a cloaked card have their own
// turn-up rules and carry none of the three family flags, so morphFaceUpCost
// reports false for them and this action is never offered: their turn-up is
// a separate subsystem. The cast side (rules/cast.go's modeFlags and
// moveResolvedOffStack) is the provenance source -- the family flag survives
// onto the battlefield permanent, and the printed face (which is retained on
// the object even while face down, CR 708.8 only hides it from the RULES)
// carries the cost.
type morphFaceUp struct {
	cost Cost
	// family is the printed keyword head ("Morph", "Megamorph", "Disguise"),
	// used only for the offer label and diagnostics.
	family string
	// megamorph is true when the turn-up must also put a +1/+1 counter.
	megamorph bool
}

// morphFaceUpCost resolves the turn-face-up action a face-down battlefield
// permanent offers, or ok=false when it offers none: not face down, not a
// creature the morph family put down (no family flag), or a face that
// carries no keyword parameter to price the action.
func morphFaceUpCost(o *state.Object) (morphFaceUp, bool) {
	if o == nil || o.Zone != state.ZBattlefield || !o.FaceDown {
		return morphFaceUp{}, false
	}
	fam := o.CastFlags & (state.FlagMorphed | state.FlagMegamorphed | state.FlagDisguised)
	if fam == 0 {
		return morphFaceUp{}, false
	}
	f := o.Face()
	if f == nil {
		return morphFaceUp{}, false
	}
	var head string
	switch {
	case fam&state.FlagMorphed != 0:
		head = "Morph"
	case fam&state.FlagMegamorphed != 0:
		head = "Megamorph"
	case fam&state.FlagDisguised != 0:
		head = "Disguise"
	}
	raw, ok := f.KeywordParam(head)
	if !ok || strings.TrimSpace(raw) == "" {
		return morphFaceUp{}, false
	}
	return morphFaceUp{
		cost:      ParseCost(raw),
		family:    head,
		megamorph: fam&state.FlagMegamorphed != 0,
	}, true
}

// turnFaceUp is handlePriority's "turn_face_up" action: it pays the
// remembered keyword cost, emits the CR 708.6 TurnFaceUp event (whose Apply
// clears the face-down marker and the folded CR 708.5 set), and, for a
// Megamorph cast, puts the +1/+1 counter. No stack object is minted: a
// special action resolves immediately (CR 116.2b). The offer gated on
// costPayable against this same floating pool, so payMana here cannot
// disagree with what was offered; a stale option (the permanent left play or
// was already turned face up) degrades to a no-op, and the inert backstop
// holds it out of the re-offer.
func (e *Engine) turnFaceUp(p state.PlayerID, opt decision.Option) {
	o := e.G.Obj(opt.Obj)
	mf, ok := morphFaceUpCost(o)
	if !ok || o.Controller != p {
		return
	}
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
	if !e.payMana(p, mf.cost) {
		return
	}
	// Read the megamorph rider BEFORE the TurnFaceUp event: its Apply leaves
	// CastFlags untouched, but the read belongs to the cast provenance the
	// action is priced from, so it is taken once here.
	megamorph := mf.megamorph
	e.emit(events.Event{Kind: events.TurnFaceUp, Obj: opt.Obj})
	if megamorph {
		e.emit(events.Event{Kind: events.CounterChange, Obj: opt.Obj, Counter: "P1P1", Amount: 1})
	}
}
