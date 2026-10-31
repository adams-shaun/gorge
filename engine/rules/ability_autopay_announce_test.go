package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	apaIsland = "Name:Test Island\nTypes:Land Island\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ Blue mana.\nOracle:x\n"
	apaForest = "Name:Test Forest\nTypes:Land Forest\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Green mana.\nOracle:x\n"
	apaDrone  = "Name:Test Drone\nManaCost:1\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n"
	apaDork   = "Name:Test Dork\nManaCost:1\nTypes:Creature Elf\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ Two colourless.\nOracle:x\n"
)

// An activation's auto-pay witness is priced before its announcements. One they
// outdate is planned again before any step runs, so the payer never sees the
// manual window for a cost its sources still pay.
func TestAbilityAutoPaySurvivesItsAnnouncements(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		card  string
		board []string
		want  int
	}{
		// Equip costs {2} less for a colorless target: {1}, not {3}.
		{"target reduces the cost", "Ghostfire Blade", []string{apaDrone, apaIsland, apaIsland, apaIsland}, 1},
		// The Phyrexian pip of {1}{G/P} settles as mana only once it is chosen.
		{"Phyrexian pip settled", "Birthing Pod", []string{apaDrone, apaForest, apaForest}, 2},
		// The creature tapped for {2}, tap a creature can also be the witness's
		// source (CR 602.2b: one permanent can't pay both), and another remains.
		{"helper was the witness", "Volrath's Gardens", []string{apaDork, apaDork}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, _ := newFixtureDeck(t, 9700, ppFiller)
			toMain1(t, e)
			e.paidManaCensus = true
			src := onBoardCard(t, e, 0, corpusCard(t, tc.card))
			e.G.Obj(src).SummonSick = false
			var board []state.ObjID
			for _, s := range tc.board {
				id := onBoard(t, e, 0, s)
				e.G.Obj(id).SummonSick = false
				board = append(board, id)
			}
			d := ppPriority(t, e)
			var opt *decision.Option
			for i := range d.Options {
				if o := &d.Options[i]; o.Kind == "ability" && o.Obj == src && opt == nil {
					opt = o
				}
			}
			if opt == nil {
				t.Fatalf("%s offers no ability: %+v", tc.card, d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
				t.Fatal(err)
			}
			for i := 0; ; i++ {
				d := e.Pending()
				if d == nil || i == 20 {
					t.Fatalf("the activation did not settle: %+v", d)
				}
				if d.PaymentFallback != nil {
					t.Fatalf("payment fell back %+v at %q", *d.PaymentFallback, d.Prompt)
				}
				if d.Kind == decision.KPriority {
					break
				}
				choice := apaAnnouncementChoice(d, board[0])
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choice}); err != nil {
					t.Fatalf("answer %q: %v", d.Prompt, err)
				}
			}
			n := 0
			for _, id := range board {
				if e.G.Obj(id).Tapped {
					n++
				}
			}
			if n != tc.want {
				t.Fatalf("tapped %d, want %d", n, tc.want)
			}
		})
	}
}

// apaAnnouncementChoice answers an announcement: the drone where it is offered,
// and otherwise the first option.
func apaAnnouncementChoice(d *decision.Decision, drone state.ObjID) []int {
	for _, o := range d.Options {
		if o.Obj == drone {
			return []int{o.Index}
		}
	}
	return []int{d.Options[0].Index}
}
