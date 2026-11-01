package host

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// BenchmarkHostBotMatch plays whole two-seat bot matches through a memory-mode
// registry, one seed per iteration. scripts/hostbench-pair.sh compares two
// revisions on it.
func BenchmarkHostBotMatch(b *testing.B) {
	reg := testutil.CorpusRegistry(b)
	names := []string{"FDN_top_04956_UG", "FDN_top_20626_WG"}
	decks := map[string][]*cards.Card{}
	for _, n := range names {
		decks[n] = dckDeck(b, reg, filepath.Join("..", "..", "cmd", "enginebench", "testdata", "decks", n+".dck"))
	}
	o := Options{
		LoadDeck: func(name string) (Deck, error) {
			cs, ok := decks[name]
			if !ok {
				return Deck{}, ErrNotFound
			}
			return Deck{Name: name, Cards: cs}, nil
		},
		Tokens:                reg.Tokens,
		Sleep:                 func(time.Duration, <-chan struct{}) {},
		BotUnboundedDecisions: true,
	}
	b.ResetTimer()
	for i := range b.N {
		r, err := New(o)
		if err != nil {
			b.Fatal(err)
		}
		cfg := TableConfig{ID: "bench", Name: "bench", Seats: 2, Decks: names, Seed: uint64(i + 1), Spectator: view.Public}
		if err := r.AddTable(cfg); err != nil {
			b.Fatal(err)
		}
		if err := r.Start("bench"); err != nil {
			b.Fatal(err)
		}
		r.Wait("bench")
		ms, err := r.Matches("bench")
		if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
			b.Fatalf("seed %d: %+v (err %v)", cfg.Seed, ms, err)
		}
		if ms[0].Turns < 2 {
			b.Fatalf("seed %d: a match of %d turns measures nothing", cfg.Seed, ms[0].Turns)
		}
		if err := r.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

// dckDeck resolves a Forge .dck main deck: "N [SET:num] Name" lines, with the
// NAME: header and section markers skipped and [sideboard] ending the deck.
func dckDeck(b *testing.B, reg *cards.Registry, path string) []*cards.Card {
	b.Helper()
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	var out []*cards.Card
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.EqualFold(line, "[sideboard]") {
			break
		}
		count, name, ok := strings.Cut(line, " ")
		n, err := strconv.Atoi(count)
		if !ok || err != nil {
			continue
		}
		if _, after, found := strings.Cut(name, "] "); found {
			name = after
		}
		name, _, _ = strings.Cut(name, "|")
		c, ok := reg.Lookup(strings.TrimSpace(name))
		if !ok {
			b.Fatalf("%s: card %q is not in the corpus", path, name)
		}
		for range n {
			out = append(out, c)
		}
	}
	if err := sc.Err(); err != nil {
		b.Fatal(err)
	}
	if len(out) == 0 {
		b.Fatalf("%s: empty deck", path)
	}
	return out
}
