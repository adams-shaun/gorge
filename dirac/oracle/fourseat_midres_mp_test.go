//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSkullwinderMidResLowers: Skullwinder's enters trigger chooses an
// opponent mid-resolution (its DB chain is ChangeZone -> ChoosePlayer ->
// ChangeZone -> Cleanup) and lowers with the ExtChoosePlayerMid ask the
// chain's readers follow, not the two-seat path: neither two-seat nor
// pod-limited, so a three- or four-seat pod does not stall on it at its
// start. The four-seat play itself is TestFourSeatMidResChoosePlayerLockstep.
func TestSkullwinderMidResLowers(t *testing.T) {
	set := chosenOppPool(t)
	i, ok := set.Lookup("Skullwinder")
	if !ok {
		t.Fatal("Skullwinder is not in the chosenopp pool")
	}
	d := set.Def(i)
	asks := 0
	for ai := range d.Abilities {
		for j := range d.Abilities[ai].Prog {
			if d.Abilities[ai].Prog[j].Ext() == kop.ExtChoosePlayerMid {
				asks++
			}
		}
	}
	if asks != 1 {
		t.Errorf("Skullwinder: %d ExtChoosePlayerMid instructions, want 1", asks)
	}
	if kop.CardTwoSeat(d) || kop.CardPodLimited(d) {
		t.Error("Skullwinder: two-seat or pod-limited, a four-seat pod stalls on it at its start")
	}
}

// midResDecks are the mid-resolution choice's matchups: the Skullwinder deck
// (lands to cast it and a cycling spell to fill its graveyard, the target its
// enters trigger returns) against three decks of lands and the same cycling
// spell, so whichever opponent the choice names has a card to return. The
// decks alternate down the pod, as chosenOppDecks does.
func midResDecks(seats int) [][]string {
	winder := deckOf(2, "Skullwinder", deckOf(8, "Go for Blood", deckOf(10, "Forest", deckOf(20, "Island")...)...)...)
	other := deckOf(8, "Go for Blood", deckOf(32, "Island")...)
	var out [][]string
	for i := 0; i < seats; i++ {
		if i%2 == 0 {
			out = append(out, winder)
		} else {
			out = append(out, other)
		}
	}
	return out
}

// midResTally counts Skullwinder's battlefield entries and the
// graveyard-to-hand moves on gorge's side: the controller's returned card,
// then the chosen opponent's.
type midResTally struct {
	entered, returned int
}

func (y *midResTally) read(g *GorgeSide) {
	evs := g.E.L.Events
	for i := range evs {
		if evs[i].Kind == events.MoveZone && evs[i].To == state.ZBattlefield && firstFaceName(g.E.G, evs[i].Obj) == "Skullwinder" {
			y.entered++
		}
		if evs[i].Kind == events.MoveZone && evs[i].To == state.ZHand && evs[i].From == state.ZGraveyard {
			y.returned++
		}
	}
}

// playMidRes plays every seats-sized matchup of midResDecks over seeds 1..seeds
// in lockstep, failing on a divergence or a kernel stop.
func playMidRes(t *testing.T, seats int, seeds uint64) (midResTally, []Canon) {
	t.Helper()
	reg := synthReg(t)
	set := chosenOppPool(t)
	all := midResDecks(seats)
	var y midResTally
	var acts []Canon
	for m0 := 0; m0 < len(all); m0 += seats {
		decks := all[m0 : m0+seats]
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
		for seed := uint64(1); seed <= seeds; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]}, Seed: seed, PassElision: true, Surface: "full"}
			for i := 2; i < seats; i++ {
				r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
				r.MoreCards = append(r.MoreCards, decks[i])
			}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions, rc.MaxTurns = 3000, 30
			rc.Finish = y.read
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("matchup %d seed %d: %v", m0/seats, seed, err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("matchup %d seed %d: divergence at %d: %s %s %v", m0/seats, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("matchup %d seed %d: kernel stopped: %s", m0/seats, seed, res.Stall)
			}
			acts = append(acts, res.Actions...)
			t.Logf("matchup %d seed %d: %d decisions, over %v winner %d turns %d %s", m0/seats, seed, res.Decisions, res.Over, res.Winner, res.Turns, res.Stall)
		}
	}
	return y, acts
}

// TestMidResChoosePlayerTwoSeatLockstep plays two-seat Skullwinder games: the
// sole opponent is recorded unasked on both sides (gorge's single-option
// ChoosePlayer is answered in place, the kernel's soleOpponent arm), and the
// chain still returns one card from each graveyard. This is the only lockstep
// that reaches the unasked arm.
func TestMidResChoosePlayerTwoSeatLockstep(t *testing.T) {
	y, acts := playMidRes(t, 2, 6)
	if y.entered == 0 {
		t.Fatal("Skullwinder never entered the battlefield")
	}
	if y.returned < 2 {
		t.Fatalf("graveyard returns %d, want at least the controller's and the opponent's", y.returned)
	}
	code, _ := chooseCode("player")
	pick := ActChoose + CanonKind(code)
	for _, a := range acts {
		if a.Kind() == pick {
			t.Fatalf("a player ask was posed over the sole opponent: %v", a)
		}
	}
	t.Logf("entered %d, graveyard returns %d, no player ask posed", y.entered, y.returned)
}

// TestFourSeatMidResChoosePlayerLockstep plays four-seat pods of Skullwinder:
// its enters trigger returns a card from its controller's graveyard, then the
// controller chooses one of three opponents (gorge effChoosePlayer's "player"
// options in AliveFrom order; the kernel's mid-resolution ask,
// ExtChoosePlayerMid), and that player returns a card from their own
// graveyard. No divergence, no stall, the choice was asked and some pick is
// not the first opponent listed, and both halves of the chain returned a card
// from a graveyard.
func TestFourSeatMidResChoosePlayerLockstep(t *testing.T) {
	y, acts := playMidRes(t, 4, 6)
	if y.entered == 0 {
		t.Fatal("Skullwinder never entered the battlefield")
	}
	if y.returned < 2 {
		t.Fatalf("graveyard returns %d, want at least the controller's and the chosen opponent's", y.returned)
	}
	code, _ := chooseCode("player")
	pick := ActChoose + CanonKind(code)
	asks, farther := 0, 0
	for _, a := range acts {
		if a.Kind() != pick {
			continue
		}
		asks++
		// seats 0 and 2 choose, and each lists the other second (AliveFrom
		// order: 1 2 3 from seat 0, 3 0 1 from seat 2)
		if p := a.Tgt() - RefPlayer; p != 1 && p != 3 {
			farther++
		}
	}
	if asks == 0 || farther == 0 {
		t.Fatalf("opponent asks %d, past the first listed %d", asks, farther)
	}
	t.Logf("opponent asks %d, past the first listed %d; entered %d, graveyard returns %d", asks, farther, y.entered, y.returned)
}
