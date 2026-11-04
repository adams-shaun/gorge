package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The DB$ Mana lockstep of a non-mana activated ability and of a your-phase
// trigger's Defined$ TriggeredPlayer. gorge's
// cards.IsManaAbilitySA classifies by the head word, so a loyalty ability
// (Quintorius Kand's -6, Narset of the Ancient Way's +1) or a targeted one
// (Red Death, Shipwrecker) with a DB$ Mana sub resolves on the stack and
// effMana adds the mana to its controller; Hulking Raptor's Main1 trigger
// adds {G}{G} to the active player (definedSpecTriggeredPlayer). Priest of
// Forgotten Gods is the any-number-of-target-players carrier:
// its LoseLife/Sacrifice chain targets both seats, and each targeted player
// is asked for a creature before the next.
var kdbmanactxDecks = [][2][]cardset.Entry{{
	{
		{Name: "Quintorius Kand", Count: 4}, {Name: "Narset of the Ancient Way", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Red Death, Shipwrecker", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Hulking Raptor", Count: 4}, {Name: "Priest of Forgotten Gods", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 11}, {Name: "Swamp", Count: 9},
	},
}, {
	{
		{Name: "Hulking Raptor", Count: 4}, {Name: "Priest of Forgotten Gods", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 11}, {Name: "Swamp", Count: 9},
	},
	{
		{Name: "Quintorius Kand", Count: 4}, {Name: "Narset of the Ancient Way", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Red Death, Shipwrecker", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 4},
	},
}}

// kdbmanactxCarrier is the ability index of each activated carrier.
var kdbmanactxCarrier = map[string]uint8{
	"Quintorius Kand": 2, "Narset of the Ancient Way": 0, "Red Death, Shipwrecker": 0,
	"Priest of Forgotten Gods": 0,
}

type kdbmanactxPending struct {
	player state.PlayerID
	turn   int32
	name   string
	seen   bool // the ability was observed on the stack
}

// kdbmanactxTally counts on gorge's side.
type kdbmanactxTally struct {
	games       int
	raptorSpent int            // Raptor's {G}{G} partly spent in the same main phase
	resolved    map[string]int // carrier activations resolved
	quintMana   int            // Quintorius -6 resolutions that left red in the pool
	pending     []kdbmanactxPending
	armed       [2]kdbmanactxArm
}

type kdbmanactxArm struct {
	turn int32
	g    int32
	on   bool
}

func (tal *kdbmanactxTally) activations() int {
	n := 0
	for _, v := range tal.resolved {
		n += v
	}
	return n
}

func (tal *kdbmanactxTally) observe(c *PickCtx) {
	g := c.G.E.G
	live := tal.pending[:0]
	for _, p := range tal.pending {
		if p.turn != g.Turn {
			continue
		}
		if len(g.Stack) > 0 {
			p.seen = true
		} else if p.seen {
			tal.resolved[p.name]++
			if p.name == "Quintorius Kand" && g.Players[p.player].Pool[state.MR] > 0 {
				tal.quintMana++
			}
			continue
		}
		live = append(live, p)
	}
	tal.pending = live
	if c.D.Kind != DPriority || g.Step != state.StepMain1 || len(g.Stack) > 0 {
		return
	}
	p := c.D.Player
	if p > 1 || state.PlayerID(p) != g.Active {
		return
	}
	rap := false
	for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
		if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Hulking Raptor" {
			rap = true
		}
	}
	if !rap {
		return
	}
	pg := g.Players[p].Pool[state.MG]
	a := &tal.armed[p]
	if a.on && a.turn == g.Turn {
		if pg < a.g {
			tal.raptorSpent++
		}
		a.g = pg
		return
	}
	if pg >= 2 {
		*a = kdbmanactxArm{turn: g.Turn, g: pg, on: true}
	}
}

