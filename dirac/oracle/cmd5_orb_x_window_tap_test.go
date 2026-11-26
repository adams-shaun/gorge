package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestCmd5OrbXWindowTap: Shivan Devastator's {X}{R} is announced at X=3 with
// three Mountains' {R} floated and an untapped Orb of Dragonkind. gorge's
// payment window activates the Orb: the {1} fee paid from the floating pool,
// then its two mana of any combination of colours floating restricted, and
// the announced X paid (CR 601.2b, 605.3b). The kernel's window walk used to
// leave the fee gap unlisted and reverse the cast, keeping the pool's three
// mana short of the four the announcement needs. The card is unverified until
// a clean run lists it, so the table is built with cardset.AllowUnverified.
func TestCmd5OrbXWindowTap(t *testing.T) {
	t.Parallel()
	const name = "cmd5_orb_x_window_tap"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Divergence == nil || r.Divergence.At != 328 || r.Divergence.Field != "Cards[98].Flags" {
		t.Fatalf("%s is not the divergence at 328 in Cards[98].Flags: %+v", name, r.Divergence)
	}
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table or corpus (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatalf("set cmd5: %v", err)
	}
	set = cardset.AllowUnverified(set)
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
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
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	requireCmd5ReproClean(t, res, 328)
}
