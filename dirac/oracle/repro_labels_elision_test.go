package oracle

import (
	"github.com/adams-shaun/gorge/dirac/cardset"
	"path/filepath"
	"testing"
)

// TestScriptPolicyLabelIndexSkipsElidedApplies: with Config.PassElision Step
// answers single-option decisions itself; those answers are not in
// Result.Actions, so the apply index a label's At is taken from must count
// only the actions the policy served. The replay of a recording with hundreds
// of elided decisions ends with applyN equal to the actions served.
func TestScriptPolicyLabelIndexSkipsElidedApplies(t *testing.T) {
	if isolate(t) {
		return
	}
	name := "sunken_citadel_two_untapped_pair"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !r.PassElision {
		t.Fatalf("%s is not recorded with pass elision: the test pins nothing", name)
	}
	reg := synthReg(t)
	set := globSet(t, r.Pool, r)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(entriesOf(r.Cards[0]), entriesOf(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	applied := -1
	rc.Finish = func(g *GorgeSide) { applied = g.applyN }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, r.Script(), rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
	}
	if res.Stats.Elided == 0 {
		t.Fatal("no decision was elided: the apply index cannot drift here")
	}
	if applied != len(res.Actions) {
		t.Fatalf("the gorge side counted %d applies for %d served actions (%d elided): a label's At would drift", applied, len(res.Actions), res.Stats.Elided)
	}
}

func entriesOf(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}
