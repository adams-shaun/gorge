// v2gorgehost.go: the gorge side of kfuzz -v2 transcript when it must deal
// the dirac kernel's chance. gorge's own v2 environment (server/spellbench/env
// over session.Game) hard-codes gamecfg.Rules, so it cannot take the kernel's
// recorded draws from config. This is session.Game and env.Server's reset/step
// path, ported line for line from the pinned gorge, with one difference: when
// the dirac host recorded chance, the game is built by
// v2ChanceFollowingGame, which installs the kernel's transcript as an
// ordinal-addressed chance planner (see its comment for why gamecfg.Rules
// cannot).
// With no transcript it is gorge's host (TestV2GorgeHostMatchesEnv pins that
// against env.Server on real games), so a transcript difference is still the
// dirac host's, not this port's.
//
// Gorge finding: gamecfg.Rules should expose a
// ChancePlanner (or session.Config / env.Server should plumb one), then this
// port's local constructor can go.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/session"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// v2GorgeHost is a gorge v2 host (hello, reset, step) that can deal the dirac
// kernel's recorded chance. It serves one game at a time and trusts its
// caller: the harness speaks well-formed requests, so the catalog and rules
// validation of env.Server is not repeated.
type v2GorgeHost struct {
	reg   *cards.Registry
	decks map[string]catalog.Deck
	// chance is the dirac kernel's recorded draws, in order (the toss,
	// library shuffles and ordinary draws). nil keeps gamecfg's own
	// secret-stream chance, which is the negative control.
	chance []rules.ChanceDraw
	game   *v2GorgeGame
}

func newV2GorgeHost(reg *cards.Registry, chance []rules.ChanceDraw) *v2GorgeHost {
	return &v2GorgeHost{reg: reg, decks: map[string]catalog.Deck{}, chance: chance}
}

func (h *v2GorgeHost) AddDeck(d catalog.Deck) { h.decks[d.CatalogID] = d }

func v2Marshal(v any) []byte {
	b, _ := wire.Canonical(v)
	return b
}

func v2ErrResp(id string, e *protocol.Error) []byte {
	return v2Marshal(protocol.ErrorResponse{ResponseType: "error", Protocol: protocol.Name, RequestID: id,
		Error: protocol.ErrorBody{Code: e.Code, Message: e.Message}})
}

// Handle answers one request line.
func (h *v2GorgeHost) Handle(line []byte) []byte {
	req, perr := protocol.Decode(line)
	if perr != nil {
		return v2ErrResp(req.ID, perr)
	}
	switch req.Type {
	case "hello":
		return v2Marshal(map[string]any{"response_type": "hello_ok", "protocol": protocol.Name, "request_id": req.ID})
	case "reset":
		return h.reset(req)
	case "step":
		if h.game == nil {
			return v2ErrResp(req.ID, protocol.Errf(protocol.CodeStepBeforeReset, "no game"))
		}
		if req.Step.GameID != h.game.id {
			return v2ErrResp(req.ID, protocol.Errf(protocol.CodeGameIDMismatch, "another game is active"))
		}
		if perr := h.game.step(req.Step); perr != nil {
			return v2ErrResp(req.ID, perr)
		}
		return h.respond(req.ID)
	}
	return v2ErrResp(req.ID, protocol.Errf(protocol.CodeUnsupportedRequest, req.Type+" is not implemented"))
}

func (h *v2GorgeHost) respond(id string) []byte {
	dec, term := h.game.pending()
	if term != nil {
		t := *term
		t.RequestID = id
		return v2Marshal(t)
	}
	d := *dec
	d.RequestID = id
	return v2Marshal(d)
}

