package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A dmgsrcpl carrier's trigger damage (Champion of the Path, Pyrotechnic
// Performer, Ingris Stingerquill) matched by two Fated Firepowers is a pure
// plus-N competition at OpDealDamageFrom's player mode: gorge collects both
// matching damage replacements and asks the affected player their CR 616.1
// order even though the additions commute, so the kernel poses the same
// no-op ask before the pass deals (dirac/repl_plus.go
// replPlusSrcPass). Deck B is the dmgsrcpl deck 2, which has no Fated
// Firepower, so only deck A's trigger damage competes. Deck A carries no
// Shock: a resolving Shock's damage would pose the already-wired damageTo
// ask (dirac/resolve.go) while the stack is non-empty too, and the tally
// keys the ask to the resolving triggers by the same stack test.
var kreplPlusSitesDecks = [2][]cardset.Entry{
	{
		{Name: "Champion of the Path", Count: 4}, {Name: "Pyrotechnic Performer", Count: 3}, {Name: "Ingris Stingerquill", Count: 3},
		{Name: "Child of Night", Count: 3}, {Name: "Vampire Nighthawk", Count: 3},
		{Name: "Priest of Urabrask", Count: 3}, {Name: "Fated Firepower", Count: 2},
		// no Plague Stinger: the pool's interaction check bars infect beside
		// a damage replacement order ask (gorge orders the conversion
		// differently, an unmodelled interaction); two more basics instead
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 9},
	},
	kdmgsrcplDecks[1],
}

// krpsCounts are what gorge's event log showed for one game: the carriers'
// trigger resolutions, and those whose trigger's controller had two or more
// Fated Firepowers on the battlefield at the resolve (the competition's
// precondition).
type krpsCounts struct {
	champion, pyro, ingris, competing int
}

// krpsScan counts the carriers' trigger resolutions and those under two live
// Firepowers. The stack is replayed from the log as kdmgsrcplScan replays
// it: a TriggerPush or a PutOnStack pushes, a Resolve pops.
func krpsScan(g *GorgeSide) krpsCounts {
	var c krpsCounts
	if g == nil {
		return c
	}
	type frame struct {
		src    string         // the carrier whose trigger this is ("" otherwise)
		player state.PlayerID // the trigger's controller (TriggerPush's Player)
	}
	var stack []frame
	fp := [2]int{} // Fated Firepower permanents on the battlefield, per controller
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.MoveZone:
			if chName(g.E.G, ev.Obj) != "Fated Firepower" {
				continue
			}
			if ev.To == state.ZBattlefield {
				fp[ev.Player]++
			}
			if ev.From == state.ZBattlefield && ev.To != state.ZBattlefield {
				fp[ev.Player]--
			}
		case events.TriggerPush:
			stack = append(stack, frame{src: chName(g.E.G, ev.Obj), player: ev.Player})
		case events.PutOnStack:
			stack = append(stack, frame{})
		case events.Resolve:
			if len(stack) == 0 {
				continue
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch f.src {
			case "Champion of the Path":
				c.champion++
			case "Pyrotechnic Performer":
				c.pyro++
			case "Ingris Stingerquill":
				c.ingris++
			default:
				continue
			}
			if f.player < 2 && fp[f.player] >= 2 {
				c.competing++
			}
		}
	}
	return c
}

// kreplPlusSitesRun plays seeds 1..seeds with deck A in both seat orders and
// both autopay surfaces, and returns the tallies, the first divergence (""
// if none) and the kernel stalls.
func kreplPlusSitesRun(t *testing.T, seeds uint64) (tal krpsTally, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dmgsrcpl")
	if err != nil {
		t.Fatal(err)
	}
	var deckCards [2][]*cards.Card
	for d, deck := range kreplPlusSitesDecks {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				deckCards[d] = append(deckCards[d], c)
			}
		}
		if len(deckCards[d]) != 40 {
			t.Fatalf("deck %d: %d cards", d, len(deckCards[d]))
		}
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		order := [2]int{0, 1}
		if seed/2%2 == 1 {
			order = [2]int{1, 0} // both seat orders
		}
		var decks [2][]*cards.Card
		var entries [2][]cardset.Entry
		for i, o := range order {
			decks[i], entries[i] = deckCards[o], kreplPlusSitesDecks[o]
		}
		tbl, _, _, err := set.Matchup(entries[0], entries[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed/4%2 == 1}
		m, err := NewMatchup(reg, [2]string{"krps-a", "krps-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		ord := 0
		pol := kdmgsrcplPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30,
			Check: func(g *GorgeSide, _ Side) *Divergence {
				// gorge's replacement-order ask with a non-empty stack: the
				// carrier triggers are the only damage under one controller
				// with two replacement candidates in these decks (deck A's
				// only replacement effects are the two Fated Firepowers)
				if d := g.E.Pending(); d != nil && d.Kind == decision.KReplacement && len(g.E.G.Stack) > 0 {
					ord++
				}
				return nil
			}})
		CloseSide(k)
		tal.games++
		tal.order += ord
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := krpsScan(gs)
		tal.champion += c.champion
		tal.pyro += c.pyro
		tal.ingris += c.ingris
		tal.competing += c.competing
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tal, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tal, "", stalls
}

// krpsTally is the summed per-game counts over a run, with the resolution
// order asks the kernel posed while gorge's stack was non-empty.
type krpsTally struct {
	games                                    int
	champion, pyro, ingris, competing, order int
}

// TestReplPlusSitesDmgSrcPlLockstep: a carrier trigger's damage with two
// Fated Firepowers live poses gorge's no-op replacement-order ask (the
// no-divergence at that decision is the proof) and the summed damage lands.
func TestReplPlusSitesDmgSrcPlLockstep(t *testing.T) {
	t.Parallel()
	tal, div, stalls := kreplPlusSitesRun(t, 200)
	t.Logf("%d games; Champion triggers %d; Pyrotechnic triggers %d; Ingris triggers %d; resolved with two Firepowers live %d; resolution order asks %d; stalls %v",
		tal.games, tal.champion, tal.pyro, tal.ingris, tal.competing, tal.order, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tal.competing < 1 {
		t.Errorf("carrier trigger resolves with two Firepowers live %d, want at least 1", tal.competing)
	}
	if tal.order < 1 {
		t.Errorf("resolution order asks %d, want at least 1 (the ask the competition poses)", tal.order)
	}
	if tal.champion < 1 || tal.pyro < 1 || tal.ingris < 1 {
		t.Errorf("Champion %d, Pyrotechnic %d, Ingris %d triggers, want at least 1 each", tal.champion, tal.pyro, tal.ingris)
	}
}
