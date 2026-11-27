package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// cedhPodRun plays seeds games of the given decks (two seats, or a pod of
// three or four) in lockstep with gorge, on a table kgen builds from the
// decks' cards, with castFirstPolicy. Each named card must lower. tally, when
// set, is called on gorge's game at every decision and its last value per
// game is summed. A divergence or a kernel stop fails the test.
func cedhPodRun(t *testing.T, name string, decks [][]cardset.Entry, lowered []string, seeds uint64, tally func(g *state.Game) int) int {
	t.Helper()
	return cedhPodRunWith(t, name, decks, lowered, seeds, tally, castFirstPolicy)
}

// cedhPodRunWith is cedhPodRun under the policy mk builds for each seed.
func cedhPodRunWith(t *testing.T, name string, decks [][]cardset.Entry, lowered []string, seeds uint64, tally func(g *state.Game) int, mk func(seed uint64) Policy) int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var pool []*cards.Card
	for _, d := range decks {
		for _, e := range d {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			pool = append(pool, c)
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, name+"-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range lowered {
		found := false
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == n {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s is not in the table (refused?)", n)
		}
	}
	if len(decks) > 2 && dirac.TwoSeatOnly(tbl) {
		t.Fatalf("the table is two-seat only: %v", dirac.TwoSeatCauses(tbl))
	}
	labels := make([]string, len(decks))
	for i := range labels {
		labels[i] = name + "-" + string(rune('a'+i))
	}
	total := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := KernelMatchupN(reg, tbl, labels, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		last := 0
		base := mk(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if tally != nil && c.G != nil {
				last = tally(c.G.E.G)
			}
			return base.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("%d seats, seed %d: diverged at decision %d: %s %s %v\nref %q\nker %q\n%s\nlast %s; recent %v", len(decks), seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions, d.Detail,
				d.LastSourced, res.RecentSources(m, 4))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("%d seats, seed %d: kernel stopped: %s", len(decks), seed, res.Stall)
		}
		total += last
	}
	return total
}

// cedhZoneCount counts the objects named n in zone z across every player.
func cedhZoneCount(g *state.Game, z state.Zone, n string) int {
	k := 0
	for p := range g.Players {
		for _, id := range g.Zone(z, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && cardName(o.Card) == n {
				k++
			}
		}
	}
	return k
}
