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

// Format is the game's construction format. FormatConstructed is the zero
// value -- every existing Config that never sets it (all today's fixtures
// and every non-Commander game) is unchanged.
type Format int

const (
	FormatConstructed Format = iota
	FormatCommander
)

type Config struct {
	Seed  uint64
	Names []string
	Decks [][]*cards.Card
	// Format names the construction format. Zero means Constructed; the other
	// tasks in the Commander milestone (the tax, CR 903.9, commander damage)
	// read it. This task is plumbing: it reads Commanders and StartingLife
	// only.
	Format Format
	// StartingLife is a game's opening life total. 0 (the zero value) means
	// the existing 20, so every Config that never sets it observes exactly
	// what it always did.
	StartingLife int32
	// Commanders holds, for each seat i, the indices into Decks[i] that are
	// that seat's commanders; nil or absent means none. Genesis places those
	// objects in the command zone instead of the library. An index out of
	// range for its deck is a config error degraded the same way New degrades
	// more decks than seats: the offending entry is skipped, not a crash.
	Commanders [][]int
	// Mulligans is the number of London mulligans each player may take in the
	// pre-game round between the opening deal and turn 1. 0 (the zero value)
	// skips the round entirely, so every Config that never sets it is
	// unchanged (all standalone fixture Configs). It sits in the same Config
	// replay is handed, so a replay reproduces the round (Ruling R-8.4): the
	// acceptance config sets it so the 12-deck suite exercises keep/mulligan
	// and bottoming.
	Mulligans int
	// Tokens is the token definitions the decks in this match can create --
	// cards.Registry.Tokens. Copied onto Game.Tokens in New so
	// events.Apply's TokenCreate case has something to mint from. Replay
	// must pass the same table a live match's Config did.
	Tokens map[string]*cards.Card
}

