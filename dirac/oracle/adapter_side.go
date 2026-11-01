package oracle

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
)

// adapterSide is a lockstep side over a served game: Step reads the game's own
// native decision one kernel pick at a time, and Apply submits a whole Intent when
// the picks finish the answer. What it plays is what a client's answers play.
type adapterSide struct {
	g      *adapter.Game
	shadow *realSide // projects the game's position
	chosen []int
	rest   []int
	closed bool
	cur    []adapter.Pick
	err    error
	dirty  bool
}

// AdapterSide is a SideFactory over adapter games on set.
func AdapterSide(e *adapter.Engine, set cardset.Set, cfg Config) SideFactory {
	return func(m *Matchup, seed uint64) (Side, error) {
		gm, err := e.New(game.Config{Seed: seed, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{m.Decks[0], m.Decks[1]}, Tokens: m.reg.Tokens, Mulligans: m.Mulligans})
		if err != nil {
			return nil, err
		}
		g := gm.(*adapter.Game)
		k, err := SetSide(set, g.Table(), cfg)(m, seed)
		if err != nil {
			return nil, err
		}
		return &adapterSide{g: g, shadow: k.(*realSide), dirty: true}, nil
	}
}

// AdapterGame is the served game behind an AdapterSide, nil for any other side.
func AdapterGame(s Side) *adapter.Game {
	if a, ok := s.(*adapterSide); ok {
		return a.g
	}
	return nil
}

func (a *adapterSide) Step() CDecision {
	d := a.g.Pending()
	if d == nil || a.err != nil {
		return CDecision{Kind: DNone}
	}
	a.cur = a.g.Picks(a.chosen, a.closed, a.rest)
	out := CDecision{Kind: decisionKinds[d.Kind], Player: uint8(d.Player)}
	for _, p := range a.cur {
		out.Options = append(out.Options, p.Canon)
	}
	sortCanon(out.Options)
	return out
}

func (a *adapterSide) Apply(c Canon) {
	a.dirty = true
	d := a.g.Pending()
	for _, p := range a.cur {
		if p.Canon == c {
			a.pick(d, p)
			return
		}
	}
	a.err = fmt.Errorf("adapter side: %v not offered", c)
}

func (a *adapterSide) pick(d *decision.Decision, p adapter.Pick) {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch {
	case p.Choice <= adapter.PickPaid:
		act := &d.PaymentActions[adapter.PickPaid-p.Choice]
		in.Payment = &decision.PaymentSelection{ActionID: act.ID, Plan: act.Plans[0]}
		a.submit(in)
		return
	case d.Kind == decision.KPriority || d.Min == 1 && d.Max == 1:
		in.Choices = []int{p.Choice}
		a.submit(in)
		return
	case p.Choice == adapter.PickDone:
		a.closed = true
	case a.closed:
		a.rest = append(a.rest, p.Choice)
	default:
		a.chosen = append(a.chosen, p.Choice)
	}
	if !a.finished(d) {
		return
	}
	in.Choices = a.chosen
	if in.Choices == nil {
		in.Choices = []int{}
	}
	if a.closed {
		in.Rest = a.restWhole(d)
	}
	a.submit(in)
}

// finished reports whether the picks so far are a whole answer.
func (a *adapterSide) finished(d *decision.Decision) bool {
	if d.Kind == decision.KArrange && a.closed {
		n := len(d.Options) - len(a.chosen)
		return n < 2 || len(a.rest) == n-1
	}
	combat := d.Kind == decision.KAttackers || d.Kind == decision.KBlockers
	if a.closed {
		return true
	}
	ps := a.g.Picks(a.chosen, false, nil)
	if !combat && len(a.chosen) >= d.Max {
		return len(ps) != 1 || ps[0].Choice != adapter.PickDone
	}
	return ps == nil
}

// restWhole is pile B in full: the kernel implies its last card.
func (a *adapterSide) restWhole(d *decision.Decision) []int {
	if d.Kind != decision.KArrange || len(a.rest) == 0 {
		return nil
	}
	out := append([]int(nil), a.rest...)
	for i := range d.Options {
		if !containsInt(a.chosen, i) && !containsInt(a.rest, i) {
			out = append(out, i)
		}
	}
	return out
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func (a *adapterSide) submit(in decision.Intent) {
	a.chosen, a.rest, a.closed = nil, nil, false
	if err := a.g.Submit(in); err != nil {
		a.err = fmt.Errorf("adapter side: %w", err)
	}
}

func (a *adapterSide) sync() { a.g.CopyState(&a.shadow.s) }

func (a *adapterSide) Terminal() (bool, int8) {
	a.sync()
	return a.shadow.Terminal()
}

func (a *adapterSide) Decided() (bool, int8) { return a.Terminal() }

func (a *adapterSide) Project(p *Projection) {
	a.sync()
	a.dirty = false
	a.shadow.Project(p)
}

func (a *adapterSide) Dirty() bool { return a.dirty }

func (a *adapterSide) Turn() int {
	a.sync()
	return a.shadow.Turn()
}

func (a *adapterSide) Err() error { return a.err }

func (a *adapterSide) CapacityStall() string {
	a.sync()
	return a.shadow.CapacityStall()
}

func (a *adapterSide) StallIsUnmodelled() bool {
	a.sync()
	return a.shadow.StallIsUnmodelled()
}

var _ Side = (*adapterSide)(nil)
