package eventhost

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
)

// gameSeedSalt is draftsim's per-game seed salt: the fixed top bits of the
// PCG stream key every game table is seeded from (cmd/draftsim/event.go).
const gameSeedSalt = 0x6A111E

// gameSeed derives one game table's seed from the event seed.
func gameSeed(seed uint64, round, a, b, game, attempt int) uint64 {
	salt := gameSeedSalt<<32 | uint64(round)<<24 | uint64(a)<<20 |
		uint64(b)<<16 | uint64(game)<<8 | uint64(attempt)
	return rand.New(rand.NewPCG(seed, salt)).Uint64()
}

// gameTableID names one game attempt's OnDemand host table. The attempt
// suffix makes every attempt's table unique, so a replayed game never
// reuses a table id whose match rows are still served.
func gameTableID(ev string, round, a, b, game, attempt int) host.TableID {
	return host.TableID(fmt.Sprintf("%s-r%d-s%d%d-g%d-a%d", ev, round, a, b, game, attempt))
}

// startGame logs and launches one series' next game attempt on its own
// OnDemand table — game 1 of a Bo1 series, or the next game
// of a Bo3 series on a NEW series table (its game 1, a retry after a
// crash, or a between-games resume). A Bo3 game continuing on the event's
// live series table is logged by advanceRounds instead, so the table keeps
// playing. The caller holds r.mu. The GameStarted entry is logged BEFORE
// the table exists, so a crash between the log line and AddTable leaves an
// in-flight attempt that resume aborts — and a table id is never reused,
// because the next attempt takes the next attempt number. Any failure
// after the log line aborts the attempt, blocks the event (no more games
// start until the process restarts; resume clears blocked) and returns
// the error.
func (r *run) startGame(sp event.SeriesSpec, game int) error {
	o := r.h.opts
	a, b := sp.A, sp.B
	attempt := r.ev.NextAttempt(a, b)
	id := gameTableID(r.meta.ID, sp.Round, a, b, game, attempt)
	seed := gameSeed(r.meta.Seed, sp.Round, a, b, game, attempt)
	series := r.meta.BestOf == 3
	var chooser *int
	if series {
		c := r.nextChooser(a, b, game, seed)
		chooser = &c
	}
	if err := r.apply(event.Entry{
		Kind: event.KindGameStarted, Round: sp.Round, A: a, B: b,
		Game: game, Attempt: attempt, Table: string(id), GameSeed: seed,
		Chooser: chooser,
	}); err != nil {
		return err
	}
	fail := func(err error) error {
		_ = r.apply(event.Entry{
			Kind: event.KindGameAborted, Round: sp.Round, A: a, B: b,
			Game: game, Attempt: attempt, Reason: "start: " + err.Error(),
		})
		r.h.mu.Lock()
		delete(r.h.tables, id)
		r.h.mu.Unlock()
		if series {
			delete(r.seriesTable, [2]int{a, b})
		}
		delete(r.tokens, id)
		r.blocked = err.Error()
		return err
	}
	revA, revB := r.ev.ReadyRev(a), r.ev.ReadyRev(b)
	if game > 1 {
		if v, ok := r.ev.SeriesState(a, b); ok {
			revA, revB = v.Locked[0], v.Locked[1]
		}
	}
	var humanSlots []int
	for i, s := range []int{a, b} {
		if r.meta.Seats[s].Kind == "human" {
			humanSlots = append(humanSlots, i)
		}
	}
	for _, slot := range humanSlots {
		if o.MintSeat == nil {
			return fail(errors.New("eventhost: Options.MintSeat is required to seat a human"))
		}
		tok, err := o.MintSeat(id, slot)
		if err != nil {
			return fail(err)
		}
		// Each human slot gets its own game token, so both humans of
		// a series join their own side.
		toks := r.tokens[id]
		toks[slot] = tok
		r.tokens[id] = toks
	}
	ref := gameRef{run: r, round: sp.Round, a: a, b: b, game: game, attempt: attempt}
	if series {
		ref.series, ref.firstGame = true, game
	}
	r.h.mu.Lock()
	r.h.tables[id] = ref
	r.h.mu.Unlock()
	if series {
		r.seriesTable[[2]int{a, b}] = id
	}
	cfg := host.TableConfig{
		ID:    id,
		Name:  fmt.Sprintf("%s round %d: %s vs %s", r.meta.ID, sp.Round, r.nameOf(a), r.nameOf(b)),
		Seats: 2,
		// Decks [b, a] + match 1's rotation seats event seat a at table
		// seat 0 and b at seat 1, matching PlayerNames and SeatBotPolicies.
		// A series table's hook supplies the per-seat decks instead, and
		// bypasses the rotation; these stay for validation and are this
		// game's revisions.
		Decks: []string{
			DeckID(r.meta.ID, b, revB),
			DeckID(r.meta.ID, a, revA),
		},
		Seed:            seed,
		PlayerNames:     []string{r.nameOf(a), r.nameOf(b)},
		SeatBotPolicies: []string{r.policyOrEmpty(a), r.policyOrEmpty(b)},
		Humans:          humanSlots,
		BotPolicy:       o.DefaultPolicy,
		Spectator:       view.Public,
		Format:          host.FormatConstructed,
		Mulligans:       o.Mulligans,
		AutoMana:        o.AutoMana,
		BotAutoPayMana:  o.BotAutoPayMana,
		BotCommonSense:  o.BotCommonSense,
		OnDemand:        true,
		Event:           r.meta.ID,
		Series:          series,
	}
	if err := o.Registry.AddTable(cfg); err != nil {
		return fail(err)
	}
	if err := o.Registry.Start(id); err != nil {
		return fail(err)
	}
	return nil
}

