package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Waterbend lockstep (CR 701.67): constructed matchups
// over the generated tlawater pool -- the lowered waterbenders beside mana
// creatures, artifacts, removal, basics and vanilla creatures -- so the
// expected answer at every decision is gorge's. Every lowered carrier must
// activate at least once; the run must see an activation with zero picks, one
// with picks, and one that taps a mana creature for waterbend (proving the
// hold).

const kwaterbendSeeds = 1024

func wbDeck(es ...cardset.Entry) []cardset.Entry { return es }

var kwaterbendDecks = [][2][]cardset.Entry{{
	wbDeck(
		cardset.Entry{Name: "Aang's Iceberg", Count: 1}, cardset.Entry{Name: "Flexible Waterbender", Count: 1},
		cardset.Entry{Name: "Foggy Swamp Vinebender", Count: 1}, cardset.Entry{Name: "Geyser Leaper", Count: 1},
		cardset.Entry{Name: "Invasion Submersible", Count: 1}, cardset.Entry{Name: "Katara, Bending Prodigy", Count: 1},
		cardset.Entry{Name: "Water Tribe Rallier", Count: 1}, cardset.Entry{Name: "Waterbender Ascension", Count: 1},
		cardset.Entry{Name: "Ruthless Waterbender", Count: 1},
		cardset.Entry{Name: "Llanowar Elves", Count: 4}, cardset.Entry{Name: "Grizzly Bears", Count: 6},
		cardset.Entry{Name: "Hill Giant", Count: 2},
		cardset.Entry{Name: "Forest", Count: 6}, cardset.Entry{Name: "Island", Count: 5},
		cardset.Entry{Name: "Plains", Count: 3}, cardset.Entry{Name: "Swamp", Count: 3},
		cardset.Entry{Name: "Mountain", Count: 2}),
	wbDeck(
		cardset.Entry{Name: "Lightning Bolt", Count: 4}, cardset.Entry{Name: "Doom Blade", Count: 4},
		cardset.Entry{Name: "Murder", Count: 4}, cardset.Entry{Name: "Unsummon", Count: 2},
		cardset.Entry{Name: "Grizzly Bears", Count: 4}, cardset.Entry{Name: "Walking Corpse", Count: 4},
		cardset.Entry{Name: "Serra Angel", Count: 2},
		cardset.Entry{Name: "Mountain", Count: 4}, cardset.Entry{Name: "Swamp", Count: 5},
		cardset.Entry{Name: "Plains", Count: 4}, cardset.Entry{Name: "Island", Count: 3}),
}, {
	wbDeck(
		cardset.Entry{Name: "Aang's Iceberg", Count: 1}, cardset.Entry{Name: "Flexible Waterbender", Count: 1},
		cardset.Entry{Name: "Foggy Swamp Vinebender", Count: 1}, cardset.Entry{Name: "Geyser Leaper", Count: 1},
		cardset.Entry{Name: "Invasion Submersible", Count: 1}, cardset.Entry{Name: "Katara, Bending Prodigy", Count: 1},
		cardset.Entry{Name: "Water Tribe Rallier", Count: 1}, cardset.Entry{Name: "Waterbender Ascension", Count: 1},
		cardset.Entry{Name: "Ruthless Waterbender", Count: 1},
		cardset.Entry{Name: "Llanowar Elves", Count: 4}, cardset.Entry{Name: "Mind Stone", Count: 2},
		cardset.Entry{Name: "Grizzly Bears", Count: 6},
		cardset.Entry{Name: "Forest", Count: 8}, cardset.Entry{Name: "Island", Count: 6},
		cardset.Entry{Name: "Plains", Count: 5}),
	wbDeck(
		cardset.Entry{Name: "Grizzly Bears", Count: 10}, cardset.Entry{Name: "Hill Giant", Count: 5},
		cardset.Entry{Name: "Serra Angel", Count: 2},
		cardset.Entry{Name: "Forest", Count: 10}, cardset.Entry{Name: "Plains", Count: 13}),
}, {
	wbDeck(
		cardset.Entry{Name: "Aang's Iceberg", Count: 1}, cardset.Entry{Name: "Flexible Waterbender", Count: 1},
		cardset.Entry{Name: "Foggy Swamp Vinebender", Count: 1}, cardset.Entry{Name: "Geyser Leaper", Count: 1},
		cardset.Entry{Name: "Invasion Submersible", Count: 1}, cardset.Entry{Name: "Katara, Bending Prodigy", Count: 1},
		cardset.Entry{Name: "Water Tribe Rallier", Count: 1}, cardset.Entry{Name: "Waterbender Ascension", Count: 1},
		cardset.Entry{Name: "Ruthless Waterbender", Count: 1},
		cardset.Entry{Name: "Invasion Submersible", Count: 2}, cardset.Entry{Name: "Flexible Waterbender", Count: 1},
		cardset.Entry{Name: "Llanowar Elves", Count: 4}, cardset.Entry{Name: "Grizzly Bears", Count: 4},
		cardset.Entry{Name: "Forest", Count: 11}, cardset.Entry{Name: "Island", Count: 9}),
	wbDeck(
		cardset.Entry{Name: "Doom Blade", Count: 4}, cardset.Entry{Name: "Murder", Count: 4},
		cardset.Entry{Name: "Lightning Bolt", Count: 4}, cardset.Entry{Name: "Serra Angel", Count: 4},
		cardset.Entry{Name: "Grizzly Bears", Count: 4},
		cardset.Entry{Name: "Swamp", Count: 8}, cardset.Entry{Name: "Mountain", Count: 6},
		cardset.Entry{Name: "Plains", Count: 6}),
}}

