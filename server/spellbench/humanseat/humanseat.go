// Package humanseat answers a human seat's native decisions through
// SpellBench v2 candidates: the codec behind an opt-in v2 client for a gorge
// table.
//
// Seat is the tap of a host.HumanSeat (host.HumanTap). The HumanSeat stays the
// seat the match plays, so its slot, think timeout, caretaker and undo are the
// native ones; Seat only watches a decision park, builds the v2 transaction
// for it through mapping, asks a far-end Answerer for one candidate id per
// v2 substep, holds the open transaction between substeps, and submits the
// native Intent through the HumanSeat's own submit path when the transaction
// commits. A think timeout mid-transaction therefore drops the partial picks
// and lets the caretaker answer the whole native decision, byte-identically to
// a native human timing out at that decision.
//
// It mirrors server/spellbench/seat (Agent), with a human at the far end of
// the question instead of a process. It does not import server/host: the tap
// contract is spelled in lower-layer types, so host's own tests can keep
// importing the v2 seat package.
package humanseat

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

// Ask is one v2 substep put to the human.
type Ask struct {
	GameID string
	// Decision is what the v2 wire shows: context, observation and candidates.
	Decision protocol.SeatDecision
	// Pose is the mapping's pose behind Decision, with each candidate's native
	// meaning (Cand.Op). It is server-internal; a transport must send only
	// Decision.
	Pose *mapping.Pose
	// View is the native view of the decision the substep belongs to, also
	// server-internal.
	View view.View
}

// NativeOnly is the signal that one native decision cannot be answered through
// v2. The transaction was aborted before any commit and the decision is still
// pending on the HumanSeat, so a client answers it over the native routes and
// resumes v2 at the next decision.
type NativeOnly struct {
	Seat state.PlayerID
	// Seq is the native decision's sequence number.
	Seq uint64
	// Route is the native route (mapping.Route) of the decision.
	Route string
	// Err is why it cannot be posed; errors.Is(Err, mapping.ErrUnmapped) for
	// an unmapped route.
	Err error
	// Decision and View are the native decision and the view that go with it,
	// server-internal like Ask.Pose: the native routes answer from them.
	Decision decision.Decision
	View     view.View
	// GameID, Game and Events let an extension builder publish the log and a
	// render plane for the decision the client must answer natively.
	GameID string
	Game   *state.Game
	Events []events.Event
	// CanUndo is the seat's undo eligibility at the native decision.
	CanUndo bool
}

// Answerer is the far end: the human, behind whatever transport carries it.
type Answerer interface {
	// Choose blocks until the human picks one of ask's candidates and returns
	// its index (the candidate id). It must return when ctx is done (the
	// decision was superseded: the think timeout fired, the decision was
	// undone, or the table closed). An index outside the candidates, or an
	// error, gives up v2 for this decision, which stays pending natively.
	Choose(ctx context.Context, ask Ask) (int, error)
	// NativeOnly reports a decision v2 cannot pose. It must not block.
	NativeOnly(NativeOnly)
}

// Extension is the server-internal state an opt-in extension builder needs
// when a v2 question is posed for a human seat. A transport that emits
// extensions supplies Config.Extend; the native wire leaves it nil.
type Extension struct {
	Seat        state.PlayerID
	Env         *mapping.Env
	Pose        *mapping.Pose
	View        view.View
	Game        *state.Game
	Events      []events.Event
	NativeIndex uint64
	CanUndo     bool
}

// ExtendFunc builds the extension map carried on one v2 seat decision.
type ExtendFunc func(Extension) (protocol.ExtensionMap, error)

// Config builds a Seat.
type Config struct {
	// Seats and Format describe the table the seat will play at. v2 is
	// two-seat and not Commander; anything else is an error.
	Seats    int
	Format   string
	Answerer Answerer
	// Extend, when non-nil, adds transport-specific extensions to every v2
	// seat decision this codec poses. The native wire never sets it.
	Extend ExtendFunc
	// NameWindow bounds the candidates of a choose_name decision (see
	// mapping.Env.NameWindow); 0 = no window. The extensions-bearing human
	// transport sets it to mapping.MaxCandidates and answers the rest through
	// its own action.
	NameWindow int
}

