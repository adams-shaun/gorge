package mtgish

import (
	"sort"
	"strings"
)

// Index finds a card by every name gorge uses.
type Index struct {
	primary   map[string]*Card // front names and "A // B" joins
	secondary map[string]*Card // every other face name
	keys      []string
}

// NewIndex indexes cards in the order given; within one map the first card
// wins. Maps are only read by key; Keys lists the sorted names.
func NewIndex(cards []*Card) *Index {
	ix := &Index{primary: map[string]*Card{}, secondary: map[string]*Card{}}
	for _, c := range cards {
		put(ix.primary, c.Name, c)
		names := c.FaceNames()
		if len(names) > 1 {
			put(ix.primary, strings.Join(names, " // "), c)
		}
		for _, n := range names[1:] {
			put(ix.secondary, n, c)
		}
	}
	seen := map[string]bool{}
	for _, m := range []map[string]*Card{ix.primary, ix.secondary} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				ix.keys = append(ix.keys, k)
			}
		}
	}
	sort.Strings(ix.keys)
	return ix
}

func put(m map[string]*Card, name string, c *Card) {
	if name == "" {
		return
	}
	if _, dup := m[name]; !dup {
		m[name] = c
	}
}

// Lookup finds a card by front face name, then by the "A // B" join of its
// face names, then by any other face name (first card in file order).
func (ix *Index) Lookup(name string) (*Card, bool) {
	if c, ok := ix.primary[name]; ok {
		return c, true
	}
	c, ok := ix.secondary[name]
	return c, ok
}

// Keys lists every indexed name, sorted.
func (ix *Index) Keys() []string { return ix.keys }