func (h *v2GorgeHost) reset(req protocol.Request) []byte {
	r := req.Reset
	var decks [2][]*cards.Card
	for i, spec := range r.Decks {
		d, ok := h.decks[spec.CatalogID]
		if !ok {
			return v2ErrResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "unknown deck "+spec.CatalogID))
		}
		cs, err := catalog.Resolve(h.reg, d)
		if err != nil {
			return v2ErrResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, err.Error()))
		}
		decks[i] = cs
	}
	sec, err := secrets.ParseGame(r.GameSecret)
	if err != nil {
		return v2ErrResp(req.ID, protocol.Errf(protocol.CodeMalformedRequest, err.Error()))
	}
	start := state.PlayerID(0)
	if r.Rules.StartingSeat != nil && *r.Rules.StartingSeat == "p1" {
		start = 1
	}
	var g *gamecfg.Game
	if h.chance != nil {
		g, err = v2ChanceFollowingGame(h.reg, sec, decks, r.Rules.Mulligan, start, h.chance)
	} else {
		g, err = gamecfg.New(h.reg, sec, decks, gamecfg.Rules{Mulligan: r.Rules.Mulligan, StartingSeat: start})
	}
	if err != nil {
		return v2ErrResp(req.ID, protocol.Errf(protocol.CodeUnsupportedDeck, "engine could not start: "+err.Error()))
	}
	tr := identity.New(g.R, sec)
	dom := map[string]bool{}
	for _, n := range r.Rules.Names {
		dom[n] = true
	}
	s := &v2GorgeGame{id: r.GameID, g: g, maxSteps: r.MaxSteps, maxDecisions: r.MaxDecisions,
		prov: protocol.Provenance{EngineName: "v2-gorge-host"},
		env:  &mapping.Env{AutoPay: true, G: g, IDs: tr, Obs: &observe.Projector{R: g.R, IDs: tr}, Domain: dom, Slots: map[string]uint32{}}}
	s.advance()
	h.game = s
	return h.respond(req.ID)
}

// v2ChanceFollowingGame is gamecfg.New with the kernel's recorded draws as a
// chance planner. The env always installs its own secret-stream chance planner
// (gorge sbp-env-chance-streams), so a gamecfg.Rules caller cannot arrive at
// the kernel's values; ChancePrefix could, but the prefix stays in every
// mapping probe's cloned chance state and changes candidate shape (pair 64
// seed 7 differs at decision 513 that way). A planner that reads the
// transcript by draw ordinal avoids both: gamecfg's own chance planner is
// ordinal-addressed, so clones need no cursor of their own.
//
// Every draw -- the genesis shuffles, later library shuffles, coin flips, dice
// and random selections -- is read from the kernel's transcript at the draw's
// own ordinal, so no draw kind can be skipped and no cursor is shared. A clone
// (resolve_board copies the chance planner into probe/search worlds) can ask
// at an ordinal whose bound is not the kernel's next draw -- a search probe's
// finish shuffle, say; that is a hypothetical world, not the game, so it gets
// gamecfg's own secret-stream value and the probe judges legality exactly as
// the env does. The real game's aligned draws are the kernel's.
func v2ChanceFollowingGame(reg *cards.Registry, sec *secrets.Game, decks [2][]*cards.Card, mulligan string, start state.PlayerID, tr []rules.ChanceDraw) (*gamecfg.Game, error) {
	if len(tr) == 0 || tr[0].Bound != 2 || tr[0].Value != int(start) {
		return nil, fmt.Errorf("kernel chance transcript does not begin with the seat-%d toss", start)
	}
	seed := sec.StreamSeed("shared", "gorge_seed", 0)
	// The dirac host takes the first seed whose toss it wins (session's
	// host-assigned seat, sbv2.startGame) and gives that seed to its gorge
	// shadow. Use the same one so the engine generator behind the forced draws
	// is at the shadow's position: a mapping probe that ignores the chance
	// planner reads the generator, and a raw-seed engine would judge it from a
	// different stream.
	seed64 := binary.BigEndian.Uint64(seed[:8])
	for k := 0; rules.TossWinner(seed64, 2) != int(start); k++ {
		if k >= 256 {
			return nil, fmt.Errorf("no seed gives seat %d the toss", start)
		}
		seed64++
	}
	cfg := rules.Config{
		Seed:         seed64,
		Names:        []string{"p0", "p1"},
		Decks:        [][]*cards.Card{decks[0], decks[1]},
		Tokens:       reg.Tokens,
		NameUniverse: oracle.NameUniverseFor(reg, decks),
	}
	if mulligan == "london" {
		cfg.Mulligans = 7
	}
	// Draw 0 is the toss, supplied by the prefix, so chance ordinal k is
	// transcript draw k+1. The engine's ordinal starts at zero and counts
	// draws, not shuffle sites. A draw whose bound is not the kernel's next
	// one is a hypothetical clone world (a mapping probe's finish shuffle, for
	// example), not the game: gamecfg's own chance planner is copied into
	// those worlds, so give it the same secret-stream value gamecfg would and
	// let the probe judge legality, exactly as the env does.
	chance := func(ctx rules.ChanceContext) (int, error) {
		if i := ctx.Ordinal + 1; i < len(tr) && tr[i].Bound == ctx.Bound {
			return tr[i].Value, nil
		}
		return sec.Stream("shared", "chance", uint64(ctx.Ordinal)).IntN(ctx.Bound), nil
	}
	e, err := rules.NewHypotheticalChancePlanned(cfg, tr[:1], nil, chance)
	if err != nil {
		return nil, err
	}
	g := gamecfg.Wrap(e, sec)
	if err := e.AdvanceHypothetical(); err != nil {
		return nil, err
	}
	// The env publishes payment plans before it poses; gamecfg.New does the
	// same and then rejects a game whose draws no planner supplied.
	e.EnsurePaymentActions()
	return g, g.CheckRandomness()
}

