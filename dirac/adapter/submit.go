package adapter

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Submit answers the pending decision. It dry-runs the whole answer first, so a
// pick the kernel does not offer at its step leaves the game untouched.
func (g *Game) Submit(in decision.Intent) error {
	d := g.pend
	if d == nil {
		return fmt.Errorf("dirac: no pending decision")
	}
	if err := d.Validate(in); err != nil {
		return err
	}
	if d.Kind == decision.KPriority && in.Payment == nil && d.Options[in.Choices[0]].Kind == "concede" {
		g.log.Intents = append(g.log.Intents, in)
		g.concede(uint8(d.Player))
		return nil
	}
	g.scratch()
	if err := g.playAnswer(in); err != nil {
		return err
	}
	g.s.CopyFrom(g.ws)
	g.log.Intents = append(g.log.Intents, in)
	g.settle()
	return nil
}

// scratch puts the dry-run position at the live one.
func (g *Game) scratch() {
	if g.ws == nil {
		g.ws = new(dirac.State)
		g.wtr.S = g.ws
	}
	g.ws.CopyFrom(&g.s)
	g.wd = g.d
	g.wkp = g.kp
}

// concede ends the game for p. The kernel is not told: its state is simply never
// advanced again.
func (g *Game) concede(p uint8) {
	g.conceded, g.loser, g.pend = true, p, nil
	g.log.Append(events.Event{Kind: events.PlayerLost, Player: state.PlayerID(p), Text: "conceded"})
	g.logOver(int8(1 - p))
	g.materialise()
}

// playAnswer plays a whole valid answer on the scratch position.
func (g *Game) playAnswer(in decision.Intent) error {
	if in.Payment != nil {
		for i := range g.pend.PaymentActions {
			if g.pend.PaymentActions[i].ID == in.Payment.ActionID {
				g.ws.Apply(g.paid[i])
				return nil
			}
		}
		return fmt.Errorf("dirac: no payment action %q", in.Payment.ActionID)
	}
	if !g.multi {
		g.ws.Apply(g.acts[in.Choices[0]])
		return nil
	}
	if err := g.playPicks(in.Choices); err != nil {
		return err
	}
	if !g.closes(len(in.Choices)) {
		return nil
	}
	if err := g.playDone(); err != nil {
		return err
	}
	rest := in.Rest
	if len(rest) == 0 {
		rest = g.complement(in.Choices)
	}
	return g.playRest(rest)
}

// closes reports whether n picks leave the kernel waiting for its Done: a combat
// declaration always, any other ask until it has its most picks or has nothing left
// to offer but Done.
func (g *Game) closes(n int) bool {
	if g.d.Kind == dirac.DkAttackers || g.d.Kind == dirac.DkBlockers || n < g.pend.Max {
		return true
	}
	g.ws.StepInto(&g.wd)
	return g.sameAsk() && g.wd.N == 1 && isDecline(g.wd.Options[0])
}

// playPicks plays native options in answer order. Options are matched by canonical
// form, since a pick can renumber the kernel's own action encoding for the ones left.
func (g *Game) playPicks(choices []int) error {
	for i, c := range choices {
		if i > 0 {
			g.ws.StepInto(&g.wd)
			g.ws.Project(&g.wkp)
			if !g.sameAsk() {
				// The kernel takes the last pick of a forced order itself.
				if i == len(choices)-1 {
					return nil
				}
				return fmt.Errorf("dirac: the ask ended before pick %d", i)
			}
		}
		a, ok := g.kernelPick(c)
		if !ok {
			return fmt.Errorf("dirac: %s option %d is not on offer at pick %d", g.pend.Kind, c, i)
		}
		g.ws.Apply(a)
	}
	return nil
}

// sameAsk reports whether the scratch position still poses the pending ask.
func (g *Game) sameAsk() bool {
	return g.wd.Kind == g.d.Kind && g.wd.Sub == g.d.Sub && g.wd.Player == g.d.Player
}

