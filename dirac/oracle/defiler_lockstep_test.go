package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The Defiler cycle's board OptionalCost (kop.StatDefiler; dirac
// cost_defiler.go, gorge defiler_reducecolor.go): "you may pay 2 life; the
// spell costs one pip of {colour} less". tail-09 holds Defiler of Vigor (G) and
// Defiler of Instinct (R); the decks run both, a duplicate Vigor, and cheap
// green and red permanent spells, so the lockstep meets yes and no answers,
// casts that pay two Defilers, the same colour twice (the second is skipped:
// no pip left) and casts the offer gate admits only on the reduction.

// defilerDeck is the 40-card RG list: lands 17, spells 23.
func defilerDeck() []string {
	var d []string
	for _, c := range []struct {
		n    int
		name string
	}{
		{4, "Defiler of Vigor"}, {3, "Defiler of Instinct"}, {3, "Boon Satyr"}, {2, "Arasta of the Endless Web"},
		{2, "Bloodfire Kavu"}, {2, "Allosaurus Shepherd"}, {2, "Elvish Handservant"}, {2, "Mwonvuli Ooze"},
		{1, "Argothian Pixies"}, {1, "Rogue Kavu"}, {1, "Grim Initiate"}, {9, "Forest"}, {8, "Mountain"},
	} {
		d = append(d, deckOf(c.n, c.name)...)
	}
	return d
}

// defilerPlay is a player of n-seat pods of decks on pool, dirac against
// gorge, one seed per call.
func defilerPlay(t *testing.T, pool string, decks [][]string) func(seed uint64) Result {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]}, Seed: seed, PassElision: true, Surface: "full"}
		for i := 2; i < len(decks); i++ {
			r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
			r.MoreCards = append(r.MoreCards, decks[i])
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		rc.Finish = defilerFinish
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		if res.Div != nil {
			t.Logf("implicates %v", res.Div.Implicates(m))
		}
		return res
	}
}

var defilerFinish func(g *GorgeSide)

func defilerCounts() [4]uint64 {
	c := &dirac.DefilerCounts
	return [4]uint64{c.Yes.Load(), c.No.Load(), c.Multi.Load(), c.OfferOnly.Load()}
}

// defilerRun plays seeds 1..n: no divergence and no kernel stop, and the
// asks the test is about happened.
func defilerRun(t *testing.T, play func(uint64) Result, n uint64) {
	t.Helper()
	before := defilerCounts()
	for seed := uint64(1); seed <= n; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	after := defilerCounts()
	got := [4]uint64{after[0] - before[0], after[1] - before[1], after[2] - before[2], after[3] - before[3]}
	t.Logf("yes %d no %d two-Defiler casts %d reduction-only offers %d", got[0], got[1], got[2], got[3])
	if got[0] == 0 || got[1] == 0 || got[2] == 0 || got[3] == 0 {
		t.Fatalf("the games missed an ask (yes, no, two-Defiler cast, reduction-only offer): %v", got)
	}
}

// defilerMutated reports whether some seed diverges (or stops the kernel) while *flag is set.
func defilerMutated(t *testing.T, play func(uint64) Result, n uint64, flag *bool) bool {
	t.Helper()
	return fourSeatAtkMutated(t, play, n, flag)
}

func TestDefilerLockstepTwoSeat(t *testing.T) {
	d := defilerDeck()
	defilerRun(t, defilerPlay(t, "tail-09", [][]string{d, d}), 60)
}

func TestDefilerLockstepFourSeat(t *testing.T) {
	d := defilerDeck()
	defilerRun(t, defilerPlay(t, "tail-09", [][]string{d, d, d, d}), 30)
}

// TestDefilerMutation: the "no pip of that colour left" gate and the
// no-spill reduction are what the lockstep reaches. Posing the ask with no pip
// left, or letting a paid Defiler take generic, diverges.
func TestDefilerMutationNoPipGate(t *testing.T) {
	d := defilerDeck()
	play := defilerPlay(t, "tail-09", [][]string{d, d})
	if !defilerMutated(t, play, 24, &dirac.DebugDefilerNoPipGate) {
		t.Fatal("asking with no pip left changed no seed: the games do not reach the gate")
	}
}

func TestDefilerMutationSpill(t *testing.T) {
	d := defilerDeck()
	play := defilerPlay(t, "tail-09", [][]string{d, d})
	if !defilerMutated(t, play, 24, &dirac.DebugDefilerSpill) {
		t.Fatal("a spilling reduction changed no seed: the games do not reach it")
	}
}

// TestDefilerLockstepOtherColours plays the Defilers tail-09 does not hold
// (Faith W, Dreams U, Flesh B), each in a mono-colour pair beside cheap
// vanilla-ish creatures of its pool: yes and no answers, no divergence.
func TestDefilerLockstepOtherColours(t *testing.T) {
	for _, tc := range []struct {
		pool, defiler, basic string
		spells               []string
	}{
		{"tail-50", "Defiler of Faith", "Plains", []string{"Akrasan Squire", "Civic Guildmage", "Serra Zealot", "Traveling Minister", "Aviary Mechanic",
			"Moorland Inquisitor", "Mesa Cavalier", "Seller of Songbirds", "Shu Defender", "Calmwater Mentor"}},
		{"tail-39", "Defiler of Dreams", "Island", []string{"Darting Merfolk", "Halimar Excavator", "Dawnray Archer", "Skystrike Officer", "Wingspan Mentor",
			"Phantom Warrior", "Streambed Aquitects", "Vedalken Mastermind", "Cruel Witness", "Siren Reaver"}},
		{"tail-71", "Defiler of Flesh", "Swamp", []string{"Raving Oni-Slave", "Splatter Goblin", "Ghost Hounds", "Deadbridge Shaman", "Doomed Necromancer",
			"Morbid Opportunist", "Nim Abomination", "Strongarm Thug", "Daggerclaw Imp", "Xathrid Necromancer"}},
	} {
		t.Run(tc.defiler, func(t *testing.T) {
			d := deckOf(3, tc.defiler)
			for _, n := range tc.spells {
				d = append(d, n, n)
			}
			d = append(d, deckOf(40-len(d), tc.basic)...)
			before := defilerCounts()
			for seed := uint64(1); seed <= 24; seed++ {
				res := defilerPlay(t, tc.pool, [][]string{d, d})(seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
				}
				if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
				}
			}
			after := defilerCounts()
			t.Logf("%s: yes %d no %d", tc.defiler, after[0]-before[0], after[1]-before[1])
			if after[0] == before[0] || after[1] == before[1] {
				t.Fatalf("%s: no yes and no answers", tc.defiler)
			}
		})
	}
}
