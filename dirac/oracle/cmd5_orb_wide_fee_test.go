package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestCmd5OrbWideFeeAnswersThePlaysColour: Orb of Dragonkind ("{1}, {T}: Add
// two mana in any combination of colors", Dragon spells and abilities only) is
// tapped in the prefix of Lathliss's {1}{R} pump. gorge's script search answers
// the colour ask with the first pair naming a colour the play announces (W R),
// so the pump is paid from the two restricted units. The kernel floated the
// source's first colour twice (W W) and the pump found no red to pay (CR
// 106.6, 601.2b, 605.3b). The card is unverified until a clean run lists it,
// so the table is built with cardset.AllowUnverified.
func TestCmd5OrbWideFeeAnswersThePlaysColour(t *testing.T) {
	t.Parallel()
	const name = "cmd5_orb_wide_fee_answers_the_plays_colour"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Divergence == nil || r.Divergence.At != 973 || r.Divergence.Field != "Pool[0][0]" {
		t.Fatalf("%s is not the divergence at 973 in Pool[0][0]: %+v", name, r.Divergence)
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
	requireCmd5ReproClean(t, res, 973)
}
