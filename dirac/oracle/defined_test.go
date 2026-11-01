package oracle

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated defined
// pool for the other:Defined$ grammar sweep. Sub-class C (this file's first
// matchups): ConditionDefined$ Sacrificed + ConditionPresent$ on Hellish
// Sideswipe and Splitting the Powerstone, with Vehicles, legendary and
// nonlegendary artifacts and Powerstone tokens to sacrifice.

// kdSacSeen counts the Sacrificed condition reads of the matchup being run.
var kdSacSeen map[string]int

func kdE(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }

var kdefinedSacDecks = [2][]cardset.Entry{
	{
		kdE("Splitting the Powerstone", 4), kdE("Hellish Sideswipe", 4), kdE("Smuggler's Copter", 3), kdE("Renegade Freighter", 2),
		kdE("Stone of Erech", 2), kdE("Horn of Gondor", 2), kdE("Feldon's Cane", 3),
		kdE("Walking Corpse", 3), kdE("Coral Merfolk", 3), kdE("Island", 7), kdE("Swamp", 7),
	},
	{
		kdE("Hellish Sideswipe", 4), kdE("Splitting the Powerstone", 3), kdE("Sky Skiff", 3), kdE("Smuggler's Copter", 2),
		kdE("Hall of Triumph", 3), kdE("Sword of the Chosen", 3), kdE("Stone of Erech", 1),
		kdE("Walking Corpse", 4), kdE("Coral Merfolk", 3), kdE("Island", 7), kdE("Swamp", 7),
	},
}

// kdefinedSplitDecks: Splitting the Powerstone alone (no Hellish Sideswipe)
// over legendary and nonlegendary artifacts, so its three branches (the
// sacrificed card legendary, not, a ceased Powerstone token) are its own.
var kdefinedSplitDecks = [2][]cardset.Entry{
	{
		kdE("Splitting the Powerstone", 6), kdE("Stone of Erech", 3), kdE("Horn of Gondor", 3), kdE("Feldon's Cane", 3),
		kdE("Walking Corpse", 3), kdE("Coral Merfolk", 3), kdE("Island", 10), kdE("Swamp", 9),
	},
	{
		kdE("Splitting the Powerstone", 5), kdE("Hall of Triumph", 3), kdE("Sword of the Chosen", 3), kdE("Feldon's Cane", 3),
		kdE("Walking Corpse", 4), kdE("Coral Merfolk", 4), kdE("Island", 9), kdE("Swamp", 9),
	},
}

// kdefinedSaprolingDecks: Thallid Omnivore and Falkenrath Torturer
// sacrificing Jade Mage's Saproling tokens (a token that DOES match
// Card.Saproling after it ceased), other creatures and Humans.
var kdefinedSaprolingDecks = [2][]cardset.Entry{
	{
		kdE("Thallid Omnivore", 4), kdE("Jade Mage", 5), kdE("Falkenrath Torturer", 3), kdE("Walking Corpse", 4),
		kdE("Giant Spider", 3), kdE("Grizzly Bears", 3), kdE("Swamp", 9), kdE("Forest", 9),
	},
	{
		kdE("Thallid Omnivore", 3), kdE("Jade Mage", 6), kdE("Falkenrath Torturer", 4), kdE("Walking Corpse", 3),
		kdE("Giant Spider", 3), kdE("Grizzly Bears", 3), kdE("Swamp", 9), kdE("Forest", 9),
	},
}

// kdefinedCragflameDecks: Splitting the Powerstone over Mabel's legendary
// Cragflame token (a token that DOES match Card.Legendary after it ceased),
// nonlegendary artifacts and Powerstone tokens.
var kdefinedCragflameDecks = [2][]cardset.Entry{
	{
		kdE("Mabel, Heir to Cragflame", 5), kdE("Splitting the Powerstone", 5), kdE("Feldon's Cane", 3), kdE("Horn of Gondor", 2),
		kdE("Savannah Lions", 3), kdE("Hill Giant", 3), kdE("Gray Ogre", 3), kdE("Mountain", 6), kdE("Plains", 6), kdE("Island", 4),
	},
	{
		kdE("Mabel, Heir to Cragflame", 4), kdE("Splitting the Powerstone", 6), kdE("Feldon's Cane", 3), kdE("Stone of Erech", 2),
		kdE("Savannah Lions", 3), kdE("Hill Giant", 3), kdE("Gray Ogre", 3), kdE("Mountain", 6), kdE("Plains", 6), kdE("Island", 4),
	},
}

