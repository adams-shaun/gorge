package shadowseat

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/shadow"
)

// plan is the continuation of a priority answer that casts or activates: the
// staged State with the answer applied, and the Numbering it was staged with.
// The target decision that follows the answer is searched from it. One State
// is allocated per Seat; held says it carries a plan.
type plan struct {
	st   *dirac.State
	num  *shadow.Numbering
	held bool
}

func newPlan() plan { return plan{st: new(dirac.State)} }

// start holds root, which must be at its pending decision, with a applied.
func (p *plan) start(root *dirac.State, num *shadow.Numbering, a dirac.Action) {
	p.st.CopyFrom(root)
	p.st.Apply(a)
	p.num, p.held = num, true
}

// apply plays acts, the picks of a search at the plan's decision.
func (p *plan) apply(acts []dirac.Action) {
	for _, a := range acts {
		p.st.Apply(a)
	}
}

func (p *plan) clear() { p.num, p.held = nil, false }

// continuesPlan reports whether priority answer a opens a target decision:
// every action but a pass and a land play.
func continuesPlan(a dirac.Action) bool {
	return a.Kind() != dirac.AkPass && a.Kind() != dirac.AkPlayLand
}
