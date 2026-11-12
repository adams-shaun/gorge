package oracle

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
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
	// want is the effect the overloaded cast must produce, observed in
	// gorge's event log between the spell's Resolve and its stack exit: one
	// entry per mapping family the case exercises.
	want []overloadEffect
}

type overloadEffect uint8

const (
	overloadBounce  overloadEffect = iota // a permanent leaves the battlefield for hand
	overloadDestroy                       // a permanent leaves the battlefield for the graveyard
	overloadDamage                        // a positive Damage event
	overloadTap                           // a Tap event
	overloadPump                          // the caster's creature changes P/T or keywords
	overloadShrink                        // an opponent's creature loses power (Downsize)
	overloadCounter                       // a positive CounterChange
	overloadGoad                          // a Goad event
	overloadDiscard                       // a card moves from hand to graveyard
)

var overloadCases = []overloadCase{
	// Cyclonic Rift: bounce each nonland permanent you don't control.
	{"Cyclonic Rift", []string{"Island"}, []string{"Fugitive Wizard"}, []string{"Grizzly Bears", "Hill Giant", "Ornithopter"}, "Forest", []overloadEffect{overloadBounce}},
	// Vandalblast: destroy each artifact you don't control.
	{"Vandalblast", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Ornithopter", "Phyrexian Walker"}, "Forest", []overloadEffect{overloadDestroy}},
	// Damn: destroy each creature, can't be regenerated.
	{"Damn", []string{"Swamp", "Plains"}, []string{"Typhoid Rats"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadDestroy}},
	// Mizzium Mortars: 4 damage to each creature you don't control.
	{"Mizzium Mortars", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadDamage}},
	// Scale Up: each creature you control becomes a 6/4 green Wurm.
	{"Scale Up", []string{"Forest"}, []string{"Grizzly Bears", "Hill Giant"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadPump}},
	// Blustersquall: tap each creature you don't control.
	{"Blustersquall", []string{"Island"}, []string{"Fugitive Wizard"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadTap}},
	// Mind Rake: each player discards two cards.
	{"Mind Rake", []string{"Swamp"}, []string{"Typhoid Rats"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadDiscard}},
	// Downsize: -4/-0 to each creature you don't control.
	{"Downsize", []string{"Island"}, []string{"Fugitive Wizard"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadShrink}},
	// Dynacharge: +2/+0 to each creature you control.
	{"Dynacharge", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadPump}},
	// Electrickery: 1 damage to each creature you don't control.
	{"Electrickery", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadDamage}},
	// Weapon Surge: +1/+0 and first strike to each creature you control.
	{"Weapon Surge", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadPump}},
	// Stirring Address: +2/+2 to each creature you control.
	{"Stirring Address", []string{"Plains"}, []string{"Elite Vanguard"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadPump}},
	// Mizzium Skin: +0/+1 and hexproof to each creature you control.
	{"Mizzium Skin", []string{"Island"}, []string{"Fugitive Wizard"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadPump}},
	// Dragonshift: each creature you control becomes a 4/4 blue and red Dragon.
	{"Dragonshift", []string{"Island", "Mountain"}, []string{"Fugitive Wizard"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadPump}},
	// Bail Out: each creature you control gains a dies-returns trigger.
	{"Bail Out", []string{"Swamp"}, []string{"Typhoid Rats"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", nil},
	// Fangs of Kalonia: a +1/+1 counter, then double each recipient's counters.
	{"Fangs of Kalonia", []string{"Forest"}, []string{"Grizzly Bears"}, []string{"Grizzly Bears"}, "Forest", []overloadEffect{overloadCounter}},
	// Spectacular Showdown: a double strike counter, then goad each recipient.
	{"Spectacular Showdown", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadCounter, overloadGoad}},
	// Street Spasm: X damage to each creature without flying you don't control.
	{"Street Spasm", []string{"Mountain"}, []string{"Raging Goblin"}, []string{"Grizzly Bears", "Hill Giant"}, "Forest", []overloadEffect{overloadDamage}},
}

// overloadPolicy keeps every hand and casts the case's card for its overload
// cost as soon as the option is offered, after floating the lands' mana. It
// casts the deck's filler permanents too, so the "each" sweeps resolve against
// a real board (a no-op or wrongly filtered overload program then fails the
// projection), but never the case card at its printed cost. Every other pick
// (mulligan, targets, choices) is the seeded random policy.
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
			// Cast a filler permanent before floating mana, so both boards
			// hold the permanents the overloaded "each" must hit. The case
			// card is excluded here and only ever cast by the arm above.
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && c.G.M.InstanceCard(int(a.Src())) != p.card {
					return a
				}
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

