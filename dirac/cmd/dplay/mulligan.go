package main

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/mull"
)

var mullPolicies = []string{"coin", "lands", "never", "constructed", "forge"}

// deckShape is what the land-counting mulligan policies read of a deck.
type deckShape struct{ total, lands int }

// mulligans answers the mulligan round for every seat.
type mulligans struct {
	policy string
	rng    dirac.Rand
	shape  [2]deckShape
	work   [2]mull.Work
	hand   []dirac.Obj
	cards  []mull.Card
	// the cards a seat is bottoming this round, served one per ask
	planFor [2]int
	plan    [2][]dirac.Obj
}

func (m *mulligans) init(policy string) {
	m.policy = policy
	m.hand = make([]dirac.Obj, 0, dirac.MaxObj)
	m.cards = make([]mull.Card, 0, dirac.MaxObj)
}

func (m *mulligans) newGame(seed uint64, shape [2]deckShape) {
	m.rng = dirac.NewRand(seed^0x6d756c6c, seed)
	m.shape = shape
	m.planFor = [2]int{}
}

func (m *mulligans) answer(s *dirac.State, d *dirac.Decision) dirac.Action {
	if d.Kind == dirac.DkMulligan {
		return m.keepOrMull(s, d)
	}
	p := d.Player
	if taken := int(s.Mulligans(p)); m.planFor[p] != taken {
		m.planFor[p] = taken
		m.plan[p] = m.bottoming(s, d, taken)
	}
	if len(m.plan[p]) > 0 {
		want := m.plan[p][0]
		m.plan[p] = m.plan[p][1:]
		for _, a := range d.Options[:d.N] {
			if dirac.Obj(a.Source()) == want {
				return a
			}
		}
	}
	return d.Options[0]
}

// handOf fills m.cards with p's hand, restricted to the objects keep accepts, in hand
// order: the order gorge's policies read. It returns the land count.
func (m *mulligans) handOf(s *dirac.State, p uint8, keep func(dirac.Obj) bool) int {
	m.hand = s.FeatZoneList(p, kop.ZoneHand, m.hand[:0])
	kept := m.hand[:0]
	m.cards = m.cards[:0]
	lands := 0
	for _, o := range m.hand {
		if !keep(o) {
			continue
		}
		c := mull.CardOf(s.Table().Cards[s.ObjCard(o)])
		if c.Land {
			lands++
		}
		kept = append(kept, o)
		m.cards = append(m.cards, c)
	}
	m.hand = kept
	return lands
}

func (m *mulligans) keepOrMull(s *dirac.State, d *dirac.Decision) dirac.Action {
	if d.N < 2 {
		return d.Options[0]
	}
	p := d.Player
	lands := m.handOf(s, p, func(dirac.Obj) bool { return true })
	taken, sh := int(s.Mulligans(p)), m.shape[p]
	var keep bool
	switch m.policy {
	case "never":
		keep = mull.KeepNever()
	case "lands":
		keep = mull.KeepLands(lands, len(m.cards), taken)
	case "constructed":
		keep = mull.KeepConstructed(m.cards, taken, sh.total, sh.lands)
	case "forge":
		keep = mull.KeepForge(m.cards, taken, sh.total, sh.lands, &m.work[p])
	default:
		keep = !mull.CoinMulls(m.rng.IntN(3))
	}
	if keep {
		return d.Options[0]
	}
	return d.Options[1]
}

// bottoming picks all the cards a seat owes at the round's first ask, while the hand is
// whole, as gorge's policies read it.
func (m *mulligans) bottoming(s *dirac.State, d *dirac.Decision, taken int) []dirac.Obj {
	p := d.Player
	m.handOf(s, p, func(o dirac.Obj) bool {
		for _, a := range d.Options[:d.N] {
			if dirac.Obj(a.Source()) == o {
				return true
			}
		}
		return false
	})
	n := min(taken, int(d.N), len(m.cards))
	var picks []int
	switch m.policy {
	case "lands", "constructed":
		picks = mull.BottomToLands(m.cards, n, &m.work[p])
	case "forge":
		picks = mull.BottomForge(m.cards, n, &m.work[p])
	default:
		picks = mull.BottomCheapest(m.cards, n, &m.work[p])
	}
	out := m.plan[p][:0]
	for _, i := range picks {
		out = append(out, m.hand[i])
	}
	return out
}
