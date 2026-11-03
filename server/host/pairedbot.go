package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// The paired bot: a human asks a bot to evaluate the decision in front of them.
//
// It runs off the game's path. A request copies the slice headers of the live
// log under the match's read lock and does everything else on its own: it
// replays the log into a private engine, rebuilds the seat's observation feed
// from the replay, and asks a fresh seat of the configured policy to search
// that seat's honest root, closed under the policy's belief. The match
// goroutine holds no reference to any of it, so the advice cannot reach an
// event, an intent or the hash chain, and a game no one asks about costs
// nothing.
//
// What the bot sees is what a closed-list hosted seat sees: its own list, what
// the seat observed, and worlds whose opponent cards come from the belief
// (server/host/closedlist.go). The note it writes goes to the requesting seat
// and is kept only until the table's next request.

// adviceWatch is how often a running search checks that its decision is still
// the pending one.
const adviceWatch = 50 * time.Millisecond

// ErrAdviceStale answers a request for a decision that is no longer pending,
// and a seat with nothing pending: the client's view lags the game, so a
// player's own answer can overtake the request.
var ErrAdviceStale = errors.New("The decision has moved on.")

// Words a player reads. A redeal's own refusal reasons can quote counts of
// hidden cards, and a search's reason is a report name, so neither reaches the
// seat.
const (
	adviceNotOffered    = "This server does not offer a paired bot."
	adviceNotLive       = "A paired bot advises a human in a live game."
	adviceNotSearched   = "Search weighs priority, attackers, blockers and single targets. This kind of decision is yours alone."
	adviceMidResolution = "This decision comes in the middle of an effect resolving, and search cannot look ahead from here without seeing hidden cards."
	adviceNoRoot        = "Search could not rebuild this position from what you can see."
	adviceNoLines       = "Search could not finish a single line from this position."
	adviceNoTime        = "Search ran out of time; these are the lines it finished."
	adviceNotPriced     = "Search cannot price the options here; this is what the bot's rules would do."
	adviceFailed        = "The bot could not evaluate this decision."
)

// adviceLinePlies is how many plies of the expected line a search reports
// after its recommended play.
const adviceLinePlies = 4

// PairedBotOff is the TableConfig.PairedBot of a table that wants no paired bot.
const PairedBotOff = "off"

// ValidatePairedBot refuses an entry whose seats could read an opponent's list
// or could not explain a search. Like closedListGuard it checks a built seat,
// not the declaration.
func ValidatePairedBot(e bots.Entry, o Options) error {
	if e.New == nil {
		return fmt.Errorf("host: paired bot %q has no factory", e.Name)
	}
	s, err := e.New(bots.Options{Seed: 1, AutoPayMana: true, Deps: o.BotDeps, Config: o.BotConfig[e.Name]})
	if err != nil {
		return fmt.Errorf("host: paired bot %q: %w", e.Name, err)
	}
	if _, ok := s.(bots.EnvSeat); !ok {
		return fmt.Errorf("host: paired bot %q does not search", e.Name)
	}
	if _, ok := s.(explain.Emitter); !ok {
		return fmt.Errorf("host: paired bot %q cannot explain its decisions", e.Name)
	}
	if cl, ok := s.(searchseat.ClosedListSeat); !ok || cl.SearchBelief() == nil {
		return fmt.Errorf("host: paired bot %q would read its opponents' real decklists; only a closed-list seat may advise a human", e.Name)
	}
	return nil
}

// pairedBotOf is the bot table config c asks for, nil when it wants none or the
// server does not offer the one it names. A table's named bot can be gone after
// a restart with other flags.
func (r *Registry) pairedBotOf(c *TableConfig) *bots.Entry {
	switch c.PairedBot {
	case "":
		return r.opts.PairedBot
	case PairedBotOff:
		return nil
	}
	if e := r.opts.PairedBot; e != nil && e.Name == c.PairedBot {
		return e
	}
	for _, e := range r.opts.PairedBotChoices {
		if e.Name == c.PairedBot {
			return e
		}
	}
	return nil
}

// checkPairedBotChoice refuses a new table that names a bot this server does
// not offer or that could not advise at that table. Left to the default, an
// unsuitable table says why in PairedBotStatus instead.
func (r *Registry) checkPairedBotChoice(c *TableConfig) error {
	if c.PairedBot == "" || c.PairedBot == PairedBotOff {
		return nil
	}
	e := r.pairedBotOf(c)
	if e == nil {
		return fmt.Errorf("paired bot %q is not offered on this server", c.PairedBot)
	}
	if reason := pairedBotMisfit(e, c); reason != "" {
		return fmt.Errorf("paired bot %q: %s", c.PairedBot, reason)
	}
	return nil
}

