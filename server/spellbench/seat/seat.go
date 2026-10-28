// Package seat adapts a Spellbench v2 stdio agent to gorge's seat.Seat
// contract, so a v2 bot can play at an ordinary gorge table.
package seat

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	sbenv "github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

const (
	forfeitTimeout   = "timeout"
	forfeitTransport = "transport_error"
	forfeitAgent     = "agent_error"
	forfeitInvalid   = "invalid_selection"

	// startupMS is the game_start StartupMS the seat advertises; the same
	// budget bounds the hello/game_start handshake. startupBudget is an
	// untyped nanosecond constant so this package need not import time (the
	// archtest clock rule allows only the host and named tools to read it).
	startupMS     = 300000
	startupBudget = startupMS * 1_000_000
)

var (
	errMalformed = errors.New("malformed response")
	errAgent     = errors.New("agent error")
	errInvalid   = errors.New("invalid selection")
	errTransport = errors.New("transport error")
)

// Agent is one Spellbench v2 agent process playing one seat of one match.
// A fresh process is started by AttachMatch and stopped by EndMatch.
type Agent struct {
	name     string
	argv     []string
	serve    func(io.Reader, io.Writer) error // an in-process agent (NewInProcess); nil runs argv
	opts     bots.Options
	fallback engineseat.Seat

	mu                sync.Mutex
	mc                *bots.MatchContext
	gameID            string
	secret            []byte
	e                 *rules.Engine
	env               *mapping.Env
	tx                mapping.Transaction
	native            *decision.Decision
	queue             []queuedIntent
	step              uint64
	seatStep, groupID [2]uint64
	decisions         uint64
	reqID             uint64
	attached          bool
	done              bool
	forfeit           string
	fallbackLogged    bool
	// startup is the handshake deadline seam. Tests replace it to shorten
	// the budget; nil uses the advertised startupBudget.
	startup func() (context.Context, context.CancelFunc)

	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *wire.Reader
}

// New builds a v2 agent seat. argv is the agent command line; the first
// element is the executable.
func New(name string, argv []string, o bots.Options) (engineseat.Seat, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("spellbench seat %q: empty command", name)
	}
	return &Agent{name: name, argv: append([]string(nil), argv...), opts: o, fallback: heuristic.NewBot(o.Seed)}, nil
}

// NewInProcess builds a v2 agent seat whose agent runs in this process: serve
// is the agent's stdio loop (agent.Serve over a server), started on a pipe pair
// at AttachMatch in place of exec'ing a command. The wire, the handshake and
// every forfeit rule are the process seat's; it is for harnesses that measure
// the seat path without a binary per seat.
func NewInProcess(name string, serve func(io.Reader, io.Writer) error, o bots.Options) (*Agent, error) {
	if serve == nil {
		return nil, fmt.Errorf("spellbench seat %q: nil serve", name)
	}
	return &Agent{name: name, serve: serve, opts: o, fallback: heuristic.NewBot(o.Seed)}, nil
}

// Forfeit is the cause the seat forfeited its match with ("" while it has not).
func (a *Agent) Forfeit() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.forfeit
}

func (a *Agent) logf(format string, args ...any) {
	if a.mc != nil && a.mc.Logf != nil {
		a.mc.Logf(format, args...)
	}
}