// kdefinedHellDecks: Hellish Sideswipe alone, over Vehicles, other
// artifacts and creatures (no tokens).
var kdefinedHellDecks = [2][]cardset.Entry{
	{
		kdE("Hellish Sideswipe", 6), kdE("Smuggler's Copter", 4), kdE("Renegade Freighter", 3), kdE("Sky Skiff", 3),
		kdE("Feldon's Cane", 3), kdE("Walking Corpse", 3), kdE("Coral Merfolk", 3), kdE("Island", 7), kdE("Swamp", 8),
	},
	{
		kdE("Hellish Sideswipe", 5), kdE("Smuggler's Copter", 3), kdE("Sky Skiff", 3), kdE("Stone of Erech", 3),
		kdE("Walking Corpse", 4), kdE("Coral Merfolk", 4), kdE("Island", 8), kdE("Swamp", 10),
	},
}

// Sub-class B (Player.<q> / Opponent.<q> sets): Feed the Infection (corrupted
// opponents) and Fade from History over infect and toxic creatures, and
// Depopulate (multicoloured creature controllers) and Outpace Oblivion
// (players without max speed) over Boros creatures and Shock.
var kdefinedPoisonDecks = [2][]cardset.Entry{
	{
		kdE("Feed the Infection", 4), kdE("Fade from History", 3), kdE("Glistener Elf", 4), kdE("Bilious Skulldweller", 4),
		kdE("Giant Spider", 3), kdE("Feldon's Cane", 6), kdE("Forest", 8), kdE("Swamp", 8),
	},
	{
		kdE("Feed the Infection", 3), kdE("Fade from History", 4), kdE("Glistener Elf", 4), kdE("Bilious Skulldweller", 4),
		kdE("Giant Spider", 3), kdE("Feldon's Cane", 5), kdE("Grizzly Bears", 1), kdE("Forest", 8), kdE("Swamp", 8),
	},
}

var kdefinedFadeDecks = [2][]cardset.Entry{
	{
		kdE("Fade from History", 2), kdE("Hall of Triumph", 6), kdE("Sword of the Chosen", 6), kdE("Feldon's Cane", 4),
		kdE("Giant Spider", 2), kdE("Forest", 20),
	},
	{
		kdE("Fade from History", 2), kdE("Hall of Triumph", 6), kdE("Sword of the Chosen", 6), kdE("Feldon's Cane", 4),
		kdE("Giant Spider", 2), kdE("Forest", 20),
	},
}

var kdefinedBorosDecks = [2][]cardset.Entry{
	{
		kdE("Depopulate", 4), kdE("Outpace Oblivion", 4), kdE("Shock", 4), kdE("Boros Swiftblade", 4), kdE("Skyknight Legionnaire", 4),
		kdE("Starting Column", 2), kdE("Hill Giant", 3), kdE("Mountain", 8), kdE("Plains", 7),
	},
	{
		kdE("Depopulate", 3), kdE("Outpace Oblivion", 4), kdE("Shock", 4), kdE("Boros Swiftblade", 4), kdE("Skyknight Legionnaire", 4),
		kdE("Starting Column", 2), kdE("Gray Ogre", 3), kdE("Mountain", 8), kdE("Plains", 8),
	},
}

