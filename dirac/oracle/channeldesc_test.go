package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The channel-description lockstep: the hand abilities
// whose cost is Discard<1/CARDNAME/this card> (the third part is prompt text,
// gorge cost.parse; the self-discard is paid from hand, CR 207.2c, 701.9a).
// The policy activates a hand ability whenever one is offered.
var kchanneldescB = []cardset.Entry{
	{Name: "Storm Crow", Count: 6}, {Name: "Ornithopter", Count: 6}, {Name: "Raging Goblin", Count: 6},
	{Name: "Shock", Count: 4}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
}

var kchanneldescDecks = [][2][]cardset.Entry{{
	{
		{Name: "Gideon's Memorial", Count: 4}, {Name: "Sureshot Sower", Count: 4}, {Name: "Action News Crew", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Mind Stone", Count: 4}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
	},
	kchanneldescB,
}, {
	{
		{Name: "The Masters of Evil", Count: 6}, {Name: "Action News Crew", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Mind Stone", Count: 4}, {Name: "Swamp", Count: 9}, {Name: "Plains", Count: 9},
	},
	kchanneldescB,
}}

var kchanneldescCarriers = []string{"Gideon's Memorial", "Sureshot Sower", "The Masters of Evil", "Action News Crew"}

// kchanneldescTally counts, on gorge's side, games in which a carrier's
// ability resolved after the card was discarded from hand (per carrier).
type kchanneldescTally struct {
	games    int
	resolved map[string]int
}

// kchanneldescGame is one game's event cursor: the carriers whose channel
// ability is on the stack, and the carriers already counted.
type kchanneldescGame struct {
	evAt    int
	pending []string
	counted map[string]bool
}

func kchanneldescPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

func kchanneldescRun(t *testing.T, n uint64, tal *kchanneldescTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("channeldesc")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kchanneldescDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"cd-a", "cd-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			gm := &kchanneldescGame{counted: map[string]bool{}}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				g := ref.E.G
				evs := ref.E.L.Events
				if len(evs) < gm.evAt {
					gm.evAt = 0
				}
				for ; gm.evAt < len(evs); gm.evAt++ {
					e := &evs[gm.evAt]
					name := chName(g, e.Obj)
					switch e.Kind {
					case events.MoveZone:
						// the channel cost: the carrier discarded from hand
						// as a cost (the ability goes on the stack above it)
						if e.From == state.ZHand && e.To == state.ZGraveyard && e.Text == "discarded as a cost" {
							for _, c := range kchanneldescCarriers {
								if c == name {
									gm.pending = append(gm.pending, name)
								}
							}
						}
					case events.Resolve:
						// an ability object has no face name; the stack is
						// LIFO, so it is the latest pending channel ability
						if name == "" && len(gm.pending) > 0 && tal != nil {
							n := gm.pending[len(gm.pending)-1]
							gm.pending = gm.pending[:len(gm.pending)-1]
							if !gm.counted[n] {
								gm.counted[n] = true
								tal.resolved[n]++
							}
						}
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, kchanneldescPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestChanneldescLockstep(t *testing.T) {
	t.Parallel()
	tal := kchanneldescTally{resolved: map[string]int{}}
	div := kchanneldescRun(t, 200, &tal)
	t.Logf("%d games, %d divergences: channel abilities resolved after a hand discard %v", tal.games, div, tal.resolved)
	for _, c := range kchanneldescCarriers {
		if tal.resolved[c] < 10 {
			t.Errorf("%s: channel ability resolved in %d games; want at least 10", c, tal.resolved[c])
		}
	}
}