// Seat is the v2 codec of one human seat of one match. Install it with
// host.HumanSeat.SetTap.
type Seat struct {
	answerer Answerer
	extend   ExtendFunc
	// nameWindow is Config.NameWindow.
	nameWindow int

	mu       sync.Mutex
	mc       *bots.MatchContext
	e        *rules.Engine
	env      *mapping.Env
	gameID   string
	gen      uint64 // the driver generation; a new park supersedes the last
	cancel   context.CancelFunc
	tx       mapping.Transaction
	contSeq  uint64 // native decision whose accepted commit left tx open; 0 when none
	queue    []queuedIntent
	queueSeq uint64 // native decision whose commit produced queue
	native   *decision.Decision

	step, decisions   uint64
	seatStep, groupID [2]uint64
	nativeOnly        int
}

type queuedIntent struct {
	in   decision.Intent
	want *decision.Decision
}

// New builds the codec for one seat. It refuses what v2 does not cover: more
// than two seats, Commander, or no answerer.
func New(cfg Config) (*Seat, error) {
	if err := checkTable(cfg.Seats, cfg.Format); err != nil {
		return nil, err
	}
	if cfg.Answerer == nil {
		return nil, errors.New("v2 human seat: nil answerer")
	}
	return &Seat{answerer: cfg.Answerer, extend: cfg.Extend, nameWindow: cfg.NameWindow}, nil
}

func checkTable(seats int, format string) error {
	if seats != 2 {
		return fmt.Errorf("v2 human seat: v2 is two-seat, table has %d seats", seats)
	}
	if format == "commander" {
		return errors.New("v2 human seat: v2 does not cover Commander")
	}
	return nil
}

// NativeOnlyCount is how many decisions v2 could not pose.
func (s *Seat) NativeOnlyCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeOnly
}

// AttachMatch receives the live engine (bots.MatchSeat).
func (s *Seat) AttachMatch(mc *bots.MatchContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mc != nil {
		return nil
	}
	if mc == nil {
		return errors.New("v2 human seat: nil match context")
	}
	if err := checkTable(mc.Seats, mc.Format); err != nil {
		return err
	}
	if mc.Engine == nil {
		return errors.New("v2 human seat: match context has no engine")
	}
	if mc.OwnDeck == nil {
		return errors.New("v2 human seat: match context has no own deck")
	}
	s.mc = mc
	s.e = mc.Engine
	s.gameID = mc.GameID
	s.attachEnv(mc)
	return nil
}

// attachEnv (re)builds the mapping environment against mc.Engine. Called with
// the seat lock held.
func (s *Seat) attachEnv(mc *bots.MatchContext) {
	sec := secrets.NewGame(mc.Secret)
	tracker := identity.New(mc.Engine, sec)
	s.env = &mapping.Env{
		AutoPay: true,
		G:       &gamecfg.Game{E: mc.Engine, Secret: sec},
		IDs:     tracker,
		Obs:     &observe.Projector{E: mc.Engine, IDs: tracker},
		Domain:  nameSet(domainNames(mc.Engine.G.NameUniverseNames, mc.OwnDeck, mc.OpponentDeck)),
		Slots:   map[string]uint32{},

		NameWindow: s.nameWindow,
	}
}

