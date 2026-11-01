package sbv2

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/session"
)

// gameEvent is what the lockstep goroutine hands the request handler: the
// next decision to pose, or the finished lockstep run.
type gameEvent struct {
	resp *protocol.DecisionResponse
	res  *oracle.Result
}

// abortSignal is the panic a pick uses to stop the lockstep loop (a halt, a
// truncation or an abandoned game); oracle.LockstepLed recovers it.
type abortSignal struct{}

// diracGame is one v2 game: the kernel and the gorge shadow in lockstep on a
// goroutine, the request handler answering the agent's picks to it. The two
// hand off strictly (events one way, picks the other), so a game is a pure
// function of its reset.
type diracGame struct {
	id       string
	srv      *Server
	m        *oracle.Matchup
	seed     uint64
	ks       oracle.Side
	gs       *oracle.GorgeSide
	env      *mapping.Env
	prov     protocol.Provenance
	rc       oracle.RunConfig
	maxSteps uint64
	maxDec   uint64

	events chan gameEvent
	picks  chan int

	// v2 bookkeeping, session.Game's.
	step_, decisions  uint64
	seatStep, groupID [2]uint64
	pose              *mapping.Pose
	resp              *protocol.DecisionResponse
	term              *protocol.TerminalResponse

	// abort is the terminal a pick chose before stopping the loop.
	abort *protocol.TerminalResponse
	// queue is the rest of a multi-pick answer, for decision queueSeq: v2
	// commits the whole pick list at once, the canonical surface takes it a
	// pick at a time and then Done.
	queue    []int
	queueSeq uint64
	queued   bool
	// pending is the rest of a folded multi-intent transaction: newDig's
	// commit carries the dig_bottom order as a second intent, reached after
	// the take's canonical picks.
	pending []decision.Intent
	// chance is the kernel's recorded random outcomes, read when the game's
	// goroutine stops (Server.Chance); nil without recording.
	chance []oracle.ChanceDraw
}

