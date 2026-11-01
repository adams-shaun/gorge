package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fetch.go: the fetchneed option. Off, a library search is
// answered with its first offer in library order (a random basic,
// weighted by the basics left). On, a search whose every offer is a land
// takes the land chooseLand would play: the most unmet colour pips of the
// castable hand covered, then basic, then the fewest colours; ties to the
// first in library order. A search offering any non-land, or nothing, is
// answered as before.

// EnableFetchNeed arms :fetchneed. The zero value is off, so every existing
// bot is unchanged.
func (bt *Bot) EnableFetchNeed() *Bot { bt.fetchNeed = true; return bt }

// FetchNeed reports whether :fetchneed is armed.
func (bt *Bot) FetchNeed() bool { return bt.fetchNeed }

// fetchLand is the :fetchneed pick of library search d, or -1 when an
// offer is not a land (or nothing but Done is offered).
func (bt *Bot) fetchLand(s *dirac.State, d *dirac.Decision) int {
	t := s.Table()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone {
			continue
		}
		if a.Kind() != dirac.AkChoose || t.Cards[s.ObjCard(dirac.Obj(a.Source()))].Types&kop.TLand == 0 {
			return -1
		}
	}
	b := &bt.b
	b.fill(s, bt.f, d.Player)
	unmet := b.unmetNeed()
	best := -1
	var bestCover int32
	var bestBasic bool
	var bestFlex int
	var bestKey uint32
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkDone {
			continue
		}
		o := dirac.Obj(a.Source())
		f := bt.f.card(s.ObjCard(o))
		cover, basic, flex := landScore(f, f.Basic, &unmet)
		key := bt.objKey(s, o)
		if landBetter(best == -1, cover, basic, flex, bestCover, bestBasic, bestFlex) ||
			cover == bestCover && basic == bestBasic && flex == bestFlex && key < bestKey {
			best, bestCover, bestBasic, bestFlex, bestKey = i, cover, basic, flex, key
		}
	}
	return best
}