// nextChooser is a Bo3 game's play/draw chooser (the event seat): game 1
// the table's CR 103.1 toss winner (slot 0 is event seat a), later games
// MTR 2.2's chooser from the series view — the loser of the previous game,
// or that game's chooser after a draw. A -1 from the view (never reached
// while every Bo3 game logs its chooser) falls back to seat a, which the
// Apply path accepts when it cannot name an expected chooser.
func (r *run) nextChooser(a, b, game int, seed uint64) int {
	if game <= 1 {
		if host.MatchTossWinner(seed, 1, 2) == 0 {
			return a
		}
		return b
	}
	if v, ok := r.ev.SeriesState(a, b); ok && v.NextChooser >= 0 {
		return v.NextChooser
	}
	return a
}

// nameOf is one seat's display name: a bot's stored name, a human's joined
// name (games never start before every human seat has joined).
func (r *run) nameOf(seat int) string {
	return r.meta.Seats[seat].Name
}

// policyOrEmpty is a seat's game policy, or "" on a human slot (its table
// caretaker follows TableConfig.BotPolicy instead).
func (r *run) policyOrEmpty(seat int) string {
	if r.meta.Seats[seat].Kind == "human" {
		return ""
	}
	return r.meta.Seats[seat].Policy
}

// advanceRounds drives the ROUND(1..3) phases: it flushes buffered game
// results into the log in pairing order, starts every pending series' next
// game attempt, and closes the event once round 3 is over. It is called
// under r.mu and reports whether it applied anything. A blocked event
// starts no more games until the process restarts.
//
// A Bo3 event drives its series through advanceSeriesBo3 instead of
// the Bo1 body; a bot-only Bo3 event processes only the first pending
// series per pass — its series' tables start one at a time, and advance's
// loop drains the rest.
func (r *run) advanceRounds() bool {
	switch r.ev.Phase() {
	case event.RoundPhase(1), event.RoundPhase(2), event.RoundPhase(3):
	default:
		return false
	}
	r.h.mu.Lock()
	closing := r.h.closed
	r.h.mu.Unlock()
	if closing || r.blocked != "" {
		return false
	}
	applied := r.flush()
	first := true
	for _, sp := range r.ev.PendingSeries() {
		if r.meta.BestOf == 3 {
			if r.botOnly() && !first {
				break
			}
			first = false
			p, err := r.advanceSeriesBo3(sp)
			if err != nil {
				return applied
			}
			applied = applied || p
			continue
		}
		if !r.ev.Started(sp.A, sp.B) {
			if err := r.apply(event.Entry{
				Kind: event.KindSeriesStarted, Round: sp.Round, A: sp.A, B: sp.B, BestOf: 1,
			}); err != nil {
				return applied
			}
			applied = true
		}
		if _, buffered := r.outcomes[[2]int{sp.A, sp.B}]; buffered {
			continue
		}
		live := false
		for _, att := range r.ev.InFlight() {
			if att.A == sp.A && att.B == sp.B {
				live = true
				break
			}
		}
		if live {
			continue
		}
		if err := r.startGame(sp, 1); err != nil {
			return applied
		}
		applied = true
	}
	if r.ev.Phase() == event.RoundPhase(3) && len(r.ev.PendingSeries()) == 0 {
		if err := r.apply(event.Entry{Kind: event.KindEventEnded}); err != nil {
			return applied
		}
		applied = true
	}
	return applied
}

// botOnly reports whether every seat is a bot.
func (r *run) botOnly() bool {
	for _, ms := range r.meta.Seats {
		if ms.Kind == "human" {
			return false
		}
	}
	return true
}

