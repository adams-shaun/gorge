package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// replayCmd5ReproUnverified is replayCmd5Repro over a cardset the Orb of
// Dragonkind is unverified on (in no evidence list yet): the same table and
// lockstep, the set's unverified cards played.
func replayCmd5ReproUnverified(t *testing.T, name string, at int, field string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Divergence == nil || r.Divergence.At != at || r.Divergence.Field != field {
		t.Fatalf("%s is not the divergence at %d in %s: %+v", name, at, field, r.Divergence)
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
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// TestCmd5OrbCensusFeePlanCast: the seat casts Taurean Mauler ({2}{R}) and
// pays it silently with three sacrificed Treasure tokens. Gorge's composed
// plan prices the census alone (paid_mana_census.go: a generic/coloured fee
// is the payment window's own price), so the kernel's plan prices no Orb of
// Dragonkind fee unit beside the census's -- the kernel used to compose
// {1},{T} + two Treasures, its {1} funded from the plan's own units, tapped
// the Orb and kept a Treasure (CR 605.3b, 601.2b).
func TestCmd5OrbCensusFeePlanCast(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_orb_census_fee_plan_cast", 794, "Cards[198].Flags")
	requireCmd5ReproClean(t, res, 794)
}

// TestCmd5OrbCensusFeePlanActivation: the seat activates an ability and pays
// it silently from the census. The kernel used to compose the payment with
// the Orb of Dragonkind's fee unit -- the phased script's plan, priced only
// for a play the census cannot pay -- tapping the Orb and floating a colour
// gorge never floated (CR 605.3b, 601.2b).
func TestCmd5OrbCensusFeePlanActivation(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_orb_census_fee_plan_activation", 2409, "Cards[398].Pos")
	requireCmd5ReproClean(t, res, 2409)
}
