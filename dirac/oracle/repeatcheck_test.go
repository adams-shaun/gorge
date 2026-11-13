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

// The lockstep: Kroxa, Titan of Death's Hunger's RepeatEach player body and
// its gate. The enter/attack trigger makes each opponent discard a card, then
// each opponent who did not discard a nonland card lose 3 life: the gate SVar
// (Remembered$Valid Card.nonLand+RememberedPlayerOwn) reads 1 exactly when
// that opponent discarded a nonland card, so a land discard costs the life
// and a nonland discard does not. The pool carries cheap discard fillers and
// outlets, the BBB/RRR floaters that pay Kroxa's escape cost (an escaped
// Kroxa survives its enter sacrifice and reaches the attack trigger),
// vanilla attackers and lands.
var krcDecks = [2][]cardset.Entry{
	{ // A: Kroxa, the floaters, discard outlets and vanilla attackers
		{Name: "Kroxa, Titan of Death's Hunger", Count: 3},
		{Name: "Priest of Gix", Count: 3}, {Name: "Coal Stoker", Count: 3},
		{Name: "Tormenting Voice", Count: 3}, {Name: "Thrill of Possibility", Count: 2},
		{Name: "Walking Corpse", Count: 3}, {Name: "Warpath Ghoul", Count: 3},
		{Name: "Barony Vampire", Count: 2}, {Name: "Sign in Blood", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 7},
	},
	{ // B: land-heavy, so its hand holds lands to discard, and cheap bodies
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
		{Name: "Rotting Mastodon", Count: 2}, {Name: "Kami of Old Stone", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Barony Vampire", Count: 3},
		{Name: "Catacomb Crocodile", Count: 3}, {Name: "Wandering Tombshell", Count: 2},
		{Name: "Thrill of Possibility", Count: 1}, {Name: "Tormenting Voice", Count: 3},
	},
}

// krcCounts are what gorge's event log showed.
type krcCounts struct {
	etb    int // Kroxa enter triggers that made the opponent discard
	attack int // Kroxa attack triggers that made the opponent discard
	lost   int // iterations where the opponent discarded and lost 3 life (the land arm)
	kept   int // iterations where the opponent discarded and lost nothing (the nonland arm)
}

// krcScan reads gorge's event log: a Kroxa trigger resolution is an ETB one
// when two or more Kroxa triggers were pushed adjacently (the enter
// sacrifice beside the discard trigger), else an attack one.
func krcScan(g *GorgeSide) krcCounts {
	var c krcCounts
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
	adj, etb := 0, map[state.ObjID]bool{}
	evs := g.E.L.Events
	for i, ev := range evs {
		switch ev.Kind {
		case events.TurnChange:
			stack, adj = stack[:0], 0
		case events.Priority, events.Resolve:
			adj = 0
			if ev.Kind == events.Priority {
				continue
			}
			fallthrough
		case events.TriggerPush, events.AbilityPush:
			if ev.Kind != events.TriggerPush && ev.Kind != events.AbilityPush {
				var top stk
				if ev.Text == "" {
					if len(stack) == 0 {
						continue
					}
					top = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				} else {
					for len(stack) > 0 && stack[len(stack)-1].src != ev.Obj {
						stack = stack[:len(stack)-1]
					}
					if len(stack) == 0 {
						continue
					}
					top = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				}
				n := name(top.src)
				j := i + 1
				for j < len(evs) && evs[j].Kind != events.Priority {
					j++
				}
				var discard, lost3 bool
				for _, w := range evs[i+1 : j] {
					switch {
					case w.Kind == events.MoveZone && w.From == state.ZHand && w.To == state.ZGraveyard:
						discard = true
					case w.Kind == events.LifeChange && w.Amount == -3:
						lost3 = true
					}
				}
				if n == "Kroxa, Titan of Death's Hunger" && top.ab && discard {
					if etb[ev.Obj] {
						c.etb++
					} else {
						c.attack++
					}
					if lost3 {
						c.lost++
					} else {
						c.kept++
					}
				}
				continue
			}
			adj++
			if name(ev.Obj) == "Kroxa, Titan of Death's Hunger" && adj >= 2 {
				etb[ev.Obj] = true
			}
			stack = append(stack, stk{ev.Obj, true})
		case events.PutOnStack:
			adj = 0
			stack = append(stack, stk{ev.Obj, false})
		}
	}
	return c
}

// krcRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func krcRun(t *testing.T, seeds uint64) (tot krcCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repeatcheck")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range krcDecks {
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
	tbl, _, _, err := set.Matchup(krcDecks[0], krcDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krc-a", "krc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := krcScan(gs)
		tot.etb += c.etb
		tot.attack += c.attack
		tot.lost += c.lost
		tot.kept += c.kept
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestRepeatcheckLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := krcRun(t, 400)
	t.Logf("%d games; Kroxa enter triggers %d, attack triggers %d; land arms (3 life lost) %d, nonland arms (no loss) %d; stalls %v",
		games, tot.etb, tot.attack, tot.lost, tot.kept, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.etb < 5 {
		t.Errorf("Kroxa enter triggers %d, want at least 5", tot.etb)
	}
	if tot.attack < 1 {
		t.Errorf("Kroxa attack triggers %d, want at least 1", tot.attack)
	}
	if tot.lost < 3 {
		t.Errorf("land arms %d, want at least 3", tot.lost)
	}
	if tot.kept < 3 {
		t.Errorf("nonland arms %d, want at least 3", tot.kept)
	}
}

// TestRepeatcheckMutation: with ExRemEachOwnNonLand counting land cards too,
// a land discard stops costing the life and the lockstep must diverge within
// 128 seeds.
func TestRepeatcheckMutation(t *testing.T) {
	dirac.DebugRemEachOwnAnyCard = true
	defer func() { dirac.DebugRemEachOwnAnyCard = false }()
	_, games, div, _ := krcRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugRemEachOwnAnyCard", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
