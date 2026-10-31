package netsimd_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The recorded eval inputs: every decision of a few default-bot games over
// FDN decks, encoded exactly as the search encodes them for the network
// (FeaturesEntity state, the decision's options, the bot's pick marked).
// Rebuilt deterministically per process; nothing is committed.
//
// NETSIMD_DECKS names the deck directory (an FDN pool holding corpusPairs'
// decks); NETSIMD_DZGW the network. Neither has a default: tests skip when
// either is unset or missing.

type record struct {
	st   policynet.State
	opts []policynet.Option
	view view.View
	seat int
	kind decision.Kind
	dopt []decision.Option
}

var corpusPairs = [][2]string{
	{"FDN_top_00001_WB", "FDN_top_00002_BR"},
	{"FDN_top_00003_BR", "FDN_top_00004_RG"},
	{"FDN_top_00005_BGpU", "FDN_top_00007_WR"},
	{"FDN_top_00008_WGpB", "FDN_top_00009_WG"},
	{"FDN_top_00010_WG", "FDN_top_00006_BGpU"},
	{"FDN_top_00011_WB", "FDN_top_00016_BG"},
}

var (
	corpusOnce sync.Once
	corpusRecs []record
	corpusErr  error
	corpusSkip string
)

func netPath(t testing.TB) string {
	t.Helper()
	p := os.Getenv("NETSIMD_DZGW")
	if p == "" {
		t.Skip("no network: NETSIMD_DZGW is unset")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("no network at %s (NETSIMD_DZGW)", p)
	}
	return p
}

func corpus(t testing.TB) []record {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	corpusOnce.Do(func() { corpusRecs, corpusSkip, corpusErr = buildCorpus(reg) })
	if corpusSkip != "" {
		t.Skip(corpusSkip)
	}
	if corpusErr != nil {
		t.Fatal(corpusErr)
	}
	return corpusRecs
}

func loadDeck(reg *cards.Registry, dir, name string) ([]*cards.Card, error) {
	raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return nil, err
	}
	f, err := deck.Parse(raw)
	if err != nil {
		return nil, err
	}
	return f.Resolve(reg)
}

func buildCorpus(reg *cards.Registry) ([]record, string, error) {
	dir := os.Getenv("NETSIMD_DECKS")
	if dir == "" {
		return nil, "no deck directory: NETSIMD_DECKS is unset", nil
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Sprintf("no deck directory at %s (NETSIMD_DECKS)", dir), nil
	}
	var out []record
	fs := policynet.FeaturesEntity
	for gi, pr := range corpusPairs {
		da, err := loadDeck(reg, dir, pr[0])
		if err != nil {
			return nil, "", err
		}
		db, err := loadDeck(reg, dir, pr[1])
		if err != nil {
			return nil, "", err
		}
		cfg := rules.Config{Seed: 1000 + uint64(gi), Names: []string{pr[0], pr[1]}, Decks: [][]*cards.Card{da, db}, Tokens: reg.Tokens}
		e := rules.New(cfg)
		e.Advance()
		rngs := searchprobe.BotRandoms(cfg.Seed, 2)
		board := botpolicy.NewBoard(2)
		for step := 0; step < 600 && !e.G.Over; step++ {
			d := e.Pending()
			if d == nil {
				break
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			v := view.Project(e.G, e, d.Player, d)
			st := policynet.EncodeStateWith(fs, v, d.Player, nil)
			enc := policynet.EncodeOptionsWith(fs, &v, d.Player, d.Kind, d.Options, nil)
			heuristic.MarkBotPicks(d, enc, in)
			dopt := append([]decision.Option(nil), d.Options...)
			out = append(out, record{st: st, opts: enc, view: v, seat: int(d.Player), kind: d.Kind, dopt: dopt})
			if err := e.Submit(in); err != nil {
				return nil, "", fmt.Errorf("pair %d step %d: %w", gi, step, err)
			}
		}
	}
	return out, "", nil
}

type corpusStats struct {
	n, opts, cards, sparse, rows, slots, hashed int
	optsMax, cardsMax                           int
	kinds                                       map[decision.Kind]int
	optHist                                     map[int]int
}

func stats(recs []record) corpusStats {
	s := corpusStats{kinds: map[decision.Kind]int{}, optHist: map[int]int{}}
	for _, r := range recs {
		s.n++
		s.opts += len(r.opts)
		s.optsMax = max(s.optsMax, len(r.opts))
		s.cards += len(r.st.Cards)
		s.cardsMax = max(s.cardsMax, len(r.st.Cards))
		s.sparse += len(r.st.Sparse)
		for _, c := range r.st.Cards {
			s.rows += len(c.Rows)
		}
		for _, o := range r.opts {
			s.slots += len(o.Slots)
			s.hashed += len(o.Hashed)
		}
		s.kinds[r.kind]++
		b := len(r.opts)
		switch {
		case b >= 32:
			b = 32
		case b >= 16:
			b = 16
		case b >= 8:
			b = 8
		case b >= 4:
			b = 4
		}
		s.optHist[b]++
	}
	return s
}

func TestCorpusStats(t *testing.T) {
	recs := corpus(t)
	s := stats(recs)
	t.Logf("decisions=%d opts/eval=%.2f (max %d) cards/state=%.1f (max %d) sparse/state=%.1f rows/card=%.2f slots/opt=%.2f hashed/opt=%.2f",
		s.n, float64(s.opts)/float64(s.n), s.optsMax, float64(s.cards)/float64(s.n), s.cardsMax,
		float64(s.sparse)/float64(s.n), float64(s.rows)/float64(max(s.cards, 1)), float64(s.slots)/float64(max(s.opts, 1)), float64(s.hashed)/float64(max(s.opts, 1)))
	kinds := make([]string, 0, len(s.kinds))
	for k := range s.kinds {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		t.Logf("  kind %-18s %5d", k, s.kinds[decision.Kind(k)])
	}
	for _, b := range []int{0, 1, 2, 3, 4, 8, 16, 32} {
		if s.optHist[b] > 0 {
			t.Logf("  options %2d+: %5d", b, s.optHist[b])
		}
	}
	if s.n < 300 {
		t.Errorf("corpus has %d decisions, want a few hundred", s.n)
	}
}
