// Package gamecfg builds a gorge engine whose every random draw comes from
//
// Ported from github.com/jackmaiorino/spellbench/engines/gorge/internal/gamecfg
// at revision 1beea4c104911bba77a5672c644e570ed9afe28a (MIT).
// Section 11.6 streams: rules.NewHypotheticalChancePlanned asks a planner for
// each library shuffle (per player and ordinal) and for every non-shuffle
// chance draw (the shared chance stream), and the chance prefix forces the
// toss to the host-assigned starting seat. The hypothetical constructor is
// used deliberately as the live engine; see the plan's Global Constraints.
package gamecfg

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

var ErrUnplannedRandomness = errors.New("engine_contract_failure:unplanned_randomness")

type Rules struct {
	Mulligan     string // "london" or "none"
	StartingSeat state.PlayerID
	// ShufflePlanner, when non-nil, supplies every library shuffle order
	// instead of the game secret's library_shuffle stream. Nil keeps the
	// default secret-derived plan.
	ShufflePlanner func(rules.ShuffleContext) ([]state.ObjID, error)
	// ChancePrefix, when non-nil, is the engine's chance prefix: its draws
	// are consumed in order from the start of the game, beginning with the
	// starting-seat toss. It replaces the forced toss and the secret-derived
	// shuffle plan, so it must cover every draw up to the point it ends; the
	// chance planner then covers non-shuffle draws past it. Nil is the
	// default: the forced toss plus the secret-derived shuffle plan.
	ChancePrefix []rules.ChanceDraw
}

type Game struct {
	E *rules.Engine
	// R is how the adapter reads the game.
	R      Reader
	Secret *secrets.Game
}

// Wrap is the Game over an engine its caller already holds: a host's live
// engine, or a clone a resample replays on.
func Wrap(e *rules.Engine, sec *secrets.Game) *Game {
	return &Game{E: e, R: engineReader{e}, Secret: sec}
}

// New returns the game at its first decision, even alongside a CheckRandomness error.
func New(reg *cards.Registry, sec *secrets.Game, decks [2][]*cards.Card, r Rules) (*Game, error) {
	g := &Game{Secret: sec}
	seed := sec.StreamSeed("shared", "gorge_seed", 0)
	cfg := rules.Config{
		Seed:         binary.BigEndian.Uint64(seed[:8]),
		Names:        []string{"p0", "p1"},
		Decks:        [][]*cards.Card{decks[0], decks[1]},
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
	}
	if r.Mulligan == "london" {
		cfg.Mulligans = 7
	}
	// The prefix is a contiguous transcript: a planned shuffle cannot be
	// appended while it still has unconsumed values (rules.forcePermutation),
	// so a supplied prefix replaces both the forced toss and the default
	// secret-derived plan. An explicit planner may still be combined with a
	// prefix that does not span its shuffles.
	prefix := []rules.ChanceDraw{{Bound: 2, Value: int(r.StartingSeat)}}
	planner := g.plan
	if r.ChancePrefix != nil {
		prefix = r.ChancePrefix
		planner = nil
	}
	if r.ShufflePlanner != nil {
		planner = r.ShufflePlanner
	}
	e, err := rules.NewHypotheticalChancePlanned(cfg, prefix, planner, g.chance)
	if err != nil {
		return nil, err
	}
	g.E, g.R = e, engineReader{e}
	if err := e.AdvanceHypothetical(); err != nil {
		return nil, err
	}
	// The engine publishes payment plans lazily, only when a host asks. The
	// adapter always asks: the priority ask it projects offers those plans.
	e.EnsurePaymentActions()
	return g, g.CheckRandomness()
}

// plan draws the shuffle of ctx.Player's library from that seat's own stream.
func (g *Game) plan(ctx rules.ShuffleContext) ([]state.ObjID, error) {
	r := g.Secret.Stream(fmt.Sprintf("p%d", ctx.Player), "library_shuffle", uint64(ctx.Ordinal))
	out := make([]state.ObjID, len(ctx.Library))
	for i, c := range ctx.Library {
		out[i] = c.ID
	}
	for i := len(out) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// chance draws one non-shuffle value from the match's shared chance stream.
func (g *Game) chance(ctx rules.ChanceContext) (int, error) {
	return g.Secret.Stream("shared", "chance", uint64(ctx.Ordinal)).IntN(ctx.Bound), nil
}

// CheckRandomness fails when the engine drew a value no planner supplied.
func (g *Game) CheckRandomness() error {
	if n := g.E.UnplannedDraws(); n != 0 {
		return fmt.Errorf("%w: %d draws", ErrUnplannedRandomness, n)
	}
	return nil
}

func (g *Game) Submit(in decision.Intent) error {
	if err := g.E.SubmitHypothetical(in); err != nil {
		return err
	}
	g.E.EnsurePaymentActions()
	return g.CheckRandomness()
}

// Probe submits ins to a clone. Clones have no planner, so a probe's own
// shuffles draw from the clone's generator; probes judge legality only.
// A clone still holds the real hidden zones and the secret-seeded generator,
// so no probe outcome may set a decision's shape through hidden-zone
// contents (Section 13 F3); Task 28's resample check is the net.
func (g *Game) Probe(ins ...decision.Intent) (*rules.Engine, error) {
	return probe(g.E, ins...)
}

func probe(e *rules.Engine, ins ...decision.Intent) (*rules.Engine, error) {
	c := e.Clone()
	if !e.Hypothetical() {
		// The host's live engine has no chance boundary; a probe only judges
		// legality, so give the clone one rather than reject every option.
		c = e.CloneHypothetical(0)
	}
	for _, in := range ins {
		if err := c.SubmitHypothetical(in); err != nil {
			return nil, err
		}
	}
	return c, nil
}
