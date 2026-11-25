package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Carnelian Orb of Dragonkind's mana carries a haste grant for the Dragon
// creature spell that spends it (CR 106.6). gorge spends a payment's plain
// units before a typed one (pay.takeUnit), so a unit the cast did not need stays
// in the pool with its grant, and the next Dragon spell that spends it gains
// haste. The recording is a full 100-card cmd5 pod (decision 1565): the oracle's
// lowered witness for Shivan Devastator over-taps for the largest X, the Orb's
// unit floats past Shivan's X = 4 payment, and Lathliss, Dragon Queen is cast
// with it. The kernel took the Orb's tap as spent on Shivan, so Lathliss had no
// haste in the kernel.
func TestFourSeatFloatingHasteRiderGoesToTheSpellThatSpendsItLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const name, at = "floating_haste_rider_spent_on_later_cast_four_seat_pod", 1565
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "one-off" || len(r.Cards) != 2 || len(r.MoreCards) != 2 {
		t.Fatalf("recording is pool %q with %d+%d decks, want a one-off four-seat pod", r.Pool, len(r.Cards), len(r.MoreCards))
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(synthReg(t), set, names))
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
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Errorf("%s: divergence at %d: %s %s %v", name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= at {
		t.Errorf("%s: %d decisions, the game must go past decision %d", name, res.Decisions, at)
	}
}
