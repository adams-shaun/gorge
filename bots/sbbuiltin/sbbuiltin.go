// Package sbbuiltin hosts the three SpellBench builtin policies (sb-first,
// sb-uniform and sb-heuristic) behind one hosted adapter. Each name keeps
// its own registry entry and constructor, so the three remain separate
// policies; only the wrapper is shared.
package sbbuiltin

import (
	"context"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// The registered policy names.
const (
	FirstPolicy     = "sb-first"
	UniformPolicy   = "sb-uniform"
	HeuristicPolicy = "sb-heuristic"
)

const (
	packageSource = "docs/design/2026-09-28-hosted-bot-packages.md §3.2"
)

func init() {
	register(FirstPolicy, "The SpellBench first-option policy", NewFirst)
	register(UniformPolicy, "The SpellBench uniform policy", NewUniform)
	register(HeuristicPolicy, "The SpellBench heuristic policy", NewHeuristic)
}

func register(name, what string, ctor func(uint64, builtins.ManaMode) *builtins.Seat) {
	bots.Register(bots.Entry{Info: bots.Info{
		Name: name, Label: name + " (SpellBench)",
		Description: what + ", answering from the seat's view and using the honest root as its payment planner at priority.",
		Tier:        bots.Experimental,
		Cost:        bots.Cost{Scope: "per decision", Note: "negligible", Source: packageSource},
		Env:         true, Formats: []string{"constructed"}, MaxSeats: 2, Caretaker: "bot",
	}, New: func(o bots.Options) (seat.Seat, error) {
		mode := builtins.Manual
		if o.AutoPayMana {
			mode = builtins.AutoPay
		}
		return &hostedSeat{bot: ctor(o.Seed, mode)}, nil
	}})
}

// NewFirst constructs the sb-first seat with the requested mana surface,
// shared by the hosted and SpellBench registries.
func NewFirst(seed uint64, m builtins.ManaMode) *builtins.Seat {
	return builtins.New(builtins.First, m, seed)
}

// NewUniform constructs the sb-uniform seat, preserving the benchmark's
// UniformSeed stream in both the hosted and bench registries.
func NewUniform(seed uint64, m builtins.ManaMode) *builtins.Seat {
	return builtins.New(builtins.Uniform, m, seed^builtins.UniformSeed)
}

// NewHeuristic constructs the sb-heuristic seat. The same constructor is
// shared with the SpellBench registry to preserve its exact seed stream.
func NewHeuristic(seed uint64, m builtins.ManaMode) *builtins.Seat {
	return builtins.New(builtins.Heuristic, m, seed)
}

type hostedSeat struct{ bot *builtins.Seat }

var _ bots.EnvSeat = (*hostedSeat)(nil)
var _ bots.RefusalAnswerer = (*hostedSeat)(nil)
var _ seat.Seat = (*hostedSeat)(nil)

func (s *hostedSeat) WantsEnv(d *decision.Decision) bool { return d.Kind == decision.KPriority }
func (s *hostedSeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	if env.Search.Engine != nil {
		s.bot.SetPlanner(env.Search.Engine)
	} else {
		s.bot.SetPlanner(nil)
	}
	return s.bot.Decide(ctx, env.View, d)
}
func (s *hostedSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.bot.Decide(ctx, v, d)
}
func (s *hostedSeat) WantsPaymentActions() bool { return s.bot.WantsPaymentActions() }

// Seat exposes the wrapped builtin for registry adapters and routing tests.
func (s *hostedSeat) Seat() *builtins.Seat { return s.bot }
func (s *hostedSeat) AnswerRefused(v view.View, d decision.Decision, in decision.Intent) decision.Intent {
	return s.bot.Refused(v, d, in)
}
