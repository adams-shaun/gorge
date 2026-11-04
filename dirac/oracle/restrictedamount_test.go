package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep (gorge, CR 106.6, 601.2g-h): a mana source whose alternatives
// make different amounts. Intrepid Stablemaster taps for {G}, or for two
// mana of one colour that only Mount and Vehicle spells may spend; gorge's
// planner lists them as one source with alternatives of {G} and of two mana
// of each colour, ranks the plan by sources, then surplus, and the surplus
// of a two-mana tap floats restricted. The decks pay Mount and Vehicle spells
// beside plain spells from boards short of lands, so the plan's choice
// between the amounts, the colour of a two-mana tap and the restricted
// leftover all reach a later decision.
//
// The mechanism is counted from gorge's log and pool: a two-mana restricted
// tap that funds a spell, and a decision posed while a restricted batch of
// the source floats.

var restrAmountDecks = [][2][]cardset.Entry{{
	// Stablemasters over a few lands against plain spells
	{
		{Name: "Intrepid Stablemaster", Count: 9}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Island", Count: 2},
		{Name: "Alacrian Jaguar", Count: 3}, {Name: "Bulwark Ox", Count: 3}, {Name: "Bridled Bighorn", Count: 3},
		{Name: "Adventurer's Airship", Count: 3}, {Name: "Smuggler's Copter", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Wind Drake", Count: 3},
	},
	{
		{Name: "Craw Wurm", Count: 3}, {Name: "Hill Giant", Count: 4}, {Name: "Serra Angel", Count: 3}, {Name: "Shivan Dragon", Count: 3},
		{Name: "Giant Growth", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Divination", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 3}, {Name: "Island", Count: 3},
	},
}, {
	// Stablemasters on both sides, almost no lands: two-mana taps overpay
	{
		{Name: "Intrepid Stablemaster", Count: 8}, {Name: "Forest", Count: 3}, {Name: "Plains", Count: 1},
		{Name: "Alacrian Jaguar", Count: 4}, {Name: "Bulwark Ox", Count: 4}, {Name: "Bridled Bighorn", Count: 4},
		{Name: "Adventurer's Airship", Count: 4}, {Name: "Smuggler's Copter", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Wind Drake", Count: 4},
	},
	{
		{Name: "Intrepid Stablemaster", Count: 8}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 2},
		{Name: "Adventurer's Airship", Count: 4}, {Name: "Smuggler's Copter", Count: 4}, {Name: "Bulwark Ox", Count: 4},
		{Name: "Craw Wurm", Count: 3}, {Name: "Shivan Dragon", Count: 2}, {Name: "Lightning Bolt", Count: 3}, {Name: "Hill Giant", Count: 2},
	},
}, {
	// Elves and Mind Stones beside Stablemasters: the plan mixes census
	// sources with the restricted alternative
	{
		{Name: "Intrepid Stablemaster", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Mind Stone", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 3}, {Name: "Mountain", Count: 2},
		{Name: "Alacrian Jaguar", Count: 3}, {Name: "Bridled Bighorn", Count: 3}, {Name: "Adventurer's Airship", Count: 3},
		{Name: "Smuggler's Copter", Count: 3}, {Name: "Craw Wurm", Count: 2},
	},
	{
		{Name: "Intrepid Stablemaster", Count: 4}, {Name: "Shivan Dragon", Count: 3}, {Name: "Serra Angel", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 3}, {Name: "Divination", Count: 3},
		{Name: "Forest", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 5},
	},
}, {
	// two-colour and double-pip Vehicles and Mounts over lands of other
	// colours: a colour only a Stablemaster's two-mana tap makes leaves
	// restricted mana over
	{
		{Name: "Intrepid Stablemaster", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 2},
		{Name: "Boosted Sloop", Count: 3}, {Name: "Haunted Hellride", Count: 3}, {Name: "Rocketeer Boostbuggy", Count: 3},
		{Name: "Seraphic Steed", Count: 3}, {Name: "Congregation Gryff", Count: 3}, {Name: "Debris Beetle", Count: 3},
		{Name: "Apocalypse Runner", Count: 3}, {Name: "Mizzium Tank", Count: 1}, {Name: "Thunderous Velocipede", Count: 1},
		{Name: "Gilded Assault Cart", Count: 1},
	},
	{
		{Name: "Intrepid Stablemaster", Count: 8}, {Name: "Swamp", Count: 3}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 2},
		{Name: "Thunderous Velocipede", Count: 3}, {Name: "Boosted Sloop", Count: 3}, {Name: "Haunted Hellride", Count: 3},
		{Name: "Apocalypse Runner", Count: 3}, {Name: "Seraphic Steed", Count: 3}, {Name: "Rocketeer Boostbuggy", Count: 3},
		{Name: "Hill Giant", Count: 3}, {Name: "Wind Drake", Count: 2}, {Name: "Lightning Bolt", Count: 1},
	},
}}

const (
	restrAmountWide  = iota // a two-mana restricted tap funds a spell
	restrAmountFloat        // a decision is posed while a restricted batch of the source floats
	restrAmountN
)

var restrAmountNames = [restrAmountN]string{"two-mana tap funds a cast", "restricted leftover floats into a decision"}

const restrAmountMinFires = 10

// restrAmountScan counts, from gorge's log, two-mana restricted batches the
// source made.
func restrAmountScan(g *GorgeSide, n *[restrAmountN]int) {
	if g == nil {
		return
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		if ev.Kind != events.ManaAdd || ev.Amount < 2 {
			continue
		}
		if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && src != 0 {
			if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil && ob.Face().Name == "Intrepid Stablemaster" {
				n[restrAmountWide]++
			}
		}
	}
}

func restrAmountPolicy(seed uint64, played map[string]int, gs **GorgeSide, n *[restrAmountN]int) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
			if c.D.Kind == DPriority {
				for p := range c.G.E.G.Players {
					for _, r := range c.G.E.G.Players[p].RestrictedMana {
						if r.Amount > 0 {
							if ob := c.G.E.G.Obj(r.Source); ob != nil && ob.Face() != nil && ob.Face().Name == "Intrepid Stablemaster" {
								n[restrAmountFloat]++
							}
						}
					}
				}
			}
		}
		return inner.Pick(c)
	})
}

func restrAmountSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("restrictedamount")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// restrAmountRun plays every matchup over seeds 1..seeds on both gorge
// surfaces and returns the divergent games.
func restrAmountRun(t *testing.T, seeds uint64, fires *[restrAmountN]int, report func(format string, args ...any)) (games, divs int) {
	reg, set := restrAmountSetup(t)
	played := map[string]int{}
	for mi, md := range restrAmountDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			for _, surface := range []bool{false, true} {
				if o := os.Getenv("KF24_ONLY"); o != "" && o != fmt.Sprintf("%d:%d:%v", mi, seed, surface) {
					continue
				}
				cfg := Config{GorgeAutopaySurface: surface}
				m, err := NewMatchup(reg, [2]string{"ra-a", "ra-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				var gs *GorgeSide
				res := Lockstep(m, seed, k, restrAmountPolicy(seed, played, &gs, fires), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				restrAmountScan(gs, fires)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Errorf("matchup %d seed %d surface %v: kernel stall %s", mi, seed, surface, res.Stall)
				}
				if res.Div != nil {
					divs++
					if divs == 1 && os.Getenv("KF24_DUMP") != "" {
						for i := max(0, len(res.Actions)-8); i < len(res.Actions); i++ {
							t.Logf("  action %d %v", i, res.Actions[i])
						}
						kcastDump(t, gs)
					}
					if divs <= 6 {
						report("matchup %d seed %d surface %v: diverged at %d: %s %s %v", mi, seed, surface, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
					}
				}
			}
		}
	}
	return games, divs
}

func TestRestrictedAmountLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(200)
	if s := os.Getenv("KF24_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	var fires [restrAmountN]int
	games, divs := restrAmountRun(t, seeds, &fires, t.Errorf)
	t.Logf("%d games, %d divergent; mechanism fires %v", games, divs, fires)
	for i, c := range fires {
		if c < restrAmountMinFires {
			t.Errorf("%s: %d times, want at least %d", restrAmountNames[i], c, restrAmountMinFires)
		}
	}
}

// TestRestrictedAmountMutation: counting the two-mana alternative as one mana
// (dirac.DebugRestrictedSource = 4), and floating what it makes as ordinary
// mana (5), must each diverge from gorge, so the pools exercise the amount and
// the restricted leftover.
func TestRestrictedAmountMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mut := range []uint8{4, 5} {
		old := dirac.DebugRestrictedSource
		dirac.DebugRestrictedSource = mut
		var fires [restrAmountN]int
		var first string
		_, divs := restrAmountRun(t, 20, &fires, func(format string, args ...any) {
			if first == "" {
				first = fmt.Sprintf(format, args...)
			}
		})
		dirac.DebugRestrictedSource = old
		if divs == 0 {
			t.Fatalf("mutation %d did not diverge", mut)
		}
		t.Logf("mutation %d: %d divergent games, first: %s", mut, divs, first)
	}
}
