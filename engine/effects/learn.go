package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("Learn", effLearn)
}

// Option kinds of the learn choice (CR 701.48a). "nothing" is always offered
// first, so index 0 -- the deterministic no-host stand-in and the bot clamp
// default -- is "do neither".
const (
	learnKindNothing = "nothing"
	learnKindLesson  = "lesson"
	learnKindRummage = "rummage"
	learnKindDiscard = "discard"
)

// learnLessonSpec is the Lesson card a player may fetch from outside the game
// (their own sideboard).
const learnLessonSpec = "Card.Lesson+YouOwn"

// effLearn implements DB$ Learn (CR 701.48a). The learning player (the
// resolving controller) may do one of: reveal a Lesson card they own from
// outside the game and put it into their hand; discard a card, then draw a
// card; or neither. One owner-routed KChoose poses the branches; choosing
// "rummage" then asks which card to discard. The no-host fallback is "do
// nothing". Every zone move, the reveal and the draw are emitted events.
func effLearn(h Host, c *Ctx, sa *cards.SA) {
	p := c.Controller
	d := LearnDecision(h.Game(), p, c.Source, sa)
	ans, ok := AskTape(h, d)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Player: p, Text: "Learn: no decision host; do nothing"})
		return
	}
	if len(ans) == 0 {
		return
	}
	switch ans[0].Kind {
	case learnKindLesson:
		learnTakeLesson(h, p, ans[0].Obj)
	case learnKindRummage:
		learnRummage(h, c, sa, p)
	}
}

// LearnDecision builds the learn choice. Keeping the construction here lets
// botpolicy validate the exact option shape: "nothing" at index 0, then one
// "lesson" option per Lesson card the player owns in their sideboard, then
// "rummage" when there is a card in hand to discard.
func LearnDecision(g *state.Game, p state.PlayerID, source state.ObjID, sa *cards.SA) *decision.Decision {
	opts := []decision.Option{{Index: 0, Kind: learnKindNothing, Label: "Do nothing", Player: p}}
	for _, id := range zoneOf(g, state.ZSideboard, p) {
		if MatchesSpecFrom(g, learnLessonSpec, id, p, source) {
			opts = append(opts, decision.Option{Index: len(opts), Kind: learnKindLesson,
				Label: "Reveal " + learnName(g, id) + " and put it into your hand", Obj: id, Player: p})
		}
	}
	if len(zoneOf(g, state.ZHand, p)) > 0 {
		opts = append(opts, decision.Option{Index: len(opts), Kind: learnKindRummage,
			Label: "Discard a card, then draw a card", Player: p})
	}
	return &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1, Source: source,
		ResumeKind: "learn", ResumeSA: sa, Prompt: "Learn", Options: opts}
}

func learnName(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return "a card"
}

// learnTakeLesson reveals the Lesson (a public ids Note, the encoding every
// reveal here uses) and moves it from outside the game into p's hand. An
// answer that no longer names an owned sideboard card moves nothing.
func learnTakeLesson(h Host, p state.PlayerID, id state.ObjID) {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZSideboard || o.Owner != p {
		return
	}
	h.Emit(events.Event{Kind: events.Note, Player: p, IDs: []state.ObjID{id}})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZSideboard, To: state.ZHand})
}

// learnRummage is the "discard a card to draw a card" branch: p picks the
// discard from their hand; the draw happens only when a card was discarded.
func learnRummage(h Host, c *Ctx, sa *cards.SA, p state.PlayerID) {
	g := h.Game()
	hand := zoneOf(g, state.ZHand, p)
	if len(hand) == 0 {
		return
	}
	opts := make([]decision.Option, 0, len(hand))
	for _, id := range hand {
		opts = append(opts, decision.Option{Index: len(opts), Kind: learnKindDiscard,
			Label: "Discard " + learnName(g, id), Obj: id, Player: p})
	}
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1, Source: c.Source,
		ResumeKind: "learn_discard", ResumeSA: sa, Prompt: "Choose a card to discard", Options: opts}
	pick := hand[0]
	if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
		pick = ans[0].Obj
	}
	if o := g.Obj(pick); o == nil || o.Zone != state.ZHand || o.Owner != p {
		return
	}
	h.Emit(events.Discard(pick, p))
	drawFor(h, p, -1, sa, drawUptoRider{})
}
