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
	// Tokens is the token definitions the decks in this match can create --
	// cards.Registry.Tokens. Copied onto Game.Tokens in New so
	// events.Apply's TokenCreate case has something to mint from. Replay
	// must pass the same table a live match's Config did.
	Tokens map[string]*cards.Card
}

type Engine struct {
	G *state.Game
	L *events.Log

	rng     *rng
	pending *decision.Decision

	// continuous holds every registered continuous effect, live or expired.
	// The layer system (layers.go) is the only reader and writer.
	continuous []ContinuousEffect

	// pendingTriggers holds matched triggers not yet placed on the stack.
	// checkTriggers appends; putTriggersOnStack drains. Task 20 (trigger.go).
	pendingTriggers []pendingTrigger
	// orderedTriggers is how many LEADING entries of pendingTriggers have
	// already had their order settled by an answered KTriggerOrder decision
	// (or, for a lone trigger, by there being nothing to decide). It is the
	// whole of Task 27's resumable-drain state, and it exists because the
	// queue can grow while a controller is being asked: Submit runs handle,
	// then checkStateBased, then Advance, and checkStateBased (sba.go) is a
	// fixed-point loop whose PlayerLost/MoveZone/GameOver emits each run
	// checkTriggers. Appends land at the END; decisions are always about the
	// FRONT; and while this is non-zero putTriggersOnStack does not re-sort,
	// so a trigger that arrives mid-drain can neither be shuffled into a
	// group the player has already ordered nor make them order the same
	// triggers twice. Zero whenever pendingTriggers is empty.
	orderedTriggers int
	// applyingReplacement guards re-entrancy: while a replacement effect's
	// own resolution is running, nested emits skip the replacement check
	// entirely, so a replacement that re-emits a matching event cannot
	// replace itself again. Task 20.
	applyingReplacement bool
	// triggerFireCount and damageOnceFired are trigger.go's own bookkeeping
	// (cascade bound and the DamageDealtOnce once-per-turn gate); see there.
	triggerFireCount map[triggerKey]int32
	damageOnceFired  map[triggerKey]int32

	// choosing says which flow is waiting on the current KChoose decision
	// (Task 8). It is plain data, not a closure, so Engine.Clone (a sibling
	// branch, not yet in this worktree) can copy it like any other field --
	// a closure captured over this Engine's own pointers would not survive a
	// clone at all. handleChoose (turn.go) switches on it; Task 9 adds
	// chooseCast in cast.go, and Tasks 12 and 18 add the "as this enters" and
	// miracle cases in their own files.
	choosing chooseFor
}

// chooseFor names the flow a pending KChoose decision belongs to. Task 9
// declares chooseCast (rules/cast.go); Tasks 12 and 18 add the "as this
// enters" and miracle cases in their own files.
type chooseFor uint8

const chooseNone chooseFor = iota

const openingHand = 7

func New(cfg Config) *Engine {
	e := &Engine{
		G:   state.NewGame(cfg.Names),
		L:   events.NewLog(cfg.Seed),
		rng: newRNG(cfg.Seed),
	}
	e.G.Tokens = cfg.Tokens
	e.emit(events.Event{Kind: events.GameStart, Amount: int32(len(cfg.Names))})
	for i, deck := range cfg.Decks {
		if i >= len(cfg.Names) {
			// Ruling T22-m (fix round 2): a malformed Config with more
			// decks than named seats has nowhere to put the rest --
			// state.NewGame above sizes g.zones from len(cfg.Names) alone,
			// so PlayerID(i) here would index outside it and panic
			// (SetZone -> zoneIndex -> an out-of-range g.zones write).
			// Task 25 wires Config from a client, so a malformed one must
			// degrade, not crash the one goroutine running the whole
			// match; the excess decks are simply never dealt, the same
			// spirit as the zero-alive guard below for a Config with no
			// seats at all.
			break
		}
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
		// Ruling T22-c: a deck smaller than the opening hand decks its owner
		// out before genesis even finishes dealing -- drawCard's own
		// checkStateBased call (below) can now actually set Over true here,
		// where nothing could before Task 22 made losing real. Genesis used
		// to plough on regardless: shuffling and dealing the NEXT seat's
		// hand, then unconditionally calling beginTurn on a game already
		// over. Every real deck this build ships is far larger than
		// openingHand, so this is not reachable from ordinary play, only
		// from a deliberately tiny Config -- but New must not hand back an
		// Engine that has already both ended and kept moving.
		if e.G.Over {
			return e
		}
	}
	alive := e.G.AliveFrom(0)
	if len(alive) == 0 {
		// Ruling T22-e: nobody survived genesis to begin a turn for --
		// every deck too small to deal (the per-seat Over check above
		// covers the ordinary "someone lost, someone remains" case; this
		// is what happens when NO seat remains at all), or, the
		// pre-existing panic this closes as a side effect, a zero-seat
		// Config with no decks even attempted. checkGameOver's own "zero
		// alive" branch is exactly CR 104.4a's draw, so run it rather than
		// calling beginTurn(0) against a Players slice that may not even
		// have an index 0: that used to reach Zone(ZBattlefield, 0)'s
		// zoneIndex arithmetic against a zero-length g.zones and panic.
		e.checkGameOver()
		return e
	}
	// Ruling T22-f: begin with the first seat still alive, not always seat
	// 0 -- an early seat that decked out during its own opening draw (Over
	// still false, since other seats remain, but that seat's own Lost is
	// true) must not receive turn 1. A player already out of the game is
	// simply skipped in turn order everywhere else (NextAlive, priority);
	// this is genesis's own equivalent for the very first turn.
	e.beginTurn(alive[0])
	return e
}

// emit is the engine's single mutation entry point. Task 20 inserts
// replacement effects ahead of logging and trigger discovery behind it:
// applyReplacements runs first (skipped, via applyingReplacement, while
// another replacement's own resolution is already in flight, which is what
// keeps a self-replacing event from looping), and if it substitutes the
// event, the original is discarded rather than logged -- the substitute's
// own emit (from inside effects.Resolve) already logged whatever needed
// logging. Otherwise the event is logged and folded into state exactly as
// before, and checkTriggers then looks for anything it just made true.
func (e *Engine) emit(ev events.Event) events.Event {
	if !e.applyingReplacement {
		if replaced, handled := e.applyReplacements(ev); handled {
			return replaced
		}
	}
	stored := events.Emit(e.G, e.L, ev)
	e.checkTriggers(stored)
	return stored
}

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
//
// Ruling T22-d: this used to call checkGameOver alone -- correct as far as
// it went (an empty-library draw is itself a loss, via the PlayerLost
// DrawFor emits directly), but it skipped destroyLethalDamage and, now,
// checkLoseConditions' own permanent-removal sweep. checkStateBased runs
// both of those in addition to checkGameOver, so a player who decks out
// here has their battlefield cleaned up the same way one who hits 0 life
// does, rather than only on whatever later checkStateBased call happens to
// come next.
func (e *Engine) drawCard(p state.PlayerID) {
	effects.DrawFor(e, p)
	e.checkStateBased()
}

// cardsKeywordHead lets layers.go strip a keyword's parameters ("Equip:2" ->
// "Equip") without importing cards itself for one call.
func cardsKeywordHead(k string) string { return cards.KeywordHead(k) }
