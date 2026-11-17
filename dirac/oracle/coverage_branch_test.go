package oracle

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestCoverageBranchKeysInGame plays one seeded lockstep game over the
// kevidenceDecks fixture with WithCoverage and expects a card the game's own
// event log shows was cast to carry a nonzero Branch count, the branch file to
// list it, and the six-column coverage TSV to stay six columns.
func TestCoverageBranchKeysInGame(t *testing.T) {
	const seed = 3
	reg, set, gd := lockstepPool(t, "evidence", kevidenceDecks)
	tbl, _, _, err := set.Matchup(kevidenceDecks[0], kevidenceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewMatchup(reg, [2]string{"evidence-a", "evidence-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, Config{})(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	cast := map[string]bool{}
	var cov *Coverage
	rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40, Finish: func(g *GorgeSide) {
		for _, ev := range g.E.L.Events {
			if ev.Kind == events.PutOnStack {
				if n := ownCardName(g.E, ev.Obj); n != "" {
					cast[n] = true
				}
			}
		}
	}}
	res := Lockstep(m, seed, k, castFirstPolicy(seed), WithCoverage(rc, m, &cov))
	if !res.Clean() || cov == nil {
		t.Fatalf("seed %d: clean %v, coverage %v", seed, res.Clean(), cov)
	}
	if len(cast) == 0 {
		t.Fatal("the game cast no card: the test would assert nothing")
	}
	var credited string
	for n := range cast {
		for _, v := range cov.Cards[n].Branch {
			if v > 0 {
				credited = n
			}
		}
	}
	if credited == "" {
		t.Fatalf("no cast card %v carries a branch count: %v", cast, cov.Cards)
	}
	cov.Games = 1
	var br, tsv bytes.Buffer
	if err := cov.WriteBranchTSV(&br); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(br.String(), credited+"\t") || !strings.HasPrefix(br.String(), "card\tkey\tuses\towed\n") {
		t.Fatalf("branch TSV lacks %q or its header:\n%s", credited, br.String())
	}
	if err := cov.WriteTSV(&tsv); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(tsv.String(), "\n"), "\n") {
		if n := strings.Count(line, "\t"); n != 5 {
			t.Fatalf("coverage TSV row has %d tabs, want 5 (six columns): %q", n, line)
		}
	}
}