// PairedBotChoices lists what a table may choose for TableConfig.PairedBot, the
// server's default first; empty when it offers no paired bot.
func (r *Registry) PairedBotChoices() []protocol.PairedBotChoice {
	if r.opts.PairedBot == nil {
		return nil
	}
	choice := func(e *bots.Entry) protocol.PairedBotChoice {
		return protocol.PairedBotChoice{Name: e.Name, Label: e.Label, Formats: e.Formats}
	}
	out := []protocol.PairedBotChoice{choice(r.opts.PairedBot)}
	for _, e := range r.opts.PairedBotChoices {
		out = append(out, choice(e))
	}
	return out
}

// pairedBotRefusal is why table t does not offer a paired bot, "" when it does.
func (r *Registry) pairedBotRefusal(t *table) string {
	e := r.pairedBotOf(&t.cfg)
	switch {
	case e == nil:
		return adviceNotOffered
	case t.review:
		return adviceNotLive
	}
	return pairedBotMisfit(e, &t.cfg)
}

// pairedBotMisfit is why bot e cannot advise at a table configured as c, "" when
// it can.
func pairedBotMisfit(e *bots.Entry, c *TableConfig) string {
	switch {
	case c.Engine != "":
		return fmt.Sprintf("A paired bot needs gorge; this table runs %q.", c.Engine)
	case len(c.Humans) == 0:
		return adviceNotLive
	case len(c.Humans) > 1 && !c.AllowPairedBot:
		return "This table has more than one human player and does not allow a paired bot."
	case e.MaxSeats > 0 && c.Seats > e.MaxSeats:
		return fmt.Sprintf("This bot plays games of at most %d seats.", e.MaxSeats)
	case len(e.Formats) > 0 && !slices.Contains(e.Formats, c.Format.String()):
		return fmt.Sprintf("This bot does not play the %s format.", c.Format)
	}
	return ""
}

// PairedBotStatus reports whether table id offers the paired bot.
func (r *Registry) PairedBotStatus(id TableID) (protocol.PairedBotStatus, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return protocol.PairedBotStatus{}, ErrNotFound
	}
	if r.opts.PairedBot == nil || t.cfg.PairedBot == PairedBotOff {
		return protocol.PairedBotStatus{}, nil
	}
	if reason := r.pairedBotRefusal(t); reason != "" {
		return protocol.PairedBotStatus{Reason: reason}, nil
	}
	return protocol.PairedBotStatus{Offered: true, Policy: r.pairedBotOf(&t.cfg).Name, Shared: len(t.cfg.Humans) > 1}, nil
}

// adviceSlot is one table's paired-bot search: the running one, or the last
// one's answer, which serves a repeated request for the same decision.
type adviceSlot struct {
	mu  sync.Mutex
	run *adviceRun
}

// adviceKey names a decision. The engine pointer tells apart two decisions
// with one sequence number across an undo, which swaps the engine.
type adviceKey struct {
	e    *rules.Engine
	seq  uint64
	seat state.PlayerID
}

// adviceRun is one search. waiters and finished are guarded by adviceSlot.mu;
// res and err are written before done closes.
type adviceRun struct {
	key      adviceKey
	cancel   context.CancelFunc
	done     chan struct{}
	waiters  int
	finished bool
	res      protocol.Advice
	err      error
}

// adviceJob is everything one search reads. Nothing in it is written after the
// read lock it was taken under is released: l holds only the prefix of an
// append-only log, and m.gc never changes.
type adviceJob struct {
	m *match
	d decision.Decision
	l *events.Log
	n int
}

// Advice returns the paired bot's evaluation of the decision seq pending on
// player's human seat. It blocks until the search ends. At most one search runs
// per table: asking about a newer decision cancels the older search, and a
// search ends early once its decision moves on or everyone asking has gone. A
// decision that is no longer pending gets ErrAdviceStale, never an answer.
func (r *Registry) Advice(ctx context.Context, id TableID, k int, player state.PlayerID, seq uint64) (protocol.Advice, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return protocol.Advice{}, ErrNotFound
	}
	if t.cfg.Engine != "" {
		return protocol.Advice{}, engineRefusal("a paired bot", t.cfg.Engine)
	}
	if reason := r.pairedBotRefusal(t); reason != "" {
		return protocol.Advice{}, errors.New(reason)
	}
	_, m, err := r.lookup(id, k)
	if err != nil {
		return protocol.Advice{}, err
	}
	d, err := r.Pending(id, k, player)
	var idle nothingPending
	if errors.As(err, &idle) {
		return protocol.Advice{}, ErrAdviceStale
	}
	if err != nil {
		return protocol.Advice{}, err
	}
	if d.Seq != seq {
		return protocol.Advice{}, ErrAdviceStale
	}
	job, key, ok := m.adviceJob(*d)
	if !ok {
		return protocol.Advice{}, ErrAdviceStale
	}

	slot := &t.advice
	slot.mu.Lock()
	prev := slot.run
	// An unfinished run with no one waiting has been cancelled and is winding down.
	if prev != nil && prev.key == key && (prev.finished && prev.err == nil || !prev.finished && prev.waiters > 0) {
		prev.waiters++
		slot.mu.Unlock()
		return awaitAdvice(ctx, slot, prev)
	}
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		slot.mu.Unlock()
		return protocol.Advice{}, errTableClosed
	}
	r.wg.Add(1)
	r.mu.RUnlock()
	runCtx, cancel := context.WithCancel(context.Background())
	run := &adviceRun{key: key, cancel: cancel, done: make(chan struct{}), waiters: 1}
	slot.run = run
	if prev != nil {
		prev.cancel()
	}
	slot.mu.Unlock()
	go r.runAdvice(runCtx, slot, run, prev, job)
	return awaitAdvice(ctx, slot, run)
}