func kdbmanactxPolicy(seed uint64, tal *kdbmanactxTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && tal != nil {
			tal.observe(c)
		}
		a := rnd.Pick(c)
		if c.G != nil && tal != nil && a.Kind() == ActActivate {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					if ix, ok := kdbmanactxCarrier[ob.Face().Name]; ok && a.Mode() == ix {
						tal.pending = append(tal.pending, kdbmanactxPending{
							player: ob.Controller, turn: c.G.E.G.Turn, name: ob.Face().Name,
						})
					}
				}
			}
		}
		return a
	})
}

// kdbmanactxMutate zeroes the AddMana / ManaChoice amount of each carrier:
// a table mutation, no kernel hook.
func kdbmanactxMutate(t *testing.T, tbl *dirac.Table) *dirac.Table {
	t.Helper()
	mt := *tbl
	mt.Cards = slices.Clone(tbl.Cards)
	hit := map[string]bool{}
	for i := range mt.Cards {
		name := mt.Cards[i].Name
		_, act := kdbmanactxCarrier[name]
		if !act && name != "Hulking Raptor" {
			continue
		}
		mt.Cards[i].Abilities = slices.Clone(mt.Cards[i].Abilities)
		for j := range mt.Cards[i].Abilities {
			a := &mt.Cards[i].Abilities[j]
			a.Prog = slices.Clone(a.Prog)
			for k := range a.Prog {
				in := &a.Prog[k]
				switch {
				case in.Op == kop.OpAddMana:
					in.A = kop.Const(0)
					hit[name] = true
				case in.Op == kop.OpExt && in.A == kop.Const(int(kop.ExtManaChoice)): // the colour choice's amount is B
					in.B = kop.Const(0)
					hit[name] = true
				}
			}
		}
	}
	for _, n := range []string{"Quintorius Kand", "Narset of the Ancient Way", "Red Death, Shipwrecker", "Hulking Raptor", "Priest of Forgotten Gods"} {
		if !hit[n] {
			t.Fatalf("no AddMana/ManaChoice to mutate in %s", n)
		}
	}
	return &mt
}

// kdbmanactxRun plays seeds 1..n of each matchup; with report it fails on
// every divergence, else it returns at the first (the mutation wants one) and
// the seed that diverged.
func kdbmanactxRun(t *testing.T, n uint64, report, mutate bool, tal *kdbmanactxTally) (div int, seed0 uint64) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dbmana")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range kdbmanactxDecks {
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
		if mutate {
			tbl = kdbmanactxMutate(t, tbl)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"dc-a", "dc-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			if tal != nil {
				tal.pending, tal.armed = tal.pending[:0], [2]kdbmanactxArm{}
			}
			res := Lockstep(m, seed, k, kdbmanactxPolicy(seed, tal), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div, seed
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div, 0
}

func TestDbmanactxLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	tal := &kdbmanactxTally{resolved: map[string]int{}}
	kdbmanactxRun(t, 256, true, false, tal)
	t.Logf("%d games: Hulking Raptor main-phase mana spent %d; activated DB$ Mana resolved %d (%v, Quintorius left red %d)",
		tal.games, tal.raptorSpent, tal.activations(), tal.resolved, tal.quintMana)
	if tal.raptorSpent < 3 {
		t.Errorf("Hulking Raptor's mana was spent %d times, want at least 3", tal.raptorSpent)
	}
	if n := tal.activations(); n < 3 {
		t.Errorf("%d loyalty/targeted activations resolved their DB$ Mana, want at least 3", n)
	}
}

// TestDbmanactxMutation: with every carrier's AddMana amount rewritten to 0
// in the table, the lockstep diverges within 64 seeds.
func TestDbmanactxMutation(t *testing.T) {
	n, seed := kdbmanactxRun(t, 64, false, true, nil)
	if n == 0 {
		t.Fatal("mutation: no seed diverged in 64 seeds")
	}
	t.Logf("mutation diverged at seed %d", seed)
}
