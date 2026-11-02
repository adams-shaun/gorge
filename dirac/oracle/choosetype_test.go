package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: the as-enters "choose a creature
// type" with an absent Defined$ (Chronicle of Victory, Collective Inferno),
// with ValidTypes$ (Dawn-Blessed Pennant, Eclipsed Realms: the ask offers
// the eight listed types only) and the AddType$ ChosenType static
// (Adaptive Automaton and Roaming Throne on themselves, Lifecraft Engine on
// Vehicles). The policy answers every type ask with the player's seeded
// favourite among the matchup's Lorwyn types, so an AddChosen permanent
// and a ChosenType reader of the same player often share the type. It
// never names a type outside the matchup: two different sources that both
// chose one are the kernel's loud StallBoard (act_choosetype.go).
var kchoosetypeDeck = [2][]cardset.Entry{
	{
		{Name: "Dawn-Blessed Pennant", Count: 3}, {Name: "Chronicle of Victory", Count: 2},
		{Name: "Adaptive Automaton", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Elvish Visionary", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Raging Goblin", Count: 3}, {Name: "Warped Tusker", Count: 2}, {Name: "Topiary Lecturer", Count: 2}, {Name: "Forest", Count: 8},
		{Name: "Mountain", Count: 4},
	},
	{
		{Name: "Eclipsed Realms", Count: 4}, {Name: "Collective Inferno", Count: 2},
		{Name: "Roaming Throne", Count: 3}, {Name: "Lifecraft Engine", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 8},
		{Name: "Island", Count: 3},
	},
}

// kchoosetypeTypes are the matchup's creature types among the eight
// ValidTypes$ words: the policy's favourites.
var kchoosetypeTypes = []string{"Elf", "Goblin", "Giant", "Merfolk"}

// kchoosetypeGrants are the AddType$ ChosenType carriers; kchoosetypeReaders
// the ChosenType readers whose bonus or trigger an AddChosen grant can reach.
var (
	kchoosetypeGrants  = map[string]bool{"Adaptive Automaton": true, "Roaming Throne": true, "Lifecraft Engine": true}
	kchoosetypeReaders = map[string]bool{
		"Chronicle of Victory": true, "Collective Inferno": true, "Dawn-Blessed Pennant": true,
		"Adaptive Automaton": true, "Lifecraft Engine": true, "Roaming Throne": true,
	}
)

type kchoosetypeTally struct {
	valid8  int  // type asks offering exactly the eight ValidTypes$ words
	reached bool // this game: an AddChosen grant met a same-type reader
}

// kchoosetypeReach reports whether a battlefield AddChosen permanent of
// some player has a chosen type it doesn't print that a different
// ChosenType reader of the same player also chose (Automaton + Chronicle:
// +2/+2; Automaton + Pennant: the life trigger; Throne + Inferno: doubled
// damage).
func kchoosetypeReach(g *GorgeSide) bool {
	for p := 0; p < 2; p++ {
		ids := g.E.G.Zone(state.ZBattlefield, state.PlayerID(p))
		for _, x := range ids {
			ox := g.E.G.Obj(x)
			if ox == nil || ox.Face() == nil || !kchoosetypeGrants[ox.Face().Name] || ox.ChosenType == "" {
				continue
			}
			printed := false
			for _, ty := range ox.Face().Types {
				printed = printed || ty == ox.ChosenType
			}
			if printed {
				continue
			}
			for _, y := range ids {
				oy := g.E.G.Obj(y)
				if y != x && oy != nil && oy.Face() != nil && kchoosetypeReaders[oy.Face().Name] && oy.ChosenType == ox.ChosenType {
					return true
				}
			}
		}
	}
	return false
}

// kchoosetypePolicy answers a type ask with the asking player's favourite
// (a ValidTypes$ ask: the type after it), casts or activates whenever it
// can, else random.
func kchoosetypePolicy(seed uint64, tl *kchoosetypeTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && !tl.reached && kchoosetypeReach(c.G) {
			tl.reached = true
		}
		if c.G != nil && len(c.D.Options) > 0 {
			if o, ok := c.G.GorgeOption(c.D.Options[0]); ok && o.Kind == "type" {
				buf = buf[:0]
				// a ValidTypes$ ask (eight options: Pennant, Eclipsed Realms)
				// takes the next type after the favourite: gorge does not
				// trigger Dawn-Blessed Pennant when an Adaptive Automaton of
				// its type enters (the report's Gorge finding), so the
				// AddChosen carriers never share the Pennant's type
				fi := int(seed) + 3*int(c.D.Player)
				if len(c.D.Options) == 8 {
					fi++
				}
				fav := kchoosetypeTypes[fi%len(kchoosetypeTypes)]
				var pick Canon
				found := false
				allowed := map[string]bool{}
				for _, a := range c.D.Options {
					oa, _ := c.G.GorgeOption(a)
					allowed[oa.Label] = true
					for _, w := range kchoosetypeTypes {
						if oa.Label == w {
							buf = append(buf, a)
						}
					}
					if oa.Label == fav {
						pick, found = a, true
					}
				}
				if len(c.D.Options) == 8 && allowed["Elemental"] && allowed["Treefolk"] && allowed["Kithkin"] {
					tl.valid8++
				}
				if found {
					return pick
				}
				if len(c.D.Options) == 8 {
					buf = buf[:0] // never a random Lorwyn pick: it could be the Automaton's type
				}
				if len(buf) > 0 {
					d := *c.D
					d.Options = buf
					return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
				}
				return rnd.Pick(c)
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// kchoosetypeSetup loads the corpus, the choosetype set, the decks and
// the matchup table, and returns the per-seed game builder.
func kchoosetypeSetup(t *testing.T) func(uint64, Config) (*Matchup, Side) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosetype")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kchoosetypeDeck {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kchoosetypeDeck[0], kchoosetypeDeck[1])
	if err != nil {
		t.Fatal(err)
	}
	mk := func(seed uint64, cfg Config) (*Matchup, Side) {
		m, err := NewMatchup(reg, [2]string{"kchoosetype-a", "kchoosetype-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		return m, k
	}
	return mk
}

func TestChoosetypeLockstep(t *testing.T) {
	t.Parallel()
	mk := kchoosetypeSetup(t)
	valid8, reached, divs, stalls := 0, 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, k := mk(seed, cfg)
		var tl kchoosetypeTally
		res := Lockstep(m, seed, k, kchoosetypePolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		valid8 += tl.valid8
		if tl.reached {
			reached++
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("256 games: %d divergences, %d kernel stalls, %d eight-option ValidTypes asks, %d games where an AddChosen grant met a same-type reader", divs, stalls, valid8, reached)
	if valid8 < 30 {
		t.Errorf("%d eight-option ValidTypes asks, want at least 30", valid8)
	}
	if reached < 20 {
		t.Errorf("%d games where an AddChosen grant met a reader, want at least 20", reached)
	}
}

// TestChoosetypeMutation: with the ValidTypes$ filter skipped the kernel
// offers the whole vocabulary where gorge offers eight types, so the
// lockstep must diverge.
func TestChoosetypeMutation(t *testing.T) {
	mk := kchoosetypeSetup(t)
	old := dirac.DebugChooseTypeAllVocab
	dirac.DebugChooseTypeAllVocab = true
	defer func() { dirac.DebugChooseTypeAllVocab = old }()
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, k := mk(seed, cfg)
		var tl kchoosetypeTally
		res := Lockstep(m, seed, k, kchoosetypePolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverged at decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("DebugChooseTypeAllVocab did not diverge in 64 seeds")
}
