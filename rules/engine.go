// Package rules is the authoritative rules engine: turn structure, priority,
// the stack, combat and state-based actions. It owns the only Game instance a
// match has and mutates it exclusively through events.Emit.
//
// The event log is NOT a complete match description by itself. Genesis --
// state.NewGame and the initial AddObject calls that build each player's
// deck into the object arena, both in New below -- runs before the log
// exists and legitimately bypasses events. Replaying L.Events alone
// reconstructs everything that happens from that point on (turn structure,
// zone moves, life, damage, priority and so on), but it can never recover
// deck contents or player names: those are never logged. A faithful replay
// needs the original Config together with the log, not the log alone.
package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

type Config struct {
	Seed  uint64
	Names []string
	Decks [][]*cards.Card
}

type Engine struct {
	G *state.Game
	L *events.Log

	rng     *rng
	pending *decision.Decision

	// continuous holds every registered continuous effect, live or expired.
	// The layer system (layers.go) is the only reader and writer.
	continuous []ContinuousEffect
}

const openingHand = 7

func New(cfg Config) *Engine {
	e := &Engine{
		G:   state.NewGame(cfg.Names),
		L:   events.NewLog(cfg.Seed),
		rng: newRNG(cfg.Seed),
	}
	e.emit(events.Event{Kind: events.GameStart, Amount: int32(len(cfg.Names))})
	for i, deck := range cfg.Decks {
		p := state.PlayerID(i)
		ids := make([]state.ObjID, 0, len(deck))
		for _, c := range deck {
			ids = append(ids, e.G.AddObject(c, p).ID)
		}
		e.G.SetZone(state.ZLibrary, p, ids)
		order := append([]state.ObjID(nil), ids...)
		e.rng.Shuffle(order)
		// Library order is hidden information: the event carries it because the
		// server needs it, and view projection redacts it for everyone else.
		e.emit(events.Event{Kind: events.Shuffle, Player: p, IDs: order, Secret: true})
		for j := 0; j < openingHand; j++ {
			e.drawCard(p)
		}
	}
	e.beginTurn(0)
	return e
}

func (e *Engine) emit(ev events.Event) { events.Emit(e.G, e.L, ev) }

func (e *Engine) Pending() *decision.Decision { return e.pending }

func (e *Engine) ask(d *decision.Decision) {
	d.Seq = uint64(len(e.L.Events))
	e.emit(events.Event{Kind: events.DecisionAsk, Player: d.Player, Text: string(d.Kind)})
	e.pending = d
}

// Advance runs engine work until a decision is required or the game ends.
func (e *Engine) Advance() {
	for !e.G.Over && e.pending == nil {
		e.step()
	}
}

// Submit applies a client's answer. Anything the engine did not offer is
// rejected, which is what keeps the client rules-ignorant.
func (e *Engine) Submit(in decision.Intent) error {
	if e.G.Over {
		return fmt.Errorf("game is over")
	}
	d := e.pending
	if d == nil {
		return fmt.Errorf("no decision pending")
	}
	if err := d.Validate(in); err != nil {
		return err
	}
	e.L.Intents = append(e.L.Intents, in)
	e.emit(events.Event{Kind: events.DecisionMade, Player: in.Player,
		Text: fmt.Sprintf("%s:%v", d.Kind, in.Choices)})
	e.pending = nil
	e.handle(d, in)
	e.checkStateBased()
	e.Advance()
	return nil
}

// drawCard draws for the turn structure, sharing effects.DrawFor with the
// Draw primitive so the draw step and a card that says "draw a card" can
// never disagree about what drawing means.
func (e *Engine) drawCard(p state.PlayerID) {
	effects.DrawFor(e, p)
	e.checkGameOver()
}

// cardsKeywordHead lets layers.go strip a keyword's parameters ("Equip:2" ->
// "Equip") without importing cards itself for one call.
func cardsKeywordHead(k string) string { return cards.KeywordHead(k) }
