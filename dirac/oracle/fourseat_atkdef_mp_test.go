//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// fourSeatAtkPlay returns a player of four-seat pods of decks on pool, dirac
// against gorge, one seed per call.
func fourSeatAtkPlay(t *testing.T, pool string, decks [4][]string) func(seed uint64) Result {
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
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, RandomPolicy(seed), rc)
	}
}

// fourSeatAtkClean fails on a divergence or a kernel stop on any of seeds.
func fourSeatAtkClean(t *testing.T, play func(uint64) Result, seeds uint64) {
	t.Helper()
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
}

// fourSeatAtkMutated reports whether some seed diverges or stops the kernel
// while *flag is set.
func fourSeatAtkMutated(t *testing.T, play func(uint64) Result, seeds uint64, flag *bool) bool {
	t.Helper()
	*flag = true
	defer func() { *flag = false }()
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return true
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Logf("mutation: seed %d stops the kernel: %s", seed, res.Stall)
			return true
		}
	}
	return false
}

// TestFourSeatDeclarationDefenderLockstep plays four-seat TDM pods of
// "whenever you attack" token makers whose tokens enter attacking (War
// Effort, Dalkovan Encampment) and mobilize cards. Past two seats the
// trigger's defending player is the seat of the declaration's first attack
// event, bound as it is queued: no divergence and no stall. Without that
// binding (dirac.DebugDropDefSeat) the kernel cannot name the defender and
// some seed stops.
func TestFourSeatDeclarationDefenderLockstep(t *testing.T) {
	red := deckOf(6, "War Effort", deckOf(8, "Shock Brigade", deckOf(4, "Dalkovan Encampment", deckOf(22, "Mountain")...)...)...)
	white := deckOf(6, "War Effort", deckOf(8, "Voice of Victory", deckOf(4, "Dalkovan Encampment", deckOf(10, "Plains", deckOf(12, "Mountain")...)...)...)...)
	play := fourSeatAtkPlay(t, "std-tdm", [4][]string{red, white, red, white})
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugDropDefSeat) {
		t.Fatal("dropping the queued defending seat changed no seed")
	}
}

// TestFourSeatAttackEventOrderLockstep plays four-seat OTJ pods of Attacks
// triggers that target a creature the defending player controls (Spring
// Splasher, Thunder Lasso). gorge emits one DeclareAttackers event per
// defending seat, in seat order, and each trigger's defending player is its
// event's seat: no divergence. With every attacked player's attackers in one
// event (dirac.DebugAtkSeatsMerged), some seed diverges.
func TestFourSeatAttackEventOrderLockstep(t *testing.T) {
	blue := deckOf(12, "Spring Splasher", deckOf(4, "Thunder Lasso", deckOf(24, "Island")...)...)
	red := deckOf(12, "Spring Splasher", deckOf(4, "Thunder Lasso", deckOf(12, "Island", deckOf(12, "Mountain")...)...)...)
	play := fourSeatAtkPlay(t, "std-otj", [4][]string{blue, red, blue, red})
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugAtkSeatsMerged) {
		t.Fatal("one attack event for every attacked player diverged on no seed")
	}
}

// TestFourSeatDefenderPresentLockstep plays four-seat DSK pods of Fear of
// the Dark against Glimmer creatures: its IsPresent$ reads the defending
// player's Glimmers as the trigger fires and again as it resolves, the seat
// its attack event names. No divergence and no stall; read as the seat
// after the active one (dirac.DebugDefendingActiveOpp), some seed diverges.
func TestFourSeatDefenderPresentLockstep(t *testing.T) {
	black := deckOf(12, "Fear of the Dark", deckOf(28, "Swamp")...)
	glim := deckOf(12, "Fear of the Dark", deckOf(8, "Enduring Tenacity", deckOf(20, "Swamp")...)...)
	play := fourSeatAtkPlay(t, "std-dsk", [4][]string{black, glim, glim, glim})
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugDefendingActiveOpp) {
		t.Fatal("reading the defending player as the seat after the active one diverged on no seed")
	}
}
