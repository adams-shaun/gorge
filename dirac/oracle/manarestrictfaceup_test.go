package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 106.6, 116.2b/m): a RestrictValid$ Static.* mana ability
// floats mana its restriction admits only for turning a permanent face up or
// unlocking a Room door. Creeping Peeper, Overgrown Zealot and Smoky Lounge
// are DSK; Tin Street Gossip is MKM. The runs below keep each carrier beside
// the cards its restriction admits: Rooms for Peeper/Smoky's Spell.Room half,
// manifest-dread creatures for a turn-up, and disguise creatures for Tin
// Street. A deck mixing Smoky Lounge's restricted red with Overgrown Zealot's
// pair of abilities is split into two runs: the pair's larger alternative is
// a planner unit the kernel prices apart, and the exact source order it taps
// is evidence the orchestrator's kfuzz sweep covers, not this scenario test.
//
// The scan reads gorge's log: a restricted positive ManaAdd names its source
// (the " @id" suffix), so each carrier's ability must actually have produced
// restricted mana; a MoveZone to the battlefield proves a carrier was cast.
var mrfaceCarriers = []string{
	"Creeping Peeper", "Overgrown Zealot", "Smoky Lounge // Misty Salon", "Tin Street Gossip",
}

func mrfaceName(n string) string {
	for _, c := range mrfaceCarriers {
		if n == c || strings.HasPrefix(n, c) || strings.HasPrefix(c, n) {
			return c
		}
	}
	return ""
}

func mrfaceScan(g *GorgeSide, produced, entered map[string]int, unlocks *int) {
	if g == nil {
		return
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.DoorUnlock:
			*unlocks++
		case events.MoveZone:
			if ev.To != state.ZBattlefield {
				continue
			}
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
				if n := mrfaceName(ob.Face().Name); n != "" {
					entered[n]++
				}
			}
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			_, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text))
			if !ok || src == 0 {
				continue
			}
			if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil {
				if n := mrfaceName(ob.Face().Name); n != "" {
					produced[n]++
				}
			}
		}
	}
}

type mrfaceCase struct {
	name     string
	pool     string
	decks    [2][]cardset.Entry
	produced []string // carriers whose restricted mana must be produced
	entered  []string // carriers that must be cast
}