// AttachMatch starts the agent process and performs the v2 hello/game_start
// handshake. A spawn, hello or game_start failure is recorded as a forfeit
// cause and returns nil: the seat's first decision concedes. A requirement
// the engine cannot meet, or a table wider than two seats, refuses the match.
func (a *Agent) AttachMatch(mc *bots.MatchContext) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.attached {
		return nil
	}
	if mc == nil {
		return errors.New("spellbench seat: nil match context")
	}
	if mc.Engine == nil {
		return errors.New("spellbench seat: match context has no engine")
	}
	if mc.OwnDeck == nil {
		return errors.New("spellbench seat: match context has no own deck")
	}
	if mc.Seats > 2 {
		return fmt.Errorf("spellbench seat: v2 is two-seat, table has %d seats", mc.Seats)
	}
	a.mc = mc
	a.gameID = mc.GameID
	if a.gameID == "" {
		sum := sha256.Sum256(append(append([]byte{}, mc.Seed...), byte(mc.Seat)))
		a.gameID = fmt.Sprintf("g-%016x", sum[:8])
	}
	if len(mc.Secret) > 0 {
		a.secret = append([]byte(nil), mc.Secret...)
	} else {
		sum := sha256.Sum256(append(append([]byte{}, mc.Seed...), []byte(a.gameID)...))
		a.secret = sum[:]
	}
	base := mc.Context
	if base == nil {
		base = context.Background()
	}
	a.ctx, a.cancel = context.WithCancel(base)
	sec := secrets.NewGame(a.secret)
	a.e = mc.Engine
	tracker := identity.New(mc.Engine, sec)
	a.env = &mapping.Env{
		AutoPay: true,
		G:       &gamecfg.Game{E: mc.Engine, Secret: sec},
		IDs:     tracker,
		Obs:     &observe.Projector{E: mc.Engine, IDs: tracker},
		Domain:  nameSet(domainNames(mc.Engine.G.NameUniverseNames, mc.OwnDeck, mc.OpponentDeck)),
		Slots:   map[string]uint32{},
	}

	if err := a.start(); err != nil {
		a.forfeit = forfeitTransport
		a.logf("spellbench seat %d (%s): start: %v", mc.Seat, a.name, err)
		a.stop()
		return nil
	}
	if err := a.hello(); err != nil {
		if errors.Is(err, errUnmetRequirement) {
			a.stop()
			return err
		}
		a.forfeit = a.cause(err)
		a.logf("spellbench seat %d (%s): hello: %v", mc.Seat, a.name, err)
		a.stop()
		return nil
	}
	if err := a.gameStart(); err != nil {
		a.forfeit = a.cause(err)
		a.logf("spellbench seat %d (%s): game_start: %v", mc.Seat, a.name, err)
		a.stop()
		return nil
	}
	a.attached = true
	return nil
}

func (a *Agent) start() error {
	if a.serve != nil {
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		go func() { outW.CloseWithError(a.serve(inR, outW)); inR.Close() }()
		a.stdin = inW
		a.reader = wire.NewReader(outR)
		return nil
	}
	cmd := exec.CommandContext(a.ctx, a.argv[0], a.argv[1:]...)
	cmd.WaitDelay = 2_000_000_000
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	a.cmd = cmd
	a.stdin = stdin
	a.reader = wire.NewReader(stdout)
	return nil
}

var errUnmetRequirement = errors.New("spellbench seat: agent requirement not offered")

func (a *Agent) hello() error {
	id := a.nextID()
	ctx, cancel := a.startupContext()
	defer cancel()
	line, err := a.roundTrip(ctx, helloReq{RequestType: "hello", Protocol: protocolName, ProtocolMinor: protocolMinor, RequestID: id})
	if err != nil {
		return err
	}
	var resp helloResp
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("%w: hello: %v", errMalformed, err)
	}
	if resp.ResponseType != "hello_ok" || resp.Protocol != protocolName || resp.RequestID != id {
		return fmt.Errorf("%w: hello response %q (request %q)", errMalformed, resp.ResponseType, resp.RequestID)
	}
	if resp.Bot.Name == "" || resp.Bot.Version == "" {
		return errors.New("spellbench seat: hello has no bot name/version")
	}
	for _, name := range resp.Requires.Observation {
		if _, ok := observe.Flags[name]; !ok {
			return fmt.Errorf("%w: observation %q", errUnmetRequirement, name)
		}
	}
	if len(resp.Requires.Extensions) > 0 {
		return fmt.Errorf("%w: extensions", errUnmetRequirement)
	}
	return nil
}