type Engine struct {
	G *state.Game
	L *events.Log

	// format is the construction format New was configured with (Config.
	// Format). It is the explicit gate the Commander rules (the tax, CR
	// 903.9, commander damage) check -- "in a non-Commander game none of
	// this runs at all" -- rather than inferring the format from the
	// incidental shape of the zones. Plain Format value; Clone copies it so
	// a cloned Commander engine still gates its command-zone rules.
	format Format

	rng     *rng
	pending *decision.Decision

	// continuous holds every registered continuous effect, live or expired.
	// The layer system (layers.go) is the only reader and writer.
	continuous []ContinuousEffect

	// pregame is true while the London mulligan round runs, between the
	// opening deal and turn 1. Config.Mulligans > 0 sets it in New; step()
	// dispatches to stepPregame (rules/mulligan.go) while it is true, and the
	// round's end clears it and hands to beginTurn. Bool field, so Clone
	// copies it like every other value field.
	pregame bool
	// mulligan is the round's plain-value state (rules/mulligan.go) -- seats,
	// kept/taken counts and the phase cursor. Never a closure, so Clone copies
	// it like cast/choosing.
	mulligan mulliganRound

	// staticContinuous memoizes the S:Mode$ Continuous statics on battlefield
	// permanents (layers.go's staticEffects), keyed on staticEpoch. staticEpoch
	// is the log length at the last build; the memo refreshes once per emitted
	// event rather than once per Derived() call, because most events
	// (Priority/Damage/Mana) leave the battlefield permanent set untouched
	// while Derived is the hottest path in a turn (every legal action, combat
	// step and trigger predicate reads it). Clone() leaves both fields zero, so
	// a cloned engine rebuilds the memo on its first Derived -- staticEffects
	// is a pure function of the current board, so the rebuilt result is
	// identical and deterministic.
	staticContinuous []ContinuousEffect
	staticEpoch      int

	// activeBuf is the cached, fully CR-613-sorted result of layers.go's
	// active(), the effect list every Derived() call ranges over for every
	// object of every board build and projection. Rebuilding that sorted list
	// once per emitted event instead of once per Derived() call is the whole
	// saving here -- active() used to allocate a fresh slice per call, and
	// Derived is the hottest path in a turn. The cache key is the pair
	// (activeEpoch, activeVersion): activeEpoch is the log length at the last
	// build and activeVersion the continuousVersion (bumped by AddContinuous
	// and EndOfTurnCleanup), because the effect list is a pure function of the
	// current board plus e.continuous, and those are exactly the two inputs
	// the key captures -- every board change moves the log head (emit), and
	// e.continuous changes through exactly the two mutators above. activeDepth
	// is a re-entry guard (Task A2's forEachObject pattern): it lets a nested
	// Derived (HasKeyword inside a MatchesSpecFrom) atomically share the
	// cached list and, on the never-happens-in-practice rebuild-mid-range
	// path, build a private list instead of clobbering the outer call's.
	// Clone() copies none of these fields (see clone.go); a cloned engine
	// starts with a zero key and rebuilds identically on its first Derived.
	activeBuf     []ContinuousEffect
	activeEpoch   int
	activeVersion int
	activeDepth   int
	// continuousVersion is bumped by every direct mutation of e.continuous
	// (layers.go's AddContinuous and EndOfTurnCleanup). It stands in for the
	// events a board change would signal through the log head: while
	// AddContinuous also emits a ClockTick, EndOfTurnCleanup rewrites
	// e.continuous in place with no event, and the active() cache must see
	// that drop (an UntilEOT pump expiring) even though the log head did not
	// move. A zero value is never taken as a valid cache hit across rebuilds
	// because active() guards hits on version as well.
	continuousVersion int

	// derivedKW / derivedTypes are Derived's scratch keyword and type buffers
	// (rules/layers.go): the full Derived(struct) build rewrites them in place
	// so repeated derived-characteristic reads do not allocate. They are pure
	// per-call scratch, rebuilt from the face and active() every call, so they
	// carry no cross-call state beyond capacity; Clone() copies none of them
	// (see clone.go), so a cloned engine grows its own — never aliasing the
	// original's mutable scratch, exactly the A2 buffer / C3 digest precedent.
	// derivedDepth is the re-entry guard for the reuse (the A2/active()
	// pattern): a nested Derived mid-build owns private buffers instead of
	// clobbering the outer build's.
	derivedKW    []string
	derivedTypes []string
	derivedDepth int

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

	// format is the game's construction format, captured from Config at New.
	// Task m33 (commander damage, CR 903.10) needs it at combat-damage time
	// and in the state-based-action pass, long after New has returned, so it is
	// stored here rather than re-derived. It is a plain value, so Clone copies
	// it; a Commander-format game keeps commanding damage and its loss
	// condition through a clone exactly as the live one does.
	format Format

	// resume is non-nil while a mid-resolution decision is pending: an
	// effect (effCharm's modal pick, effCopySpellAbility's UnlessCost$
	// may-pay — M2d-2, closing R-8) asked through effects.Host.Ask and the
	// resolution of the top-of-stack object is suspended with the object
	// still on the stack. It is plain value/pointer data (resumePoint:
	// kind, obj and the shared-immutable *cards.SA that asked), never a
	// closure, so Clone copies it like cast/choosing and a replay re-derives
	// the same branch. resolveTop checks it after each resolution pass;
	// handleModes clears it and calls resumeResolution (rules/resolution.go)
	// with the recorded answer. Nil whenever no resolution is suspended.
	resume *resumePoint

	// cast holds the in-progress cast-flow state while choosing ==
	// chooseCast (Task 9, rules/cast.go). Nil whenever no cast is mid-flow.
	cast *pendingCast

	// suppressedCast holds the card object ids whose cast option is held out
	// of the current priority window because their last cast attempt aborted
	// unpayable with no state change (E2 round 2: see commitCast). This is
	// the no-progress answer for a hash-chained, replayable engine: instead
	// of counting no-progress aborts and killing the match, suppress the
	// exact card that produced one, so the re-offer loop cannot even begin a
	// second iteration -- nobody's match dies, and the seat may still do
	// anything else. Cleared on any genuinely state-changing event (see
	// emit), so a declined card's option comes back the moment the window
	// ends or the mana/board changes. The id already names the one seat that
	// holds it, so two different cards' declines never interact and two
	// seats' never do either.
	suppressedCast map[state.ObjID]bool

	// drainAwaitsTarget is true while a decision asked from inside the trigger
	// drain is pending, so its answer resumes the drain rather than granting
	// priority. Task 7 sets it for a TargetMin/TargetMax-bearing triggered
	// ability's own KTarget decision (pushTrigger, cleared by handleTarget);
	// Task 18 sets it for a Miracle cast's own X/Delve/Sac decision
	// (castMiracle, cleared by handleChoose once the cast commits). When set,
	// the handler for the pending decision calls resumeTriggerDrain (the same
	// continuation handleTriggerOrder uses) instead of granting the caster
	// priority, so a later, unrelated trigger in the same batch is still
	// placed before any player acts. Plain scalar, so Clone copies it like
	// every other field here. (Tasks 7, 18.)
	drainAwaitsTarget bool

	// damaging names the source object responsible for the damage emit
	// currently in flight (CR 609.7a): the resolution source for a spell or
	// ability being resolved, or the dealing creature for a combat
	// assignment. Task 15 sets it around resolveTop's two resolution
	// calls and combat's assignment loop, and emit consults it to prevent
	// damage to a protection-bearer whose protecting quality the source
	// carries (CR 702.16d). Zero when nothing is resolving/assigning damage;
	// a zero damaging never suppresses a Damage event.
	//
	// Not copied by Clone (Task 15 fix round 1, M5): Clone runs only at an
	// intent boundary, after New/Advance/Submit has returned, at which point
	// every resolution and damage-step that EVER sets damaging has completed
	// and reset it to zero -- an unset non-zero damaging would mean an emit
	// was still in flight, which is exactly the boundary Clone is prohibited
	// from crossing. So the field is always zero at a clone boundary and
	// copying it would copy a constant.
	damaging state.ObjID

	// foreachBuf is forEachObject's (trigger_match.go) scratch snapshot
	// buffer. forEachObject copies each zone into it before walking it -- fn
	// may move objects between zones (a trigger match putting something on
	// the stack), so iterating the live, mutating zone slice would be a bug.
	// append(buf[:0], zone...) grows it in place, so it settles at the size
	// of the largest zone seen and then stops allocating -- a fresh zone
	// copy per zone per event used to be forEachObject's 11.51 GB allocation
	// footprint (Task A2), the single largest allocator in this package.
	// Owned by this Engine alone: Clone leaves both fields zero, so a clone
	// and the original share no snapshot mid-walk (the clone just lets it
	// grow again). foreachDepth guards re-entry (see forEachObject): zero
	// outside a walk, one inside the depth-0 walk, higher inside a
	// re-entrant nested walk.
	foreachBuf   []state.ObjID
	foreachDepth int
}

