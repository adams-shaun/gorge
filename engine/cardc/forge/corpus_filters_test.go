package forge

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const censusShards = 8

// filterUses walks one card's abilities (with their Sub chains), triggers
// (with their Effect), statics, replacements (with their With) and SVar
// bodies, counting the values of FilterKeys into uses.
func filterUses(card *cards.Card, uses map[string]int) {
	visitParams := func(params map[string]string) {
		for _, k := range FilterKeys {
			if v, ok := params[k]; ok {
				uses[strings.TrimSpace(v)]++
			}
		}
	}
	visitSA := func(sa *cards.SA) {
		for seen := 0; sa != nil && seen < 64; sa, seen = sa.Sub, seen+1 {
			visitParams(sa.Params)
		}
	}
	for _, f := range card.Faces {
		for _, sa := range f.Abilities {
			visitSA(sa)
		}
		for _, tr := range f.Triggers {
			visitParams(tr.Params)
			visitSA(tr.Effect)
		}
		for _, st := range f.Statics {
			visitParams(st.Params)
		}
		for _, r := range f.Repls {
			visitParams(r.Params)
			visitSA(r.With)
		}
		names := make([]string, 0, len(f.SVars))
		for name := range f.SVars {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			visitSA(cards.ResolveSVar(f.SVars, name))
		}
	}
}

// corpusFilterUses counts every corpus filter text, in censusShards parallel
// shards over reg.AllCards() by index modulo censusShards.
func corpusFilterUses(t *testing.T, all []*cards.Card) map[string]int {
	shards := make([]map[string]int, censusShards)
	var mu sync.Mutex
	t.Run("walk", func(t *testing.T) {
		for i := range censusShards {
			t.Run(fmt.Sprintf("shard-%d", i), func(t *testing.T) {
				t.Parallel()
				uses := map[string]int{}
				for j := i; j < len(all); j += censusShards {
					filterUses(all[j], uses)
				}
				mu.Lock()
				shards[i] = uses
				mu.Unlock()
			})
		}
	})
	merged := map[string]int{}
	for _, s := range shards {
		for text, n := range s {
			merged[text] += n
		}
	}
	return merged
}

// readCensus reads testdata/filter-refused.tsv into "kind\tatom" rows.
func readCensus(t *testing.T) map[string]bool {
	f, err := os.Open("testdata/filter-refused.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		kind, _, ok := strings.Cut(line, "\t")
		if _, known := ParseRefusalKind(kind); !ok || !known {
			t.Fatalf("filter-refused.tsv: bad row %q", line)
		}
		rows[line] = true
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// TestCorpusFiltersLower lowers every distinct corpus filter text and holds
// the refused (kind, atom) pairs to testdata/filter-refused.tsv, both ways.
func TestCorpusFiltersLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	uses := corpusFilterUses(t, reg.AllCards())
	texts := make([]string, 0, len(uses))
	for text := range uses {
		texts = append(texts, text)
	}
	slices.Sort(texts)

	total, lowered := 0, 0
	perKind := map[RefusalKind]int{}
	measured := map[string]bool{}
	for _, text := range texts {
		n := uses[text]
		total += n
		f, r := LowerFilter(text)
		if r != nil {
			perKind[r.Kind] += n
			measured[r.Kind.String()+"\t"+r.Atom] = true
			continue
		}
		if err := ir.ValidateFilter(f); err != nil {
			t.Errorf("LowerFilter(%q) does not validate: %v", text, err)
			continue
		}
		lowered += n
	}
	if total < 40000 {
		t.Fatalf("census read %d filter uses, want at least 40000: the walk is not reading the corpus", total)
	}
	ratio := float64(lowered) / float64(total)
	t.Logf("lowered %d of %d uses (ratio %.4f) over %d distinct texts", lowered, total, ratio, len(texts))
	for k := RefuseNone + 1; k < refusalKindCount; k++ {
		if perKind[k] > 0 {
			t.Logf("refused %-18s %6d uses", k, perKind[k])
		}
	}
	if ratio < 0.95 {
		t.Errorf("lowered ratio %.4f, want at least 0.95", ratio)
	}

	rows := make([]string, 0, len(measured))
	for row := range measured {
		rows = append(rows, row)
	}
	slices.Sort(rows)
	if os.Getenv("FORGE_FILTER_CENSUS_DUMP") != "" {
		t.Logf("measured set:\n%s", strings.Join(rows, "\n"))
	}
	want := readCensus(t)
	for _, row := range rows {
		if !want[row] {
			t.Errorf("new refusal %q: read it or add the row to testdata/filter-refused.tsv", row)
		}
	}
	stale := make([]string, 0)
	for row := range want {
		if !measured[row] {
			stale = append(stale, row)
		}
	}
	slices.Sort(stale)
	for _, row := range stale {
		t.Errorf("stale: no corpus filter refuses %q any more; delete the row", row)
	}
}

// TestSubtypeVocabularyCoversCorpus holds every corpus type-line word to ir's
// type vocabulary.
func TestSubtypeVocabularyCoversCorpus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	missing := map[string]bool{}
	words := 0
	for _, card := range reg.AllCards() {
		for _, f := range card.Faces {
			for _, w := range f.Types {
				words++
				if ir.TypeWord(w) == ir.WordNone {
					missing[w] = true
				}
			}
		}
	}
	if words < 10000 {
		t.Fatalf("read %d type words, want at least 10000: the walk is not reading the corpus", words)
	}
	out := make([]string, 0, len(missing))
	for w := range missing {
		out = append(out, w)
	}
	slices.Sort(out)
	for _, w := range out {
		t.Errorf("type word %q is in no ir type list", w)
	}
}