func (a *Agent) gameStart() error {
	mc := a.mc
	seat := fmt.Sprintf("p%d", mc.Seat)
	own := mc.OwnDeck
	if own == nil {
		own = &deck.Manifest{}
	}
	names := domainNames(mc.Engine.G.NameUniverseNames, mc.OwnDeck, mc.OpponentDeck)
	mana := "engine_autopay"
	combat := "engine_order"
	profile := engineProfile{
		DecisionKinds: seatDecisionKinds(),
		EngineDefaults: map[string]*string{
			"combat_damage_assignment": &combat,
			"mana_payment":             &mana,
			"replacement_order":        nil,
			"trigger_order":            nil,
		},
		Extensions:     []protocol.Extension{},
		Fairness:       map[string]bool{"noninterference_probe": false},
		Observation:    observe.Flags,
		Rewind:         false,
		RulesSupported: map[string][]string{"mulligan": {"london", "none"}, "starting_player": {"host_assigned", "toss_winner_chooses"}},
	}
	startingPlayer, startingSeat := "host_assigned", fmt.Sprintf("p%d", mc.StartingSeat)
	if mc.Engine != nil && mc.Engine.StartingChoicePending() {
		startingPlayer, startingSeat = "toss_winner_chooses", ""
	}
	var startSeat *string
	if startingSeat != "" {
		startSeat = &startingSeat
	}
	id := a.nextID()
	req := gameStartReq{
		RequestType:   "game_start",
		Protocol:      protocolName,
		RequestID:     id,
		AgentSeed:     agentSeed(a.secret, a.gameID, seat),
		Engine:        sbenv.EngineIdentity(nil, mc.Registry),
		EngineProfile: profile,
		Format:        formatOr(mc.Format),
		GameID:        a.gameID,
		Limits: limits{
			MaxDecisions:            10000,
			MaxSeatDecisionsPerGame: 4999,
			MaxSeatDecisionsPerTurn: 500,
			MaxSeatStepsPerGame:     49999,
			MaxSteps:                100000,
		},
		OwnDeck:   deckWire(own),
		Resources: resources{CPUs: 1, EngineCPUs: 1, MemoryMB: 4096},
		Rules: gameRules{
			CardNameDomain:   nameDomain{DomainID: wire.DomainID(names), Names: names},
			Extensions:       []string{},
			Mulligan:         mulliganOr(mc.Mulligan),
			OpponentDecklist: "hidden",
			Probe:            false,
			StartingPlayer:   startingPlayer,
			StartingSeat:     startSeat,
		},
		Seat: seat,
		TimeControl: timeControl{
			BankMS:        600000,
			EngineStepMS:  120000,
			GameStartMS:   60000,
			IncrementMS:   2000,
			MaxDecisionMS: int64(a.deadlineMS()),
			StartupMS:     startupMS,
		},
	}
	if mc.OpponentVisible && mc.OpponentDeck != nil {
		opp := deckWire(mc.OpponentDeck)
		req.OpponentDeck = &opp
		req.Rules.OpponentDecklist = "visible"
	}
	ctx, cancel := a.startupContext()
	defer cancel()
	line, err := a.roundTrip(ctx, req)
	if err != nil {
		return err
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("%w: game_start: %v", errMalformed, err)
	}
	if resp.ResponseType != "ack" || resp.Protocol != protocolName || resp.RequestID != id {
		return fmt.Errorf("%w: game_start response %q (request %q)", errMalformed, resp.ResponseType, resp.RequestID)
	}
	return nil
}

func (a *Agent) startupContext() (context.Context, context.CancelFunc) {
	if a.startup != nil {
		return a.startup()
	}
	return context.WithTimeout(a.ctx, startupBudget)
}

// projected runs f under the host's projection lock when the MatchContext
// supplies one. Engine reads that reach rules.Derived (identity keys, mapping
// refs and the heuristic fallback) share Engine's scratch walk, while the
// host projects views for subscribers under its own lock; serializing the two
// keeps a live match from racing its own fanout. The agent round-trip is
// never inside f.
func (a *Agent) projected(f func()) {
	if a.mc != nil && a.mc.Projection != nil {
		a.mc.Projection(f)
		return
	}
	f()
}

func (a *Agent) nextID() string {
	a.reqID++
	return fmt.Sprintf("r-%d", a.reqID)
}

func (a *Agent) roundTrip(ctx context.Context, req any) ([]byte, error) {
	if a.stdin == nil || a.reader == nil {
		return nil, errors.New("spellbench seat: process not started")
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// The deadline covers the WRITE as well as the read: a child that never
	// drains its stdin blocks Write, not ReadBytes, and the handshake budget
	// must still end it. Killing the process unblocks both sides.
	if ctx != nil {
		if cmd := a.cmd; cmd != nil && cmd.Process != nil {
			stopKill := context.AfterFunc(ctx, func() { _ = cmd.Process.Kill() })
			defer stopKill()
		}
	}
	if _, err := a.stdin.Write(append(b, '\n')); err != nil {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: write: %v", errTransport, err)
	}
	line, err := a.reader.ReadLine()
	if err != nil {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if a.ctx != nil && a.ctx.Err() != nil {
			return nil, a.ctx.Err()
		}
		return nil, fmt.Errorf("%w: read: %v", errTransport, err)
	}
	return line, nil
}

func (a *Agent) stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.stdin != nil {
		_ = a.stdin.Close()
	}
	if a.cmd != nil {
		_ = a.cmd.Wait()
	}
	a.stdin, a.reader, a.cmd = nil, nil, nil
}

