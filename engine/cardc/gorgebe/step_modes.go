package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbChooseModes, printChooseModes) }

// printChooseModes prints a modal step as a Charm node: Choices$ names one
// SVar per mode, each holding that mode's chain. A choose-N count other than
// one is CharmNum$.
func printChooseModes(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:choose-modes", st.Src)
	if st.Optional || st.Who != nil || len(st.Modes) < 2 || st.Cond != nil {
		return nil, 0, bad
	}
	n := &node{API: "Charm"}
	var names []string
	var heads []*node
	for _, m := range st.Modes {
		h, err := p.program(m)
		if err != nil {
			return nil, 0, err
		}
		heads = append(heads, h)
	}
	for _, h := range heads {
		names = append(names, p.sub("DB"+h.API, h))
	}
	n.set("Choices", strings.Join(names, ","))
	if st.N != nil && !(st.N.Kind == ir.VConst && st.N.N == 1) {
		a, err := p.amount(st.N)
		if err != nil {
			return nil, 0, err
		}
		n.set("CharmNum", a)
	}
	return n, 1, nil
}