// Sub-class A (Defined$ Valid <filter> mass subjects): Double Trouble,
// Unnatural Growth, Roar of Endless Song (a Double per creature), Bristly
// Bill and Kalonian Hydra (double the +1/+1 counters on each creature),
// Summon: Knights of Round (a counter on each other creature) and Voltstorm
// Angel (a constant pump of each other creature).
var kdefinedSweepDecks = [2][]cardset.Entry{
	{
		kdE("Bristly Bill, Spine Sower", 4), kdE("Kalonian Hydra", 3), kdE("Unnatural Growth", 2), kdE("Double Trouble", 3),
		kdE("Roar of Endless Song", 3), kdE("Grizzly Bears", 4), kdE("Giant Spider", 2), kdE("Hill Giant", 2), kdE("Gray Ogre", 2),
		kdE("Forest", 9), kdE("Mountain", 3), kdE("Island", 3),
	},
	{
		kdE("Bristly Bill, Spine Sower", 3), kdE("Kalonian Hydra", 4), kdE("Unnatural Growth", 3), kdE("Double Trouble", 2),
		kdE("Roar of Endless Song", 4), kdE("Grizzly Bears", 3), kdE("Giant Spider", 3), kdE("Hill Giant", 2), kdE("Gray Ogre", 1),
		kdE("Forest", 9), kdE("Mountain", 3), kdE("Island", 3),
	},
}

var kdefinedKnightDecks = [2][]cardset.Entry{
	{
		kdE("Summon: Knights of Round", 8), kdE("Voltstorm Angel", 2), kdE("Bristly Bill, Spine Sower", 2), kdE("Grizzly Bears", 4),
		kdE("Savannah Lions", 4), kdE("Plains", 12), kdE("Forest", 8),
	},
	{
		kdE("Summon: Knights of Round", 8), kdE("Voltstorm Angel", 2), kdE("Bristly Bill, Spine Sower", 2), kdE("Grizzly Bears", 4),
		kdE("Savannah Lions", 4), kdE("Plains", 12), kdE("Forest", 8),
	},
}

// kdefinedMatchup is one lockstep matchup, its seed count and the minimum
// occurrence of each KdefinedHits branch it must reach (-1: not required).
type kdefinedMatchup struct {
	name  string
	decks [2][]cardset.Entry
	seeds uint64
	min   [dirac.DefinedNumHits]int
	sac   map[string]int // Sacrificed condition reads: "<carrier> <card|token> <matched>" -> minimum
	turns int            // MaxTurns (0: 24)
}

// kdMin is a minimum-occurrence table: index -> count, the rest 0.
func kdMin(m map[int]int) (out [dirac.DefinedNumHits]int) {
	for i, v := range m {
		out[i] = v
	}
	return
}

// kdPred is the KdefinedHits index of predicate kind (0 controlsCreature, 1
// MaxSpeed, 2 IsCorrupted, 3 Fade's artifact-or-enchantment) for the controller (seat 0) or opponent (seat 1),
// kept (0) or dropped (1).
func kdPred(kind, seat, dropped int) int { return dirac.KdPredBase + kind*4 + seat*2 + dropped }

var kdefinedSacMatchups = []kdefinedMatchup{
	{name: "sacrifice mixed", decks: kdefinedSacDecks, seeds: 256, sac: map[string]int{
		"Splitting the Powerstone card true": 10, "Splitting the Powerstone card false": 10, "Splitting the Powerstone token false": 10,
		"Hellish Sideswipe card true": 10, "Hellish Sideswipe card false": 10, "Hellish Sideswipe token false": 10,
	}},
	{name: "splitting", decks: kdefinedSplitDecks, seeds: 384, sac: map[string]int{
		"Splitting the Powerstone card true": 10, "Splitting the Powerstone card false": 10, "Splitting the Powerstone token false": 10,
	}},
	{name: "hellish", decks: kdefinedHellDecks, seeds: 384, sac: map[string]int{
		"Hellish Sideswipe card true": 10, "Hellish Sideswipe card false": 10,
	}},
}

// kdefinedTokenMatchups: a ceased token whose card MATCHES the filter still
// counts (gorge keeps the ZCeased object's face): Saproling for Omnivore,
// Cragflame for Splitting.
var kdefinedTokenMatchups = []kdefinedMatchup{
	{name: "saproling tokens", decks: kdefinedSaprolingDecks, seeds: 384, sac: map[string]int{
		"Thallid Omnivore token true": 10, "Thallid Omnivore card false": 10,
		"Falkenrath Torturer card true": 10, "Falkenrath Torturer card false": 10, "Falkenrath Torturer token false": 10,
	}},
	{name: "cragflame", decks: kdefinedCragflameDecks, seeds: 384, sac: map[string]int{
		"Splitting the Powerstone token true": 10, "Splitting the Powerstone token false": 10, "Splitting the Powerstone card false": 10,
	}},
}

