package rules

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestDumpSixStream runs the 6-seat acceptance game and dumps the event log
// to /tmp/stream6.txt for base-vs-worktree divergence analysis.
func TestDumpSixStream(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	all := testutil.LegacyDeckNames()
	seats := 6
	names := make([]string, seats)
	decks := make([][]*cards.Card, seats)
	for i := 0; i < seats; i++ {
		names[i] = all[i%len(all)]
		decks[i] = testutil.RepoDeck(t, reg, all[i%len(all)])
	}
	cfg := Config{Seed: 42, Names: names, Decks: decks, Tokens: reg.Tokens, Mulligans: 1}
	e := New(cfg)
	b := newTestBot(7)
	e.Advance()
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 400000 {
		if err := e.Submit(b.answer(e, e.Pending())); err != nil {
			t.Fatalf("intent %d: %v", n, err)
		}
		n++
	}
	f, _ := os.Create("/tmp/stream6.txt")
	defer f.Close()
	for _, ev := range e.L.Events {
		fmt.Fprintf(f, "%v\n", ev)
	}
	fmt.Println("events:", len(e.L.Events), "head:", e.L.Head())
}