// MatchRewound replaces the engine a host undo swapped in. The host calls it
// on the match goroutine under the match lock, before re-parking the rewound
// decision, so a v2 tap never compares or commits against the discarded
// engine. The old driver (if any) is invalidated; the following Parked call
// starts a fresh one.
func (s *Seat) MatchRewound(v any) {
	e, ok := v.(*rules.Engine)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e == nil || s.mc == nil || s.e == e {
		return
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.gen++
	s.e = e
	s.mc.Engine = e
	if s.env != nil {
		s.env.CloseLooks()
	}
	s.tx = nil
	s.queue = nil
	s.native = nil
	s.contSeq, s.queueSeq = 0, 0
	s.attachEnv(s.mc)
}

func (s *Seat) logf(format string, args ...any) {
	if s.mc != nil && s.mc.Logf != nil {
		s.mc.Logf(format, args...)
	}
}

// Parked starts answering a decision the HumanSeat parked (host.HumanTap).
// It never blocks the match goroutine: the transaction runs on a goroutine of
// its own, and a newer park supersedes it.
func (s *Seat) Parked(ctx context.Context, v view.View, d decision.Decision, answer func(decision.Intent) error, gone <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.env == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.gen++
	// A transaction survives a park only when the previous native decision's
	// commit was accepted and left it open (a multi-step action). One that a
	// timeout, an undo or an engine refusal interrupted is dropped: the engine
	// either took another answer or still asks the same decision.
	if s.tx != nil && !(s.contSeq != 0 && s.contSeq < d.Seq) {
		s.tx = nil
		s.env.CloseLooks()
	}
	s.contSeq = 0
	if len(s.queue) > 0 && s.queueSeq >= d.Seq {
		s.queue = nil
	}
	dctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go func() {
		select {
		case <-gone:
			cancel()
		case <-dctx.Done():
		}
	}()
	go s.drive(dctx, cancel, s.gen, gone, v, d, answer)
}

// projected runs f under the host's projection lock (see bots.MatchContext).
func (s *Seat) projected(f func()) {
	if s.mc != nil && s.mc.Projection != nil {
		s.mc.Projection(f)
		return
	}
	f()
}

// section runs f against the engine for driver generation gen, or reports
// false without running it when the driver is stale: superseded by a newer
// park, the seat gave up the decision (gone), or the engine no longer asks it.
// The seat lock is held throughout, so only one driver touches the
// transaction; gone and the pending decision are checked inside the
// projection lock, which every engine mutation also takes.
func (s *Seat) section(gen uint64, gone <-chan struct{}, d *decision.Decision, f func()) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return false
	}
	live := false
	s.projected(func() {
		select {
		case <-gone:
			return
		default:
		}
		if p := s.e.Pending(); p == nil || p.Seq != d.Seq || p.Player != d.Player {
			return
		}
		live = true
		f()
	})
	return live
}

// drop abandons the open transaction if gen is still the current driver.
func (s *Seat) drop(gen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return
	}
	s.tx = nil
	s.queue = nil
	s.env.CloseLooks()
}

func (s *Seat) signalNativeOnly(gen uint64, v view.View, d *decision.Decision, err error) {
	s.mu.Lock()
	if s.gen != gen {
		s.mu.Unlock()
		return
	}
	s.tx = nil
	s.queue = nil
	s.env.CloseLooks()
	s.nativeOnly++
	canUndo := s.canUndo(d.Player)
	gameID, game, evs := s.gameID, s.e.G, s.e.L.Events
	s.mu.Unlock()
	s.logf("v2 human seat %d: native only (%s): %v", d.Player, mapping.Route(d), err)
	s.answerer.NativeOnly(NativeOnly{Seat: d.Player, Seq: d.Seq, Route: mapping.Route(d), Err: err, Decision: *d, View: v, GameID: gameID, Game: game, Events: evs, CanUndo: canUndo})
}