// Decide answers one gorge decision. Once the agent has forfeited, a priority
// ask takes the engine's concede option; every other ask is answered by the
// deterministic heuristic fallback so the game reaches a priority window.
func (a *Agent) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return decision.Intent{}, errors.New("spellbench seat: decision after match end")
	}
	if a.forfeit != "" {
		return a.forfeitAnswer(ctx, v, d), nil
	}
	if a.mc == nil || a.e == nil || a.env == nil {
		return decision.Intent{}, errors.New("spellbench seat: Decide before AttachMatch")
	}
	var syncErr error
	a.projected(func() { syncErr = a.env.IDs.Sync(a.e) })
	if syncErr != nil {
		a.forfeit = forfeitAgent
		return a.forfeitAnswer(ctx, v, d), nil
	}
	if len(a.queue) > 0 {
		q := a.queue[0]
		a.queue = a.queue[1:]
		if q.in.Seq == 0 {
			if q.want == nil || !sameAsk(&d, q.want) {
				a.queue = nil
				a.tx = nil
				a.forfeit = forfeitAgent
				a.logf("spellbench seat %d (%s): folded follow-up does not match the engine decision", d.Player, a.name)
				return a.forfeitAnswer(ctx, v, d), nil
			}
			q.in.Seq, q.in.Player = d.Seq, d.Player
		}
		if q.in.Seq != d.Seq || q.in.Player != d.Player {
			a.queue = nil
			a.tx = nil
			a.forfeit = forfeitAgent
			a.logf("spellbench seat %d (%s): folded intent %d/%d for decision %d/%d", d.Player, a.name, q.in.Seq, q.in.Player, d.Seq, d.Player)
			return a.forfeitAnswer(ctx, v, d), nil
		}
		return q.in, nil
	}
	if d.Kind == decision.KPriority {
		a.env.Action, a.env.Slots = nil, map[string]uint32{}
	}
	if a.tx == nil {
		if in, ok := autoPayment(&d); ok {
			return in, nil
		}
		var (
			in  decision.Intent
			ok  bool
			tx  mapping.Transaction
			err error
		)
		a.projected(func() { in, ok, err = mapping.Internal(a.env, &d) })
		if ok || err != nil {
			if err != nil {
				a.forfeit = forfeitAgent
				a.logf("spellbench seat %d (%s): internal decision: %v", d.Player, a.name, err)
				return a.forfeitAnswer(ctx, v, d), nil
			}
			return in, nil
		}
		a.projected(func() { tx, err = mapping.Begin(a.env, &d) })
		if err != nil {
			a.forfeit = forfeitAgent
			a.logf("spellbench seat %d (%s): unmapped decision: %v", d.Player, a.name, err)
			return a.forfeitAnswer(ctx, v, d), nil
		}
		a.tx, a.native = tx, &d
	}
	// A v2 choose is one substep of a gorge decision: a selection of several
	// cards (or an arrange) poses again after each answer, and only the last
	// answer produces the engine Intent. Keep the transaction and ask again
	// until it commits.
	for {
		var (
			p   *mapping.Pose
			err error
		)
		a.projected(func() { p, err = a.tx.Pose() })
		if err != nil {
			a.tx = nil
			a.forfeit = forfeitAgent
			a.logf("spellbench seat %d (%s): pose %s (%q, %d options, min %d max %d repeat %v src %d): %v", d.Player, a.name, d.Kind, mapping.Route(&d), len(d.Options), d.Min, d.Max, d.Repeatable, d.Source, err)
			return a.forfeitAnswer(ctx, v, d), nil
		}
		i, err := a.ask(ctx, p)
		if err != nil {
			a.forfeit = a.cause(err)
			a.logf("spellbench seat %d (%s): choose: %v (forfeit %s)", p.Seat, a.name, err, a.forfeit)
			return a.forfeitAnswer(ctx, v, d), nil
		}
		op := p.Candidates[i].Op
		if p.Context.Kind == "priority" {
			if obj := a.actionObject(op); obj != 0 {
				a.env.Action = &mapping.ActionContext{Seat: p.Seat, Obj: obj, Since: a.e.G.NextID}
			}
		}
		var (
			commit []decision.Intent
			done   bool
		)
		a.projected(func() { commit, done, err = a.tx.Answer(i) })
		a.step++
		a.seatStep[p.Seat]++
		if p.SubstepIndex+1 == p.SubstepCount {
			a.groupID[p.Seat]++
			a.decisions++
		}
		if err != nil {
			a.tx = nil
			a.forfeit = forfeitInvalid
			return a.forfeitAnswer(ctx, v, d), nil
		}
		if len(commit) == 0 {
			if done {
				a.tx = nil
				a.env.CloseLooks()
				a.forfeit = forfeitInvalid
				a.logf("spellbench seat %d (%s): transaction completed without an intent", p.Seat, a.name)
				return a.forfeitAnswer(ctx, v, d), nil
			}
			continue
		}
		if done {
			a.tx = nil
			a.env.CloseLooks()
		}
		if len(commit) > 1 {
			keys := followKeys(op)
			for k, in := range commit[1:] {
				var want *decision.Decision
				if in.Seq == 0 && k < len(keys) {
					want = p.Followups[keys[k]]
				}
				a.queue = append(a.queue, queuedIntent{in: in, want: want})
			}
		}
		in := commit[0]
		if in.Seq == 0 {
			in.Seq, in.Player = d.Seq, d.Player
		}
		if in.Seq != d.Seq || in.Player != d.Player {
			a.tx = nil
			a.forfeit = forfeitAgent
			a.logf("spellbench seat %d (%s): mapped intent %d/%d does not answer decision %d/%d", d.Player, a.name, in.Seq, in.Player, d.Seq, d.Player)
			return a.forfeitAnswer(ctx, v, d), nil
		}
		return in, nil
	}
}

