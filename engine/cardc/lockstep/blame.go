package lockstep

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Only restricts swap to one Forge card: every other card keeps its Forge pointer.
func Only(swap Swap, forge *cards.Card) Swap {
	return func(d []*cards.Card) []*cards.Card {
		out := swap(d)
		for i, c := range d {
			if i < len(out) && c != forge {
				out[i] = c
			}
		}
		return out
	}
}

// Blame names the served card a non-clean result is due to: for each distinct card the swap changes in g, in
// deck order (seat 0 first, then by position), it runs g in dir with Only(swap, card), and returns the name
// (Faces[0].Name) of the first whose result is not clean. "" when none is (the divergence needs two cards).
func Blame(g Game, dir Direction, swap Swap, newBot func(seed uint64) rules.Answerer) string {
	seen := map[*cards.Card]bool{}
	for _, d := range g.Cfg.Decks {
		sw := swap(d)
		for j, c := range d {
			if j >= len(sw) || sw[j] == c || seen[c] {
				continue
			}
			seen[c] = true
			if Run(g, dir, Only(swap, c), newBot).Outcome != "clean" {
				return c.Faces[0].Name
			}
		}
	}
	return ""
}
