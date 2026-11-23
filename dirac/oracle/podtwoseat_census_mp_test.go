package oracle

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
)

// podCensusPools are the Commander pools the census reads: each needs its
// committed slice (ExtractSlice) and its table (make dirac-tables). A pool
// with neither is skipped, so the census covers a pool from the change that
// lands it.
var podCensusPools = []string{"cmd5", "edh"}

// podCensusPods is how many random four-seat pods (PodAt, seed 1) the census
// draws per pool, as kfuzz -seats 4 -seed 1 draws them.
const podCensusPods = 1000

// podCensusDir holds one golden file per pool: a row per (deck, card,
// class) that holds the full precon to two seats.
const podCensusDir = "testdata/twoseat-census"

// TestCommanderPodTwoSeatCensus is the four-seat Commander census: per
// Commander pool, every full precon (no -cmd-mini cut) whose cards or
// tokens hold a game of three or more seats to its start (StallTwoSeat),
// by class (dirac.TwoSeatCauses; a kgen-marked card by its lowering's
// reason, kgen CardSupport.TwoSeatWhy). One such card anywhere in a pod's
// table refuses the whole pod, so a class blocks every pod holding a deck
// it blocks.
//
// The golden rows only shrink: a new (deck, card, class) row fails, and so
// does a golden row that no longer holds (delete it, or regenerate with
// DIRAC_UPDATE_TWOSEAT_CENSUS=1). A pool with no golden file yet is logged
// only, so a pool lands first and its golden file after. The log is the
// ranking: per class, its cards, the precons and the random pods it blocks,
// and the pods it alone blocks (the pods fixing it would free).
func TestCommanderPodTwoSeatCensus(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, pool := range podCensusPools {
		set, err := cardset.ByName(pool)
		if err != nil {
			t.Logf("%s: no table (make dirac-tables, or the pool is not in this tree)", pool)
			continue
		}
		dir, remove, err := ExtractSlice(pool)
		if err != nil {
			t.Logf("%s: %v", pool, err)
			continue
		}
		ran++
		t.Run(pool, func(t *testing.T) {
			defer remove()
			podCensus(t, reg, cardsDir, cardset.AllowUnverified(set), pool, dir)
		})
	}
	if ran == 0 {
		t.Skip("no Commander pool has both a slice and a table")
	}
}

// censusRow is one golden row: a deck, a card of it (or a token its cards
// make, "token #<id>"), and a class.
type censusRow struct{ deck, card, class string }

func (r censusRow) String() string { return r.deck + "\t" + r.card + "\t" + r.class }

func podCensus(t *testing.T, reg *cards.Registry, cardsDir string, set cardset.Set, pool, dir string) {
	ps, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	var rows []censusRow
	kgenCards := map[string]bool{}
	type hit struct {
		deck, card string
		classes    []string
	}
	var hits []hit
	for _, p := range ps {
		f, err := c.deckFile(p.Name)
		if err != nil {
			t.Fatalf("deck %s: %v", p.Name, err)
		}
		var es []cardset.Entry
		for _, e := range f.Cards {
			es = append(es, cardset.Entry{Name: e.Name, Count: e.Count})
		}
		tbl, ids, err := set.MatchupNInert([][]cardset.Entry{es})
		// A deck holding a card refused by name (dirac/support/refused) never
		// reaches a table: the kernel plays no game of it, two seats or more.
		var re *cardset.RefusedError
		if errors.As(err, &re) && (re.Status == cardset.StatusRefused || re.Status == cardset.StatusNotLowered) {
			t.Logf("deck %s: skipped, %s is refused", p.Name, re.Card)
			continue
		}
		if err != nil {
			t.Fatalf("deck %s: %v", p.Name, err)
		}
		name := map[int]string{}
		k := 0
		for _, e := range f.Cards {
			for n := 0; n < e.Count; n++ {
				name[int(ids[0][k])] = e.Name
				k++
			}
		}
		causes := dirac.TwoSeatCauses(tbl)
		blocked := false
		for id, cs := range causes {
			if len(cs) == 0 {
				continue
			}
			blocked = true
			n, ok := name[id]
			if !ok {
				n = fmt.Sprintf("token #%d", id)
			}
			hits = append(hits, hit{p.Name, n, cs})
			if ok && slices.Contains(cs, dirac.TwoSeatKgen) {
				kgenCards[n] = true
			}
		}
		if only := dirac.TwoSeatOnly(tbl); only != blocked {
			t.Errorf("deck %s: the kernel's preparation says two-seat only %v, TwoSeatCauses %v", p.Name, only, blocked)
		}
	}
	var names []string
	for n := range kgenCards {
		names = append(names, n)
	}
	slices.Sort(names)
	sup, err := kgen.SupportFor(cardsDir, names)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		for _, cl := range h.classes {
			if cl != dirac.TwoSeatKgen {
				rows = append(rows, censusRow{h.deck, h.card, cl})
				continue
			}
			why := sup[h.card].TwoSeatWhy
			if len(why) == 0 {
				why = []string{"(reason unrecorded)"}
			}
			for _, w := range why {
				rows = append(rows, censusRow{h.deck, h.card, "kgen: " + w})
			}
		}
	}
	slices.SortFunc(rows, func(a, b censusRow) int {
		return cmp.Or(strings.Compare(a.deck, b.deck), strings.Compare(a.card, b.card), strings.Compare(a.class, b.class))
	})
	rows = slices.Compact(rows)
	logCensus(t, pool, ps, rows)
	checkCensusGolden(t, pool, rows)
}

