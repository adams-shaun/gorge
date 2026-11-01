package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Earthbend lockstep (CR 701.66): constructed matchups
// over the generated tlaearth pool -- the lowered TLA earthbenders beside
// destroy, exile and bounce removal -- so the expected answer at every
// decision is gorge's.
//
// The 1024-seed sweep is split into four top-level shards so each test stays
// under the 60 s cap on its own; every shard covers all three matchups and
// therefore every carrier.

const (
	kearthbendSeeds  = 1024
	kearthbendShardN = 4
)

func ebDeck(es ...cardset.Entry) []cardset.Entry { return es }

var kearthbendDecks = [][2][]cardset.Entry{{
	ebDeck(
		cardset.Entry{Name: "Earthen Ally", Count: 1}, cardset.Entry{Name: "Rebellious Captives", Count: 1}, cardset.Entry{Name: "Haru, Hidden Talent", Count: 1},
		cardset.Entry{Name: "Badgermole", Count: 2}, cardset.Entry{Name: "Earth Kingdom General", Count: 2}, cardset.Entry{Name: "Earthbending Lesson", Count: 1},
		cardset.Entry{Name: "Rockalanche", Count: 1}, cardset.Entry{Name: "Earth Rumble", Count: 1}, cardset.Entry{Name: "The Boulder, Ready to Rumble", Count: 1},
		cardset.Entry{Name: "Grizzly Bears", Count: 9}, cardset.Entry{Name: "Hill Giant", Count: 5},
		cardset.Entry{Name: "Forest", Count: 7}, cardset.Entry{Name: "Plains", Count: 2}, cardset.Entry{Name: "Island", Count: 2},
		cardset.Entry{Name: "Swamp", Count: 2}, cardset.Entry{Name: "Mountain", Count: 2}),
	ebDeck(
		cardset.Entry{Name: "Lightning Bolt", Count: 4}, cardset.Entry{Name: "Doom Blade", Count: 4}, cardset.Entry{Name: "Murder", Count: 4},
		cardset.Entry{Name: "Swords to Plowshares", Count: 4}, cardset.Entry{Name: "Path to Exile", Count: 4},
		cardset.Entry{Name: "Mountain", Count: 7}, cardset.Entry{Name: "Swamp", Count: 7}, cardset.Entry{Name: "Plains", Count: 6}),
}, {
	ebDeck(
		cardset.Entry{Name: "Beifong's Bounty Hunters", Count: 3}, cardset.Entry{Name: "Dai Li Agents", Count: 2}, cardset.Entry{Name: "Earth Village Ruffians", Count: 3},
		cardset.Entry{Name: "Murder", Count: 5}, cardset.Entry{Name: "Doom Blade", Count: 5},
		cardset.Entry{Name: "Grizzly Bears", Count: 4}, cardset.Entry{Name: "Swamp", Count: 9}, cardset.Entry{Name: "Forest", Count: 9}),
	ebDeck(
		cardset.Entry{Name: "Swords to Plowshares", Count: 4}, cardset.Entry{Name: "Path to Exile", Count: 4}, cardset.Entry{Name: "Unsummon", Count: 2},
		cardset.Entry{Name: "Murder", Count: 4}, cardset.Entry{Name: "Serra Angel", Count: 2},
		cardset.Entry{Name: "Plains", Count: 10}, cardset.Entry{Name: "Island", Count: 6}, cardset.Entry{Name: "Swamp", Count: 8}),
}, {
	ebDeck(
		cardset.Entry{Name: "Bitter Work", Count: 1},
		cardset.Entry{Name: "Sandbenders' Storm", Count: 2}, cardset.Entry{Name: "The Boulder, Ready to Rumble", Count: 1},
		cardset.Entry{Name: "Grizzly Bears", Count: 12}, cardset.Entry{Name: "Hill Giant", Count: 5}, cardset.Entry{Name: "Forest", Count: 8}, cardset.Entry{Name: "Mountain", Count: 8},
		cardset.Entry{Name: "Plains", Count: 3}),
	ebDeck(
		cardset.Entry{Name: "Unsummon", Count: 2}, cardset.Entry{Name: "Swords to Plowshares", Count: 4}, cardset.Entry{Name: "Path to Exile", Count: 4},
		cardset.Entry{Name: "Murder", Count: 4}, cardset.Entry{Name: "Doom Blade", Count: 4},
		cardset.Entry{Name: "Serra Angel", Count: 2}, cardset.Entry{Name: "Plains", Count: 8}, cardset.Entry{Name: "Island", Count: 4},
		cardset.Entry{Name: "Swamp", Count: 8}),
}, {
	// Toph Hardheaded triggers on spell casts. The opponent has no removal,
	// so Toph survives; four Bolts leave a spell to cast after it, and a low
	// turn cap keeps the fixed MaxDelayed capacity in view.
	ebDeck(
		cardset.Entry{Name: "Toph, Hardheaded Teacher", Count: 1},
		cardset.Entry{Name: "Lightning Bolt", Count: 6},
		cardset.Entry{Name: "Forest", Count: 11}, cardset.Entry{Name: "Mountain", Count: 11},
		cardset.Entry{Name: "Plains", Count: 11}),
	ebDeck(
		cardset.Entry{Name: "Savannah Lions", Count: 4}, cardset.Entry{Name: "Serra Angel", Count: 4},
		cardset.Entry{Name: "Plains", Count: 32}),
}, {
	// Toph the First Metalbender triggers at the end step. The opponent has
	// no removal, so it survives to make one record a turn; the turn cap
	// keeps that under the fixed MaxDelayed capacity.
	ebDeck(
		cardset.Entry{Name: "Toph, the First Metalbender", Count: 1},
		cardset.Entry{Name: "Forest", Count: 13}, cardset.Entry{Name: "Mountain", Count: 13},
		cardset.Entry{Name: "Plains", Count: 13}),
	ebDeck(
		cardset.Entry{Name: "Savannah Lions", Count: 4}, cardset.Entry{Name: "Serra Angel", Count: 4},
		cardset.Entry{Name: "Plains", Count: 32}),
}}