// awaitAdvice waits for run on behalf of one request. The last request to leave
// cancels the search.
func awaitAdvice(ctx context.Context, slot *adviceSlot, run *adviceRun) (protocol.Advice, error) {
	select {
	case <-run.done:
		return run.res, run.err
	case <-ctx.Done():
		slot.mu.Lock()
		run.waiters--
		if run.waiters == 0 {
			run.cancel()
		}
		slot.mu.Unlock()
		return protocol.Advice{}, ctx.Err()
	}
}

// adviceJob snapshots what a search of d reads, or reports that d is no longer
// the pending decision.
func (m *match) adviceJob(d decision.Decision) (adviceJob, adviceKey, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := adviceKey{e: m.e, seq: d.Seq, seat: d.Player}
	if !m.adviceCurrent(key) {
		return adviceJob{}, adviceKey{}, false
	}
	live := m.g.Log()
	l := &events.Log{Seed: live.Seed, Events: live.Events[:len(live.Events):len(live.Events)], Intents: live.Intents[:len(live.Intents):len(live.Intents)]}
	return adviceJob{m: m, d: d, l: l, n: m.intents}, key, true
}

// adviceCurrent reports whether key's decision is the pending one. Called with
// m.mu held.
func (m *match) adviceCurrent(key adviceKey) bool {
	if m.state != protocol.MatchLive || m.e != key.e {
		return false
	}
	p := m.e.Pending()
	return p != nil && p.Seq == key.seq && p.Player == key.seat
}

// runAdvice is one search's goroutine. It exists only while a request is
// waiting on it, and waits out the search it replaced, which keeps a table to
// one search at a time.
func (r *Registry) runAdvice(ctx context.Context, slot *adviceSlot, run *adviceRun, prev *adviceRun, job adviceJob) {
	defer r.wg.Done()
	defer run.cancel()
	stopWatch := make(chan struct{})
	go job.m.watchAdvice(run.key, run.cancel, stopWatch, r.done)

	res, err := r.searchAdvice(ctx, prev, job)
	close(stopWatch)
	job.m.mu.RLock()
	current := job.m.adviceCurrent(run.key)
	job.m.mu.RUnlock()
	switch {
	case !current:
		err = ErrAdviceStale
	case err != nil && ctx.Err() != nil:
		// Everyone asking has gone, or a request for another decision took over.
	case err != nil:
		// An engine's error can quote what it replayed, so it stays here.
		log.Printf("host: table %s match %d: paired bot: %v", job.m.table.cfg.ID, job.m.k, err)
		err = errors.New(adviceFailed)
	}
	slot.mu.Lock()
	run.res, run.err, run.finished = res, err, true
	slot.mu.Unlock()
	close(run.done)
}

// searchAdvice takes its turn and searches. A panic in the replay or the search
// is that request's failure, never the server's.
func (r *Registry) searchAdvice(ctx context.Context, prev *adviceRun, job adviceJob) (res protocol.Advice, err error) {
	if prev != nil {
		<-prev.done
	}
	if err := r.adviceGate.acquire(ctx, nil); err != nil {
		return protocol.Advice{}, err
	}
	defer r.adviceGate.release()
	defer func() {
		if p := recover(); p != nil {
			res, err = protocol.Advice{}, fmt.Errorf("panic: %v\n%s", p, debug.Stack())
		}
	}()
	return r.advise(ctx, job)
}

// watchAdvice cancels a search once its decision stops being the pending one
// or the registry closes.
func (m *match) watchAdvice(key adviceKey, cancel context.CancelFunc, stop, closed <-chan struct{}) {
	tick := time.NewTicker(adviceWatch)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-closed:
			cancel()
			return
		case <-tick.C:
			m.mu.RLock()
			current := m.adviceCurrent(key)
			m.mu.RUnlock()
			if !current {
				cancel()
				return
			}
		}
	}
}

