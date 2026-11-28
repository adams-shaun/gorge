package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const rayOfRevelationRepro = "grove_gap_flashback_script_budget"

// CR 605.3b, 601.2g: Ray of Revelation's flashback {G} is payable by Grove of
// the Burnwillows' "{T}: Add {R} or {G}" (a census gap through its
// opponent-gains-1-life rider) with Racecourse Fury a legal target. Darkwater
// Catacombs' {1},{T} filter and Gemstone Caverns' counter-conditioned mana are
// two more gaps, so gorge's prefix search walks their colour and funding
// branches, exhausts its potentialScriptNodes budget (potentialScriptNodes,
// limited, no script) and prices the cast census_incomplete: it offers pass and
// two activations at decision 1854, while the kernel also offers the cast
// (tail-04-c000 pair 3, seed 755213). That was the divergence the card was
// refused for. At decision 1854 both engines now offer the cast: the recorded
// prefix is served whole and the decision compares clean. Past it, about fifty
// decisions on, the kernel ends the game in an unmodelled band stall whose
// recent sources do not include Ray of Revelation, so the recording is no clean
// proof; this test is the card's unrefusal proof (dirac/support/unrefused.tsv).
func TestMBCRayOfRevelationReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", rayOfRevelationRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(printedDelveSet(t, r.Cards))
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 1
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	sp := &ScriptPolicy{Actions: r.Actions}
	offered := false
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.Seq == len(r.Actions) {
			for _, o := range c.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Ray of Revelation" {
					offered = true
				}
			}
		}
		return sp.Pick(c)
	})
	res := Lockstep(m, r.Seed, k, pol, rc)
	if sp.Skipped != 0 || !sp.Exhausted() {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", sp.next, len(r.Actions), sp.Skipped)
	}
	if !res.Clean() {
		t.Fatalf("decision %d diverges: %v (stall %q)", len(r.Actions), res.Div, res.Stall)
	}
	if !offered {
		t.Fatalf("decision %d offers no cast of Ray of Revelation: the recording no longer reaches the flashback cast", len(r.Actions))
	}
}
