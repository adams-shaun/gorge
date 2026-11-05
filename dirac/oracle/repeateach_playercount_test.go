package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: RepeatEach player bodies whose amount counts the repeated
// player's hand or library cards (gorge effRepeatEach binds each player as
// Remembered and reads X per iteration).
//
//   - Worlds Within Worlds: each player, controller first, may put any number
//     of creature cards from their hand onto the battlefield (ChangeNum$
//     Count$ValidHand Creature.RememberedPlayerCtrl);
//   - Doomsday Excruciator: entering cast, each player exiles all but the
//     bottom six cards of their library face down (DigNum$
//     Count$ValidLibrary Card.RememberedPlayerCtrl/Minus.6).
//
// High-toughness creatures keep the games long enough for six- and
// seven-mana spells; both decks hold creatures in hand when Worlds resolves.
var krpcountDecks = [2][]cardset.Entry{
	{
		{Name: "Doomsday Excruciator", Count: 4}, {Name: "Wandering Tombshell", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Catacomb Crocodile", Count: 3}, {Name: "Undead Minotaur", Count: 3}, {Name: "Night's Whisper", Count: 2},
		{Name: "Sign in Blood", Count: 2}, {Name: "Swamp", Count: 18},
	},
	{
		{Name: "Worlds Within Worlds", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Ornithopter", Count: 2},
		{Name: "Horned Turtle", Count: 4}, {Name: "Giant Spider", Count: 3}, {Name: "Rotting Mastodon", Count: 3},
		{Name: "Craw Wurm", Count: 2}, {Name: "Forest", Count: 9}, {Name: "Island", Count: 9},
	},
}

// krpcCounts are what gorge's event log showed.
type krpcCounts struct {
	worldsBoth int // Worlds Within Worlds resolutions where both players put a creature from hand onto the battlefield
	worldsAny  int // ... where at least one did
	excBoth    int // Excruciator ETB resolutions that exiled face down from both libraries
	excSix     int // decisions right after a face-down exile from both libraries that saw both libraries at six (krpcSix)
}

// krpcSix reports whether the gorge events since from exiled face down from
// both players' libraries, and if so whether both libraries now hold six.
func krpcSix(g *GorgeSide, from int) (exiled, six bool) {
	var both [2]bool
	for _, w := range g.E.L.Events[from:] {
		if w.Kind == events.MoveZone && w.From == state.ZLibrary && w.To == state.ZExile && strings.HasPrefix(w.Counter, "exiled_with_face_down") {
			if o := g.E.G.Obj(w.Obj); o != nil && o.Owner < 2 {
				both[o.Owner] = true
			}
		}
	}
	if !both[0] || !both[1] {
		return false, false
	}
	return true, len(g.E.G.Zone(state.ZLibrary, 0)) == 6 && len(g.E.G.Zone(state.ZLibrary, 1)) == 6
}

func krpcScan(g *GorgeSide) krpcCounts {
	var c krpcCounts
	if g == nil {
		return c
	}
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	type stk struct {
		src state.ObjID
		ab  bool
	}
	var stack []stk
	evs := g.E.L.Events
	owner := func(ev events.Event) int {
		if o := g.E.G.Obj(ev.Obj); o != nil && o.Owner < 2 {
			return int(o.Owner)
		}
		return -1
	}
	for i := 0; i < len(evs); i++ {
		ev := evs[i]
		switch ev.Kind {
		case events.TurnChange:
			stack = stack[:0]
		case events.PutOnStack:
			stack = append(stack, stk{ev.Obj, false})
		case events.TriggerPush, events.AbilityPush:
			stack = append(stack, stk{ev.Obj, true})
		case events.Resolve:
			for len(stack) > 0 && ev.Text != "" && stack[len(stack)-1].src != ev.Obj {
				stack = stack[:len(stack)-1]
			}
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n := name(top.src)
			j := i + 1
			for j < len(evs) && evs[j].Kind != events.Priority {
				j++
			}
			var put, exiled [2]bool
			for _, w := range evs[i+1 : j] {
				if w.Kind != events.MoveZone {
					continue
				}
				p := owner(w)
				if p < 0 {
					continue
				}
				if w.From == state.ZHand && w.To == state.ZBattlefield {
					put[p] = true
				}
				if w.From == state.ZLibrary && w.To == state.ZExile && strings.HasPrefix(w.Counter, "exiled_with_face_down") {
					exiled[p] = true
				}
			}
			i = j - 1
			switch {
			case n == "Worlds Within Worlds" && !top.ab:
				if put[0] && put[1] {
					c.worldsBoth++
				}
				if put[0] || put[1] {
					c.worldsAny++
				}
			case n == "Doomsday Excruciator" && top.ab && exiled[0] && exiled[1]:
				c.excBoth++
			}
		}
	}
	return c
}

// krpcRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func krpcRun(t *testing.T, seeds uint64) (tot krpcCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repeatplcount")
	if err != nil {
		t.Skipf("repeatplcount tables: %v (run make dirac-tables)", err)
	}
	var decks [2][]*cards.Card
	for s, d := range krpcountDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Skipf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(krpcountDecks[0], krpcountDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krpc-a", "krpc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		seen, six := 0, 0
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
				if ex, ok := krpcSix(gs, seen); ex && ok {
					six++
				}
				seen = len(gs.E.L.Events)
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := krpcScan(gs)
		tot.worldsBoth += c.worldsBoth
		tot.worldsAny += c.worldsAny
		tot.excBoth += c.excBoth
		tot.excSix += six
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestRepeatEachPlayerCountLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := krpcRun(t, 200)
	t.Logf("%d games; Worlds resolutions (both put / any put) %d / %d; Excruciator ETBs (both exiled / both left at six) %d / %d; stalls %v",
		games, tot.worldsBoth, tot.worldsAny, tot.excBoth, tot.excSix, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.worldsBoth < 3 {
		t.Errorf("Worlds Within Worlds resolutions where both players put a creature in: %d, want at least 3", tot.worldsBoth)
	}
	if tot.excSix < 3 || tot.excSix != tot.excBoth {
		t.Errorf("Excruciator ETBs leaving both libraries at six: %d of %d, want at least 3 and all", tot.excSix, tot.excBoth)
	}
}