// drive answers one parked native decision: queued folded follow-ups first,
// then engine-internal and auto-paid decisions, then the v2 transaction, one
// question per substep.
func (s *Seat) drive(ctx context.Context, cancel context.CancelFunc, gen uint64, gone <-chan struct{}, v view.View, d decision.Decision, answer func(decision.Intent) error) {
	defer cancel()
	var syncErr error
	if !s.section(gen, gone, &d, func() { syncErr = s.env.IDs.Sync(s.e) }) {
		return
	}
	if syncErr != nil {
		s.signalNativeOnly(gen, v, &d, syncErr)
		return
	}
	if in, ok := s.queued(gen, &d); ok {
		_ = answer(in)
		return
	}
	if d.Kind == decision.KPriority {
		s.mu.Lock()
		if s.gen == gen {
			s.env.Action, s.env.Slots = nil, map[string]uint32{}
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	resume := s.tx != nil && s.gen == gen
	if resume {
		s.native = &d
	}
	s.mu.Unlock()
	if !resume {
		if in, ok := autoPayment(&d); ok {
			_ = answer(in)
			return
		}
		var (
			in  decision.Intent
			ok  bool
			tx  mapping.Transaction
			err error
		)
		if !s.section(gen, gone, &d, func() { in, ok, err = mapping.Internal(s.env, &d) }) {
			return
		}
		if ok || err != nil {
			if err != nil {
				s.signalNativeOnly(gen, v, &d, err)
				return
			}
			_ = answer(in)
			return
		}
		if !s.section(gen, gone, &d, func() { tx, err = mapping.Begin(s.env, &d) }) {
			return
		}
		if err != nil {
			s.signalNativeOnly(gen, v, &d, err)
			return
		}
		s.mu.Lock()
		if s.gen != gen {
			s.mu.Unlock()
			return
		}
		s.tx, s.native = tx, &d
		s.mu.Unlock()
	}
	for {
		ask, ok, err := s.pose(gen, gone, &d, v)
		if !ok {
			return
		}
		if err != nil {
			s.signalNativeOnly(gen, v, &d, err)
			return
		}
		i, err := s.answerer.Choose(ctx, ask)
		if err != nil || i < 0 || i >= len(ask.Pose.Candidates) {
			if ctx.Err() == nil {
				s.logf("v2 human seat %d: choose: candidate %d of %d: %v", d.Player, i, len(ask.Pose.Candidates), err)
			}
			s.drop(gen)
			return
		}
		commit, done, ok, err := s.commit(gen, gone, &d, ask.Pose, i)
		if !ok {
			return
		}
		if err != nil {
			s.logf("v2 human seat %d: answer %d: %v", d.Player, i, err)
			s.drop(gen)
			return
		}
		if len(commit) == 0 {
			if done {
				s.logf("v2 human seat %d: transaction completed without an intent", d.Player)
				s.drop(gen)
				return
			}
			continue
		}
		in := s.first(gen, &d, ask.Pose, i, commit, done)
		if in == nil {
			return
		}
		if err := answer(*in); err != nil {
			s.drop(gen)
		}
		return
	}
}

// pose asks the open transaction for its next substep and builds the v2
// question for it.
func (s *Seat) pose(gen uint64, gone <-chan struct{}, d *decision.Decision, v view.View) (Ask, bool, error) {
	var (
		p   *mapping.Pose
		sd  protocol.SeatDecision
		err error
	)
	if !s.section(gen, gone, d, func() {
		if p, err = s.tx.Pose(); err != nil {
			return
		}
		sd, err = s.seatDecision(p, v)
	}) {
		return Ask{}, false, nil
	}
	if err != nil {
		return Ask{}, true, err
	}
	return Ask{GameID: s.gameID, Decision: sd, Pose: p, View: v}, true, nil
}

// seatDecision is the v2 wire form of a pose. Called under the seat lock.
func (s *Seat) seatDecision(p *mapping.Pose, v view.View) (protocol.SeatDecision, error) {
	holder := s.priorityHolder(p)
	known := append([]protocol.Known(nil), p.Known...)
	observe.SortKnown(known)
	obs, err := s.env.Obs.Observation(p.Seat, observe.State{PriorityHolder: holder, Known: known})
	if err != nil {
		return protocol.SeatDecision{}, err
	}
	sd := protocol.SeatDecision{
		ActingSeat:  observe.Seat(p.Seat),
		SeatStep:    s.seatStep[p.Seat],
		Group:       protocol.Group{GroupID: s.groupID[p.Seat], SubstepIndex: p.SubstepIndex, SubstepCount: p.SubstepCount},
		Context:     p.Context,
		Observation: obs,
		Extensions:  protocol.ExtensionMap{},
	}
	for i, c := range p.Candidates {
		sd.Candidates = append(sd.Candidates, protocol.Candidate{CandidateID: uint32(i), Semantic: c.Sem})
	}
	if s.extend != nil {
		sd.Extensions, err = s.extend(Extension{
			Seat: p.Seat, Env: s.env, Pose: p, View: v, Game: s.e.G, Events: s.e.L.Events,
			NativeIndex: p.Native.Seq, CanUndo: s.canUndo(p.Seat),
		})
		if err != nil {
			return protocol.SeatDecision{}, err
		}
	}
	return sd, nil
}

// canUndo reports whether the seat has an intent the host's undo queue can
// rewind. Called under the seat lock.
func (s *Seat) canUndo(seat state.PlayerID) bool {
	for i := range s.e.L.Intents {
		if s.e.L.Intents[i].Player == seat {
			return true
		}
	}
	return false
}

// commit applies candidate i to the open transaction.
func (s *Seat) commit(gen uint64, gone <-chan struct{}, d *decision.Decision, p *mapping.Pose, i int) (commit []decision.Intent, done, ok bool, err error) {
	ok = s.section(gen, gone, d, func() {
		op := p.Candidates[i].Op
		if p.Context.Kind == "priority" {
			if obj := s.actionObject(op); obj != 0 {
				s.env.Action = &mapping.ActionContext{Seat: p.Seat, Obj: obj, Since: s.e.G.NextID}
			}
		}
		commit, done, err = s.tx.Answer(i)
		s.step++
		s.seatStep[p.Seat]++
		if p.SubstepIndex+1 == p.SubstepCount {
			s.groupID[p.Seat]++
			s.decisions++
		}
	})
	return commit, done, ok, err
}

// first closes a finished transaction, queues the folded follow-up intents for
// the native decisions that come next, and returns the intent that answers d.
// Nil means the driver is stale or the mapped intent does not answer d.
func (s *Seat) first(gen uint64, d *decision.Decision, p *mapping.Pose, i int, commit []decision.Intent, done bool) *decision.Intent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return nil
	}
	if done {
		s.tx = nil
		s.env.CloseLooks()
	} else {
		s.contSeq = d.Seq
	}
	if len(commit) > 1 {
		keys := followKeys(p.Candidates[i].Op)
		for k, in := range commit[1:] {
			var want *decision.Decision
			if in.Seq == 0 && k < len(keys) {
				want = p.Followups[keys[k]]
			}
			s.queue = append(s.queue, queuedIntent{in: in, want: want})
		}
		s.queueSeq = d.Seq
	}
	in := commit[0]
	if in.Seq == 0 {
		in.Seq, in.Player = d.Seq, d.Player
	}
	// A pool-only payment plan is mapped to its plain cast, which executes the
	// same; commit the plan the candidate stands for, so the log reads as the
	// native payment answer.
	if op := p.Candidates[i].Op; op.Op == "choose" && op.Payment != nil && in.Payment == nil && in.Announce == nil && len(commit) == 1 {
		in = decision.Intent{Seq: d.Seq, Player: d.Player, Payment: decision.ClonePaymentSelection(op.Payment)}
	}
	if in.Seq != d.Seq || in.Player != d.Player {
		s.logf("v2 human seat %d: mapped intent %d/%d does not answer decision %d/%d", d.Player, in.Seq, in.Player, d.Seq, d.Player)
		s.tx = nil
		s.queue = nil
		s.env.CloseLooks()
		return nil
	}
	return &in
}

