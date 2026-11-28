package neteval

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
)

// eval with no options (a value-only evaluation) leaves the hidden partial
// untouched; eval with options computes it and scores against it exactly.
func TestValueOnlySkipsPartial(t *testing.T) {
	const states = 50
	_, m := policynetNet(t, 0)
	ps := positions(4 * states)
	enc := feat.NewEncoder(vanillaPool())
	p := newPNScratch(m, pnMaxCards)
	sentinel := float32(math.Inf(1))
	var d dirac.Decision
	checked := 0
	for i, s := range ps {
		if checked == states {
			break
		}
		s.StepInto(&d)
		if d.N == 0 {
			continue
		}
		bot := make([]bool, d.N)
		bot[i%int(d.N)] = true
		st, opts := gorgeInputs(enc, s, &d, d.Player, bot)
		if len(opts) == 0 {
			t.Fatalf("state %d: decision with %d answers encoded no options", checked, d.N)
		}
		for h := range p.part {
			p.part[h] = sentinel
		}
		v := p.eval(&st, nil, nil)
		for h := range p.part {
			if p.part[h] != sentinel {
				t.Fatalf("state %d: value-only eval wrote the partial (unit %d = %v)", checked, h, p.part[h])
			}
		}
		out := make([]float32, len(opts))
		if got := p.eval(&st, opts, out); math.Float32bits(got) != math.Float32bits(v) {
			t.Fatalf("state %d: value %v with options, %v without", checked, got, v)
		}
		overwritten := 0
		for h := range p.part {
			if p.part[h] != sentinel {
				overwritten++
			}
		}
		if overwritten != len(p.part) {
			t.Fatalf("state %d: eval with options overwrote %d of %d partial units", checked, overwritten, len(p.part))
		}
		for j := range opts {
			if want := scoreRef(p, &opts[j]); math.Float32bits(out[j]) != math.Float32bits(want) {
				t.Fatalf("state %d option %d: eval %v, full pass %v", checked, j, out[j], want)
			}
		}
		checked++
	}
	if checked != states {
		t.Fatalf("checked %d of %d states", checked, states)
	}
}
