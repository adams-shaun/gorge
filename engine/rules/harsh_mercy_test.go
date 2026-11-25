package rules

// Harsh Mercy (xproof xpt-0789604f / xpt-62011b6a): "Each player chooses a
// creature type. Destroy all creatures that aren't of a type chosen this
// way." The spell's RepeatEach loop asks each player in turn; the DestroyAll
// keeps every creature of every type chosen. Until the guard fix this test
// pins, the second player was never asked (the first answer's Choose event
// folded a ChosenType onto the resolving spell and effChooseType returned
// early), so only the first player's type survived.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const harshMercyHumanSrc = "Name:Test Human\nManaCost:W\nTypes:Creature Human\nPT:2/2\nOracle:x\n"
const harshMercyBeastSrc = "Name:Test Beast\nManaCost:G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n"

func TestHarshMercyEachPlayerChoosesAType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	hm := lookup(t, reg, "Harsh Mercy")
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{hm, card(t, harshMercyHumanSrc)},
		[]*cards.Card{card(t, harshMercyBeastSrc)})
	humanID := moveByName(t, e, 0, "Test Human", state.ZBattlefield)
	beastID := moveByName(t, e, 1, "Test Beast", state.ZBattlefield)
	moveByName(t, e, 0, "Harsh Mercy", state.ZHand)
	addMana(t, e, 0, "11W")
	castNamed(t, e, "Harsh Mercy")

	// Each player is asked for a creature type; seat 0 answers with its own
	// creature's type, seat 1 with the other one's.
	askedP1 := 0
	drainDecisions(t, e, 200, func(d *decision.Decision) []int {
		if d.Kind == decision.KChoose {
			want := "Human"
			if d.Player == 1 {
				want = "Beast"
				askedP1++
			}
			for _, o := range d.Options {
				if o.Label == want {
					return []int{o.Index}
				}
			}
		}
		return firstAnswer(d)
	})
	if askedP1 == 0 {
		t.Fatalf("seat 1 was never asked for a creature type")
	}
	if o := e.G.Obj(humanID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Test Human (a chosen type) left the battlefield: %+v", o)
	}
	if o := e.G.Obj(beastID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Test Beast (seat 1's chosen type) left the battlefield: %+v", o)
	}
	replayCheck(t, e, cfg)
}