var kearthbendTurns = []int32{40, 40, 40, 10, 8}

// kearthbendCarriers are every card in the tlaearth pool whose ability carries
// OpEarthbend. Each must resolve an earthbend at least once, not merely be
// cast: the kernel's resolution hook counts the resolution itself.
var kearthbendCarriers = []string{
	"Badgermole", "Beifong's Bounty Hunters", "Bitter Work", "Dai Li Agents",
	"Earth Kingdom General", "Earth Rumble",
	"Earth Village Ruffians", "Earthbending Lesson", "Earthen Ally",
	"Haru, Hidden Talent", "Rebellious Captives", "Rockalanche",
	"Sandbenders' Storm", "The Boulder, Ready to Rumble",
	"Toph, Hardheaded Teacher", "Toph, the First Metalbender",
}

// kearthbendWatch counts, from gorge's own state, the earthbend return
// registrations as the game goes: each new one is an earthbend that
// resolved; one that disappears is consumed, and the land's zone then says
// whether it returned from a graveyard or from exile (or bounced away).
type kearthbendWatch struct {
	prev []state.DelayedTrigger
}

func (w *kearthbendWatch) note(g *state.Game, played map[string]int) {
	var cur []state.DelayedTrigger
	for _, d := range g.Delayed {
		if d.Execute == "__kwEarthbendReturn" {
			cur = append(cur, d)
		}
	}
	if len(cur) > played["max earthbend records"] {
		played["max earthbend records"] = len(cur)
	}
	for _, d := range cur {
		found := false
		for _, p := range w.prev {
			found = found || p.ID == d.ID
		}
		if !found {
			played["earthbend"]++
		}
	}
	for _, p := range w.prev {
		found := false
		for _, d := range cur {
			found = found || p.ID == d.ID
		}
		if found {
			continue
		}
		if ob := g.Obj(p.Source); ob != nil {
			switch ob.Zone {
			case state.ZGraveyard:
				played["return from graveyard"]++
			case state.ZExile:
				played["return from exile"]++
			default:
				played["consumed elsewhere"]++
			}
		}
	}
	w.prev = cur
}

func kearthbendPolicy(seed uint64, played map[string]int) Policy {
	var inner Policy
	if seed%4 == 0 {
		inner = RandomPolicy(seed)
	} else {
		inner = eagerPolicy(seed, played)
	}
	w := &kearthbendWatch{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			w.note(c.G.E.G, played)
		}
		// The only two-mode choice in these decks is Sandbenders' Storm,
		// whose Choices$ put Earthbend second. Keep the lockstep on that
		// mode. (Dai Li Indoctrination is still a pool entry, but kgen
		// refuses its DiscardValid$ mode, so the generated card has no
		// abilities and it is not in these decks.)
		if len(c.D.Options) == 2 &&
			c.D.Options[0] == Pack(ActMode, RefNone, RefNone, 0) &&
			c.D.Options[1] == Pack(ActMode, RefNone, RefNone, 1) {
			return c.D.Options[1]
		}
		return inner.Pick(c)
	})
}

