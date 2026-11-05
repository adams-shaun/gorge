package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Belief is a closed-list world's corpus of opponent deck lists
// (searchprobe belief.go): a world deals the opponent's hidden cards from
// a list drawn uniformly among those whose colour identity covers the
// colours of the opponent's seen cards (all lists when none does), minus
// the seen cards, shuffled, padded with the list's basic lands.
type Belief struct {
	decks  [][]dirac.CardID // each sorted by card id
	colors []uint8
	cover  [32][]int32
	basics [][]dirac.CardID
	tbl    *kop.Table
	// Marked, when set, flags cards whose deals a search counts
	// (Stats.MarkedDealt): a host marks the pool's inert fallbacks.
	Marked []bool
}

// CardColours is a card's belief colours: its colours, its pips' colours
// and the colours it can produce (not "any").
func CardColours(t *kop.Table, c dirac.CardID) uint8 {
	cd := &t.Cards[c]
	m := uint8(cd.Colors) & 0x1f
	for i := 0; i < 5; i++ {
		if cd.Cost.Pips[i] > 0 {
			m |= 1 << i
		}
	}
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbMana && a.ManaColors != kop.ColAny {
			m |= uint8(a.ManaColors) & 0x1f
		}
	}
	return m
}

// NewBelief builds a belief corpus over decks (allocates; init only).
func NewBelief(t *kop.Table, decks [][]dirac.CardID) *Belief {
	b := &Belief{tbl: t}
	for _, d := range decks {
		l := append([]dirac.CardID(nil), d...)
		for i := 1; i < len(l); i++ { // insertion sort by id
			for j := i; j > 0 && l[j] < l[j-1]; j-- {
				l[j], l[j-1] = l[j-1], l[j]
			}
		}
		var col uint8
		var bas []dirac.CardID
		for _, c := range l {
			col |= CardColours(t, c)
			if t.Cards[c].Types&kop.TBasic != 0 {
				bas = append(bas, c)
			}
		}
		idx := int32(len(b.decks))
		b.decks = append(b.decks, l)
		b.colors = append(b.colors, col)
		b.basics = append(b.basics, bas)
		for m := 0; m < 32; m++ {
			if uint8(m)&^col == 0 {
				b.cover[m] = append(b.cover[m], idx)
			}
		}
	}
	return b
}

// deal draws a list for seen (sorted, with its colour mask) and writes n
// cards for the hidden objects into out.
func (b *Belief) deal(r *dirac.Rand, seen []dirac.CardID, mask uint8, n int, out []dirac.CardID) int {
	var di int
	if c := b.cover[mask&0x1f]; len(c) > 0 {
		di = int(c[r.IntN(len(c))])
	} else {
		di = r.IntN(len(b.decks))
	}
	l := b.decks[di]
	m := 0
	si := 0
	for _, c := range l { // l minus the seen multiset (both sorted)
		for si < len(seen) && seen[si] < c {
			si++
		}
		if si < len(seen) && seen[si] == c {
			si++
			continue
		}
		if m < len(out) {
			out[m] = c
			m++
		}
	}
	r.ShuffleCards(out[:m])
	if m > n {
		m = n
	}
	for m < n && m < len(out) {
		if bas := b.basics[di]; len(bas) > 0 {
			out[m] = bas[r.IntN(len(bas))]
		} else {
			out[m] = l[r.IntN(len(l))]
		}
		m++
	}
	return m
}

// marked counts the flagged cards among cs.
func (b *Belief) marked(cs []dirac.CardID) int {
	if b.Marked == nil {
		return 0
	}
	n := 0
	for _, c := range cs {
		if int(c) < len(b.Marked) && b.Marked[c] {
			n++
		}
	}
	return n
}

// Decks is the number of lists in the corpus.
func (b *Belief) Decks() int { return len(b.decks) }
