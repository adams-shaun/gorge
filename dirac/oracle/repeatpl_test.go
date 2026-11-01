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

// The lockstep: RepeatEach player bodies that read the
// repeated player's life total, hand size or permanents.
//
//   - Pox Plague: each player loses half their life, discards half their hand
//     (read after the life loss), sacrifices half their permanents
//     (PlayerCountRemembered$ and RememberedPlayerCtrl);
//   - Zodiark, Umbral God: each player sacrifices half their non-God creatures
//     (Count$Valid ...nonGod+RememberedPlayerCtrl/HalfDown);
//   - Betor, Kin to All: with 40 total toughness each opponent loses half their
//     life, rounded up (PlayerCountRemembered$LifeTotal/HalfUp).
//
// Life gain and loss and card draw vary life totals and hands, odd values
// included; high-toughness creatures let Betor reach 40.
var krepeatplDecks = [2][]cardset.Entry{
	{
		{Name: "Pox Plague", Count: 2}, {Name: "Zodiark, Umbral God", Count: 2}, {Name: "Sign in Blood", Count: 3},
		{Name: "Night's Whisper", Count: 3}, {Name: "Sovereign's Bite", Count: 3}, {Name: "Blood Pact", Count: 2},
		{Name: "Warpath Ghoul", Count: 3}, {Name: "Barony Vampire", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Undead Minotaur", Count: 1}, {Name: "Swamp", Count: 15},
	},
	{
		{Name: "Betor, Kin to All", Count: 3}, {Name: "Indomitable Ancients", Count: 4}, {Name: "Rotting Mastodon", Count: 3},
		{Name: "Kami of Old Stone", Count: 3}, {Name: "Looming Altisaur", Count: 3}, {Name: "Catacomb Crocodile", Count: 3},
		{Name: "Wandering Tombshell", Count: 2}, {Name: "Sovereign's Bite", Count: 2}, {Name: "Night's Whisper", Count: 1},
		{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 4},
	},
}

// krpCounts are what gorge's event log showed.
type krpCounts struct {
	pox       int // Pox Plague resolutions where both players lost life and permanents were sacrificed
	zodiark   int // Zodiark ETB resolutions
	betorLose int // Betor end-step resolutions that made an opponent lose life
}

func krpScan(g *GorgeSide) krpCounts {
	var c krpCounts
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
	for i, ev := range evs {
		switch ev.Kind {
		case events.TurnChange:
			stack = stack[:0]
		case events.PutOnStack:
			stack = append(stack, stk{ev.Obj, false})
		case events.TriggerPush, events.AbilityPush:
			stack = append(stack, stk{ev.Obj, true})
		case events.Resolve:
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
			var lost [2]bool
			sacs := 0
			for _, w := range evs[i+1 : j] {
				switch {
				case w.Kind == events.LifeChange && w.Amount < 0 && w.Player < 2:
					lost[w.Player] = true
				case w.Kind == events.MoveZone && w.From == state.ZBattlefield && w.To == state.ZGraveyard:
					sacs++
				}
			}
			switch {
			case n == "Pox Plague" && !top.ab && lost[0] && lost[1] && sacs > 0:
				c.pox++
			case n == "Zodiark, Umbral God" && !top.ab:
				c.zodiark++
			case n == "Betor, Kin to All" && top.ab && (lost[0] || lost[1]):
				c.betorLose++
			}
		}
	}
	return c
}

// krpRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func krpRun(t *testing.T, seeds uint64) (tot krpCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repeatpl")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range krepeatplDecks {
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
	tbl, _, _, err := set.Matchup(krepeatplDecks[0], krepeatplDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krp-a", "krp-b"}, decks, 1)
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
		c := krpScan(gs)
		tot.pox += c.pox
		tot.zodiark += c.zodiark
		tot.betorLose += c.betorLose
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestRepeatplLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := krpRun(t, 400)
	t.Logf("%d games; Pox Plague resolutions (both lose life, sacrifice) %d; Zodiark ETBs %d; Betor life losses %d; stalls %v",
		games, tot.pox, tot.zodiark, tot.betorLose, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.pox < 10 {
		t.Errorf("Pox Plague resolutions %d, want at least 10", tot.pox)
	}
	if tot.zodiark < 10 {
		t.Errorf("Zodiark ETBs %d, want at least 10", tot.zodiark)
	}
	if tot.betorLose < 3 {
		t.Errorf("Betor life losses %d, want at least 3", tot.betorLose)
	}
}

// TestRepeatplMutation: with kop.CtrlEachPl matching the resolving
// controller instead of the repeated player, the lockstep must diverge
// within 128 seeds.
func TestRepeatplMutation(t *testing.T) {
	dirac.DebugEachPlCtrlYou = true
	defer func() { dirac.DebugEachPlCtrlYou = false }()
	_, games, div, _ := krpRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugEachPlCtrlYou", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
