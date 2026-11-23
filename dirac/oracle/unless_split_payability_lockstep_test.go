package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 605.3a with CR 118.12: Glaciers' upkeep "sacrifice unless you pay {W}{U}"
// is payable by tapping one Sage of the Maze ("Add two mana in any combination
// of colors") for {W}{U} when no other source makes either colour. gorge's
// unless census priced a Combo production as pinned one-alt-per-colour
// alternatives ({W}{W} or {U}{U}, never the split), so the offer gate saw the
// cost as unpayable and posed decline only where the kernel offered the
// pay/decline election (seed 3 of the Glaciers/Sage/Forest matchup, decision
// 282). comboSplitAlts now prices every mixed-colour allocation too, each
// resolved onto its ability so the tap ask offers it with no colour sub-ask.
//
// The replay must pose the two-option election (the recorded game's last
// decision, where the sides diverged) and serve every recorded action.
func TestUnlessSplitPayabilityReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "unless_split_payability.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set := globSet(t, r.Pool, r)
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
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	script := &ScriptPolicy{Actions: r.Actions}
	elections := 0
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DModes && len(c.D.Options) == 2 {
			elections++
		}
		return script.Pick(c)
	})
	res := Lockstep(m, r.Seed, k, pol, rc)
	if msg := reproSkipFailure(r, script); msg != "" {
		t.Fatal(msg)
	}
	if d := res.Div; d != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v\nref %v\nker %v", d.At, len(r.Actions), d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
	}
	if elections == 0 {
		t.Fatal("precondition: the replay never posed a two-option pay/decline election")
	}
}
