package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The overload lockstep (CR 702.96a): each card is cast for its overload cost
// with no target announcement, and the "each" program resolves identically in
// gorge and the kernel. The decks hold the matching permanents and enough
// lands for the overload price; the policy floats the lands' mana so gorge's
// priority walk offers the alternative cost.
type overloadCase struct {
	card    string
	lands   []string // the caster's lands
	own     []string // the caster's other cards
	opp     []string // the opponent's permanents
	oppLand string
}

var overloadCases = []overloadCase{
	// Cyclonic Rift: bounce each nonland permanent you don't control.
	{"Cyclonic Rift", []string{"Island"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant", "Ornithopter"}, "Forest"},
	// Vandalblast: destroy each artifact you don't control.
	{"Vandalblast", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Ornithopter", "Phyrexian Walker"}, "Forest"},
	// Damn: destroy each creature, can't be regenerated.
	{"Damn", []string{"Swamp", "Plains"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Mizzium Mortars: 4 damage to each creature you don't control.
	{"Mizzium Mortars", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Scale Up: each creature you control becomes a 6/4 green Wurm.
	{"Scale Up", []string{"Forest"}, []string{"Grizzly Bears", "Hill Giant"}, []string{"Grizzly Bears"}, "Forest"},
	// Blustersquall: tap each creature you don't control.
	{"Blustersquall", []string{"Island"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Mind Rake: each player discards two cards.
	{"Mind Rake", []string{"Swamp"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Downsize: -4/-0 to each creature you don't control.
	{"Downsize", []string{"Island"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Dynacharge: +2/+0 to each creature you control.
	{"Dynacharge", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Electrickery: 1 damage to each creature you don't control.
	{"Electrickery", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Weapon Surge: +1/+0 and first strike to each creature you control.
	{"Weapon Surge", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Stirring Address: +2/+2 to each creature you control.
	{"Stirring Address", []string{"Plains"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Mizzium Skin: +0/+1 and hexproof to each creature you control.
	{"Mizzium Skin", []string{"Island"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Dragonshift: each creature you control becomes a 4/4 blue and red Dragon.
	{"Dragonshift", []string{"Island", "Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Bail Out: each creature you control gains a dies-returns trigger.
	{"Bail Out", []string{"Swamp"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Fangs of Kalonia: a +1/+1 counter, then double each recipient's counters.
	{"Fangs of Kalonia", []string{"Forest"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest"},
	// Spectacular Showdown: a double strike counter, then goad each recipient.
	{"Spectacular Showdown", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
	// Street Spasm: X damage to each creature without flying you don't control.
	{"Street Spasm", []string{"Mountain"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest"},
}

// overloadPolicy keeps every hand and casts the case's card for its overload
// cost as soon as the option is offered, after floating the lands' mana. Every
// other pick (mulligan, targets, choices) is the seeded random policy; at
// priority it plays a land, activates a mana ability, or passes, so the card
// is never slipped into its printed-cost cast.
type overloadPolicy struct {
	rnd   Policy
	card  string
	fired int
}

func (p *overloadPolicy) Pick(c *PickCtx) Canon {
	switch c.D.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DPriority:
		if c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && a.Mode()&0xf == castModeOther && c.G.M.InstanceCard(int(a.Src())) == p.card {
					p.fired++
					return a
				}
			}
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
				return a
			}
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActActivate }); ok {
				return a // float the lands' mana so the overloaded cast is offered
			}
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPass }); ok {
				return a
			}
		}
	}
	return p.rnd.Pick(c)
}

// overloadDeck builds a 40-card deck: four copies of card, twelve fillers and
// twenty-four lands (twelve of each when there are two land types).
func overloadDeck(reg *cards.Registry, card string, lands, fillers []string) []*cards.Card {
	var deck []*cards.Card
	add := func(name string, n int) {
		c, ok := reg.Lookup(name)
		if !ok {
			panic("overload deck: " + name + " not in the corpus")
		}
		for i := 0; i < n; i++ {
			deck = append(deck, c)
		}
	}
	add(card, 4)
	for len(deck) < 28 {
		add(fillers[len(deck)%len(fillers)], 1)
	}
	for i := 0; len(deck) < 40; i++ {
		add(lands[i%len(lands)], 1)
	}
	return deck
}

// overloadOpponent builds the 40-card opposing deck: twenty permanent cards
// and twenty lands, never the case card.
func overloadOpponent(reg *cards.Registry, land string, fillers []string) []*cards.Card {
	var deck []*cards.Card
	add := func(name string, n int) {
		c, ok := reg.Lookup(name)
		if !ok {
			panic("overload opponent: " + name + " not in the corpus")
		}
		for i := 0; i < n; i++ {
			deck = append(deck, c)
		}
	}
	for len(deck) < 20 {
		add(fillers[len(deck)%len(fillers)], 1)
	}
	for len(deck) < 40 {
		add(land, 1)
	}
	return deck
}

// overloadEntries is deck's cards as sorted cardset entries.
func overloadEntries(deck []*cards.Card) []cardset.Entry {
	seen := map[string]int{}
	for _, c := range deck {
		seen[c.Faces[0].Name]++
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	es := make([]cardset.Entry, 0, len(names))
	for _, n := range names {
		es = append(es, cardset.Entry{Name: n, Count: seen[n]})
	}
	return es
}

// TestOverloadLockstep plays each lowered overload card identically; a seed
// counts only when the policy actually cast it for its overload cost.
func TestOverloadLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	// One table over every case's cards keeps the kgen build to one.
	var union []string
	for _, tc := range overloadCases {
		union = append(union, tc.card)
		union = append(union, tc.own...)
		union = append(union, tc.opp...)
		union = append(union, tc.lands...)
		union = append(union, tc.oppLand)
	}
	set := printedDelveSet(t, [2][]string{union})
	var unionDeck []*cards.Card
	seen := map[string]bool{}
	for _, n := range union {
		if seen[n] {
			continue
		}
		seen[n] = true
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		unionDeck = append(unionDeck, c)
	}
	ue := overloadEntries(unionDeck)
	tbl, _, _, err := set.Matchup(ue, ue)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range overloadCases {
		tc := tc
		t.Run(tc.card, func(t *testing.T) {
			main := overloadDeck(reg, tc.card, tc.lands, tc.own)
			opp := overloadOpponent(reg, tc.oppLand, tc.opp)
			m, err := NewMatchup(reg, [2]string{tc.card + " caster", "opponent"}, [2][]*cards.Card{main, opp}, 0)
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= 24; seed++ {
				k, err := SetSide(set, tbl, Config{})(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				pol := &overloadPolicy{rnd: RandomPolicy(seed), card: tc.card}
				res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 6000, MaxTurns: 18})
				CloseSide(k)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
				}
				if pol.fired == 0 {
					continue
				}
				t.Logf("seed %d: %s cast overloaded %d time(s)", seed, tc.card, pol.fired)
				return
			}
			t.Fatalf("no seed cast %s for its overload cost", tc.card)
		})
	}
}
