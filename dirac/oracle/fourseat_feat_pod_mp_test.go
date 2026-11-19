package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// podColumn is the dense index of pod field f for relative seat r.
func podColumn(r, f int) int { return policynet.DenseWidth + r*policynet.PodSeatWidth + f }

// podFixtureLists turns 40-card name lists into one-of entries.
func podFixtureLists(t *testing.T, decks [][]string) [][]cardset.Entry {
	t.Helper()
	var lists [][]cardset.Entry
	for i, d := range decks {
		if len(d) != 40 {
			t.Fatalf("deck %d: %d cards", i, len(d))
		}
		es := make([]cardset.Entry, len(d))
		for j, n := range d {
			es[j] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	return lists
}

// playPodFeat plays seeds of a four-seat fixture pod with FeatCheck on under
// FeaturesPod and returns the summed stats. A dense mismatch (the pod
// blocks are dense) or a panic fails. Any other mismatch ends that game's
// comparison and is logged: both fixtures carry a FeaturesEntity sparse gap
// of their own (the monarch pod's seed 3 at a combat-damage target ask, the
// initiative pod's burn seat's priority bag), which the pod set inherits and
// does not cause; a rules divergence is the fixture's lockstep test's.
func playPodFeat(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, decks [4][]string, seeds int, pol func(uint64) Policy) FeatStats {
	t.Helper()
	fp, err := FeatPool(reg, tbl, nil)
	if err != nil {
		t.Fatal(err)
	}
	var total FeatStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 60
		rc.Check = FeatCheckFS(fp, &st, policynet.FeaturesPod)
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, pol(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			if d.Field == "panic" || d.What == "features" && strings.Contains(d.Field, "dense") {
				t.Errorf("seed %d: after %d compared decisions: %s %s: %s", seed, st.Decisions, d.What, d.Field, d.Detail)
			} else {
				t.Logf("seed %d: comparison ends after %d decisions: %s %s", seed, st.Decisions, d.What, d.Field)
			}
		}
		total.Decisions += st.Decisions
		for i, n := range st.DenseHit {
			total.DenseHit[i] += n
		}
	}
	return total
}

// TestFeatFourSeatPodDesignations is the FeaturesPod FeatCheck over the
// four-seat monarch and initiative fixture pods and the poison carriers:
// gorge and the kernel encode the same pod blocks at every decision, and
// across the games the monarch, initiative and poison columns are reached.
func TestFeatFourSeatPodDesignations(t *testing.T) {
	reg := synthReg(t)
	hit := func(st FeatStats, f int) int {
		n := 0
		for r := 0; r < 4; r++ {
			n += st.DenseHit[podColumn(r, f)]
		}
		return n
	}

	set, err := cardset.RawByName("monarch")
	if err != nil {
		t.Fatal(err)
	}
	var wb, rw []string
	for _, e := range []cardset.Entry{
		{Name: "Knights of the Black Rose", Count: 2}, {Name: "Custodi Lich", Count: 2}, {Name: "Palace Sentinels", Count: 3},
		{Name: "Thorn of the Black Rose", Count: 2}, {Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Walking Corpse", Count: 3}, {Name: "Throne of the High City", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
	} {
		wb = append(wb, deckOf(e.Count, e.Name)...)
	}
	for _, e := range []cardset.Entry{
		{Name: "Palace Sentinels", Count: 3}, {Name: "Court of Grace", Count: 2}, {Name: "Crimson Fleet Commodore", Count: 2},
		{Name: "Goblin Piker", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Throne of the High City", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 7},
	} {
		rw = append(rw, deckOf(e.Count, e.Name)...)
	}
	mdecks := [4][]string{wb, rw, wb, rw}
	tbl, _, err := set.MatchupN(podFixtureLists(t, mdecks[:]))
	if err != nil {
		t.Fatal(err)
	}
	mon := playPodFeat(t, reg, set, tbl, mdecks, 4, func(s uint64) Policy { return castFirstPolicy(s) })

	iset, err := cardset.RawByName("init")
	if err != nil {
		t.Fatal(err)
	}
	holder := initDeck([]cardset.Entry{
		{Name: "Avenging Hunter", Count: 4}, {Name: "Stirring Bard", Count: 4},
		{Name: "Dungeoneer's Pack", Count: 4},
		{Name: "Forest", Count: 16}, {Name: "Mountain", Count: 12},
	})
	burn := initDeck([]cardset.Entry{{Name: "Shock", Count: 12}, {Name: "Mountain", Count: 28}})
	idecks := [4][]string{holder, burn, burn, burn}
	itbl, _, err := iset.MatchupN(podFixtureLists(t, idecks[:]))
	if err != nil {
		t.Fatal(err)
	}
	withoutArenaGoad(itbl)
	if !gorgeDigWithCounters() {
		throneAsGorge(itbl)
	}
	ini := playPodFeat(t, reg, iset, itbl, idecks, 4, func(s uint64) Policy { return killHolder(avoidArena(castCarrierFirst(s))) })

	// Poison: the poisonhigh carriers, two seats (the pod blocks encode any
	// seat count).
	pset, err := cardset.RawByName("poisonhigh")
	if err != nil {
		t.Fatal(err)
	}
	md := poisonhighDecks[0]
	var pdecks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				pdecks[s] = append(pdecks[s], c)
			}
		}
	}
	ptbl, _, _, err := pset.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	pfp, err := FeatPool(reg, ptbl, nil)
	if err != nil {
		t.Fatal(err)
	}
	var poi FeatStats
	casts, acts := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= 6; seed++ {
		m, err := NewMatchup(reg, [2]string{"poisonhigh-a", "poisonhigh-b"}, pdecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(pset, ptbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st FeatStats
		res := Lockstep(m, seed, k, poisonhighPolicy(seed, casts, acts), RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: FeatCheckFS(pfp, &st, policynet.FeaturesPod)})
		CloseSide(k)
		if d := res.Div; d != nil && (d.What == "features" || d.Field == "panic") {
			t.Errorf("poison seed %d: after %d compared decisions: %s %s: %s", seed, st.Decisions, d.What, d.Field, d.Detail)
		}
		poi.Decisions += st.Decisions
		for i, n := range st.DenseHit {
			poi.DenseHit[i] += n
		}
	}
	t.Logf("monarch pods: %d decisions, monarch column hit %d; initiative pods: %d decisions, initiative column hit %d; poison games: %d decisions, poison column hit %d",
		mon.Decisions, hit(mon, policynet.PodMonarch), ini.Decisions, hit(ini, policynet.PodInitiative), poi.Decisions, hit(poi, policynet.PodPoison))
	if hit(poi, policynet.PodPoison) == 0 {
		t.Error("no compared decision had a poisoned seat")
	}
	if hit(mon, policynet.PodMonarch) == 0 {
		t.Error("no compared decision had a monarch")
	}
	if hit(ini, policynet.PodInitiative) == 0 {
		t.Error("no compared decision had an initiative holder")
	}
}