var kwaterbendCarriers = []string{
	"Aang's Iceberg", "Flexible Waterbender", "Foggy Swamp Vinebender",
	"Geyser Leaper", "Invasion Submersible", "Katara, Bending Prodigy",
	"Water Tribe Rallier", "Waterbender Ascension", "Ruthless Waterbender",
}

func kwaterbendPolicy(seed uint64, played map[string]int) Policy {
	var inner Policy
	if seed%4 == 0 {
		inner = RandomPolicy(seed)
	} else {
		inner = eagerPolicy(seed, played)
	}
	// Some seeds decline every second waterbend pick (legal: Min 0), the
	// rest take every pick, so both the aborting and the paying paths run.
	declines := seed%3 == 1
	asked := 0
	cp, haveWB := chooseCode("waterbend_generic")
	return PolicyFunc(func(c *PickCtx) Canon {
		if haveWB {
			var pick Canon
			found := false
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose+CanonKind(cp) {
					pick, found = a, true
					break
				}
			}
			if found {
				asked++
				if declines && asked%2 == 0 {
					for _, a := range c.D.Options {
						if a.Kind() == ActDone {
							played["waterbend declines"]++
							return a
						}
					}
				}
				return pick
			}
		}
		return inner.Pick(c)
	})
}

func kwaterbendRun(t *testing.T, lo, hi uint64, played map[string]int, quiet bool) (diverged, stalls int, stats Stats) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tlawater")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	why := map[string]int{}
	for seed := lo; seed <= hi; seed++ {
		mi := int((seed - lo) % uint64(len(kwaterbendDecks)))
		md := kwaterbendDecks[mi]
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
		prev := dirac.DebugWaterbendPaid
		dirac.DebugWaterbendPaid = func(card dirac.CardID, picks, manaPicks int) {
			name := "?"
			if int(card) < len(tbl.Cards) {
				name = tbl.Cards[card].Name
			}
			played["activated:"+name]++
			if picks == 0 {
				played["zero-pick activations"]++
			} else {
				played["picked activations"]++
			}
			if manaPicks > 0 {
				played["picked mana creature"]++
			}
			played["picks"] += picks
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwaterbend-a", "kwaterbend-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := kwaterbendPolicy(seed, played)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		dirac.DebugWaterbendPaid = prev
		games++
		stats.Unknown = addCounts(stats.Unknown, res.Stats.Unknown)
		stats.Unpriced = addCounts(stats.Unpriced, res.Stats.Unpriced)
		stats.UnpricedPlays = addCounts(stats.UnpricedPlays, res.Stats.UnpricedPlays)
		stats.Fallbacks += res.Stats.Fallbacks
		if len(res.Stall) > len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			why[res.Stall]++
			if stalls <= 3 && !quiet {
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
	t.Logf("seeds %d-%d: %d games, %d diverged, %d kernel stalls %v, plays %v", lo, hi, games, diverged, stalls, why, played)
	return diverged, stalls, stats
}

func addCounts(dst, src map[string]int) map[string]int {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = map[string]int{}
	}
	for k, v := range src {
		dst[k] += v
	}
	return dst
}

func TestWaterbendLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	played := map[string]int{}
	diverged, stalls, stats := kwaterbendRun(t, 1, kwaterbendSeeds, played, false)
	if diverged != 0 {
		t.Errorf("%d games diverged", diverged)
	}
	if stalls != 0 {
		t.Errorf("%d kernel capacity stalls, want zero", stalls)
	}
	for _, n := range kwaterbendCarriers {
		if played["activated:"+n] == 0 {
			t.Errorf("carrier %s never activated its waterbend ability", n)
		}
	}
	if played["waterbend declines"] < 10 {
		t.Errorf("only %d waterbend declines, want >= 10", played["waterbend declines"])
	}
	for _, n := range []string{"zero-pick activations", "picked activations", "picked mana creature"} {
		if played[n] == 0 {
			t.Errorf("%s never happened", n)
		}
	}
	if len(stats.Unknown) != 0 {
		t.Errorf("Unknown options: %v", stats.Unknown)
	}
	// Gorge's payment plan declines every waterbend shape (payment_plan.go
	// "shape:contribution"), so each activation falls back to its manual
	// window; the kernel emulates that window (waterbendManualPay). Fallbacks
	// are therefore expected and only reported.
	t.Logf("autopay fallbacks: %d (expected: waterbend is a shape:contribution)", stats.Fallbacks)
	if len(stats.Unpriced) != 0 {
		t.Errorf("Unpriced plays: %v", stats.Unpriced)
	}
	for k := range stats.UnpricedPlays {
		for _, n := range kwaterbendCarriers {
			if strings.Contains(k, n) {
				t.Errorf("Unpriced play names %s: %s", n, k)
			}
		}
	}
}

// An equipped Equipment can pay: it is still an untapped artifact (CR 701.67a).
func TestWaterbendTapsAttachedEquipment(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := wbDeck(
		cardset.Entry{Name: "Foggy Swamp Vinebender", Count: 8}, cardset.Entry{Name: "Bonesplitter", Count: 8},
		cardset.Entry{Name: "Grizzly Bears", Count: 8}, cardset.Entry{Name: "Forest", Count: 16},
	)
	lockstepRun(t, "waterbendequip", [2][]cardset.Entry{deck, deck}, 80, true, nil)
}

// TestWaterbendMutation: with the offer credit dropped the lockstep must
// diverge.
func TestWaterbendMutation(t *testing.T) {
	old := dirac.DebugWaterbendNoCredit
	dirac.DebugWaterbendNoCredit = true
	defer func() { dirac.DebugWaterbendNoCredit = old }()
	if diverged, _, _ := kwaterbendRun(t, 1, 24, map[string]int{}, true); diverged == 0 {
		t.Fatal("DebugWaterbendNoCredit did not diverge the lockstep")
	}
}
