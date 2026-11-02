package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Start your engines! lockstep: constructed
// matchups over the generated speed pool. It proves CR 702.179 against
// gorge's engine/rules/speed.go:
//
//   - 702.179a: a Start your engines! permanent's entry starts its
//     controller's speed (Ascendant, Necroregent, Pride, Zahur,
//     Syphon, Kickoff, Starting Column, Raceway, Point the
//     Way, the Speed Demon, Samut), and a CONTROL CHANGE of one starts the
//     new controller's (Involuntary Employment taking a creature from a
//     speed-less seat's opponent);
//   - 702.179d: an opponent losing life on the speed player's turn bumps it
//     to 2, 3, 4 once a turn (the latch), never on its own turn;
//   - 702.179e: a Max-speed static's AddTrigger$ (Ascendant, Necroregent,
//     Pride, Syphon) and AddAbility$ (Kickoff, Starting Column, Raceway) exist only at speed 4;
//   - Count$YourSpeed (Point the Way, Samut, the Speed Demon) and the
//     ChangeSpeed decrease (Spikeshell Harrier, never below 1).

var speedDecks = [][2][]cardset.Entry{{
	{
		{Name: "Embalmed Ascendant", Count: 4}, {Name: "Risen Necroregent", Count: 3},
		{Name: "Pride of the Road", Count: 4}, {Name: "Zahur, Glory's Past", Count: 3},
		{Name: "Amonkhet Raceway", Count: 4}, {Name: "Aether Syphon", Count: 3},
		{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Aether Syphon", Count: 4}, {Name: "Kickoff Celebrations", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Involuntary Employment", Count: 4},
		{Name: "Spikeshell Harrier", Count: 3},
		{Name: "Starting Column", Count: 3}, {Name: "Amonkhet Raceway", Count: 1},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
}, {
	{
		{Name: "Embalmed Ascendant", Count: 4}, {Name: "Risen Necroregent", Count: 3},
		{Name: "Pride of the Road", Count: 4}, {Name: "Zahur, Glory's Past", Count: 3},
		{Name: "Amonkhet Raceway", Count: 4}, {Name: "Aether Syphon", Count: 3},
		{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 10},
	},
	{
		// no Start your engines! permanent of its own: its speed can only
		// begin by taking one (702.179a, the control-change hook)
		{Name: "Shock", Count: 4}, {Name: "Involuntary Employment", Count: 4},
		{Name: "Spikeshell Harrier", Count: 4},
		{Name: "Hill Giant", Count: 6}, {Name: "Wind Drake", Count: 5},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
	},
}, {
	{
		{Name: "Point the Way", Count: 4}, {Name: "The Speed Demon", Count: 3},
		{Name: "Risen Necroregent", Count: 5}, {Name: "Starting Column", Count: 3},
		{Name: "Aether Syphon", Count: 2}, {Name: "Amonkhet Raceway", Count: 3},
		{Name: "Swamp", Count: 11}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Samut, the Driving Force", Count: 3}, {Name: "Kickoff Celebrations", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Pride of the Road", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 7},
	},
}}

// speedCarriers names the cards whose casts and activations the test counts
// (each must be played at least once).
var speedCarriers = []string{
	"Embalmed Ascendant", "Risen Necroregent", "Pride of the Road", "Zahur, Glory's Past",
	"Aether Syphon", "Kickoff Celebrations", "Starting Column", "Point the Way",
	"The Speed Demon", "Samut, the Driving Force", "Spikeshell Harrier", "Involuntary Employment", "Shock",
}

// speedFired are the kernel's speed events the test requires, by counter.
var speedFired = []struct {
	cnt  int
	what string
}{
	{dirac.SpeedCntStartEntry, "702.179a an entry starts engines"},
	{dirac.SpeedCntStartControl, "702.179a a control change starts engines"},
	{dirac.SpeedCntQueue, "702.179d the opponent's life loss queues the trigger"},
	{dirac.SpeedCntLatch, "702.179d a second loss in the turn is refused"},
	{dirac.SpeedCntOwnTurn, "702.179d a loss on its own turn queues nothing"},
	{dirac.SpeedCntTo2, "702.179d speed 1 -> 2"},
	{dirac.SpeedCntTo3, "702.179d speed 2 -> 3"},
	{dirac.SpeedCntTo4, "702.179d speed 3 -> 4"},
	{dirac.SpeedCntDecrease, "ChangeSpeed decreased a speed"},
	{dirac.SpeedCntDecreaseHeld, "ChangeSpeed left a speed alone"},
	{dirac.SpeedCntGatedTrigger, "702.179e a max-speed trigger exists at speed 4"},
	{dirac.SpeedCntGatedOffer, "702.179e a max-speed ability is offered at speed 4"},
}

func speedMatchup(t *testing.T, reg *cards.Registry, set cardset.Set, mi int) ([2][]*cards.Card, [2][]cardset.Entry) {
	t.Helper()
	md := speedDecks[mi]
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
	return decks, md
}

func speedEnv(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("speed")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// speedGames plays seeds 1..n of every matchup (or matchup only) and
// returns the first divergence report ("" when none).
func speedGames(t *testing.T, reg *cards.Registry, set cardset.Set, n uint64, only int, played map[string]int) (games int, report string) {
	t.Helper()
	for mi := range speedDecks {
		if only >= 0 && mi != only {
			continue
		}
		decks, md := speedMatchup(t, reg, set, mi)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"speed-a", "speed-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := eagerPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				return games, "matchup " + string(rune('0'+mi)) + " seed " + itoa(seed) + ": diverged at decision " + itoa(uint64(d.At)) + ": " + d.What + " " + d.Field + " " + fmt.Sprint(d.Diff) + " " + d.Detail + fmt.Sprintf("\nref %v\nker %v", d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, ""
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}
	return string(b[i:])
}

func TestStartenginesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := speedEnv(t)
	dirac.SpeedCounting = true
	defer func() { dirac.SpeedCounting = false }()
	var before [dirac.NumSpeedCnt]uint64
	for i := range before {
		before[i] = dirac.SpeedCounts[i].Load()
	}
	played := map[string]int{}
	games, report := speedGames(t, reg, set, 200, -1, played)
	if report != "" {
		t.Fatal(report)
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for _, n := range speedCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
	for _, f := range speedFired {
		if dirac.SpeedCounts[f.cnt].Load() == before[f.cnt] {
			t.Errorf("%s: never fired", f.what)
		}
	}

	// Mutation: each once-only hook, removed, must produce a divergence.
	// The control-change hook is not mutated here: Involuntary Employment
	// creates its Treasure after the gain, and that entry re-runs the global
	// scan (gorge checkSpeedStart scans every seat), starting seat 1 anyway.
	// TestSpeedControlChangeStartsEngines covers the hook with no later entry.
	t.Run("mutation-entry-start", func(t *testing.T) {
		dirac.DebugIgnoreEntryStart = true
		defer func() { dirac.DebugIgnoreEntryStart = false }()
		games, report := speedGames(t, reg, set, 100, 1, map[string]int{})
		if report == "" {
			t.Fatalf("%d games without the battlefield-entry start hook: no divergence", games)
		}
		t.Logf("entry-start mutation diverged: %s", report)
	})
	t.Run("mutation-latch", func(t *testing.T) {
		dirac.DebugIgnoreSpeedLatch = true
		defer func() { dirac.DebugIgnoreSpeedLatch = false }()
		games, report := speedGames(t, reg, set, 400, -1, map[string]int{})
		if report == "" {
			t.Fatalf("%d games without the once-per-turn latch: no divergence", games)
		}
		t.Logf("latch mutation diverged: %s", report)
	})
}
