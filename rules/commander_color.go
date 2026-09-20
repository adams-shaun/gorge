package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The CR 903.4b commander colour-choice round: a commander whose printed
// characteristic-defining static is `SetColor$ ChosenColor` (Faceless One,
// The Prismatic Piper, Clara Oswald) says "choose a color before the game
// begins". The round runs between the opening deal and turn 1, BEFORE the
// London mulligan round (both are pregame; the choice is made before the game
// begins, so it precedes every other pregame ask), and records its answer on
// the commander object with the SAME `events.Choose {Counter: "color"}` event
// the as-enters colour ask emits -- no event-schema change, and
// events/apply.go folds it onto Object.ChosenColor exactly like an ETB choice.
//
// The round's state is plain data (colorRound), so Engine.Clone and replay
// reproduce it. A game with no qualifying commander never opens the round, so
// every existing golden replay is byte-identical by construction.

// colorRound is the pregame commander colour-choice round's plain-value state:
// one ask per qualifying (seat, commander) pair in `AliveFrom(StartingPlayer)`
// order, and a cursor. asks[i] is the i-th ask; cursor is the next one to
// pose. Never a closure, so Clone copies it like the mulligan round.
type colorRound struct {
	asks   []colorAsk
	cursor int
}

// colorAsk is one qualifying commander's pending or answered ask.
type colorAsk struct {
	player state.PlayerID
	cmd    state.ObjID
}

// chooseCommanderColor is the pregame round's KChoose flow marker. It is
// deliberately outside every other chooseFor range (25, past
// chooseTriggeredMandatory = 24) and carries no resume point, so
// handleChoose's mid-resolution guard does not route it away.
const chooseCommanderColor chooseFor = 25

// newColorRound builds the round from the live command zones: for each seat in
// AliveFrom(StartingPlayer) order, each of the seat's commanders whose first
// face carries the chosen-colour CDA (cards.Face.CommanderColourChoiceCDA, the
// same gate the layer-5 scan and the identity derivation key on). An empty
// round means no seat qualifies and genesis skips the round entirely.
func (e *Engine) newColorRound() colorRound {
	var r colorRound
	for _, p := range e.G.AliveFrom(e.G.StartingPlayer) {
		if int(p) >= len(e.G.Players) {
			continue
		}
		for _, cmd := range e.G.Players[p].Commanders {
			o := e.G.Obj(cmd)
			if o == nil || o.Card == nil {
				continue
			}
			f := o.Card.Faces[0]
			if f == nil || !f.CommanderColourChoiceCDA() {
				continue
			}
			r.asks = append(r.asks, colorAsk{player: p, cmd: cmd})
		}
	}
	return r
}

// stepColorRound issues the round's next ask, or hands to the rounds that
// follow it. step() calls it once per engine step while e.coloring, so there
// is never more than one ask outstanding.
func (e *Engine) stepColorRound() {
	if e.G.Over {
		return
	}
	r := &e.colorRound
	if r.cursor >= len(r.asks) {
		e.coloring = false
		e.colorRound = colorRound{}
		e.startMulliganOrTurn()
		return
	}
	a := r.asks[r.cursor]
	o := e.G.Obj(a.cmd)
	name := "your commander"
	if o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	opts := make([]decision.Option, len(etbColourLabels))
	for i, cl := range etbColourLabels {
		opts[i] = decision.Option{Index: i, Kind: "color", Label: cl.name}
	}
	e.choosing = chooseCommanderColor
	e.ask(decision.New(a.player, decision.KChoose,
		"Choose a color for "+name+" before the game begins", 1, 1, opts))
}

// answerCommanderColor records the answered colour on the commander object and
// advances the round. The recorded event is the ETB ask's own shape, so
// events/apply.go's existing `o.ChosenColor = e.Text` fold reads it and the
// layer-5 SetColor$ ChosenColor static and commanderIdentityColours both see
// it. The letter rides Option.Label ("White".."Green") through the shared
// etbColourLetter mapping, never the choice index.
func (e *Engine) answerCommanderColor(d *decision.Decision, chosen []decision.Option) {
	r := &e.colorRound
	if r.cursor >= len(r.asks) {
		e.choosing = chooseNone
		return
	}
	a := r.asks[r.cursor]
	if len(chosen) == 1 {
		if letter := etbColourLetter(chosen[0].Label); letter != "" {
			e.emit(events.Event{Kind: events.Choose, Obj: a.cmd, Counter: "color", Text: letter})
		}
	}
	r.cursor++
	e.choosing = chooseNone
	e.stepColorRound()
}
