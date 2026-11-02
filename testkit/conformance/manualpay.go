package conformance

import (
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/spellbench/payexec"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
)

// manualPayer plays what only PotentialActions lists on an engine whose seats tap
// their own mana. A client chooses the sources a play needs; here the planner chooses
// them, on a copy of the game fed every accepted answer, and only the taps go over the
// wire.
type manualPayer struct {
	t testing.TB
	e *rules.Engine
}

func newManualPayer(t testing.TB, e game.Engine, c game.Config) *manualPayer {
	t.Helper()
	c.Seed = host.MatchSeed(c.Seed, 1) // the match startTable plays
	g, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	gg, ok := g.(gorgegame.Game)
	if !ok {
		t.Fatalf("native wire: %s has manual mana and no planner to pay it with", e.Name())
	}
	return &manualPayer{t: t, e: gg.E}
}

// play pays for the play key names and makes it. It refuses a play the planner cannot
// price, and nothing has changed. A witness counts as accepted from its first tap, as
// tapping cannot be taken back. A script's play is answered as any other once its
// sources are tapped, so a refusal there shows as a difference between the engines.
func (m *manualPayer) play(s *nativeSession, key string) error {
	m.t.Helper()
	d := m.e.Pending()
	for _, pp := range m.e.PotentialPaymentPlans(d.Acting()) {
		a := pp.Action
		if playKey(a.Kind, a.Obj, a.Ability, a.Mode, s.names) != key {
			continue
		}
		switch {
		case len(pp.Script) > 0:
			m.runScript(s, pp.Script)
			s.pending, _ = s.poll()
			m.settleAsks(s, d.Player)
			return s.Answer(key)
		case pp.Plan != nil:
			play := payexec.Play{Kind: a.Kind, Obj: a.Obj, Ability: a.Ability, Mode: a.Mode}
			m.lower(s, payexec.StartPlay(d.Player, play, *pp.Plan, payexec.PoolOf(m.e, d.Player)))
			return nil
		}
		return fmt.Errorf("the planner cannot price %q: %s %s", key, pp.Reason, pp.Detail)
	}
	return fmt.Errorf("%q is not a potential play", key)
}

// lower answers the witness's activations, their colour asks and the play's option.
func (m *manualPayer) lower(s *nativeSession, x *payexec.Execution) {
	m.t.Helper()
	for x.Status() == payexec.InProgress {
		d := m.e.Pending()
		if d == nil {
			m.t.Fatal("native wire: the game ended while a witness was being tapped")
		}
		in, st := x.StepOn(d, payexec.SurfaceOf(m.e, d.Player))
		if st != payexec.InProgress && st != payexec.Done {
			m.t.Fatalf("native wire: lowering %+v stopped at %s (%s): %s %s", x.Play, d.Kind, d.Prompt, x.Reason, x.Detail)
		}
		if err := s.submit(in); err != nil {
			m.t.Fatalf("native wire: lowering %+v: %v", x.Play, err)
		}
		s.pending, _ = s.poll() // the host parks the next decision on its own goroutine
	}
}

// runScript answers a scripted prefix: activations of sources the planner's census
// cannot price, and their asks.
func (m *manualPayer) runScript(s *nativeSession, script []rules.ScriptStep) {
	m.t.Helper()
	for i, st := range script {
		d := m.e.Pending()
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		for _, pk := range st.Picks {
			j := slices.IndexFunc(d.Options, func(o decision.Option) bool {
				return pk.Matches(&o) && !slices.Contains(in.Choices, o.Index)
			})
			if j < 0 {
				m.t.Fatalf("native wire: script step %d: pick %q is not offered", i, pk.Label)
			}
			in.Choices = append(in.Choices, d.Options[j].Index)
		}
		if err := s.submit(in); err != nil {
			m.t.Fatalf("native wire: script step %d: %v", i, err)
		}
		s.pending, _ = s.poll()
	}
}

// settleAsks answers the colour asks the last scripted activation posed, the first
// option of each, until the payer has priority again.
func (m *manualPayer) settleAsks(s *nativeSession, p state.PlayerID) {
	m.t.Helper()
	for d := s.pending; d != nil && d.Kind != decision.KPriority && d.Player == p; d = s.pending {
		if err := s.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
			m.t.Fatalf("native wire: a scripted activation's ask: %v", err)
		}
		s.pending, _ = s.poll()
	}
}