// v2GorgeGame is session.Game for an engine_autopay environment with no
// extensions and no audit.
type v2GorgeGame struct {
	id                string
	g                 *gamecfg.Game
	env               *mapping.Env
	prov              protocol.Provenance
	tx                mapping.Transaction
	native            *decision.Decision
	pose              *mapping.Pose
	resp              *protocol.DecisionResponse
	term              *protocol.TerminalResponse
	step_, decisions  uint64
	seatStep, groupID [2]uint64
	maxSteps          uint64
	maxDecisions      uint64
	autoPayFallbacks  uint64
}

func (s *v2GorgeGame) pending() (*protocol.DecisionResponse, *protocol.TerminalResponse) {
	if s.term != nil {
		return nil, s.term
	}
	return s.resp, nil
}

func (s *v2GorgeGame) recoverPanic() {
	if r := recover(); r != nil {
		fmt.Fprintf(os.Stderr, "gorge engine panic: %v\n", r)
		if _, ok := r.(*rules.LivelockError); ok {
			s.halt("livelock")
		} else {
			s.halt("panic")
		}
	}
}

func (s *v2GorgeGame) advance() {
	defer s.recoverPanic()
	for s.term == nil {
		if s.tx == nil {
			e := s.g.E
			if e.G.Over {
				s.natural()
				return
			}
			d := e.Pending()
			if d == nil {
				s.halt("no_pending_decision")
				return
			}
			if in, ok := v2AutoPayment(d); ok {
				s.autoPayFallbacks++
				if err := s.submit(in, nil); err != nil {
					s.halt(v2Cause(err, "submit_rejected"))
					return
				}
				continue
			}
			if in, ok, err := mapping.Internal(s.env, d); ok || err != nil {
				if err == nil {
					err = s.submit(in, nil)
				}
				if err != nil {
					s.halt(v2Cause(err, "submit_rejected"))
					return
				}
				continue
			}
			if d.Kind == decision.KPriority {
				s.env.Action, s.env.Slots = nil, map[string]uint32{}
			}
			tx, err := mapping.Begin(s.env, d)
			if err != nil {
				s.halt(v2Cause(err, "unmapped_decision"))
				return
			}
			s.tx, s.native = tx, d
		}
		p, err := s.tx.Pose()
		if err != nil {
			s.halt(v2Cause(err, "dead_end"))
			return
		}
		if p.GroupStart {
			if s.decisions >= s.maxDecisions {
				s.truncate("max_decisions")
				return
			}
			if s.step_+uint64(p.SubstepCount) > s.maxSteps {
				s.truncate("max_steps")
				return
			}
		}
		if err := s.present(p); err != nil {
			s.halt(v2Cause(err, "projection"))
		}
		return
	}
}