// queued pops the folded follow-up intent that answers d, if one is due. A
// follow-up that does not match d is dropped and d is posed afresh.
func (s *Seat) queued(gen uint64, d *decision.Decision) (decision.Intent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen || len(s.queue) == 0 {
		return decision.Intent{}, false
	}
	q := s.queue[0]
	s.queue = s.queue[1:]
	if q.in.Seq == 0 {
		if q.want == nil || !sameAsk(d, q.want) {
			s.queue, s.tx = nil, nil
			s.env.CloseLooks()
			return decision.Intent{}, false
		}
		q.in.Seq, q.in.Player = d.Seq, d.Player
	}
	if q.in.Seq != d.Seq || q.in.Player != d.Player {
		s.queue, s.tx = nil, nil
		s.env.CloseLooks()
		return decision.Intent{}, false
	}
	return q.in, true
}

// followKeys names the pose follow-ups a candidate's folded intents answer,
// in commit order, exactly as session.answer keys them.
func followKeys(op mapping.NativeOp) []string {
	keys := make([]string, 0, len(op.Followup))
	key := strconv.Itoa(op.Option)
	for _, f := range op.Followup {
		keys = append(keys, key)
		key += "/" + strconv.Itoa(f)
	}
	return keys
}

func sameAsk(d, want *decision.Decision) bool {
	if d.Kind != want.Kind || d.Player != want.Player || len(d.Options) != len(want.Options) {
		return false
	}
	for i, o := range d.Options {
		w := want.Options[i]
		if o.Kind != w.Kind || o.Obj != w.Obj || o.ManaSymbol != w.ManaSymbol || o.Ability != w.Ability {
			return false
		}
	}
	return true
}