// logCensus logs the ranking: per class, the cards, the precons it blocks,
// the random pods it blocks and those it alone blocks.
func logCensus(t *testing.T, pool string, ps []PoolDeck, rows []censusRow) {
	deckClasses := map[string]map[string]bool{}
	classCards := map[string]map[string]bool{}
	classDecks := map[string]map[string]bool{}
	for _, r := range rows {
		if deckClasses[r.deck] == nil {
			deckClasses[r.deck] = map[string]bool{}
		}
		deckClasses[r.deck][r.class] = true
		if classCards[r.class] == nil {
			classCards[r.class], classDecks[r.class] = map[string]bool{}, map[string]bool{}
		}
		classCards[r.class][r.card] = true
		classDecks[r.class][r.deck] = true
	}
	podsBy, aloneBy := map[string]int{}, map[string]int{}
	blocked := 0
	for i := 0; i < podCensusPods; i++ {
		in := map[string]bool{}
		for _, d := range PodAt(ps, i, 1, 4).Decks() {
			for cl := range deckClasses[d] {
				in[cl] = true
			}
		}
		if len(in) > 0 {
			blocked++
		}
		for cl := range in {
			podsBy[cl]++
			if len(in) == 1 {
				aloneBy[cl]++
			}
		}
	}
	var classes []string
	for cl := range classCards {
		classes = append(classes, cl)
	}
	slices.SortFunc(classes, func(a, b string) int {
		return cmp.Or(cmp.Compare(podsBy[b], podsBy[a]), cmp.Compare(len(classDecks[b]), len(classDecks[a])), strings.Compare(a, b))
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d of %d precons blocked, %d of %d random 4-pods blocked\n", pool, len(deckClasses), len(ps), blocked, podCensusPods)
	b.WriteString("| class | precons | pods | pods it alone blocks | cards |\n|---|---:|---:|---:|---|\n")
	for _, cl := range classes {
		var cs []string
		for c := range classCards[cl] {
			cs = append(cs, c)
		}
		slices.Sort(cs)
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %s |\n", cl, len(classDecks[cl]), podsBy[cl], aloneBy[cl], strings.Join(cs, "; "))
	}
	t.Log(b.String())
}

// checkCensusGolden holds rows to the pool's golden file, in both directions.
func checkCensusGolden(t *testing.T, pool string, rows []censusRow) {
	path := filepath.Join(podCensusDir, pool+".tsv")
	var got []string
	for _, r := range rows {
		got = append(got, r.String())
	}
	if os.Getenv("DIRAC_UPDATE_TWOSEAT_CENSUS") != "" {
		body := "# deck\tcard\tclass: TestCommanderPodTwoSeatCensus, rows only shrink\n" + strings.Join(got, "\n") + "\n"
		if err := os.MkdirAll(podCensusDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// a pool enters the ratchet when its golden file is committed
		// (DIRAC_UPDATE_TWOSEAT_CENSUS=1 writes it); until then the log is
		// the census
		t.Logf("%s: no golden file yet; %d rows", path, len(got))
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			want[l] = true
		}
	}
	for _, r := range got {
		if !want[r] {
			t.Errorf("%s: new two-seat-only row (the census only shrinks): %s", path, r)
		}
		delete(want, r)
	}
	var stale []string
	for r := range want {
		stale = append(stale, r)
	}
	slices.Sort(stale)
	for _, r := range stale {
		t.Errorf("%s: row no longer holds, delete it: %s", path, r)
	}
}