// advise rebuilds the seat's position and asks a fresh seat about it. The
// inputs are the ones envData builds for a closed-list hosted seat, from a
// replayed engine instead of the live one, and the seeds are that seat's.
func (r *Registry) advise(ctx context.Context, job adviceJob) (protocol.Advice, error) {
	m, d := job.m, job.d
	p := d.Player
	entry := r.pairedBotOf(&m.table.cfg)
	out := protocol.Advice{Table: string(m.table.cfg.ID), Match: m.k, AskSeq: d.Seq, Seat: uint8(p), Policy: entry.Name}

	seatSeed := m.seed ^ uint64(int(p)+1)
	s, err := entry.New(bots.Options{Seed: seatSeed, AutoPayMana: m.table.cfg.AutoMana, SearchParallelism: 1, Deps: r.opts.BotDeps, DecisionDeadlineMS: hostedDecisionDeadlineMS, ExpectedLine: adviceLinePlies, Config: r.opts.BotConfig[entry.Name]})
	if err != nil {
		return protocol.Advice{}, err
	}
	es := s.(bots.EnvSeat)
	if !es.WantsEnv(&d) {
		out.State, out.Reason = protocol.AdviceNone, adviceNotSearched
		return out, nil
	}

	feed, e, err := searchseat.RebuildFeedEngine(ctx, gorgegame.RulesConfig(m.gc), job.l, job.n, p)
	if errors.Is(err, searchseat.ErrFeedStopped) {
		log.Printf("host: table %s match %d: paired bot: %v", m.table.cfg.ID, m.k, err)
		out.State, out.Reason = protocol.AdviceRefused, adviceNoRoot
		return out, nil
	}
	if err != nil {
		return protocol.Advice{}, err
	}
	setup := searchseat.SeatSetup(searchprobe.PublicGame{
		Names: m.gc.Names, Decks: m.gc.Decks, Tokens: m.gc.Tokens, StartingLife: m.gc.StartingLife,
	}, es, p)
	setup, closed := closedSetup(m.table.cfg, setup, p)
	if !closed {
		return protocol.Advice{}, errors.New(adviceFailed)
	}
	if closedRootRefusal(m.table.cfg, e) != "" {
		out.State, out.Reason = protocol.AdviceRefused, adviceMidResolution
		return out, nil
	}
	root, why := searchseat.HonestRoot(setup, feed, e, bots.RootSeed(seatSeed, d.Seq))
	if root == nil {
		log.Printf("host: table %s match %d: paired bot: no root: %s", m.table.cfg.ID, m.k, why)
		out.State, out.Reason = protocol.AdviceRefused, adviceNoRoot
		return out, nil
	}
	v := view.ProjectForControlled(e.G, e, p, view.Seat, controlledSeats(e.G, p), &d)
	v.Round = view.RoundOf(e.G, e.L.Events)
	var brd botpolicy.Board
	b := botpolicy.BoardFromGameInto(e.G, e, p, &brd).Detached()

	var note *explain.Note
	s.(explain.Emitter).SetExplain(func(n explain.Note) { note = &n })
	cctx, cancel := decisionCtx(ctx, s)
	defer cancel()
	if _, err := es.DecideEnv(cctx, bots.Env{
		View: v, Board: b,
		Search: searchseat.Env{Setup: setup, Engine: root, Board: b, Feed: feed},
	}, d); err != nil && ctx.Err() == nil {
		return protocol.Advice{}, err
	}
	if err := ctx.Err(); err != nil {
		return protocol.Advice{}, err
	}
	if note == nil {
		out.State, out.Reason = protocol.AdviceNone, adviceNotSearched
		return out, nil
	}
	if !wordAdvice(&out, note) {
		return out, nil
	}
	note.Policy = entry.Name
	if note.Search != nil {
		note.Search.MS = 0
	}
	raw, err := json.Marshal(note)
	if err != nil {
		return protocol.Advice{}, err
	}
	out.State, out.Explain = protocol.AdviceReady, raw
	return out, nil
}

// wordAdvice sets out's state from the seat's note and replaces the note's
// reason, a report name, with words on out. It reports whether the note is
// worth sending: a search that finished no line has no numbers to show.
func wordAdvice(out *protocol.Advice, note *explain.Note) bool {
	reason := note.Reason
	note.Reason = ""
	finished := note.Search != nil && note.Search.Sims > 0
	switch {
	case strings.HasPrefix(reason, "redeal refused"):
		out.State, out.Reason = protocol.AdviceRefused, adviceNoRoot
		return false
	case note.Search != nil && !finished:
		out.State, out.Reason = protocol.AdviceRefused, adviceNoLines
		return false
	case reason == "deadline":
		out.Reason = adviceNoTime
	case reason != "":
		out.Reason = adviceNotPriced
	}
	out.State = protocol.AdviceReady
	return true
}
