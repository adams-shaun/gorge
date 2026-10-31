// Package gorgegame runs engine/rules behind the game interfaces.
package gorgegame

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// Name is the name the rules engine reports and a host selects it by.
const Name = "gorge"

var (
	_ game.Game           = Game{}
	_ game.PaymentPlanner = Game{}
	_ game.Engine         = Engine{}
)

// Game is a running rules engine. It is one pointer wide, so a game.Game holds
// it without allocating: a snapshot per turn costs only its Clone.
type Game struct{ E *rules.Engine }

func (g Game) Board() *state.Game { return g.E.G }

// Chars is the engine itself, so the optional interfaces view and botpolicy
// probe for stay visible through it.
func (g Game) Chars() view.Chars { return g.E }

func (g Game) Log() *events.Log { return g.E.L }

func (g Game) Pending() *decision.Decision { return g.E.Pending() }

func (g Game) Submit(in decision.Intent) error { return g.E.Submit(in) }

func (g Game) Fork() game.Game { return Game{E: g.E.Clone()} }

func (g Game) Caps() game.Caps {
	return game.Caps{Engine: Name, SubBurstViews: true, ManualMana: true, SearchRoots: true, StartingAsk: true}
}

func (g Game) EnsurePaymentActions() { g.E.EnsurePaymentActions() }

func (g Game) EnsureTargetHints() { g.E.EnsureTargetHints() }

func (g Game) ValidateCastPayment(p state.PlayerID, cast decision.PlannedCast, plan decision.PaymentPlan) error {
	return g.E.ValidateCastPayment(p, cast, plan)
}

func (g Game) ValidateCastAnnounce(p state.PlayerID, cast decision.PlannedCast) error {
	return g.E.ValidateCastAnnounce(p, cast)
}

// Rules returns g's rules engine, or nil when another engine runs g.
func Rules(g game.Game) *rules.Engine {
	if gg, ok := g.(Game); ok {
		return gg.E
	}
	return nil
}

// RulesConfig maps c to the rules.Config that builds and replays its game.
func RulesConfig(c game.Config) rules.Config {
	cfg := rules.Config{
		Seed:                  c.Seed,
		Names:                 c.Names,
		PlayerNames:           c.PlayerNames,
		Decks:                 c.Decks,
		Sideboards:            c.Sideboards,
		Archetypes:            c.Archetypes,
		Tokens:                c.Tokens,
		NameUniverse:          c.NameUniverse,
		NameUniverseNames:     c.NameUniverseNames,
		Mulligans:             c.Mulligans,
		WindowDiagnostics:     c.WindowDiagnostics,
		DisablePaidManaCensus: c.DisablePaidManaCensus,
		Format:                rules.Format(c.Format),
		StartingLife:          c.StartingLife,
		Commanders:            c.Commanders,
		StartChooser:          c.StartChooser,
		ExpectedEvents:        c.ExpectedEvents,
	}
	switch {
	case c.GuardDisabled:
		cfg.LoopGuard = &rules.LoopGuard{Disabled: true}
	case !c.NoObjectCap:
		// A token storm ends as a recorded stall instead of growing the arena
		// and log until the process runs out of memory.
		cfg.LoopGuard = rules.WithDefaultObjectCap(nil)
	}
	return cfg
}

// GameConfig is RulesConfig's inverse for a config a host builds: one with no
// fields game.Config does not carry, whose LoopGuard is nil, disabled or the
// default object cap.
func GameConfig(cfg rules.Config) game.Config {
	return game.Config{
		Seed:                  cfg.Seed,
		Names:                 cfg.Names,
		PlayerNames:           cfg.PlayerNames,
		Decks:                 cfg.Decks,
		Sideboards:            cfg.Sideboards,
		Archetypes:            cfg.Archetypes,
		Tokens:                cfg.Tokens,
		NameUniverse:          cfg.NameUniverse,
		NameUniverseNames:     cfg.NameUniverseNames,
		Mulligans:             cfg.Mulligans,
		WindowDiagnostics:     cfg.WindowDiagnostics,
		DisablePaidManaCensus: cfg.DisablePaidManaCensus,
		Format:                game.Format(cfg.Format),
		StartingLife:          cfg.StartingLife,
		Commanders:            cfg.Commanders,
		StartChooser:          cfg.StartChooser,
		GuardDisabled:         cfg.LoopGuard != nil && cfg.LoopGuard.Disabled,
		NoObjectCap:           cfg.LoopGuard == nil,
		ExpectedEvents:        cfg.ExpectedEvents,
	}
}

// Engine builds rules-engine games.
type Engine struct{}

func (Engine) Name() string { return Name }

// Accepts is nil: the rules engine plays every format and seat count a Config
// can name.
func (Engine) Accepts(game.Config) error { return nil }

func (Engine) New(c game.Config) (game.Game, error) {
	e := rules.NewStartingPlayerChoice(RulesConfig(c))
	// Pose CR 103.1's choice before advancing, or Advance applies the toss
	// winner's deterministic default.
	e.AskStartingPlayer()
	e.Advance()
	return Game{E: e}, nil
}

func (Engine) Replay(l *events.Log, c game.Config) (game.Game, error) {
	e, err := replay.Replay(l, RulesConfig(c))
	return wrap(e), err
}

func (Engine) ReplayTo(l *events.Log, c game.Config, n int) (game.Game, error) {
	e, err := replay.ReplayTo(l, RulesConfig(c), n)
	return wrap(e), err
}

func (Engine) Walk(l *events.Log, c game.Config, n int, visit func(g game.Game, i int) error) (game.Game, error) {
	e, err := replay.Walk(l, RulesConfig(c), n, func(e *rules.Engine, i int) error {
		return visit(Game{E: e}, i)
	})
	return wrap(e), err
}

// wrap keeps a nil engine, which replay returns for a nil log, a nil Game.
func wrap(e *rules.Engine) game.Game {
	if e == nil {
		return nil
	}
	return Game{E: e}
}