// kernelPick is the scratch step's action for native option c.
func (g *Game) kernelPick(c int) (dirac.Action, bool) {
	want := g.canons[c]
	for i := 0; i < int(g.wd.N); i++ {
		if g.wtr.Action(g.wd.Options[i]) == want {
			return g.wd.Options[i], true
		}
	}
	return 0, false
}

func (g *Game) playDone() error {
	g.ws.StepInto(&g.wd)
	g.ws.Project(&g.wkp)
	if g.sameAsk() {
		for i := 0; i < int(g.wd.N); i++ {
			if isDecline(g.wd.Options[i]) {
				g.ws.Apply(g.wd.Options[i])
				return nil
			}
		}
	}
	return fmt.Errorf("dirac: the answer leaves the ask unfinished")
}

// isDecline reports the kernel's closing action: Done, or declining an "up to"
// target.
func isDecline(a dirac.Action) bool {
	return a.Kind() == dirac.AkDone || a.Kind() == dirac.AkTarget && a.Target() == dirac.NoTarget
}

// complement is the options an answer did not choose, in offered order.
func (g *Game) complement(choices []int) []int {
	var out []int
	for i := range g.pend.Options {
		if !slices.Contains(choices, i) {
			out = append(out, i)
		}
	}
	return out
}

// playRest plays a scry or surveil's other pile, all but the last card, which the
// kernel implies.
func (g *Game) playRest(rest []int) error {
	if g.pend.Kind != decision.KArrange || len(rest) < 2 {
		return nil
	}
	for i, c := range rest[:len(rest)-1] {
		g.ws.StepInto(&g.wd)
		g.ws.Project(&g.wkp)
		a, ok := g.kernelPick(c)
		if !ok {
			return fmt.Errorf("dirac: option %d is not on offer at rest pick %d", c, i)
		}
		g.ws.Apply(a)
	}
	return nil
}

// Pick is one thing the next kernel step of an answer can be. Choice is the native
// option it stands for, PickDone closing the ask, or PickPaid-k payment action k.
type Pick struct {
	Canon  canon.Canon
	Choice int
}

const (
	PickDone = -1
	PickPaid = -2
)

// Picks lists the kernel's next step of a partial answer to the pending decision:
// the options chosen so far, with closed once the ask's Done is played and rest the
// other pile's picks. It returns nil when those picks finish the answer. A wire that
// asks one pick at a time reads it, and so does the lockstep that checks the fold.
func (g *Game) Picks(chosen []int, closed bool, rest []int) []Pick {
	if g.pend == nil {
		return nil
	}
	g.scratch()
	if len(chosen) > 0 && g.playPicks(chosen) != nil {
		return nil
	}
	if closed && (g.playDone() != nil || g.playPartialRest(rest) != nil) {
		return nil
	}
	if len(chosen) > 0 || closed {
		g.ws.StepInto(&g.wd)
		g.ws.Project(&g.wkp)
		if !g.sameAsk() {
			return nil
		}
	}
	out := make([]Pick, 0, g.wd.N)
	for i := 0; i < int(g.wd.N); i++ {
		a := g.wd.Options[i]
		c := g.wtr.Action(a)
		p := Pick{Canon: c, Choice: PickDone}
		if !isDecline(a) {
			p.Choice = g.choiceOf(c, chosen)
		}
		out = append(out, p)
	}
	return out
}

func (g *Game) playPartialRest(rest []int) error {
	for i, c := range rest {
		g.ws.StepInto(&g.wd)
		g.ws.Project(&g.wkp)
		a, ok := g.kernelPick(c)
		if !ok {
			return fmt.Errorf("dirac: option %d is not on offer at rest pick %d", c, i)
		}
		g.ws.Apply(a)
	}
	return nil
}

// choiceOf is the native option or payment action a step's canonical action is.
func (g *Game) choiceOf(c canon.Canon, chosen []int) int {
	for i, x := range g.canons {
		if x == c && !slices.Contains(chosen, i) {
			return i
		}
	}
	for i, a := range g.paid {
		if g.tr.Action(a) == c {
			return PickPaid - i
		}
	}
	return PickDone
}