func kearthbendRun(t *testing.T, lo, hi uint64, played map[string]int, quiet bool) (diverged, stalls int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tlaearth")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	why := map[string]int{}
	for mi, md := range kearthbendDecks {
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
		prevResolved, prevFull := dirac.DebugEarthbendResolved, dirac.DebugEarthbendFull
		dirac.DebugEarthbendResolved = func(card dirac.CardID, live int) {
			if int(card) < len(tbl.Cards) {
				played["resolved:"+tbl.Cards[card].Name]++
			}
			if live > played["max earthbend records"] {
				played["max earthbend records"] = live
			}
		}
		dirac.DebugEarthbendFull = func(card dirac.CardID, eb, other int) {
			name := "?"
			if int(card) < len(tbl.Cards) {
				name = tbl.Cards[card].Name
			}
			played[fmt.Sprintf("stall card %s records earthbend=%d other=%d", name, eb, other)]++
		}
		mstalls := 0
		for seed := lo; seed <= hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kearthbend-a", "kearthbend-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := kearthbendPolicy(seed, played)
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: kearthbendTurns[mi]})
			CloseSide(k)
			games++
			if len(res.Stall) > len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				mstalls++
				why[res.Stall]++
				if mstalls <= 3 && !quiet {
					t.Logf("matchup %d seed %d: stall %s at turn %d after %d decisions", mi, seed, res.Stall, res.Turns, res.Decisions)
				}
			}
			if d := res.Div; d != nil {
				diverged++
				if diverged <= 2 && !quiet {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
		dirac.DebugEarthbendResolved, dirac.DebugEarthbendFull = prevResolved, prevFull
		t.Logf("matchup %d seeds %d-%d: %d games, %d stalls", mi, lo, hi, games, mstalls)
	}
	t.Logf("seeds %d-%d: %d games, %d diverged, %d kernel stalls %v, plays %v", lo, hi, games, diverged, stalls, why, played)
	return diverged, stalls
}

func kearthbendShard(t *testing.T, lo, hi uint64) {
	played := map[string]int{}
	diverged, stalls := kearthbendRun(t, lo, hi, played, false)
	if diverged != 0 {
		t.Errorf("%d games diverged", diverged)
	}
	if stalls != 0 {
		t.Errorf("%d kernel capacity stalls, want zero", stalls)
	}
	for _, n := range kearthbendCarriers {
		if played["resolved:"+n] == 0 {
			t.Errorf("carrier %s never resolved an earthbend (cast count %d)", n, played[n])
		}
	}
	for _, n := range []string{"earthbend", "return from graveyard", "return from exile"} {
		if played[n] == 0 {
			t.Errorf("%s never happened", n)
		}
	}
}

func TestEarthbendLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	kearthbendShard(t, 1, kearthbendSeeds/kearthbendShardN)
}

func TestEarthbendLockstepShard2(t *testing.T) {
	if isolate(t) {
		return
	}

	kearthbendShard(t, kearthbendSeeds/kearthbendShardN+1, 2*kearthbendSeeds/kearthbendShardN)
}

func TestEarthbendLockstepShard3(t *testing.T) {
	if isolate(t) {
		return
	}

	kearthbendShard(t, 2*kearthbendSeeds/kearthbendShardN+1, 3*kearthbendSeeds/kearthbendShardN)
}

func TestEarthbendLockstepShard4(t *testing.T) {
	if isolate(t) {
		return
	}

	kearthbendShard(t, 3*kearthbendSeeds/kearthbendShardN+1, kearthbendSeeds)
}

// TestEarthbendMutation: with the return registration skipped the lockstep
// must diverge.
func TestEarthbendMutation(t *testing.T) {
	old := dirac.DebugEarthbendNoReturn
	dirac.DebugEarthbendNoReturn = true
	defer func() { dirac.DebugEarthbendNoReturn = old }()
	if diverged, _ := kearthbendRun(t, 1, 24, map[string]int{}, true); diverged == 0 {
		t.Fatal("DebugEarthbendNoReturn did not diverge the lockstep")
	}
}
