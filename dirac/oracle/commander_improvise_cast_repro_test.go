package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A commander cast from the command zone is a plain cast to gorge's payment
// planner (payment_plan.go paymentPlanCastShapeDetailAltAdd: only a may-play
// or flashback scope, a Sac/TapX part or an X cost makes an Improvise cast
// "shape:contribution"; CR 702.126a, 903.8). Inspiring Statuary ("Nonartifact
// spells you cast have improvise", Deep Clue Sea) gives Morska, Undersea
// Sleuth that keyword, so gorge prices its {G}{W}{U} cast from the command zone
// (no generic for an artifact to credit) and offers it. The kernel lifted the
// improvise credit for a hand cast only (improviseLifted), so it declined the
// command-zone cast as a contribution shape and dropped the offer gorge made
// (pairs 4 and 49 of the edh -seats 4 -cmd-mini 100 sweep).
var commanderImproviseRepros = []struct {
	file    string
	firstAt int
}{
	{"commander_improvise_credit_plain_command_cast_a", 410},
	{"commander_improvise_credit_plain_command_cast_b", 249},
}

func commanderImproviseReplay(t *testing.T, reg *cards.Registry, file string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", file+".repro.json"))
	if err != nil {
		t.Fatal(err)
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
	decks := append([][]string{r.Cards[0], r.Cards[1]}, r.MoreCards...)
	es := make([][]cardset.Entry, 0, len(decks))
	for _, names := range decks {
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

// TestFourSeatCommanderImproviseCastLockstep: both four-seat Deep Clue Sea pods
// replay clean past the decision where the kernel left out Morska's cast.
func TestFourSeatCommanderImproviseCastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	for _, tc := range commanderImproviseRepros {
		res := commanderImproviseReplay(t, reg, tc.file)
		if res.Div != nil {
			t.Errorf("%s: divergence at %d: %s %s %v %v / %v", tc.file, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
		}
		if res.Decisions <= tc.firstAt {
			t.Errorf("%s: %d decisions, the game must go past decision %d", tc.file, res.Decisions, tc.firstAt)
		}
	}
}
