package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The corpus-wide parameter census: the scan and labels of
// TestEveryStandardSetParamsAreRead, walked over every card of the corpus in
// corpusCensusChunks chunks (chunk k holds the cards whose index in
// AllCards() sorted by front-face name is k mod corpusCensusChunks). The
// record is testdata/paramcensus-corpus/chunk-NN.tsv ("label\tcards", sorted
// by label): per label, the number of the chunk's cards carrying it. No card
// names are recorded. PARAMCENSUS_CORPUS_REGEN=1 rewrites the chunk files.
//
// A corpus pin change (FORGE_REF) moves chunk membership: regenerate every
// chunk file in the same change.

const corpusCensusChunks = 16

const corpusCensusDir = "testdata/paramcensus-corpus"

// corpusCensusSkipTypes are front-face types of non-deck cards the census
// leaves out.
var corpusCensusSkipTypes = []string{
	"Scheme", "Plane", "Phenomenon", "Vanguard",
	"Conspiracy", "Attraction", "Contraption", "Sticker",
}

func corpusCensusSkipped(c *cards.Card) bool {
	if len(c.Faces) == 0 {
		return true
	}
	f := c.Faces[0]
	if strings.HasPrefix(f.Name, "A-") {
		return true
	}
	for _, ty := range f.Types {
		for _, s := range corpusCensusSkipTypes {
			if ty == s {
				return true
			}
		}
	}
	return false
}

func corpusCensusChunkPath(k int) string {
	return filepath.Join(corpusCensusDir, fmt.Sprintf("chunk-%02d.tsv", k))
}

func readCorpusCensusRecord(t *testing.T, path string) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) == 0 || lines[0] != "label\tcards" {
		t.Fatalf("%s: missing header %q", path, "label\tcards")
	}
	for _, line := range lines[1:] {
		label, n, ok := strings.Cut(line, "\t")
		v, err := strconv.Atoi(n)
		if !ok || label == "" || err != nil || v <= 0 {
			t.Errorf("%s: line %q is not <label>\\t<cards>", path, line)
			continue
		}
		out[label] = v
	}
	return out
}

func writeCorpusCensusRecord(t *testing.T, path string, got map[string]int) {
	t.Helper()
	labels := make([]string, 0, len(got))
	for l := range got {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	var b strings.Builder
	b.WriteString("label\tcards\n")
	for _, l := range labels {
		fmt.Fprintf(&b, "%s\t%d\n", l, got[l])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func corpusParamCensusChunk(t *testing.T, k int) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	reg := testutil.CorpusRegistry(t)
	start := time.Now()
	all := append([]*cards.Card(nil), reg.AllCards()...)
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Faces[0].Name < all[j].Faces[0].Name
	})
	got := map[string]int{}
	cardsIn, skipped, withUnread := 0, 0, 0
	for i, c := range all {
		if i%corpusCensusChunks != k {
			continue
		}
		if corpusCensusSkipped(c) {
			skipped++
			continue
		}
		cardsIn++
		labels := cardCensusLabels(c, d, nil)
		if len(labels) > 0 {
			withUnread++
		}
		for _, l := range labels {
			got[l]++
		}
	}
	t.Logf("chunk %02d: cards=%d skipped=%d cards with unread params=%d distinct labels=%d wall=%v",
		k, cardsIn, skipped, withUnread, len(got), time.Since(start))
	path := corpusCensusChunkPath(k)
	if os.Getenv("PARAMCENSUS_CORPUS_REGEN") != "" {
		writeCorpusCensusRecord(t, path, got)
		return
	}
	want := readCorpusCensusRecord(t, path)
	labels := make([]string, 0, len(got)+len(want))
	for l := range got {
		labels = append(labels, l)
	}
	for l := range want {
		if _, ok := got[l]; !ok {
			labels = append(labels, l)
		}
	}
	sort.Strings(labels)
	for _, l := range labels {
		n, m := got[l], want[l]
		switch {
		case n > m:
			t.Errorf("label %s: %d cards, record %d: new unread parameter", l, n, m)
		case n < m:
			t.Logf("label %s: %d cards, record %d: the record can be lowered", l, n, m)
		}
	}
}

func TestCorpusParamsAreRead00(t *testing.T) { corpusParamCensusChunk(t, 0) }
func TestCorpusParamsAreRead01(t *testing.T) { corpusParamCensusChunk(t, 1) }
func TestCorpusParamsAreRead02(t *testing.T) { corpusParamCensusChunk(t, 2) }
func TestCorpusParamsAreRead03(t *testing.T) { corpusParamCensusChunk(t, 3) }
func TestCorpusParamsAreRead04(t *testing.T) { corpusParamCensusChunk(t, 4) }
func TestCorpusParamsAreRead05(t *testing.T) { corpusParamCensusChunk(t, 5) }
func TestCorpusParamsAreRead06(t *testing.T) { corpusParamCensusChunk(t, 6) }
func TestCorpusParamsAreRead07(t *testing.T) { corpusParamCensusChunk(t, 7) }
func TestCorpusParamsAreRead08(t *testing.T) { corpusParamCensusChunk(t, 8) }
func TestCorpusParamsAreRead09(t *testing.T) { corpusParamCensusChunk(t, 9) }
func TestCorpusParamsAreRead10(t *testing.T) { corpusParamCensusChunk(t, 10) }
func TestCorpusParamsAreRead11(t *testing.T) { corpusParamCensusChunk(t, 11) }
func TestCorpusParamsAreRead12(t *testing.T) { corpusParamCensusChunk(t, 12) }
func TestCorpusParamsAreRead13(t *testing.T) { corpusParamCensusChunk(t, 13) }
func TestCorpusParamsAreRead14(t *testing.T) { corpusParamCensusChunk(t, 14) }
func TestCorpusParamsAreRead15(t *testing.T) { corpusParamCensusChunk(t, 15) }

// TestCorpusParamCensusTotals sums the committed chunk records (no corpus
// needed): distinct labels, label-carrying card count, the top twenty.
func TestCorpusParamCensusTotals(t *testing.T) {
	sum := map[string]int{}
	for k := 0; k < corpusCensusChunks; k++ {
		for l, n := range readCorpusCensusRecord(t, corpusCensusChunkPath(k)) {
			sum[l] += n
		}
	}
	type lc struct {
		l string
		n int
	}
	top := make([]lc, 0, len(sum))
	total := 0
	for l, n := range sum {
		top = append(top, lc{l, n})
		total += n
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].n != top[j].n {
			return top[i].n > top[j].n
		}
		return top[i].l < top[j].l
	})
	t.Logf("TOTAL: chunks=%d labels=%d cards (label-card pairs)=%d", corpusCensusChunks, len(sum), total)
	for i, e := range top {
		if i == 20 {
			break
		}
		t.Logf("top label %2d: %-60s cards=%d", i+1, e.l, e.n)
	}
}
