package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The pod limits the kernel's StallTwoSeat once held (speed, the skip-untap
// marks, a battle's protector) are retired: a four-seat pod holding such a card
// plays to its end in lockstep. Each test deals the evidence loop's focus pool
// (kfuzz -synth <pool> -seats 4 -focus <card> -seed 1), plays the listed pods
// (oracle.PodAt, the kfuzz pairing rule), and requires every one clean and the
// mechanism to have fired in some of them.

// podFocusTally is what a focus pod's gorge game showed of the mechanism.
type podFocusTally struct {
	protectors int // Choose "protector" events (CR 310.10)
	speedSeat  [4]int
	played     int // objects of the focus card that left the library
}

func playFocusPods(t *testing.T, pool, card, focusText string, pods []int, watch ...func(g *GorgeSide)) podFocusTally {
	t.Helper()
	const seed, nDecks, perDeck = 1, 400, 4
	reg := synthReg(t)
	set, err := cardset.SynthByName(pool)
	if err != nil {
		t.Skipf("no %s table (make dirac-tables): %v", pool, err)
	}
	focus, err := ParseFocus([]byte(focusText))
	if err != nil {
		t.Fatal(err)
	}
	poolDecks, files, err := FocusPoolWith(reg, set, nil, "fx-"+strings.ReplaceAll(pool, "-", ""), focus, perDeck, seed, nDecks)
	if err != nil {
		t.Fatal(err)
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), "")
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	var y podFocusTally
	for _, i := range pods {
		p := PodAt(poolDecks, i, seed, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Logf("pod %d refused (an interaction guard of the dealt decks): %v", i, err)
			continue
		}
		rc := RunConfig{Config: Config{}, MaxDecisions: 20000, MaxTurns: 100}
		rc.Finish = func(g *GorgeSide) {
			for _, w := range watch {
				w(g)
			}
			for j := range g.E.L.Events {
				e := &g.E.L.Events[j]
				switch e.Kind {
				case events.Choose:
					if e.Counter == "protector" {
						y.protectors++
					}
				case events.SpeedChange:
					if int(e.Player) < len(y.speedSeat) {
						y.speedSeat[e.Player]++
					}
				case events.MoveZone:
					if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == card && e.To != state.ZHand && e.To != state.ZLibrary {
						y.played++
					}
				}
			}
		}
		k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", i, err)
		}
		res := Lockstep(m, p.Seed, k, AffordableTargetPolicy(RandomPolicy(p.Seed)), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("pod %d (%v): divergence at decision %d: %s %s %v %s", i, p.Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.Stall != "" {
			t.Fatalf("pod %d: stall %s", i, res.Stall)
		}
		t.Logf("pod %d: %d decisions", i, res.Decisions)
	}
	return y
}

// TestFourSeatBattleProtectorPodsLockstep: Invasion of Zendikar cast in four-seat
// pods asks its controller for the protector among the three opponents before
// it enters (CR 310.10; gorge applySiegeProtector's "protector" ask, the kernel's
// pre-entry ChGenPlayer), and the protector is the defender the other seats
// attack it through.
func TestFourSeatBattleProtectorPodsLockstep(t *testing.T) {
	y := playFocusPods(t, "tail-62", "Invasion of Zendikar", "Invasion of Zendikar\n", []int{1, 2, 3, 4, 6, 8, 11, 13})
	t.Logf("%+v", y)
	if y.protectors == 0 || y.played == 0 {
		t.Fatalf("no battle entered with a protector: %+v", y)
	}
}

// TestFourSeatSpeedPodsLockstep: Start your engines! cards in four-seat pods
// start and raise speed at seats 2 and 3 as at seats 0 and 1 (CR 702.179).
func TestFourSeatSpeedPodsLockstep(t *testing.T) {
	y := playFocusPods(t, "std-dft", "Pride of the Road", "Pride of the Road\n", []int{0, 1, 2, 3, 4, 5})
	t.Logf("%+v", y)
	if y.speedSeat[2] == 0 || y.speedSeat[3] == 0 {
		t.Fatalf("speed never changed at seat 2 or 3: %+v", y)
	}
}

// TestFourSeatSkipUntapPodsLockstep: Homarid Warrior's "doesn't untap during
// your next untap step" in four-seat pods keeps one mark per seat (the kernel's
// per-slot cold record, act_skipuntap.go).
func TestFourSeatSkipUntapPodsLockstep(t *testing.T) {
	y := playFocusPods(t, "tail-45", "Homarid Warrior", "Homarid Warrior\n", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})
	t.Logf("%+v", y)
	if y.played == 0 {
		t.Fatalf("Homarid Warrior never entered: %+v", y)
	}
}
