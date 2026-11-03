package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestGrantedManaTapFloatsTheTapExtra replays a game where Honest Work turns
// a creature land into a Humble Merchant ("{T}: Add {C}"), which a Badgermole
// Cub's "whenever you tap a creature for mana, add an additional {G}" must
// follow when the merchant is tapped in a payment window: gorge floats {C}{G},
// and the pool must agree after it.
func TestGrantedManaTapFloatsTheTapExtra(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tla")
	r, err := ReadRepro("testdata/repro/humble_merchant_tap_extra.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	var es [2][]cardset.Entry
	for i := range es {
		for _, n := range r.Cards[i] {
			es[i] = append(es[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := set.Matchup(es[0], es[1])
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	CloseSide(k)
	if res.Div != nil && strings.HasPrefix(res.Div.Field, "Pool") {
		t.Fatalf("divergence at %d: %s %v", res.Div.At, res.Div.Field, res.Div.Diff)
	}
}
