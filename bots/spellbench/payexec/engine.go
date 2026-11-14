package payexec

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// ActionFor returns the payment action gorge's planner offers for casting
// obj at e's pending priority decision (building the lazy extension if it
// is not built yet), and whether one with a plan is offered.
func ActionFor(e *rules.Engine, obj state.ObjID) (decision.PaymentAction, bool) {
	for _, a := range e.EnsurePaymentActions() {
		if a.Cast.Object == obj && len(a.Plans) > 0 {
			return decision.ClonePaymentAction(a), true
		}
	}
	return decision.PaymentAction{}, false
}

// Prover is an Execution.Prove for a live engine: it submits the answer on a
// throwaway clone of e (e is untouched) and reports whether the payer's pool
// then gained exactly the witness's production. An answer that poses a
// further ask (a stage-2 colour pick) or is refused proves nothing.
func Prover(e *rules.Engine) func(d *decision.Decision, act decision.PaymentActivation, choices []int) bool {
	return func(d *decision.Decision, act decision.PaymentActivation, choices []int) bool {
		c := e.Clone()
		before := PoolOf(c, d.Player)
		if c.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}) != nil {
			return false
		}
		after := PoolOf(c, d.Player)
		var gained decision.ManaAmount
		for i := range after {
			if after[i] > before[i] {
				gained[i] = after[i] - before[i]
			}
		}
		return gained == act.Produces
	}
}

// PoolOf is player p's floating pool in e.
func PoolOf(e *rules.Engine, p state.PlayerID) decision.ManaAmount {
	var m decision.ManaAmount
	if int(p) >= len(e.G.Players) {
		return m
	}
	for i, n := range e.G.Players[p].Pool {
		if i < len(m) && n > 0 {
			m[i] = uint32(n)
		}
	}
	return m
}

// PoolFromView is player p's floating pool as a View projects it.
func PoolFromView(v view.View, p state.PlayerID) decision.ManaAmount {
	var m decision.ManaAmount
	for i := range v.Players {
		if v.Players[i].ID != p {
			continue
		}
		for c := 0; c < len(syms); c++ {
			if n := v.Players[i].Pool[syms[c:c+1]]; n > 0 {
				m[c] = uint32(n)
			}
		}
	}
	return m
}

// Run lowers a (an action offered at e's pending priority decision) on e
// itself: Start, then Step and Submit until the cast is submitted (Done) or
// the surface diverges (Aborted, with e left at the decision the Execution
// declined to answer). err reports an engine refusal of a lowered answer.
func Run(e *rules.Engine, a decision.PaymentAction) (x *Execution, err error) {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		x = &Execution{cur: -1}
		x.abort(ReasonUnexpected, "no pending priority decision")
		return x, nil
	}
	return Drive(e, Start(d.Player, &a, PoolOf(e, d.Player)), nil)
}

// SurfaceOf is player p's surface in e: its floating pool and the stack.
func SurfaceOf(e *rules.Engine, p state.PlayerID) Surface {
	return Surface{Pool: PoolOf(e, p), Stack: len(e.G.Stack)}
}

// Drive runs x on e until Done or Aborted: each payer decision is Stepped
// and submitted. A Yield is answered by yield (nil: the decision's first Min
// options), and another seat's priority decision posed while the lowering
// waits for the stack is answered with pass; any other decision for
// another seat aborts (ReasonWrongPlayer). A test and tooling driver.
func Drive(e *rules.Engine, x *Execution, yield func(*decision.Decision) decision.Intent) (*Execution, error) {
	for x.Status() == InProgress {
		d := e.Pending()
		if d == nil || e.G.Over {
			x.abort(ReasonUnexpected, "no pending decision")
			return x, nil
		}
		var in decision.Intent
		if d.Player != x.Player && d.Kind == decision.KPriority && x.Waits > 0 {
			in = decision.Intent{Seq: d.Seq, Player: d.Player}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					in.Choices = []int{o.Index}
					break
				}
			}
		} else {
			var st Status
			in, st = x.StepOn(d, SurfaceOf(e, d.Player))
			switch st {
			case Aborted:
				return x, nil
			case Yield:
				if yield != nil {
					in = yield(d)
				} else {
					in = decision.Intent{}
					for j := 0; j < d.Min && j < len(d.Options); j++ {
						in.Choices = append(in.Choices, d.Options[j].Index)
					}
				}
				in.Seq, in.Player = d.Seq, d.Player
			}
		}
		if err := e.Submit(in); err != nil {
			x.abort(ReasonUnexpected, "refused: "+err.Error())
			return x, fmt.Errorf("payexec: lowered answer refused: %w", err)
		}
	}
	return x, nil
}