func (s *Seat) priorityHolder(p *mapping.Pose) *state.PlayerID {
	if p.Context.Kind == "priority" || (s.env.Action != nil && s.env.Action.Seat == p.Seat) {
		seat := p.Seat
		return &seat
	}
	return nil
}

func (s *Seat) actionObject(op mapping.NativeOp) state.ObjID {
	if s.native == nil {
		return 0
	}
	if op.Payment != nil {
		for _, act := range s.native.PaymentActions {
			if act.ID == op.Payment.ActionID {
				return act.Cast.Object
			}
		}
	}
	idx := op.Option
	if op.Op == "cast" && len(op.Covers) > 0 {
		idx = op.Covers[0]
	}
	if idx >= 0 && idx < len(s.native.Options) {
		return s.native.Options[idx].Obj
	}
	return 0
}

// autoPayment answers the payment asks an engine_autopay environment never
// poses (the same table server/spellbench/seat and session keep): a mana
// window, a mana ability's colour ask, a triggered cost's pay/decline.
func autoPayment(d *decision.Decision) (decision.Intent, bool) {
	switch mapping.Route(d) {
	case "choose/mana_window":
		for _, kind := range []string{"autofill", "activate", "done"} {
			for _, o := range d.Options {
				if o.Kind == kind {
					return mapping.Intent(d, o.Index), true
				}
			}
		}
	case "choose/mana":
		if len(d.Options) > 0 {
			return mapping.Intent(d, d.Options[0].Index), true
		}
	case "choose/trigger_cost":
		for _, kind := range []string{"trigger_cost_pay", "trigger_cost_decline", "trigger_cost_pay_manual"} {
			for _, o := range d.Options {
				if o.Kind == kind {
					return mapping.Intent(d, o.Index), true
				}
			}
		}
	}
	return decision.Intent{}, false
}

// domainNames is the card-name domain of the table: the engine's name
// universe plus every name in the decks the seat may see.
func domainNames(names []string, decks ...*deck.Manifest) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	add := func(n string) {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range names {
		add(n)
	}
	for _, d := range decks {
		if d == nil {
			continue
		}
		for _, r := range d.Main {
			add(r.Name)
		}
		for _, r := range d.Sideboard {
			add(r.Name)
		}
		for _, n := range d.Commanders {
			add(n)
		}
	}
	return out
}

func nameSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}