func startGame(s *Server, r *protocol.ResetReq, sec *secrets.Game, decks [2][]*cards.Card, names [2]string) (*diracGame, error) {
	start := 0
	if r.Rules.StartingSeat != nil && *r.Rules.StartingSeat == "p1" {
		start = 1
	}
	// The kernel and the shadow draw their game from one seed, and with no
	// chooser the toss winner takes the first turn (gorge's R-9 fallback), so
	// the host-assigned starting seat picks the first seed whose toss it wins.
	seed := binary.BigEndian.Uint64(func() []byte { b := sec.StreamSeed("shared", "gorge_seed", 0); return b[:8] }())
	for k := 0; rules.TossWinner(seed, 2) != start; k++ {
		if k >= 256 {
			return nil, errors.New("no seed gives the starting seat the toss")
		}
		seed++
	}
	mulligans := 0
	if r.Rules.Mulligan == "london" {
		mulligans = 7
	}
	m, err := oracle.NewMatchup(s.reg, names, decks, mulligans)
	if err != nil {
		return nil, err
	}
	var es [2][]cardset.Entry
	for i, d := range decks {
		idx := map[string]int{}
		for _, c := range d {
			n := frontName(c)
			if j, ok := idx[n]; ok {
				es[i][j].Count++
				continue
			}
			idx[n] = len(es[i])
			es[i] = append(es[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := s.set.Matchup(es[0], es[1])
	if err != nil {
		return nil, err
	}
	// The v2 mapping carries a scry/surveil's pile-B order in Intent.Rest
	// (mapping's order_pick pick list), so both sides must model the kernel's
	// second arrange phase: the kernel appends the graveyard / bottom cards in
	// that order, and the shadow must pose the same canonical picks.
	cfg := oracle.Config{GorgeAutopaySurface: true, ArrangeRestOrder: true}
	kcfg := cfg
	kcfg.RecordChance = s.recordChance
	if s.kernelConfig != nil {
		s.kernelConfig(&kcfg)
	}
	ks, err := oracle.SetSide(s.set, tbl, kcfg)(m, seed)
	if err != nil {
		return nil, err
	}
	gs, err := oracle.NewGorgeSide(m, seed, cfg, nil)
	if err != nil {
		oracle.CloseSide(ks)
		return nil, err
	}
	gc := gamecfg.Wrap(gs.E, sec)
	tr := identity.New(gc.R, sec)
	dom := map[string]bool{}
	for _, n := range r.Rules.Names {
		dom[n] = true
	}
	g := &diracGame{id: r.GameID, srv: s, m: m, seed: seed, ks: ks, gs: gs, maxSteps: r.MaxSteps, maxDec: r.MaxDecisions,
		prov:   protocol.Provenance{EngineName: s.engine.Name, EngineVersion: s.engine.Version, RulesSnapshotID: s.engine.RulesSnapshotID, CardPoolIdentity: s.engine.CardPoolIdentity},
		events: make(chan gameEvent, 1), picks: make(chan int)}
	g.env = &mapping.Env{AutoPay: true, G: gc, IDs: tr, Obs: &observe.Projector{R: gc.R, IDs: tr},
		Domain: dom, Slots: map[string]uint32{}}
	// v2's own limits govern; the lockstep loop's caps are out of the way.
	g.rc = oracle.RunConfig{Config: cfg, MaxDecisions: math.MaxInt32, MaxTurns: math.MaxInt32}
	go g.run()
	g.take(<-g.events)
	return g, nil
}

// run plays the lockstep game: the kernel leads, the shadow follows, and this
// game's Pick is the policy that poses the shadow's decision to the agent.
func (g *diracGame) run() {
	var res oracle.Result
	defer func() {
		if r := recover(); r != nil {
			res = oracle.Result{Winner: -1, Div: &oracle.Divergence{What: "adapter", Field: "panic", Detail: fmt.Sprint(r)}}
		}
		if cs, ok := g.ks.(oracle.ChanceSource); ok && g.srv.recordChance {
			g.chance = cs.ChanceTranscript()
		}
		oracle.CloseSide(g.ks)
		g.events <- gameEvent{res: &res}
	}()
	res = oracle.LockstepLed(g.m, g.seed, g.ks, g.gs, g, g.rc)
}

// Pending is the decision awaiting a pick, or the terminal.
func (g *diracGame) Pending() (*protocol.DecisionResponse, *protocol.TerminalResponse) {
	if g.term != nil {
		return nil, g.term
	}
	return g.resp, nil
}

func (g *diracGame) over() bool { return g.term != nil }

// abandon stops a game in progress and waits for its goroutine.
func (g *diracGame) abandon() {
	if g.term != nil {
		return
	}
	close(g.picks)
	g.take(<-g.events)
}

func (g *diracGame) take(ev gameEvent) {
	if ev.resp != nil {
		g.resp = ev.resp
		return
	}
	g.finish(*ev.res)
}

func (g *diracGame) terminal(outcome, class, reason string, winner *string) *protocol.TerminalResponse {
	t := &protocol.TerminalResponse{ResponseType: "terminal", Protocol: protocol.Name, GameID: g.id,
		Outcome: outcome, Classification: class, Winner: winner, Reason: reason,
		StepCount: g.step_, DecisionCount: g.decisions, Provenance: g.prov,
		XGorge: &protocol.TerminalDiagnostics{AutoPayFallbacks: uint64(g.gs.Stats.Internal)}}
	return t
}

// halt ends the game with engine_contract_failure:cause, the detail going to
// stderr and never into the reason.
func (g *diracGame) halt(cause string, detail any) {
	fmt.Fprintf(os.Stderr, "sbv2dirac: %s: %v\n", cause, detail)
	g.stop(g.terminal("halted", "halted", "engine_contract_failure:"+cause, nil))
}

func (g *diracGame) truncate(cap string) { g.stop(g.terminal("truncated", "truncated", cap, nil)) }

func (g *diracGame) stop(t *protocol.TerminalResponse) {
	g.abort = t
	panic(abortSignal{})
}

// finish turns the lockstep result into the game's terminal.
func (g *diracGame) finish(res oracle.Result) {
	g.resp = nil
	switch {
	case g.abort != nil:
		g.term = g.abort
	case res.Div != nil && res.Over && res.Div.What == "terminal" && strings.HasSuffix(res.Div.Field, "asks after the loss"):
		// gorge keeps asking after a loss the kernel has already ended the
		// game on: the result stands (lockstep.go).
		g.term = g.natural(res.Winner)
	case res.Div != nil && res.Div.What == "adapter":
		fmt.Fprintf(os.Stderr, "sbv2dirac: adapter panic: %s\n", res.Div.Detail)
		g.term = g.terminal("halted", "halted", "engine_contract_failure:panic", nil)
	case res.Div != nil:
		fmt.Fprintf(os.Stderr, "sbv2dirac: %s\n", g.report(res.Div))
		g.srv.divergences = append(g.srv.divergences, res.Div)
		g.term = g.terminal("halted", "halted", "engine_contract_failure:divergence", nil)
	case strings.HasPrefix(res.Stall, oracle.KernelStallPrefix):
		fmt.Fprintf(os.Stderr, "sbv2dirac: kernel capacity stall: %s\n", res.Stall)
		g.term = g.terminal("halted", "halted", "engine_contract_failure:kernel_capacity", nil)
	case res.Over:
		g.term = g.natural(res.Winner)
	default:
		g.term = g.terminal("halted", "halted", "engine_contract_failure:no_result", nil)
	}
}

// report is the oracle's divergence report as stderr text.
func (g *diracGame) report(d *oracle.Divergence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (lead %s)", d, d.Lead)
	for _, x := range d.Diff {
		fmt.Fprintf(&b, "\n  %s", x)
	}
	if len(d.RefOptions)+len(d.KerOptions) > 0 {
		fmt.Fprintf(&b, "\n  gorge options  %v\n  kernel options %v", d.RefOptions, d.KerOptions)
	}
	return b.String()
}

func (g *diracGame) natural(winner int8) *protocol.TerminalResponse {
	if winner < 0 {
		return g.terminal("draw", "natural", "draw", nil)
	}
	w := observe.Seat(state.PlayerID(winner))
	return g.terminal(w+"_win", "natural", lossReason(g.gs.E.L.Events, 1-state.PlayerID(winner)), &w)
}

func lossReason(evs []events.Event, loser state.PlayerID) string {
	seat := observe.Seat(loser)
	for i := len(evs) - 1; i >= 0; i-- {
		if ev := evs[i]; ev.Kind == events.PlayerLost && ev.Player == loser {
			switch ev.Text {
			case "life total is 0 or less":
				return seat + "_life_zero"
			case "drew from an empty library":
				return seat + "_decked"
			case "ten or more poison counters":
				return seat + "_poison"
			}
			break
		}
	}
	return seat + "_lost"
}

// step is session.Game.Step: validate the pick against the pending pose,
// hand it to the lockstep goroutine and take what it poses next.
func (g *diracGame) step(req *protocol.StepReq) *protocol.Error {
	switch {
	case g.term != nil:
		return protocol.Errf(protocol.CodeGameAlreadyTerminal, "the game has ended")
	case req.ExpectedStep != g.step_:
		return protocol.Errf(protocol.CodeExpectedStepMismatch, fmt.Sprintf("pending step is %d", g.step_))
	case req.CandidateID >= uint64(len(g.pose.Candidates)):
		return protocol.Errf(protocol.CodeCandidateIDOutOfRange, "no such candidate")
	case !session.EchoEqual(req.Echo, g.pose.Candidates[req.CandidateID].Sem):
		return protocol.Errf(protocol.CodeSemanticEchoMismatch, "semantic_echo differs from the candidate")
	}
	g.picks <- int(req.CandidateID)
	g.take(<-g.events)
	return nil
}

// Pick implements oracle.Policy. The lockstep loop calls it once per
// canonical decision of the kernel; it poses the shadow's native decision to
// the agent, as many v2 steps as the transaction takes, and returns the
// canonical action the final pick stands for.
func (g *diracGame) Pick(c *oracle.PickCtx) oracle.Canon {
	e := g.gs.E
	d := e.Pending()
	if d == nil {
		g.halt("no_pending_decision", "the shadow poses a canonical decision with none pending")
	}
	if g.queued && g.queueSeq == d.Seq {
		return g.nextPick()
	}
	g.queued = false
	if len(g.pending) > 0 && g.pending[0].Seq == d.Seq && g.pending[0].Player == d.Player {
		in := g.pending[0]
		g.pending = g.pending[1:]
		return g.applyIntent(in, d)
	}
	if err := g.env.IDs.Sync(g.env.G.R); err != nil {
		g.halt("identity", err)
	}
	if in, ok := autoPayment(d); ok {
		return g.canon([]decision.Intent{in}, d)
	}
	if in, ok, err := mapping.Internal(g.env, d); ok || err != nil {
		if err != nil {
			g.mapFail(err, "submit_rejected", d)
		}
		return g.canon([]decision.Intent{in}, d)
	}
	if d.Kind == decision.KPriority {
		g.env.Action, g.env.Slots = nil, map[string]uint32{}
	}
	tx, err := mapping.Begin(g.env, d)
	if err != nil {
		g.mapFail(err, "unmapped_decision", d)
	}
	for {
		p, err := tx.Pose()
		if err != nil {
			g.mapFail(err, "dead_end", d)
		}
		i := g.ask(p, d)
		commit, done, err := tx.Answer(i)
		if err != nil {
			g.mapFail(err, "dead_end", d)
		}
		if done {
			g.env.CloseLooks()
			return g.canon(commit, d)
		}
	}
}

// mapFail halts on an error of gorge's mapping layer, the detail naming the
// decision and its source card.
func (g *diracGame) mapFail(err error, fallback string, d *decision.Decision) {
	g.halt(cause(err, fallback), fmt.Sprintf("%v (%s decision, source %s)", err, d.Kind, g.sourceName(d)))
}

// sourceName names the card behind d for a halt's stderr detail.
func (g *diracGame) sourceName(d *decision.Decision) string {
	if o := g.gs.E.G.Obj(d.Source); o != nil && o.Card != nil {
		return frontName(o.Card)
	}
	return "none"
}

// ask poses p to the agent and returns its candidate index (session.present
// and the head of session.answer).
func (g *diracGame) ask(p *mapping.Pose, d *decision.Decision) int {
	if p.GroupStart {
		if g.decisions >= g.maxDec {
			g.truncate("max_decisions")
		}
		if g.step_+uint64(p.SubstepCount) > g.maxSteps {
			g.truncate("max_steps")
		}
	}
	resp, err := g.present(p)
	if err != nil {
		g.halt("projection", err)
	}
	g.pose = p
	g.events <- gameEvent{resp: resp}
	i, ok := <-g.picks
	if !ok {
		g.stop(g.terminal("halted", "halted", "engine_contract_failure:abandoned", nil))
	}
	op := p.Candidates[i].Op
	if p.Context.Kind == "priority" {
		if obj := actionObject(d, op); obj != 0 {
			g.env.Action = &mapping.ActionContext{Seat: p.Seat, Obj: obj, Since: g.gs.E.G.NextID}
		}
	}
	g.step_++
	g.seatStep[p.Seat]++
	if p.SubstepIndex+1 == p.SubstepCount {
		g.groupID[p.Seat]++
		g.decisions++
	}
	return i
}

func (g *diracGame) present(p *mapping.Pose) (*protocol.DecisionResponse, error) {
	var holder *state.PlayerID
	if p.Context.Kind == "priority" || (g.env.Action != nil && g.env.Action.Seat == p.Seat) {
		h := p.Seat
		holder = &h
	}
	known := append([]protocol.Known(nil), p.Known...)
	observe.SortKnown(known)
	obs, err := g.env.Obs.Observation(p.Seat, observe.State{PriorityHolder: holder, Known: known})
	if err != nil {
		return nil, err
	}
	sd := protocol.SeatDecision{ActingSeat: observe.Seat(p.Seat), SeatStep: g.seatStep[p.Seat],
		Group:   protocol.Group{GroupID: g.groupID[p.Seat], SubstepIndex: p.SubstepIndex, SubstepCount: p.SubstepCount},
		Context: p.Context, Observation: obs, Extensions: map[string]json.RawMessage{}}
	for i, c := range p.Candidates {
		sd.Candidates = append(sd.Candidates, protocol.Candidate{CandidateID: uint32(i), Semantic: c.Sem})
	}
	return &protocol.DecisionResponse{ResponseType: "decision", Protocol: protocol.Name, GameID: g.id, Step: g.step_,
		SeatDecision: sd, Provenance: g.prov}, nil
}

// actionObject is the object a priority candidate acts from (session.actionObject).
func actionObject(d *decision.Decision, op mapping.NativeOp) state.ObjID {
	if op.Payment != nil {
		for _, a := range d.PaymentActions {
			if a.ID == op.Payment.ActionID {
				return a.Cast.Object
			}
		}
	}
	idx := op.Option
	if op.Op == "cast" && len(op.Covers) > 0 {
		idx = op.Covers[0]
	}
	if idx >= 0 && idx < len(d.Options) {
		return d.Options[idx].Obj
	}
	return 0
}

// canon translates the intents a finished transaction commits into the
// canonical action the lockstep loop applies. v2 commits a multi-pick answer
// whole; the canonical surface takes its picks one at a time, closed by Done,
// so the rest waits in the queue for the loop's next Pick calls. A folded
// transaction commits more than one intent (Vivien Reid's dig take then its
// dig_bottom order); the rest wait in pending for the Pick that reaches the
// decision each answers.
func (g *diracGame) canon(commit []decision.Intent, d *decision.Decision) oracle.Canon {
	if len(commit) == 0 {
		g.halt("followup_mismatch", "no intent commits for a canonical decision")
	}
	if len(commit) > 1 {
		g.pending = append(g.pending, commit[1:]...)
	}
	return g.applyIntent(commit[0], d)
}

// queueDone is the queue's sentinel for the Done that closes pile A; the
// rest of a scry/surveil's pile-B order follows it (Config.ArrangeRestOrder).
const queueDone = -1

// applyIntent translates one native intent into the canonical action it
// stands for, walking a multi-pick answer one CanonOfOption at a time. A
// scry/surveil's Intent.Rest carries the pile-B order: the canonical surface
// takes pile A's picks, then Done opening the kernel's rest phase, then
// pile B's picks but the last (the kernel implies it).
func (g *diracGame) applyIntent(in decision.Intent, d *decision.Decision) oracle.Canon {
	if g.gs.Multi() {
		g.queue = append([]int(nil), in.Choices...)
		if g.gs.Cfg.ArrangeRestOrder && len(in.Rest) >= 2 {
			g.queue = append(g.queue, queueDone)
			g.queue = append(g.queue, in.Rest[:len(in.Rest)-1]...)
		}
		g.queueSeq, g.queued = d.Seq, true
		return g.nextPick()
	}
	a, ok := g.gs.CanonOfIntent(in)
	if !ok {
		g.halt("untranslatable", fmt.Sprintf("no canonical action for %+v on %s", in, d.Kind))
	}
	return a
}

func (g *diracGame) nextPick() oracle.Canon {
	if len(g.queue) == 0 {
		a, ok := g.gs.CanonDone()
		if !ok {
			g.halt("untranslatable", "the multi-pick answer is not whole at its Done")
		}
		return a
	}
	idx := g.queue[0]
	g.queue = g.queue[1:]
	if idx == queueDone {
		a, ok := g.gs.CanonDone()
		if !ok {
			g.halt("untranslatable", "the multi-pick answer is not whole at its Done")
		}
		return a
	}
	a, ok := g.gs.CanonOfOption(idx)
	if !ok {
		g.halt("untranslatable", fmt.Sprintf("native option %d has no canonical pick", idx))
	}
	return a
}

// autoMana, autoPayment and cause are session's unexported helpers, which an
// engine_autopay environment applies before it poses anything.
func autoMana(d *decision.Decision) (decision.Intent, bool) {
	for _, kind := range []string{"autofill", "activate", "done"} {
		for _, o := range d.Options {
			if o.Kind == kind {
				return mapping.Intent(d, o.Index), true
			}
		}
	}
	return decision.Intent{}, false
}

func autoPayment(d *decision.Decision) (decision.Intent, bool) {
	switch mapping.Route(d) {
	case "choose/mana_window":
		return autoMana(d)
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

var causes = []struct {
	err  error
	name string
}{
	{mapping.ErrUnmapped, "unmapped_decision"}, {mapping.ErrDeadEnd, "dead_end"},
	{mapping.ErrCandidateLimit, "candidate_limit"}, {mapping.ErrDuplicate, "duplicate_candidates"},
	{mapping.ErrUnresolvableSource, "unresolvable_source"}, {gamecfg.ErrUnplannedRandomness, "unplanned_randomness"},
	{identity.ErrIDCollision, "id_collision"}, {identity.ErrShadowDiverged, "identity_shadow_diverged"},
}

func cause(err error, fallback string) string {
	for _, c := range causes {
		if errors.Is(err, c.err) {
			return c.name
		}
	}
	return fallback
}