type queuedIntent struct {
	in   decision.Intent
	want *decision.Decision
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

func (a *Agent) cause(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || (a.ctx != nil && errors.Is(a.ctx.Err(), context.DeadlineExceeded)) {
		return forfeitTimeout
	}
	switch {
	case errors.Is(err, errAgent):
		return forfeitAgent
	case errors.Is(err, errInvalid):
		return forfeitInvalid
	case errors.Is(err, errMalformed):
		return forfeitAgent
	case errors.Is(err, errTransport):
		return forfeitTransport
	default:
		return forfeitTransport
	}
}

func (a *Agent) ask(ctx context.Context, p *mapping.Pose) (int, error) {
	if a.stdin == nil {
		return 0, errors.New("spellbench seat: process is not running")
	}
	holder := a.priorityHolder(p)
	known := append([]protocol.Known(nil), p.Known...)
	observe.SortKnown(known)
	var obs protocol.Observation
	var err error
	a.projected(func() {
		obs, err = a.env.Obs.Observation(p.Seat, observe.State{PriorityHolder: holder, Known: known})
	})
	if err != nil {
		return 0, err
	}
	sd := protocol.SeatDecision{
		ActingSeat:  observe.Seat(p.Seat),
		SeatStep:    a.seatStep[p.Seat],
		Group:       protocol.Group{GroupID: a.groupID[p.Seat], SubstepIndex: p.SubstepIndex, SubstepCount: p.SubstepCount},
		Context:     p.Context,
		Observation: obs,
		Extensions:  protocol.ExtensionMap{},
	}
	for i, c := range p.Candidates {
		sd.Candidates = append(sd.Candidates, protocol.Candidate{CandidateID: uint32(i), Semantic: c.Sem})
	}
	id := a.nextID()
	line, err := a.roundTrip(ctx, chooseReq{
		RequestType: "choose",
		Protocol:    protocolName,
		RequestID:   id,
		GameID:      a.gameID,
		Clock:       clock{MaxDecisionMS: int64(a.deadlineMS()), RemainingMS: 600000},
		Decision:    sd,
	})
	if err != nil {
		return 0, err
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return 0, fmt.Errorf("%w: %v", errMalformed, err)
	}
	if resp.Protocol != protocolName || resp.RequestID != id {
		return 0, fmt.Errorf("%w: response to request %q", errMalformed, resp.RequestID)
	}
	switch resp.ResponseType {
	case "choice":
		if resp.Selection == nil {
			return 0, fmt.Errorf("%w: choice has no selection", errMalformed)
		}
		if int(resp.Selection.CandidateID) >= len(p.Candidates) {
			return 0, fmt.Errorf("%w: candidate %d out of range", errInvalid, resp.Selection.CandidateID)
		}
		return int(resp.Selection.CandidateID), nil
	case "error":
		return 0, fmt.Errorf("%w %s: %s", errAgent, resp.Error.Code, resp.Error.Message)
	default:
		return 0, fmt.Errorf("%w: unexpected response %q", errMalformed, resp.ResponseType)
	}
}

func (a *Agent) priorityHolder(p *mapping.Pose) *state.PlayerID {
	if p.Context.Kind == "priority" || (a.env.Action != nil && a.env.Action.Seat == p.Seat) {
		s := p.Seat
		return &s
	}
	return nil
}

func (a *Agent) actionObject(op mapping.NativeOp) state.ObjID {
	if a.native == nil {
		return 0
	}
	if op.Payment != nil {
		for _, act := range a.native.PaymentActions {
			if act.ID == op.Payment.ActionID {
				return act.Cast.Object
			}
		}
	}
	idx := op.Option
	if op.Op == "cast" && len(op.Covers) > 0 {
		idx = op.Covers[0]
	}
	if idx >= 0 && idx < len(a.native.Options) {
		return a.native.Options[idx].Obj
	}
	return 0
}

func (a *Agent) forfeitAnswer(ctx context.Context, v view.View, d decision.Decision) decision.Intent {
	if d.Kind == decision.KPriority {
		for _, o := range d.Options {
			if o.Kind == "concede" {
				return mapping.Intent(&d, o.Index)
			}
		}
	}
	if !a.fallbackLogged {
		a.fallbackLogged = true
		a.logf("spellbench seat %d (%s): forfeit %s: defaulting non-priority %s until a concede window", d.Player, a.name, a.forfeit, d.Kind)
	}
	var (
		in  decision.Intent
		err error
	)
	a.projected(func() { in, err = a.fallback.Decide(ctx, v, d) })
	if err == nil {
		return in
	}
	if len(d.Options) > 0 {
		return mapping.Intent(&d, d.Options[0].Index)
	}
	return mapping.Intent(&d)
}

// EndMatch sends the v2 game_over notice (when the process is still alive)
// and stops the agent process. A forfeited seat reports classification
// "forfeit" with its cause, overriding the host's natural result.
func (a *Agent) EndMatch(result bots.MatchResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return
	}
	a.done = true
	if a.stdin != nil {
		classification, outcome, reason, winner := result.Classification, result.Outcome, result.Reason, result.Winner
		if a.forfeit != "" {
			classification = "forfeit"
			outcome = ""
			winner = otherSeat(a.mc)
			reason = "forfeit:" + a.forfeit
		}
		// The protocol has no empty outcome: a forfeit is the opponent's win
		// and any other unfinished game is "halted".
		if outcome == "" {
			outcome = "halted"
			if classification == "forfeit" && winner != "" {
				outcome = winner + "_win"
			}
		}
		var wp *string
		if winner != "" {
			w := winner
			wp = &w
		}
		req := gameOverReq{
			RequestType: "game_over",
			Protocol:    protocolName,
			RequestID:   a.nextID(),
			GameID:      a.gameID,
			Terminal: terminalNotice{
				Classification: classification,
				Outcome:        outcome,
				Reason:         reason,
				SeatStepCount:  a.seatStep[seatIndex(a.mc)],
				Winner:         wp,
			},
		}
		if b, err := json.Marshal(req); err == nil {
			_, _ = a.stdin.Write(append(b, '\n'))
		}
	}
	a.stop()
}

