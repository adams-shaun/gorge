package oracle

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// MiniCommanderDeck is commander deck f cut to n cards: its commanders (a
// partner pair is two), then n minus that many of its other cards, taken
// evenly across the rest of the list (expanded by count, in list order: the
// i-th of k picks is card i*len/k), so a list that ends with its lands keeps
// about its land share. The result is a commander deck whose commanders
// lead its list; it is a pure function of f and n.
//
// A pod of four 40-card mini decks holds 160 objects and at most 160 names,
// which fits the kernel's four-seat build and the oracle's card table, where
// four 100-card decks do not.
func MiniCommanderDeck(f deck.File, n int) (deck.File, error) {
	names := f.CommanderNames()
	if !strings.EqualFold(f.Format, "commander") || len(names) == 0 {
		return deck.File{}, fmt.Errorf("deck %s: not a commander deck", f.Name)
	}
	isCmd := func(name string) bool {
		for _, c := range names {
			if cards.NormalizeName(c) == cards.NormalizeName(name) {
				return true
			}
		}
		return false
	}
	var cmds []deck.Entry
	var rest []string
	for _, e := range f.Cards {
		if isCmd(e.Name) {
			cmds = append(cmds, deck.Entry{Name: e.Name, Count: 1})
			continue
		}
		for range e.Count {
			rest = append(rest, e.Name)
		}
	}
	if len(cmds) != len(names) {
		return deck.File{}, fmt.Errorf("deck %s: %d of its %d commanders are on its list", f.Name, len(cmds), len(names))
	}
	k := n - len(cmds)
	if k < 0 || k > len(rest) {
		return deck.File{}, fmt.Errorf("deck %s: a %d-card cut of %d commanders and %d other cards", f.Name, n, len(cmds), len(rest))
	}
	out := f
	out.Cards = cmds
	out.Sideboard = nil
	for i := range k {
		name := rest[i*len(rest)/k]
		if last := &out.Cards[len(out.Cards)-1]; len(out.Cards) > len(cmds) && last.Name == name {
			last.Count++
			continue
		}
		out.Cards = append(out.Cards, deck.Entry{Name: name, Count: 1})
	}
	return out, nil
}

// MiniCommanderDecks cuts each named deck of c to n cards
// (MiniCommanderDeck), keyed by name for Corpus.AddDecks: added, the pool's
// names play the mini decks.
func MiniCommanderDecks(c *Corpus, names []string, n int) (map[string]deck.File, error) {
	out := make(map[string]deck.File, len(names))
	for _, name := range names {
		f, err := c.deckFile(name)
		if err != nil {
			return nil, fmt.Errorf("deck %s: %w", name, err)
		}
		m, err := MiniCommanderDeck(f, n)
		if err != nil {
			return nil, err
		}
		out[name] = m
	}
	return out, nil
}
