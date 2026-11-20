package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Two full 100-card four-seat edh pods that the first real four-seat sweep
// (kfuzz -kernel edh -seats 4 -seed 1) found divergent. Each recording is
// replayed here under its own name: its table is the edh kernel tables over
// its own four decks, which no glob helper builds (replayDecodedScript builds
// one from the first two decks).
//
//   - convoke_window_owed_colour_four_seat_pod (pair 30, decision 2536):
//     Captain Lannery Storm cast under We Ride at Dawn's convoke ({2}{R}, two
//     creatures convoked). The classic payment window carries no ManaPayment, so
//     the oracle derives the colour a dual land answers from the cost the window
//     still owes (CR 601.2g): rules.PendingCastOwed read nothing for a cast
//     without a fallback, so Tainted Peak answered {B} where the kernel answered
//     the owed {R}; gorge then paid the {R} through Rakdos Signet's filter
//     ability, tapping it and floating a third mana.
//   - active_player_hand_count_four_seat_pod (pair 58, decision 2618): Fevered
//     Visions deals 2 damage to an opponent with four or more cards in hand
//     (Count$ValidHand Card.ActivePlayerCtrl, the active player's hand). The
//     kernel lowered it as "yours on your turn, else the one opponent's", which
//     read one fixed opponent's hand on every opponent's turn (CR 102.1): wrong
//     from the third seat on. It now counts the hand walk's PredActiveCtrl.
func fourSeatPodReproReplay(t *testing.T, name string, reg *cards.Registry) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "one-off" {
		t.Fatalf("pool %q, want one-off", r.Pool)
	}
	if len(r.Cards) != 2 || len(r.MoreCards) != 2 {
		t.Fatalf("recording has %d+%d decks, want a four-seat pod", len(r.Cards), len(r.MoreCards))
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(reg, set, names))
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

func TestFourSeatConvokeWindowOwedColourLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatPodReproReplay(t, "convoke_window_owed_colour_four_seat_pod", synthReg(t))
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= 2536 {
		t.Errorf("%d decisions, the game must go past decision 2536", res.Decisions)
	}
}

func TestFourSeatActivePlayerHandCountLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatPodReproReplay(t, "active_player_hand_count_four_seat_pod", synthReg(t))
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= 2618 {
		t.Errorf("%d decisions, the game must go past decision 2618", res.Decisions)
	}
}
