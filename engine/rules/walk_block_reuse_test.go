package rules

import (
	"fmt"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The potential walk serves the priority walk's pool-independent blocks in
// real games, and (walkCacheVerify being on in this binary) every reusing
// walk is recomputed without reuse and must be identical, or the walk
// panics. The pricing readers run at every priority decision, as a builtin
// SpellBench seat runs them.
func TestWalkBlockReuseInGames(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	all := testutil.LegacyDeckNames()
	var mu sync.Mutex
	var served, members uint64
	var ran int
	for _, seed := range []uint64{3, 8} {
		seed := seed
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			t.Parallel()
			names := make([]string, 2)
			decks := make([][]*cards.Card, 2)
			for i := range names {
				names[i] = all[(int(seed)+i*5)%len(all)]
				decks[i] = testutil.RepoDeck(t, reg, names[i])
			}
			e := New(Config{Seed: seed, Names: names, Decks: decks, Tokens: reg.Tokens, NameUniverse: reg.Universe()})
			b := newTestBot(seed)
			e.Advance()
			for n := 0; !e.G.Over && e.Pending() != nil && n < 6000; n++ {
				if d := e.Pending(); d.Kind == decision.KPriority {
					e.PotentialPaymentPlans(d.Player)
					e.EnsurePaymentActions()
					e.PotentialActions(d.Player)
				}
				if err := e.Submit(b.answer(e, e.Pending())); err != nil {
					t.Fatalf("seed %d intent %d: %v", seed, n, err)
				}
			}
			mu.Lock()
			served += e.walkBlocksServed
			members += e.walkMembersServed
			ran++
			mu.Unlock()
		})
	}
	t.Cleanup(func() {
		if ran < 2 { // one seed selected with -run, or a seed already failed: the aggregate needs both
			return
		}
		if served == 0 {
			t.Fatal("no potential walk served a recorded block: the reuse path went unexercised")
		}
		if members == 0 {
			t.Fatal("no PotentialMana membership list was served from the record")
		}
		t.Logf("%d blocks and %d membership lists served", served, members)
	})
}
