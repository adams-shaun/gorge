//go:build dirac_mp

package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestPartnerPodLibraryLockstep replays a four-seat Mystery Booster Commander
// draft pod, every seat a partner pair, that diverged on one library: on the
// turn The Mindskinner's controller attacked two opponents, gorge milled every
// opponent of its controller by the damage dealt ("each opponent mills that
// many cards", CR 102.3) and the kernel only the damaged ones, so a third
// library kept cards gorge had put in the graveyard and the rest of it sat one
// place off. The record carries no draft-pairing flag: the replay has to
// derive it from the decks, or gorge rejects the pairs at decision 0 and the
// game is another one.
func TestPartnerPodLibraryLockstep(t *testing.T) {
	base, _ := testCorpus(t)
	set, err := cardset.ByName("mbc")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no mbc table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	r, err := ReadRepro(filepath.Join("testdata", "partner_pod", "mindskinner_mills_each_opponent.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(base.Reg)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Commander || m.NSeat() != 4 || !m.DraftPartners {
		t.Fatalf("precondition: commander %v, %d seats, draft pairs %v: want a four-seat Commander pod of draft pairs", m.Commander, m.NSeat(), m.DraftPartners)
	}
	for s := range m.NSeat() {
		if c, p := m.SeatCommanders(s); c < 0 || p < 0 {
			t.Fatalf("precondition: seat %d commanders %d, %d: want a partner pair", s, c, p)
		}
	}
	lists := make([][]cardset.Entry, m.NSeat())
	for s, d := range append([][]string{r.Cards[0], r.Cards[1]}, r.MoreCards...) {
		for _, n := range d {
			lists[s] = append(lists[s], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, err := set.MatchupN(lists)
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
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions < len(r.Actions) {
		t.Errorf("the replay ended after %d of %d decisions (stall %q)", res.Decisions, len(r.Actions), res.Stall)
	}
}