var kdefinedSweepMatchups = []kdefinedMatchup{
	{name: "doubles", decks: kdefinedSweepDecks, seeds: 512, min: kdMin(map[int]int{
		dirac.KdPumpEach: 10, dirac.KdPumpEachDiff: 10, dirac.KdPumpEmpty: 10, dirac.KdDoubleSweep: 10,
	})},
	{name: "knights", decks: kdefinedKnightDecks, seeds: 512, min: kdMin(map[int]int{
		dirac.KdPutSweep: 10, dirac.KdPumpSweep: 10, dirac.KdDoubleSweep: 10,
	}), turns: 40},
}

var kdefinedPlayerMatchups = []kdefinedMatchup{
	{name: "poison", decks: kdefinedPoisonDecks, seeds: 512, min: kdMin(map[int]int{
		kdPred(2, 1, 0): 10, kdPred(2, 1, 1): 10,
	})},
	{name: "fade", decks: kdefinedFadeDecks, seeds: 384, min: kdMin(map[int]int{
		kdPred(3, 0, 0): 10, kdPred(3, 0, 1): 10, kdPred(3, 1, 0): 10, kdPred(3, 1, 1): 10,
	})},
	{name: "boros", decks: kdefinedBorosDecks, seeds: 512, min: kdMin(map[int]int{
		kdPred(0, 0, 0): 10, kdPred(0, 0, 1): 10, kdPred(0, 1, 0): 10, kdPred(0, 1, 1): 10,
		kdPred(1, 0, 0): 10, kdPred(1, 0, 1): 10, kdPred(1, 1, 0): 10, kdPred(1, 1, 1): 10,
	})},
}

func kdefinedSetup(t *testing.T, md [2][]cardset.Entry) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("defined")
	if err != nil {
		t.Fatal(err)
	}
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// kdefinedRun plays seeds 1..n of one matchup; it returns the games played
// and whether any diverged (logging each divergence when report is set).
func kdefinedRun(t *testing.T, mi int, md [2][]cardset.Entry, n uint64, turns int, report bool) (games int, diverged bool) {
	t.Helper()
	if turns == 0 {
		turns = 24
	}
	reg, set, tbl, decks := kdefinedSetup(t, md)
	if report {
		dirac.DebugDefinedSac = func(card dirac.CardID, token, matched bool) {
			kind := "card"
			if token {
				kind = "token"
			}
			kdSacSeen[fmt.Sprintf("%s %s %v", tbl.Cards[card].Name, kind, matched)]++
		}
		defer func() { dirac.DebugDefinedSac = nil }()
	}
	for seed := uint64(1); seed <= n; seed++ {
		if v := os.Getenv("KD_SEED"); v != "" && v != strconv.FormatUint(seed, 10) {
			continue
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0 != (os.Getenv("KD_FLIP") != "")}
		m, err := NewMatchup(reg, [2]string{"kdefined-a", "kdefined-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := Policy(HeuristicPolicy(seed))
		if seed%3 == 0 {
			base = RandomPolicy(seed)
		}
		names := map[int]string{}
		var gs *GorgeSide
		probe := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			if c.G != nil && report && os.Getenv("KD_DUMP") != "" {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
							names[int(o.Obj)] = ob.Face().Name
						}
					}
				}
			}
			return base.Pick(c)
		})
		res := Lockstep(m, seed, k, probe, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: int32(turns)})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				if gs != nil && os.Getenv("KD_EV") != "" {
					kcastDump(t, gs)
					for _, ev := range gs.E.L.Events {
						if ev.Kind == events.CounterChange {
							t.Logf("  counterchange obj %d kind %q amt %d", ev.Obj, ev.Counter, ev.Amount)
						}
					}
				}
				if gs != nil && os.Getenv("KD_DUMP") != "" {
					for i := range gs.E.G.Objs {
						ob := &gs.E.G.Objs[i]
						if ob.Face() != nil && (ob.Zone == state.ZHand || ob.Zone == state.ZBattlefield) {
							t.Logf("  obj %d %q zone %v ctrl %d tapped %v", i+1, ob.Face().Name, ob.Zone, ob.Controller, ob.Tapped)
						}
					}
				}
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			} else {
				t.Logf("mutant diverged at seed %d: %s", seed, d)
				return
			}
		}
	}
	return
}