func mrfaceCases() []mrfaceCase {
	return []mrfaceCase{
		{
			name: "peeper-smoky", pool: "std-dsk",
			decks: [2][]cardset.Entry{
				{
					{Name: "Creeping Peeper", Count: 6}, {Name: "Smoky Lounge", Count: 6},
					{Name: "Roaring Furnace", Count: 6}, {Name: "Bottomless Pool", Count: 6},
					{Name: "Manifest Dread", Count: 4}, {Name: "Innocuous Rat", Count: 6},
					{Name: "Island", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Swamp", Count: 1}, {Name: "Forest", Count: 1},
				},
				{
					{Name: "Creeping Peeper", Count: 6}, {Name: "Smoky Lounge", Count: 6},
					{Name: "Roaring Furnace", Count: 6}, {Name: "Bottomless Pool", Count: 6},
					{Name: "Manifest Dread", Count: 4}, {Name: "Innocuous Rat", Count: 6},
					{Name: "Island", Count: 2}, {Name: "Mountain", Count: 2}, {Name: "Swamp", Count: 1}, {Name: "Forest", Count: 1},
				},
			},
			produced: []string{"Creeping Peeper", "Smoky Lounge // Misty Salon"},
		},
		{
			// Smoky Lounge's floating {R}{R} admits only a Room spell or an unlock:
			// Grab the Prize (an instant-speed-free sorcery with a discard cost) must
			// still be paid by tapping lands, gorge's planned payment, before the
			// discard is asked (manaStep).
			name: "smoky-discard", pool: "std-dsk",
			decks: [2][]cardset.Entry{
				{
					{Name: "Smoky Lounge", Count: 8}, {Name: "Grab the Prize", Count: 10},
					{Name: "Innocuous Rat", Count: 4}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
				},
				{
					{Name: "Smoky Lounge", Count: 8}, {Name: "Grab the Prize", Count: 10},
					{Name: "Innocuous Rat", Count: 4}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
				},
			},
			produced: []string{"Smoky Lounge // Misty Salon"},
		},
		{
			// Enduring Vitality grants every creature "{T}: Add one mana of any
			// color", Creeping Peeper's creature body included: gorge's prefix search
			// taps the printed restricted {U} before the grant (pair 243), and an
			// impending Overlord of the Hauntwoods is the enchantment spell it pays.
			name: "peeper-vitality", pool: "std-dsk",
			decks: [2][]cardset.Entry{
				{
					{Name: "Creeping Peeper", Count: 8}, {Name: "Enduring Vitality", Count: 8},
					{Name: "Overlord of the Hauntwoods", Count: 6}, {Name: "Innocuous Rat", Count: 6},
					{Name: "Forest", Count: 6}, {Name: "Island", Count: 6},
				},
				{
					{Name: "Creeping Peeper", Count: 8}, {Name: "Enduring Vitality", Count: 8},
					{Name: "Overlord of the Hauntwoods", Count: 6}, {Name: "Innocuous Rat", Count: 6},
					{Name: "Forest", Count: 6}, {Name: "Island", Count: 6},
				},
			},
			// seed 23: both Peepers are census units of the Overlord cast, so the
			// planner leaves a Forest beside them rather than an Island (rank key 7).
			produced: []string{"Creeping Peeper"},
		},
		{
			name: "zealot", pool: "std-dsk",
			decks: [2][]cardset.Entry{
				{
					{Name: "Overgrown Zealot", Count: 8}, {Name: "Creeping Peeper", Count: 6},
					{Name: "Bottomless Pool", Count: 6}, {Name: "Innocuous Rat", Count: 8},
					{Name: "Island", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 3},
				},
				{
					{Name: "Overgrown Zealot", Count: 8}, {Name: "Creeping Peeper", Count: 6},
					{Name: "Bottomless Pool", Count: 6}, {Name: "Innocuous Rat", Count: 8},
					{Name: "Island", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 3},
				},
			},
			entered: []string{"Overgrown Zealot"},
		},
		{
			name: "gossip", pool: "std-mkm",
			decks: [2][]cardset.Entry{
				{
					{Name: "Tin Street Gossip", Count: 20}, {Name: "Dog Walker", Count: 10},
					{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
				},
				{
					{Name: "Tin Street Gossip", Count: 20}, {Name: "Bubble Smuggler", Count: 10},
					{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
				},
			},
			entered: []string{"Tin Street Gossip"},
		},
	}
}

func mrfaceRun(t *testing.T, c mrfaceCase, seeds int) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, c.pool)
	var cs [2][]*cards.Card
	for s, d := range c.decks {
		n := 0
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], card)
			}
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("%s deck %d has %d cards", c.name, s, n)
		}
	}
	tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
	if err != nil {
		t.Fatal(err)
	}
	up, upOK := specialCode("turn_face_up")
	unl, unlOK := specialCode("unlock")
	if !upOK || !unlOK {
		t.Fatal("special action codes missing")
	}
	produced, entered := map[string]int{}, map[string]int{}
	unlocks, games, divs := 0, 0, 0
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{c.name + "-a", c.name + "-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, map[string]int{})
		pol := PolicyFunc(func(pc *PickCtx) Canon {
			if pc.G != nil {
				gs = pc.G
			}
			for _, a := range pc.D.Options {
				if a.Kind() == ActSpecial && (a.Mode() == up || a.Mode() == unl) {
					return a
				}
			}
			return inner.Pick(pc)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		mrfaceScan(gs, produced, entered, &unlocks)
		if res.Div != nil {
			divs++
			if divs <= 4 {
				lo := res.Div.At - 10
				if lo < 0 {
					lo = 0
				}
				t.Errorf("%s seed %d: diverged at %d: %s %s %v %s\nref %v\nker %v\nrecent %v\nactions %v", c.name, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, res.RecentSources(m, 8), actionStrings(res.Actions[lo:res.Div.At]))
			}
		}
	}
	t.Logf("%s: %d games, %d divergent, produced %v, entered %v, unlocks %d", c.name, games, divs, produced, entered, unlocks)
	for _, n := range c.produced {
		if produced[n] == 0 {
			t.Errorf("%s: %s produced no restricted mana", c.name, n)
		}
	}
	for _, n := range c.entered {
		if entered[n] == 0 {
			t.Errorf("%s: %s never entered the battlefield", c.name, n)
		}
	}
}

func TestManaRestrictionFaceupLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range mrfaceCases() {
		mrfaceRun(t, c, 96)
	}
}