func (s *v2GorgeGame) present(p *mapping.Pose) error {
	var holder *state.PlayerID
	if p.Context.Kind == "priority" || (s.env.Action != nil && s.env.Action.Seat == p.Seat) {
		h := p.Seat
		holder = &h
	}
	known := append([]protocol.Known(nil), p.Known...)
	observe.SortKnown(known)
	obs, err := s.env.Obs.Observation(p.Seat, observe.State{PriorityHolder: holder, Known: known})
	if err != nil {
		return err
	}
	sd := protocol.SeatDecision{ActingSeat: observe.Seat(p.Seat), SeatStep: s.seatStep[p.Seat],
		Group:   protocol.Group{GroupID: s.groupID[p.Seat], SubstepIndex: p.SubstepIndex, SubstepCount: p.SubstepCount},
		Context: p.Context, Observation: obs, Extensions: map[string]json.RawMessage{}}
	for i, c := range p.Candidates {
		sd.Candidates = append(sd.Candidates, protocol.Candidate{CandidateID: uint32(i), Semantic: c.Sem})
	}
	s.pose = p
	s.resp = &protocol.DecisionResponse{ResponseType: "decision", Protocol: protocol.Name, GameID: s.id, Step: s.step_,
		SeatDecision: sd, Provenance: s.prov}
	return nil
}

func (s *v2GorgeGame) step(req *protocol.StepReq) *protocol.Error {
	switch {
	case s.term != nil:
		return protocol.Errf(protocol.CodeGameAlreadyTerminal, "the game has ended")
	case req.ExpectedStep != s.step_:
		return protocol.Errf(protocol.CodeExpectedStepMismatch, fmt.Sprintf("pending step is %d", s.step_))
	case req.CandidateID >= uint64(len(s.pose.Candidates)):
		return protocol.Errf(protocol.CodeCandidateIDOutOfRange, "no such candidate")
	case !session.EchoEqual(req.Echo, s.pose.Candidates[req.CandidateID].Sem):
		return protocol.Errf(protocol.CodeSemanticEchoMismatch, "semantic_echo differs from the candidate")
	}
	s.answer(int(req.CandidateID))
	return nil
}

