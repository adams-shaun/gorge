package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// copyFirstLockstep plays seeds first..last of two decks on set and counts the
// optional copy-instead elections gorge poses (the prompt of
// poseChosenTokenReplacement), failing when fewer than want were posed: the
// fixture must reach the replacement for the lockstep to mean anything.
func copyFirstLockstep(t *testing.T, setName string, decks [2][]cardset.Entry, last, want int) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, setName)
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	asked, copies := 0, 0
	lastAsk, evAt := -1, 0
	stalls := map[string]int{}
	check := func(ref *GorgeSide, _ Side) *Divergence {
		if d := ref.E.Pending(); d != nil && strings.Contains(d.Prompt, "instead create tokens that are copies") {
			if n := len(ref.E.L.Events); n != lastAsk {
				lastAsk = n
				asked++
			}
		}
		if evs := ref.E.L.Events; len(evs) >= evAt {
			for _, e := range evs[evAt:] {
				if e.Kind == events.CopyToken {
					copies++
				}
			}
			evAt = len(evs)
		} else {
			evAt = 0
		}
		return nil
	}
	for seed := uint64(1); seed <= uint64(last); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		lastAsk, evAt = -1, 0
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if res.Stall != "" {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
	}
	t.Logf("%d copy-instead elections, %d token copies in all, game ends %v", asked, copies, stalls)
	// two copy-instead replacements apply to one creation only with two
	// equipped Crowns (gorge's CR 616.1 order ask: the declared StallTokRewrite)
	if n := stalls[KernelStallPrefix+"tokrewrite"]; n > last/10 {
		t.Fatalf("%d of %d games stalled on a token replacement: the kernel should ask, not stall", n, last)
	}
	if asked < want {
		t.Fatalf("only %d copy-instead elections posed in %d seeds, want %d: the fixture does not reach the replacement", asked, last, want)
	}
}

// Mirrormind Crown equipped to a creature: the first token its controller
// would create each turn may be a copy of the equipped creature (an election,
// decline first); a token that has entered closes the gate for the rest of the
// turn, so a creation of two tokens makes one copy and a plain token.
func TestMirrormindCrownLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Mirrormind Crown", Count: 2}, {Name: "Elder Auntie", Count: 7},
		{Name: "Flamekin Gildweaver", Count: 5}, {Name: "Scuzzback Scrounger", Count: 3},
		{Name: "Mountain", Count: 23},
	}
	b := []cardset.Entry{
		{Name: "Mirrormind Crown", Count: 1}, {Name: "Elder Auntie", Count: 8},
		{Name: "Flamekin Gildweaver", Count: 5}, {Name: "Scuzzback Scrounger", Count: 3},
		{Name: "Mountain", Count: 23},
	}
	copyFirstLockstep(t, "std-ecl", [2][]cardset.Entry{a, b}, 300, 5)
}

// Moonlit Meditation enchanting a creature or artifact its controller
// controls: the same election, whoever the Aura's host is, and the two
// Soldiers of a Cosmogrand Zenith make one copy and a plain token.
func TestMoonlitMeditationLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Moonlit Meditation", Count: 3}, {Name: "Cosmogrand Zenith", Count: 5},
		{Name: "Honored Knight-Captain", Count: 4}, {Name: "Rayblade Trooper", Count: 3},
		{Name: "Mechan Assembler", Count: 2}, {Name: "Plains", Count: 12}, {Name: "Island", Count: 11},
	}
	b := []cardset.Entry{
		{Name: "Moonlit Meditation", Count: 1}, {Name: "Cosmogrand Zenith", Count: 4},
		{Name: "Knight Luminary", Count: 4}, {Name: "Station Monitor", Count: 4},
		{Name: "Rayblade Trooper", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Island", Count: 11},
	}
	copyFirstLockstep(t, "std-eoe", [2][]cardset.Entry{a, b}, 300, 5)
}

// The two committed repros (TestStdSetReprosLockstep) pass when the kernel
// stops at a declared stall, so each must play its whole script and the
// decisions after it with the kernel still in the game.
func TestCopyFirstReprosPlayThrough(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	for _, c := range [][2]string{
		{"std-ecl", "mirrormind_crown_first_token_is_a_copy"},
		{"std-eoe", "moonlit_meditation_two_tokens_one_copy"},
	} {
		r, err := ReadRepro("testdata/repro/" + c[1] + ".repro.json")
		if err != nil {
			t.Fatal(err)
		}
		res := replayStdReproResult(t, reg, c[0], c[1], false)
		if res.Div != nil {
			t.Fatalf("%s: diverged at %d: %s %s", c[1], res.Div.At, res.Div.What, res.Div.Field)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions < len(r.Actions) {
			t.Fatalf("%s: the game ended at decision %d of %d's script (%q), before the election was answered", c[1], res.Decisions, len(r.Actions), res.Stall)
		}
	}
}