// advanceSeriesBo3 advances one best-of-three series and reports whether
// it applied anything. One entry at a time, in series order: SeriesStarted,
// then each game's GameStarted, then — between games — each side's
// Sideboarded, then the next GameStarted, then the series' SeriesEnded.
// The series' own entries are a pure function of the seed and the game
// outcomes, so a restart mid-series resumes on the same path. A human side
// still to sideboard waits here; a bot side
// not locked keeps the deck it played last game.
func (r *run) advanceSeriesBo3(sp event.SeriesSpec) (bool, error) {
	if !r.ev.Started(sp.A, sp.B) {
		if err := r.apply(event.Entry{
			Kind: event.KindSeriesStarted, Round: sp.Round, A: sp.A, B: sp.B, BestOf: 3,
		}); err != nil {
			return false, err
		}
		return true, nil
	}
	v, ok := r.ev.SeriesState(sp.A, sp.B)
	if !ok {
		return false, nil
	}
	if v.InFlight != nil {
		return false, nil
	}
	if v.Started && v.NextGame == 0 {
		// The series is over — its table is done, so the live
		// series table row goes too (the hook's stop answer drops the
		// table's ref; this drops the run's row).
		delete(r.seriesTable, [2]int{sp.A, sp.B})
		if err := r.apply(event.Entry{
			Kind: event.KindSeriesEnded, Round: sp.Round, A: sp.A, B: sp.B, Score: v.Wins,
		}); err != nil {
			return false, err
		}
		return true, nil
	}
	if !v.Between {
		// Not between games and no attempt in flight: game 1 is about to
		// start, or a crashed attempt is restarting — either way on a new
		// table. (A between-games game continues on the series' own table
		// below.)
		if err := r.startGame(sp, v.NextGame); err != nil {
			return false, err
		}
		return true, nil
	}
	game := v.NextGame
	for side, seat := range []int{sp.A, sp.B} {
		if v.Locked[side] != 0 {
			continue
		}
		if r.meta.Seats[seat].Kind != "bot" {
			return false, nil // a human side waits until it sideboards
		}
		if err := r.apply(event.Entry{
			Kind: event.KindSideboarded, Round: sp.Round, A: sp.A, B: sp.B,
			Game: game, Seat: seat, Rev: v.PlayRevs[side],
		}); err != nil {
			return false, err
		}
		return true, nil
	}
	// Both sides locked: game 2/3 starts. On the series' own table it is
	// logged here; a game whose table died restarts on a new
	// table through startGame.
	id, hasTable := r.seriesTable[[2]int{sp.A, sp.B}]
	v, ok = r.ev.SeriesState(sp.A, sp.B)
	if !ok {
		return false, nil
	}
	if hasTable {
		if _, derr := r.h.opts.Store.readDeck(r.meta.ID, sp.A, v.Locked[0]); derr != nil {
			r.blocked = fmt.Sprintf("event %s: seat %d deck rev %d unreadable: %v", r.meta.ID, sp.A, v.Locked[0], derr)
			return false, nil
		}
		if _, derr := r.h.opts.Store.readDeck(r.meta.ID, sp.B, v.Locked[1]); derr != nil {
			r.blocked = fmt.Sprintf("event %s: seat %d deck rev %d unreadable: %v", r.meta.ID, sp.B, v.Locked[1], derr)
			return false, nil
		}
		seed, okSeed := r.tableSeed(id)
		if !okSeed {
			r.blocked = fmt.Sprintf("event %s: series table %s has no GameStarted to resume from", r.meta.ID, id)
			return false, nil
		}
		chooser := v.NextChooser
		if chooser < 0 {
			chooser = sp.A
		}
		if err := r.apply(event.Entry{
			Kind: event.KindGameStarted, Round: sp.Round, A: sp.A, B: sp.B,
			Game: game, Attempt: r.ev.NextAttempt(sp.A, sp.B), Table: string(id),
			GameSeed: seed, Chooser: &chooser,
		}); err != nil {
			return false, err
		}
		return true, nil
	}
	// The series' table is gone (its match crashed): the game restarts on
	// a fresh table with the same locked decks.
	if err := r.startGame(sp, game); err != nil {
		return false, err
	}
	return true, nil
}

// abortInFlight aborts every game attempt a restart caught in flight, so
// the first advance starts each one again on a fresh table with a fresh
// seed. It is called under r.mu from New, before the first advance.
func (r *run) abortInFlight() {
	for _, att := range r.ev.InFlight() {
		if err := r.apply(event.Entry{
			Kind: event.KindGameAborted, Round: att.Round, A: att.A, B: att.B,
			Game: att.Game, Attempt: att.Attempt, Reason: "server restart",
		}); err != nil {
			return
		}
	}
}
