package oracle

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Suspend lockstep (CR 702.62): the hand special action under each
// carrier's cast timing (sorcery, instant, creature, artifact and Aura), the
// owner's upkeep counter removal, the may-cast ask of the last counter (the
// free cast, and the card left in exile when it is declined or cannot be
// cast), a creature's haste from that cast, and the spells that exile
// themselves with time counters. Both seats play the same deck over the
// generated suspend pool, so every card is cast by both engines.

var ksDecks = [][]cardset.Entry{
	{ // red and green
		{Name: "Rift Bolt", Count: 4}, {Name: "Rift Sower", Count: 4}, {Name: "Nantuko Shaman", Count: 3},
		{Name: "Search for Tomorrow", Count: 4}, {Name: "Lotus Bloom", Count: 2}, {Name: "Shivan Meteor", Count: 2},
		{Name: "Wheel of Fate", Count: 2}, {Name: "Petrified Plating", Count: 2}, {Name: "Mox Tantalite", Count: 1}, {Name: "Rousing Refrain", Count: 3},
		{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 6},
	},
	{ // blue and white
		{Name: "Inspiring Refrain", Count: 4}, {Name: "Ancestral Vision", Count: 4}, {Name: "Riftwing Cloudskate", Count: 4},
		{Name: "Infiltrator il-Kor", Count: 4}, {Name: "Duskrider Peregrine", Count: 4}, {Name: "Star Whale", Count: 2},
		{Name: "Shade of Trokair", Count: 3}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 7},
	},
	{ // black and the colourless carriers
		{Name: "Profane Tutor", Count: 4}, {Name: "Charnel Serenade", Count: 4}, {Name: "Suspended Sentence", Count: 4},
		{Name: "Festering March", Count: 3}, {Name: "Sol Talisman", Count: 2}, {Name: "Deep Dish Pizza", Count: 2}, {Name: "Phthisis", Count: 3},
		{Name: "Atraxi Warden", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 4},
	},
}

// ksScan counts, in gorge's event log, each carrier's suspend action and its
// casts from exile.
func ksScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.MoveZone:
			if ev.Text == "suspended" {
				n["suspended"]++
			}
			if ev.From == state.ZStack && ev.To == state.ZExile {
				if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Counter("TIME") > 0 {
					n["self-exiled"]++
				}
			}
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand {
				switch nm := ob.Face().Name; nm {
				case "Phthisis", "Rousing Refrain", "Inspiring Refrain":
					n[nm+" hard cast"]++
				}
			}
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZExile {
				n["cast from exile"]++
				if ob.Face().IsCreature() {
					n["creature cast"]++
				}
			}
		}
	}
}

// runSuspend plays the seeded games of deck di; mutate applies one of the
// kernel's debug switches and stops at the first divergence.
func runSuspend(t *testing.T, di int, seeds uint64, mutate func()) (games, divs int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("suspend")
	if err != nil {
		t.Fatal(err)
	}
	deck := ksDecks[di]
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = deck
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate()
		defer func() { dirac.DebugSuspendNoHaste, dirac.DebugSuspendNoTick = false, false }()
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ks-a", "ks-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 30})
		CloseSide(k)
		games++
		ksScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate != nil {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			if gs != nil && os.Getenv("KS_EVENTS") != "" {
				evs := gs.E.L.Events
				for _, ev := range evs[max(0, len(evs)-40):] {
					t.Logf("  ev %v obj=%d p%d %v->%v %q c=%q", ev.Kind, ev.Obj, ev.Player, ev.From, ev.To, ev.Text, ev.Counter)
				}
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func ksReport(branch map[string]int) string {
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	return strings.Join(bs, " ")
}

func TestSuspendLockstep(t *testing.T) {
	t.Parallel()

	for di := range ksDecks {
		games, divs, branch := runSuspend(t, di, 150, nil)
		t.Logf("deck %d: %d games, %d divergences; %s", di, games, divs, ksReport(branch))
		for _, want := range []string{"suspended", "cast from exile"} {
			if branch[want] < 10 {
				t.Errorf("deck %d: %s %d, want at least 10", di, want, branch[want])
			}
		}
		for _, want := range [][2]string{{"0", "Rousing Refrain hard cast"}, {"1", "Inspiring Refrain hard cast"}, {"2", "Phthisis hard cast"}} {
			if want[0] == string(rune('0'+di)) && branch[want[1]] < 5 {
				t.Errorf("deck %d: %s %d, want at least 5", di, want[1], branch[want[1]])
			}
		}
	}
}