// overloadSheet is one battlefield creature's observable characteristics.
type overloadSheet struct {
	power, toughness int32
	keywords         string
}

// overloadSnapshot maps every battlefield creature to the characteristics a
// pump/animate changes.
func overloadSnapshot(ref *GorgeSide) map[state.ObjID]overloadSheet {
	g := ref.E.G
	out := make(map[state.ObjID]overloadSheet)
	for p := 0; p < len(g.Players); p++ {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			_, _, kws, power, toughness := ref.E.ViewCharacteristics(id)
			ks := append([]string(nil), kws...)
			sort.Strings(ks)
			out[id] = overloadSheet{power: power, toughness: toughness, keywords: strings.Join(ks, ",")}
		}
	}
	return out
}

// overloadWitness observes one seed: the overloaded cast of the case card
// must resolve, and between its Resolve event and its stack exit the log (or,
// for a continuous pump/animate, the board) must show the case's effect.
type overloadWitness struct {
	card       string
	want       []overloadEffect
	seen       map[overloadEffect]bool
	evAt       int
	armed      bool
	armedObj   state.ObjID
	resolved   bool
	overloaded map[state.ObjID]bool // CastInfo said this stack object was overloaded
	prev       map[state.ObjID]overloadSheet
}

func newOverloadWitness(tc overloadCase) *overloadWitness {
	return &overloadWitness{card: tc.card, want: tc.want, seen: map[overloadEffect]bool{},
		overloaded: map[state.ObjID]bool{}}
}

func (w *overloadWitness) complete() bool {
	if !w.resolved {
		return false
	}
	for _, e := range w.want {
		if !w.seen[e] {
			return false
		}
	}
	return true
}

func (w *overloadWitness) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if w.evAt > len(evs) {
		w.evAt, w.armed = 0, false
	}
	armedAtAny := w.armed
	for i := w.evAt; i < len(evs); i++ {
		e := &evs[i]
		if !w.armed {
			if e.Kind == events.CastInfo && strings.Contains(e.Counter, "overloaded") {
				w.overloaded[e.Obj] = true
			}
			if e.Kind == events.Resolve && e.Text == w.card && w.overloaded[e.Obj] {
				w.armed, w.armedObj, w.resolved = true, e.Obj, true
			}
			continue
		}
		armedAtAny = true
		switch {
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZHand:
			w.seen[overloadBounce] = true
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
			w.seen[overloadDestroy] = true
		case e.Kind == events.Damage && e.Amount > 0:
			w.seen[overloadDamage] = true
		case e.Kind == events.Tap:
			w.seen[overloadTap] = true
		case e.Kind == events.CounterChange && e.Amount > 0:
			w.seen[overloadCounter] = true
		case e.Kind == events.Goad:
			w.seen[overloadGoad] = true
		case e.Kind == events.MoveZone && e.From == state.ZHand && e.To == state.ZGraveyard:
			w.seen[overloadDiscard] = true
		}
		if e.Kind == events.MoveZone && e.Obj == w.armedObj && e.From == state.ZStack {
			w.armed = false // the spell left the stack: its resolution is over
		}
	}
	w.evAt = len(evs)
	cur := overloadSnapshot(ref)
	if armedAtAny {
		for id, c := range cur {
			p, ok := w.prev[id]
			if !ok {
				continue
			}
			o := g.Obj(id)
			if o == nil {
				continue
			}
			if o.Controller == 0 {
				if c.power != p.power || c.toughness != p.toughness || c.keywords != p.keywords {
					w.seen[overloadPump] = true
				}
			} else if c.power < p.power {
				w.seen[overloadShrink] = true
			}
		}
	}
	w.prev = cur
	return nil
}

// overloadDeck builds a 40-card deck: four copies of card, twenty-four filler
// permanents and twelve lands (six of each when there are two land types).
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
			fired := 0
			for seed := uint64(1); seed <= 24; seed++ {
				k, err := SetSide(set, tbl, Config{})(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				pol := &overloadPolicy{rnd: RandomPolicy(seed), card: tc.card}
				w := newOverloadWitness(tc)
				res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 6000, MaxTurns: 18, Check: w.check})
				CloseSide(k)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
				}
				if pol.fired == 0 {
					continue
				}
				fired += pol.fired
				if !w.complete() {
					continue
				}
				t.Logf("seed %d: %s cast overloaded %d time(s); observed %v", seed, tc.card, pol.fired, w.seen)
				return
			}
			t.Fatalf("no seed cast and resolved %s with its each-variant observable (overloaded casts: %d)", tc.card, fired)
		})
	}
}
