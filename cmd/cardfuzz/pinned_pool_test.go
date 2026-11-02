package main

import (
	_ "embed"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

//go:embed testdata/pinned_pool.txt
var pinnedPoolNames string

// pinnedPool is the pool the recorded games deal from, one card name per line. A
// game seed names decks only through the pool, so buildPool, which grows with
// every card the engine learns to play, would re-deal those games out from under
// the decisions their tests pin. A corpus that drops a listed card still re-deals.
func pinnedPool(reg *cards.Registry) (*pool, error) {
	names := make(map[string]bool)
	for _, n := range strings.Split(strings.TrimSpace(pinnedPoolNames), "\n") {
		names[n] = true
	}
	return buildPoolOf(reg, func(c *cards.Card) bool { return names[cardName(c)] })
}
