package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A four-seat edh pod (seed 2 of the 100-card sweep) with Faeburrow Elder
// ("{T}: for each colour among permanents you control, add one mana of that
// colour") and Orzhov Signet ("{1},{T}: Add {W}{B}") untapped beside every land
// tapped, Staff of Compleation ({3}) in hand. gorge's PotentialMana folds the
// Elder through addPotentialMana, where producedOpen("Special EachColorAmong...")
// is true: the Elder prices unbounded in every colour, covers the Signet's {1}
// fee, and the Signet is a census gap beside it (CR 605.3a, 601.2g), so the
// Elder's G, the Signet's {W}{B} pay the cast. The kernel's first-only fee cover
// (filtOtherMana) left each-colour sources out, so it priced neither the Signet
// nor the cast.
const eachColorFeePod = "eachcolor_filter_fee_staff_pod"

// eachColorFeePodAt is the decision the recording diverged at.
const eachColorFeePodAt = 946

// eachColorFeeReplay is fourSeatEdhReplay with the one card the edh table spells
// by its Universes Within name (E. Honda, Sumo Champion is Baldin, Century
// Herdmaster there) mapped.
func eachColorFeeReplay(t *testing.T) Result {
	t.Helper()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", eachColorFeePod+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("edh")
	if err != nil {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		en := ReproEntries(reg, set, names)
		for i := range en {
			if en[i].Name == "E. Honda, Sumo Champion" {
				en[i].Name = "Baldin, Century Herdmaster"
			}
		}
		es = append(es, en)
	}
	tbl, _, err := set.MatchupN(es)
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

// TestFourSeatEachColorFeesFirstOnlyFilterLockstep: the pod plays past the cast.
func TestFourSeatEachColorFeesFirstOnlyFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := eachColorFeeReplay(t)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= eachColorFeePodAt {
		t.Fatalf("%d decisions, the game must go past decision %d", res.Decisions, eachColorFeePodAt)
	}
}

// TestFourSeatEachColorFeesFirstOnlyFilterMutation: each half of the fix, left
// out, diverges at the recorded decision: the fee cover (filtOtherMana) and the
// filter search's each-colour step (mixEach). It also shows the pod reaches the
// interaction.
func TestFourSeatEachColorFeesFirstOnlyFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, sw := range map[string]*bool{"fee cover": &dirac.DebugEachGapNoPotential, "filter step": &dirac.DebugEachNoFilterChain} {
		*sw = true
		res := eachColorFeeReplay(t)
		*sw = false
		if res.Div == nil {
			t.Errorf("%s: no divergence with the fix left out", name)
		} else if res.Div.At != eachColorFeePodAt {
			t.Errorf("%s: diverged at %d, want %d", name, res.Div.At, eachColorFeePodAt)
		}
	}
}