func (a *Agent) DecisionBudgetMS() int {
	return a.deadlineMS()
}

// deadlineMS is the v2 clock budget: the policy's Options deadline when the
// caller set one (the host builds every seat with the table budget), else the
// match context's. Zero means unbounded, the bench default.
func (a *Agent) deadlineMS() int {
	if a.opts.DecisionDeadlineMS > 0 {
		return a.opts.DecisionDeadlineMS
	}
	if a.mc != nil {
		return a.mc.DeadlineMS
	}
	return 0
}

func otherSeat(mc *bots.MatchContext) string {
	if mc == nil {
		return ""
	}
	return fmt.Sprintf("p%d", 1-mc.Seat)
}

func seatIndex(mc *bots.MatchContext) int {
	if mc == nil || mc.Seat < 0 || mc.Seat > 1 {
		return 0
	}
	return mc.Seat
}

func nameSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

func formatOr(s string) string {
	if s == "" {
		return "constructed"
	}
	return s
}

func mulliganOr(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

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

func seatDecisionKinds() []string {
	kinds := append([]string(nil), sbenv.DecisionKinds...)
	found := false
	for _, k := range kinds {
		if k == "choose_starting_player" {
			found = true
			break
		}
	}
	if !found {
		kinds = append(kinds, "choose_starting_player")
	}
	sort.Strings(kinds)
	return kinds
}

var _ engineseat.Seat = (*Agent)(nil)
var _ bots.BudgetedSeat = (*Agent)(nil)
var _ bots.MatchSeat = (*Agent)(nil)
var _ bots.MatchEnd = (*Agent)(nil)