func (s *v2GorgeGame) actionObject(op mapping.NativeOp) state.ObjID {
	if op.Payment != nil {
		for _, a := range s.native.PaymentActions {
			if a.ID == op.Payment.ActionID {
				return a.Cast.Object
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

func (s *v2GorgeGame) answer(i int) {
	defer s.recoverPanic()
	p := s.pose
	seat := p.Seat
	op := p.Candidates[i].Op
	if p.Context.Kind == "priority" {
		if obj := s.actionObject(op); obj != 0 {
			s.env.Action = &mapping.ActionContext{Seat: seat, Obj: obj, Since: s.g.E.G.NextID}
		}
	}
	commit, done, err := s.tx.Answer(i)
	s.step_++
	s.seatStep[seat]++
	if p.SubstepIndex+1 == p.SubstepCount {
		s.groupID[seat]++
		s.decisions++
	}
	if err != nil {
		s.halt(v2Cause(err, "dead_end"))
		return
	}
	keys := v2FollowKeys(op)
	for k, in := range commit {
		var want *decision.Decision
		if k > 0 && in.Seq == 0 && k-1 < len(keys) {
			want = p.Followups[keys[k-1]]
		}
		if err := s.submit(in, want); err != nil {
			s.halt(v2Cause(err, "submit_rejected"))
			return
		}
	}
	if done {
		s.tx = nil
		s.env.CloseLooks()
	}
	s.advance()
}

var errV2Followup = errors.New("engine_contract_failure:followup_mismatch")

func v2FollowKeys(op mapping.NativeOp) []string {
	keys := make([]string, 0, len(op.Followup))
	key := strconv.Itoa(op.Option)
	for _, f := range op.Followup {
		keys = append(keys, key)
		key += "/" + strconv.Itoa(f)
	}
	return keys
}

func v2SameAsk(d, want *decision.Decision) bool {
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

func (s *v2GorgeGame) submit(in decision.Intent, want *decision.Decision) error {
	d := s.g.E.Pending()
	if d == nil {
		return errV2Followup
	}
	if in.Seq == 0 {
		if want == nil || !v2SameAsk(d, want) {
			return errV2Followup
		}
		in.Seq, in.Player = d.Seq, d.Player
	}
	if in.Seq != d.Seq || in.Player != d.Player {
		return errV2Followup
	}
	if err := s.g.Submit(in); err != nil {
		fmt.Fprintf(os.Stderr, "gorge rejected intent %+v: %v\n", in, err)
		return err
	}
	return s.env.IDs.Sync(s.g.R)
}

// v2AutoPayment is session.autoPayment: the asks an engine_autopay
// environment answers itself before posing anything.
func v2AutoPayment(d *decision.Decision) (decision.Intent, bool) {
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

var v2Causes = []struct {
	err  error
	name string
}{
	{mapping.ErrUnmapped, "unmapped_decision"}, {mapping.ErrDeadEnd, "dead_end"},
	{mapping.ErrCandidateLimit, "candidate_limit"}, {mapping.ErrDuplicate, "duplicate_candidates"},
	{mapping.ErrUnresolvableSource, "unresolvable_source"}, {gamecfg.ErrUnplannedRandomness, "unplanned_randomness"},
	{identity.ErrIDCollision, "id_collision"}, {identity.ErrShadowDiverged, "identity_shadow_diverged"},
	{errV2Followup, "followup_mismatch"},
}

func v2Cause(err error, fallback string) string {
	fmt.Fprintf(os.Stderr, "gorge adapter: %v\n", err) // detail stays on stderr, never in reason
	for _, c := range v2Causes {
		if errors.Is(err, c.err) {
			return c.name
		}
	}
	return fallback
}

func (s *v2GorgeGame) terminal(outcome, class, reason string, winner *string) {
	s.term = &protocol.TerminalResponse{ResponseType: "terminal", Protocol: protocol.Name, GameID: s.id,
		Outcome: outcome, Classification: class, Winner: winner, Reason: reason,
		StepCount: s.step_, DecisionCount: s.decisions, Provenance: s.prov,
		XGorge: &protocol.TerminalDiagnostics{AutoPayFallbacks: s.autoPayFallbacks}}
	s.tx, s.resp = nil, nil
}

func (s *v2GorgeGame) halt(c string) {
	s.terminal("halted", "halted", "engine_contract_failure:"+c, nil)
}

func (s *v2GorgeGame) truncate(cap string) { s.terminal("truncated", "truncated", cap, nil) }

func (s *v2GorgeGame) natural() {
	e := s.g.E
	if e.G.Draw {
		s.terminal("draw", "natural", "draw", nil)
		return
	}
	w := observe.Seat(e.G.Winner)
	s.terminal(w+"_win", "natural", v2LossReason(e.L.Events, 1-e.G.Winner), &w)
}

func v2LossReason(evs []events.Event, loser state.PlayerID) string {
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
