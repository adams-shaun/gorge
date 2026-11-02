package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed decks over the generated
// nonactive pool, whose carriers read a SpellCast trigger's
// ValidActivatingPlayer$ NonActive:
//
//   - Player.NonActive (the caster is not the active player; the trigger
//     hits the caster): Scytheclaw Raptor, Adrenaline Jockey;
//   - Opponent.NonActive (an opponent casts on the controller's turn): March
//     of the World Ooze, which makes a 3/3 Elephant.
//
// Both decks hold instants and flash creatures, so spells are cast on both
// players' turns.

var knaDecks = [2][]cardset.Entry{
	{
		{Name: "March of the World Ooze", Count: 5}, {Name: "Scytheclaw Raptor", Count: 3}, {Name: "Adrenaline Jockey", Count: 2},
		{Name: "Giant Growth", Count: 3}, {Name: "Ambush Viper", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Shock", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Scytheclaw Raptor", Count: 3}, {Name: "Adrenaline Jockey", Count: 3}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Opt", Count: 4}, {Name: "Unsummon", Count: 3}, {Name: "Spectral Sailor", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

// knaCounts are what one game showed in gorge's event log.
type knaCounts struct {
	trig    int // Raptor / Jockey triggers pushed
	ownOff  int // ... for the controller's own off-turn cast
	oppOff  int // ... for the opponent's off-turn cast
	oozeTok int // Elephant tokens made
	oozeTrg int // Ooze triggers pushed
	oozeIn  int // Oozes that entered
	oozeOpp int
}

func knaScan(g *GorgeSide) knaCounts {
	var c knaCounts
	if g == nil {
		return c
	}
	active, caster, oozeCtrl := -1, -1, -1
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			active = int(ev.Player)
		case events.PutOnStack:
			caster = int(ev.Player)
			if oozeCtrl >= 0 && caster != oozeCtrl && active == oozeCtrl {
				c.oozeOpp++
			}
		case events.TriggerPush:
			ob := g.E.G.Obj(ev.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			switch ob.Face().Name {
			case "Scytheclaw Raptor", "Adrenaline Jockey":
				if caster >= 0 && caster != active {
					c.trig++
					if caster == int(ev.Player) {
						c.ownOff++
					} else {
						c.oppOff++
					}
				}
			case "March of the World Ooze":
				c.oozeTrg++
			}
		case events.MoveZone:
			if ob := g.E.G.Obj(ev.Obj); ev.To == state.ZBattlefield && ob != nil && ob.Face() != nil && ob.Face().Name == "March of the World Ooze" {
				c.oozeIn++
				oozeCtrl = int(ob.Controller)
			}
		case events.TokenCreate:
			if ev.Text == "g_3_3_elephant" {
				c.oozeTok++
			}
		}
	}
	return c
}

// knaRun plays seeds 1..256 and returns the summed counts and the first
// divergence (nil if none) and the kernel stalls.
func knaRun(t *testing.T) (tot knaCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("nonactive")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range knaDecks {
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
	tbl, _, _, err := set.Matchup(knaDecks[0], knaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	played := map[string]int{}
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kna-a", "kna-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := eagerPolicy(seed, played)
		// instants and flash creatures are mostly held on their caster's own
		// turn, so they are cast on the other player's turn
		rnd, x := RandomPolicy(seed+1000), seed
		var keep []Canon
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			a := pol.Pick(c)
			if c.G == nil || !knaHeld(c.G, a) {
				return a
			}
			// a deep stack (response chains, each off-turn cast also
			// pushes a trigger) holds everything; otherwise only the
			// caster's own turn does, and one cast in eight goes through
			deep := len(c.G.E.G.Stack) >= 6
			if !deep && c.G.E.G.Active != state.PlayerID(c.D.Player) {
				return a
			}
			x = x*6364136223846793005 + 1442695040888963407
			if !deep && x>>61 == 0 {
				return a
			}
			keep = keep[:0]
			for _, o := range c.D.Options {
				if !knaHeld(c.G, o) {
					keep = append(keep, o)
				}
			}
			if len(keep) == 0 {
				return a
			}
			d := *c.D
			d.Options = keep
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := knaScan(gs)
		tot.trig += c.trig
		tot.ownOff += c.ownOff
		tot.oppOff += c.oppOff
		tot.oozeTok += c.oozeTok
		tot.oozeTrg += c.oozeTrg
		tot.oozeIn += c.oozeIn
		tot.oozeOpp += c.oozeOpp
		if d := res.Div; d != nil {
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestNonactiveLockstep(t *testing.T) {
	t.Parallel()

	tot, games, div, stalls := knaRun(t)
	t.Logf("%d games; Raptor/Jockey off-turn triggers %d (own %d, opponent's %d); Oozes in %d (opp casts on its turn %d), Ooze triggers %d, Elephants %d; stalls %v",
		games, tot.trig, tot.ownOff, tot.oppOff, tot.oozeIn, tot.oozeOpp, tot.oozeTrg, tot.oozeTok, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.trig < 20 || tot.ownOff < 5 {
		t.Errorf("Raptor/Jockey triggers %d (own off-turn %d), want at least 20 and 5", tot.trig, tot.ownOff)
	}
	if tot.oozeTok < 10 {
		t.Errorf("Ooze tokens %d, want at least 10", tot.oozeTok)
	}
}

// TestNonactiveMutation: with the NonActive gate off (the trigger fires on
// every cast) the lockstep must diverge.
func TestNonactiveMutation(t *testing.T) {
	dirac.DebugNonActiveIgnore = true
	defer func() { dirac.DebugNonActiveIgnore = false }()
	_, games, div, _ := knaRun(t)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugNonActiveIgnore", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}

// knaHeld: a is a cast of an instant or a flash creature (held on its own
// turn).
func knaHeld(g *GorgeSide, a Canon) bool {
	if a.Kind() != ActCast {
		return false
	}
	o, ok := g.GorgeOption(a)
	if !ok {
		return false
	}
	ob := g.E.G.Obj(o.Obj)
	if ob == nil || ob.Face() == nil {
		return false
	}
	switch ob.Face().Name {
	case "Ambush Viper", "Spectral Sailor", "Shock", "Lightning Bolt", "Giant Growth", "Opt", "Unsummon":
		return true
	}
	return false
}