// chooseFor names the flow a pending KChoose decision belongs to. Task 9
// declares chooseCast (rules/cast.go); Tasks 12 and 18 add the "as this
// enters" and miracle cases in their own files.
type chooseFor uint8

const chooseNone chooseFor = iota

// commandersFor returns the VALID commander indices (into deck of length
// deckLen) that Config names for seat i, in Config order. An index out of
// range for the deck, or a seat with no Commanders entry, contributes
// nothing -- the same degrade-don't-crash stance New already takes for more
// decks than seats: the bogus entry is skipped, never a panic.
func (c *Config) commandersFor(i, deckLen int) []int {
	if i < 0 || i >= len(c.Commanders) {
		return nil
	}
	var out []int
	for _, idx := range c.Commanders[i] {
		if idx >= 0 && idx < deckLen {
			out = append(out, idx)
		}
	}
	return out
}

const openingHand = 7

func New(cfg Config) *Engine {
	life := int32(20)
	if cfg.StartingLife > 0 {
		life = cfg.StartingLife
	}
	e := &Engine{
		G:      state.NewGameLife(cfg.Names, life),
		L:      events.NewLog(cfg.Seed),
		format: cfg.Format,
		rng:    newRNG(cfg.Seed),
	}
	e.G.Tokens = cfg.Tokens
	e.format = cfg.Format
	e.emit(events.Event{Kind: events.GameStart, Amount: int32(len(cfg.Names))})
	// Match-wide dense commander indexing for Player.CmdDamage (assigned at
	// genesis): a commander's dense index is the sum of (valid commanders in
	// seats before its owner) + (its own position within its owner's
	// Commanders list, which is the order Commanders is built in the loop
	// below) -- both deterministically derivable from this Config, so no
	// separate index needs storing. Every seat's CmdDamage is sized to total
	// (the whole match's commander count) so it can be indexed by ANY
	// commander's match-wide dense index: seat B's damage holds a slot for
	// seat A's commander at A's commander's dense index. Sized here and
	// never grown; three small copy() calls per seat carry all three across
	// Game.Clone.
	totalCmd := 0
	for i := range cfg.Names {
		if i < len(cfg.Decks) {
			totalCmd += len(cfg.commandersFor(i, len(cfg.Decks[i])))
		}
	}
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
		// Commanders leave the library for the command zone here, BEFORE the
		// shuffle and BEFORE the opening hand is dealt, so they are neither
		// shuffled into the library nor drawable. Emitted as real MoveZone
		// events (one per commander, in Config order) -- the log is the only
		// source of truth, and replay, which folds the logged events back
		// through this same New, reproduces the identical command zone. For a
		// non-Commander Config commandersFor is empty, so nothing is emitted
		// and the Shuffle below covers the whole library exactly as before.
		var myCmds []state.ObjID
		for _, idx := range cfg.commandersFor(i, len(deck)) {
			id := ids[idx]
			myCmds = append(myCmds, id)
			e.emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: state.ZCommand})
		}
		e.G.Players[p].Commanders = myCmds
		if len(myCmds) > 0 {
			e.G.Players[p].CmdCasts = make([]int32, len(myCmds))
		}
		if totalCmd > 0 {
			e.G.Players[p].CmdDamage = make([]int32, totalCmd)
		}
		// Shuffle only what is left in the library -- the commanders have just
		// moved out, so a non-Commander seat's library and the original ids
		// are one and the same and the event is byte-identical to before.
		order := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
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
	if !e.G.Over {
		if cfg.Mulligans > 0 {
			// Ruling R-8.4: the London mulligan round lives between the deal
			// and turn 1. e.pregame makes step() dispatch to stepPregame
			// (rules/mulligan.go) instead of the ordinary turn steps; the
			// round's end calls beginTurn below. Over is already false (the
			// per-seat deck-out guard above returned early) -- a game that
			// ended during the deal never starts a round.
			e.pregame = true
			e.mulligan = mulliganRound{
				seats: alive, kept: make([]bool, len(alive)),
				taken: make([]int, len(alive)), limit: cfg.Mulligans,
			}
		} else {
			e.beginTurn(alive[0])
		}
	}
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
	// Task 15 protection (CR 702.16d/e): a Damage event dealt to a
	// protection-bearer by a source it is protected from is prevented -- the
	// damage never happens, reported as a Note rather than silently dropped.
	// Same for an Attach whose target is protected from the attachment -- CR
	// 702.16e: attachment points defer to protection before an Equip/Enchant
	// resolves onto a protected permanent. Both are checked BEFORE
	// replacement substitution: prevention is unconditional and must not be
	// handed to card text as if it had actually happened (and the recursive
	// emit for the Note re-enters cleanly because a Note matches neither
	// clause). e.damaging is the source of in-flight damage; a zero damaging
	// (no source recorded) never suppresses a Damage event.
	if ev.Kind == events.Damage && ev.Obj != 0 && e.damaging != 0 &&
		e.protectedFrom(ev.Obj, e.damaging) {
		return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Text: "prevented: protection"})
	}
	if ev.Kind == events.Attach && ev.Obj != 0 && len(ev.IDs) > 0 &&
		e.protectedFrom(ev.IDs[0], ev.Obj) {
		// Task 15 fix round 1 (Important I1): this clause is defence-in-depth.
		// Under the five registered colour protections there is NO reachable
		// path that fires it -- askTarget withholds a coloured Aura from ever
		// targeting a protected permanent (CR 702.16c) so no such Attach is
		// ever offered to resolve, and for Equip the legalTargets fizzle in
		// resolveTop fires first (the equipment's source is colourless, so a
		// colour-protected permanent was never going to be non-legal by
		// protection anyway). The clause stays because it is the engine's one
		// guard for the moment a future task registers type or "everything"
		// protection (or an Aura that enters the battlefield pre-attached, or
		// any Attach emit produced without a targeting step) makes a actually
		// protected permanent the direct object of an Attach; removing it now
		// would silently re-open that hole. Do not test it by driving emit
		// directly -- that would prove only that the if-lookup works, not that
		// a game state reaches it.
		return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Text: "cannot attach: protected"})
	}
	if !e.applyingReplacement {
		if replaced, handled := e.applyReplacements(ev); handled {
			return replaced
		}
	}
	// LKI (CR 603.10 "look back in time") is captured HERE, before
	// events.Emit runs Apply and mutates the object -- a zone-change trigger
	// needs the object exactly as it was a moment ago (its counters, tapped
	// state, damage, controller, zone), not the reset state Move leaves it
	// in. See effects.Ctx.LKI.
	var lki *state.Object
	switch ev.Kind {
	case events.MoveZone, events.Draw, events.PutOnStack:
		if o := e.G.Obj(ev.Obj); o != nil {
			cp := o.CloneDeep()
			lki = &cp
		}
	}
	stored := events.Emit(e.G, e.L, ev)
	e.checkTriggers(stored, lki)
	// E2: any genuinely state-changing event proves the game is making
	// progress, so it clears the held-out cast suppression (suppressedCast,
	// see engine.go): a declined card's option comes back the moment the
	// game does anything else, which is exactly when ending the current
	// priority window (a spell resolves, a step or turn changes) and any
	// mana/board change both land. The four kinds a declined-Delve abort
	// emits -- a Priority regrant, the KChoose decision bookkeeping, and the
	// abort Note -- are excluded, so suppression survives only as long as
	// NOTHING else is happening, which is precisely the no-progress-interval
	// it is there to bound.
	if ev.Kind != events.Priority && ev.Kind != events.DecisionAsk &&
		ev.Kind != events.DecisionMade && ev.Kind != events.Note {
		e.suppressedCast = nil
	}
	return stored
}

func (e *Engine) Pending() *decision.Decision { return e.pending }

func (e *Engine) ask(d *decision.Decision) {
	// Option.Index/position identity (finding bi). Every decision that can
	// reach a seat flows through ask -- ask is what sets d.Seq and e.pending,
	// so a decision that skipped it is not pending and no seat can answer it
	// -- which makes this the ONE place the invariant is enforced for every
	// construction site at once, including the sites that never call
	// decision.New (all but mulligan's two build the struct literal directly;
	// effects' two mid-resolution asks reach e.pending through Engine.Ask,
	// which calls back into ask below). A mis-indexed list is a programming
	// error, not a bad client answer: Chosen resolves an intent by position,
	// so an option whose Index has drifted off its slot makes the engine
	// resolve a different option than the client named, silently. Panic here
	// rather than return an error because an error hands the caller the
	// choice to swallow it and ship the broken Decision to a seat -- exactly
	// the silent wrong-option failure the invariant exists to make
	// unrepresentable.
	for i := range d.Options {
		if d.Options[i].Index != i {
			panic(fmt.Sprintf("rules: decision option %d has Index %d, want position %d (%s)",
				i, d.Options[i].Index, i, d.Kind))
		}
	}
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
