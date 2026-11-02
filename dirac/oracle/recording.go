package oracle

import (
	"fmt"
	"math"
	"slices"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
)

// LockstepRecording plays the answers a served game recorded against gorge from the
// same seed, comparing the two at every decision. The recording's own game has
// already replayed: an error here is a recording that game cannot play.
func LockstepRecording(e *adapter.Engine, reg *cards.Registry, l *events.Log, c game.Config) (Result, error) {
	if len(c.Decks) != 2 {
		return Result{}, fmt.Errorf("a recording of %d seats", len(c.Decks))
	}
	m, err := NewMatchup(reg, [2]string{"p0", "p1"}, [2][]*cards.Card{c.Decks[0], c.Decks[1]}, c.Mulligans)
	if err != nil {
		return Result{}, err
	}
	return lockstepRecording(e, m, l)
}

func lockstepRecording(e *adapter.Engine, m *Matchup, l *events.Log) (Result, error) {
	cfg := Config{ArrangeRestOrder: true, GorgeAutopaySurface: e.Autopay()}
	side := AdapterSide(e, cfg)
	k, err := side(m, l.Seed)
	if err != nil {
		return Result{}, err
	}
	defer CloseSide(k)
	actions, err := k.(*adapterSide).recordedActions(l)
	if err != nil {
		return Result{}, err
	}
	if len(actions) == 0 {
		return Result{}, nil
	}
	follow, err := side(m, l.Seed)
	if err != nil {
		return Result{}, err
	}
	defer CloseSide(follow)
	pol := PolicyFunc(func(c *PickCtx) Canon { return actions[c.Seq] })
	return Lockstep(m, l.Seed, follow, pol, RunConfig{Config: cfg, MaxDecisions: len(actions), MaxTurns: math.MaxInt32}), nil
}

// recordedActions plays l's intents on a and returns the kernel picks they made. A
// concession ends them: the kernel is not told of it.
func (a *adapterSide) recordedActions(l *events.Log) ([]Canon, error) {
	var out []Canon
	for i, in := range l.Intents {
		d := a.g.Pending()
		if d == nil {
			return nil, fmt.Errorf("no decision pending at intent %d", i)
		}
		if d.Kind == decision.KPriority && len(in.Choices) > 0 && d.Options[in.Choices[0]].Kind == "concede" {
			break
		}
		before := len(a.g.Log().Intents)
		for len(a.g.Log().Intents) == before {
			a.Step()
			want := a.nextChoice(d, in)
			j := slices.IndexFunc(a.cur, func(p adapter.Pick) bool { return p.Choice == want })
			if j < 0 {
				return nil, fmt.Errorf("intent %d: pick %d is not on offer", i, want)
			}
			c := a.cur[j].Canon
			out = append(out, c)
			a.Apply(c)
			if a.err != nil {
				return nil, fmt.Errorf("intent %d: %w", i, a.err)
			}
		}
	}
	return out, nil
}

// nextChoice is the pick that continues answer in, as Picks numbers them.
func (a *adapterSide) nextChoice(d *decision.Decision, in decision.Intent) int {
	id := ""
	switch {
	case in.Payment != nil:
		id = in.Payment.ActionID
	case in.Announce != nil:
		id = in.Announce.ActionID
	}
	if id != "" {
		k := slices.IndexFunc(d.PaymentActions, func(p decision.PaymentAction) bool { return p.ID == id })
		return adapter.PickPaid - k
	}
	switch {
	case d.Kind == decision.KPriority || d.Min == 1 && d.Max == 1:
		return in.Choices[0]
	case !a.closed && len(a.chosen) < len(in.Choices):
		return in.Choices[len(a.chosen)]
	case !a.closed:
		return adapter.PickDone
	}
	rest := in.Rest
	if len(rest) == 0 {
		for i := range d.Options {
			if !slices.Contains(in.Choices, i) {
				rest = append(rest, i)
			}
		}
	}
	return rest[len(a.rest)]
}