// kdefinedLockstep plays each matchup and checks its branch minima.
func kdefinedLockstep(t *testing.T, ms []kdefinedMatchup) {
	games := 0
	for mi, mu := range ms {
		var h [dirac.DefinedNumHits]uint32
		dirac.DebugDefinedHits = &h
		kdSacSeen = map[string]int{}
		g, _ := kdefinedRun(t, mi, mu.decks, mu.seeds, mu.turns, true)
		dirac.DebugDefinedHits = nil
		games += g
		if len(mu.sac) > 0 {
			t.Logf("%s: sacrificed reads %v", mu.name, kdSacSeen)
		}
		for key, want := range mu.sac {
			if kdSacSeen[key] < want {
				t.Errorf("%s: %q observed %d times, want >= %d", mu.name, key, kdSacSeen[key], want)
			}
		}
		t.Logf("%s: %d games, branch hits %v", mu.name, g, h)
		for i, v := range h {
			if int(v) < mu.min[i] {
				t.Errorf("%s: branch %d observed %d times, want >= %d", mu.name, i, v, mu.min[i])
			}
		}
	}
	t.Logf("%d games", games)
}

// TestDefinedLockstep: ConditionDefined$ Sacrificed (hits 0..2:
// the card matched, did not match, a ceased token).
func TestDefinedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	kdefinedLockstep(t, kdefinedSacMatchups)
}

// TestDefinedTokensLockstep: the Sacrificed condition over ceased tokens
// whose card matches the filter (Saproling, Cragflame).
func TestDefinedTokensLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	kdefinedLockstep(t, kdefinedTokenMatchups)
}

// TestDefinedSweepsLockstep: Defined$ Valid mass subjects (hits per op:
// the sweep over two or more objects, creatures of different powers, none).
func TestDefinedSweepsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	kdefinedLockstep(t, kdefinedSweepMatchups)
}

// TestDefinedPlayersLockstep: Player.<q> / Opponent.<q> sets (hits per
// predicate kind: controlsCreature, MaxSpeed, IsCorrupted; controller and
// opponent kept / dropped).
func TestDefinedPlayersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	kdefinedLockstep(t, kdefinedPlayerMatchups)
}

// TestDefinedMutation: each lifted behaviour broken must diverge.
func TestDefinedMutation(t *testing.T) {
	for _, c := range []struct {
		name string
		md   [2][]cardset.Entry
		set  func(bool)
	}{
		{"sacrificed group empty", kdefinedHellDecks, func(on bool) { dirac.DebugSacrificedNone = on }},
		{"ceased token skipped (saproling)", kdefinedSaprolingDecks, func(on bool) { dirac.DebugSacrificedTokenSkip = on }},
		{"ceased token skipped (cragflame)", kdefinedCragflameDecks, func(on bool) { dirac.DebugSacrificedTokenSkip = on }},
		{"pump doubles by the first creature's power", kdefinedSweepDecks, func(on bool) { dirac.DebugPumpEachFirst = on }},
		{"player predicate ignored (boros)", kdefinedBorosDecks, func(on bool) { dirac.DebugPlayerPredNone = on }},
		{"player predicate ignored (poison)", kdefinedPoisonDecks, func(on bool) { dirac.DebugPlayerPredNone = on }},
		{"player predicate ignored (fade)", kdefinedFadeDecks, func(on bool) { dirac.DebugPlayerPredNone = on }},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.set(true)
			defer c.set(false)
			if _, div := kdefinedRun(t, 0, c.md, 128, 0, false); !div {
				t.Fatalf("%s: no divergence in 128 seeds", c.name)
			}
		})
	}
}
