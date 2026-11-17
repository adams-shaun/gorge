package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	amplifyBeastSrc  = "Name:Amp Beast\nManaCost:G\nTypes:Creature Beast\nPT:1/1\nOracle:x\n"
	amplifyBeast2Src = "Name:Amp Beast Two\nManaCost:G\nTypes:Creature Beast\nPT:1/1\nOracle:x\n"
	amplifyElfSrc    = "Name:Amp Elf\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
)

// TestAmplifyEntersWithCounters is CR 702.38a: Canopy Crawler's Amplify 1,
// "as this creature enters, put a +1/+1 counter on it for each Beast card you
// reveal in your hand". The reveal offers the two Beasts and not the Elf;
// revealing both, it enters with two +1/+1 counters.
func TestAmplifyEntersWithCounters(t *testing.T) {
	reg := searchTestRegistry(t)
	deck := []*cards.Card{searchCorpusCard(t, reg, "Canopy Crawler"),
		card(t, amplifyBeastSrc), card(t, amplifyBeast2Src), card(t, amplifyElfSrc)}
	deck = append(deck, mountainDeck(t, 36)...)
	cfg := seatZeroStart(Config{Seed: 9441, Names: []string{"amp", "opp"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	for _, name := range []string{"Amp Beast", "Amp Beast Two", "Amp Elf"} {
		searchMoveByName(t, e, name, state.ZHand)
	}
	crawler := searchMoveByName(t, e, "Canopy Crawler", state.ZHand)
	addMana(t, e, 0, "GGGG")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == crawler {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Canopy Crawler: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	asked := false
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passPriorityOnce(t, e)
			continue
		}
		var all []int
		for _, o := range d.Options {
			if o.Kind == "reveal" {
				if f := e.G.Obj(o.Obj).Face(); f == nil || (f.Name != "Amp Beast" && f.Name != "Amp Beast Two") {
					t.Fatalf("Amplify offered %v, want only the Beasts", f)
				}
				all = append(all, o.Index)
			}
		}
		if len(all) == 0 {
			all = []int{d.Options[0].Index}
		} else {
			asked = true
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: all}); err != nil {
			t.Fatalf("submit %s: %v", d.Kind, err)
		}
	}
	if !asked {
		t.Fatal("Amplify posed no reveal")
	}
	if got := e.G.Obj(crawler).Counter("P1P1"); got != 2 {
		t.Fatalf("Canopy Crawler entered with %d +1/+1 counters, want 2", got)
	}
	replayCheck(t, e, cfg)
}
