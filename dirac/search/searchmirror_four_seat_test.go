package search_test

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// searchMirrorPods is how many four-seat cmd5 pods the mirror plays: PodAt
// indices 0 to searchMirrorPods-1 under seed 1.
const searchMirrorPods = 24

// searchMirrorKnown is every class of difference
// (oracle.SearchMirrorMismatch.Class) the two searches still show over those
// pods, with its cause. A difference of a class not listed fails at its first
// decision. It is keyed by class alone, never by pod, seed or decision. A
// hidden-card pool, a zone size or the searching player's hand parting between
// the root worlds (root-pool), a candidate without a canonical action and any
// other candidate difference are not listed and fail.
//
// A structural row is a difference every searched decision shows whatever the
// game does; no pod showing it fails, so closing it deletes its row. Every
// other row rests on the witnesses the pods' trajectories happen to reach
// (pods are driven by the kernel search's choices, so a card lowering or a
// search change moves them), and a pass in which none shows is only logged:
// deleting such a row is the change that closes its cause, proved by that
// class failing no pod over a wider range.
var searchMirrorKnown = map[string]searchMirrorClass{
	// The root worlds, and the picks over equal candidate lists.
	"root-world": {structural: true, cause: "the two redealers draw different streams over the same hidden cards; the kernel deals no honest root"},
	"choice":     {cause: "equal candidate lists, a different pick; the searches walk different worlds (root-world)"},
	"visits":     {cause: "equal candidate lists, different visits; the same, and candidate order is not held equal"},
	// The candidate enumerators.
	"candidates/limit-cut":            {cause: "the engines order candidates differently, so the candidate limit keeps different ones"},
	"candidates/multi-select":         {cause: "gorge searches sets of options where the kernel poses the pick one option at a time (dirac/search/doc.go)"},
	"candidates/priority/kernel-more": {cause: "the kernel offers a cast gorge does not"},
	"candidates/priority/kernel-only": {cause: "the kernel searches a priority decision where gorge has nothing to search"},
	// The decision counters.
	"ordinal": {cause: "gorge answers decisions beneath the canonical surface that SeqNo never counts (OrdinalCheck holds the gap to those)"},
}

// searchMirrorClass is one searchMirrorKnown row.
type searchMirrorClass struct {
	cause string
	// structural: every searched decision shows it, so a run without it is a
	// stale row and fails.
	structural bool
}

// TestSearchMirrorsAzmctsFourSeat plays four-seat cmd5 pods in lockstep and,
// at every searched decision of each pod's first 10 turns, asks both searches
// the same position: bots/azmcts on the gorge side (an honest-root redeal from
// that seat's feed) and dirac/search on the kernel side, sims 8, heuristic
// leaf, seat seeds 11, 22, 33 and 44. It compares the candidate lists, the
// root worlds' hidden zones, the chosen candidate and the visits per
// candidate, and fails at the first difference searchMirrorKnown does not
// name, and when a structural row shows nowhere. The searches do not mirror
// yet: the table is what is left.
func TestSearchMirrorsAzmctsFourSeat(t *testing.T) {
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no gorge corpus: %v", err)
	}
	dir, remove, err := oracle.ExtractSlice("cmd5")
	if err != nil {
		t.Skipf("no cmd5 slice: %v", err)
	}
	defer remove()
	c, err := oracle.OpenCorpus(cardsDir, filepath.Join(dir, "decks"))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := oracle.ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(pool))
	for i, p := range pool {
		names[i] = p.Name
	}
	minis, err := oracle.MiniCommanderDecks(c, names, 40)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(minis)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	decks := make([]oracle.PoolDeck, len(names))
	for i, n := range names {
		decks[i] = oracle.PoolDeck{Name: n}
	}

	seen := map[string]int{}
	compared, worlds := 0, 0
	for pod := range searchMirrorPods {
		p := oracle.PodAt(decks, pod, 1, 4)
		m, tbl, err := oracle.SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d: %v", pod, err)
		}
		res, err := oracle.SearchMirrorPod(m, tbl, set, p.Seed, [4]uint64{11, 22, 33, 44}, 10)
		if err != nil {
			t.Fatalf("pod %d: %v", pod, err)
		}
		for i := range res.Mismatches {
			d := &res.Mismatches[i]
			if _, known := searchMirrorKnown[d.Class()]; !known {
				t.Fatalf("pod %d: the searches differ: %s", pod, d)
			}
			seen[d.Class()]++
		}
		if res.Res.Div != nil {
			t.Fatalf("pod %d lockstep diverged: %v", pod, res.Res.Div)
		}
		if res.Refused != 0 {
			t.Fatalf("pod %d's honest root refused at %d decisions", pod, res.Refused)
		}
		compared += res.Compared
		worlds += res.Worlds
	}

	candidates := 0
	for _, class := range slices.Sorted(maps.Keys(seen)) {
		t.Logf("%s: %d", class, seen[class])
		if strings.HasPrefix(class, "candidates/") {
			candidates += seen[class]
		}
	}
	t.Logf("%d decisions searched on both engines over %d pods, %d root worlds compared", compared, searchMirrorPods, worlds)
	if compared <= candidates {
		t.Fatalf("%d decisions searched on both engines, %d candidate differences: no equal candidate list was compared", compared, candidates)
	}
	if worlds != compared {
		t.Fatalf("%d root worlds compared at %d searched decisions", worlds, compared)
	}
	for _, class := range slices.Sorted(maps.Keys(searchMirrorKnown)) {
		k := searchMirrorKnown[class]
		switch {
		case seen[class] != 0:
		case k.structural:
			t.Errorf("no pod shows %q (%s): delete its searchMirrorKnown row", class, k.cause)
		default:
			t.Logf("no pod shows %q (%s) this run; its row goes when its cause is closed", class, k.cause)
		}
	}
}
