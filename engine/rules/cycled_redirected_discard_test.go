package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Mode$ Cycled self trigger (Gempalm Incinerator's shape) must still fire
// when a Rest-in-Peace-style replacement redirects the cycling cost discard
// to exile (CR 614.6: the modified event is still the discard, so the card
// was still cycled, CR 702.29d). The replacement's substituted exile carries
// the exiler stamp in its Counter, which must not crowd out the cycling
// cause the trigger matcher reads.

const redirectedCyclerSrc = "Name:Redirected Cycler\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/1\nK:Cycling:R\nOracle:x\n" +
	"T:Mode$ Cycled | ValidCard$ Card.Self | Execute$ CycledDraw | TriggerDescription$ When you cycle CARDNAME, its controller draws.\n" +
	"SVar:CycledDraw:DB$ Draw | Defined$ You | NumCards$ 1\n"

const graveyardExilerSrc = "Name:Graveyard Exiler\nManaCost:1 W\nTypes:Enchantment\nOracle:x\n" +
	"R:Event$ Moved | ActiveZones$ Battlefield | Destination$ Graveyard | ValidCard$ Card | ReplaceWith$ RedirectToExile | Description$ If a card would be put into a graveyard from anywhere, exile it instead.\n" +
	"SVar:RedirectToExile:DB$ ChangeZone | Defined$ ReplacedCard | Hidden$ True | Origin$ All | Destination$ Exile\n"

func TestCycledTriggerFiresFromRedirectedDiscard(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, redirectedCyclerSrc))
	id := e.G.Zone(state.ZHand, 0)[0]
	onBoard(t, e, 0, graveyardExilerSrc)
	addMana(t, e, 0, "R")

	var opt decision.Option
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == id {
			opt = o
			break
		}
	}
	if opt.Kind != "ability" {
		t.Fatal("precondition: the cycling activation was not offered")
	}
	e.beginActivation(0, opt)
	e.pending = nil
	e.Advance()

	// Preconditions: the discard was redirected to exile, and the redirected
	// move is still recorded as the cycling cost discard.
	if got := e.G.Obj(id).Zone; got != state.ZExile {
		t.Fatalf("precondition: the redirected cycle left the card in %s, want exile", got)
	}
	discards := 0
	for _, ev := range e.L.Events {
		if ev.Obj == id && events.IsDiscardCost(ev) {
			discards++
			if ev.To != state.ZExile {
				t.Fatalf("precondition: the cost discard went to %s, want exile (replacement not applied)", ev.To)
			}
		}
	}
	if discards != 1 {
		t.Fatalf("precondition: cost discards of the cycler = %d, want 1", discards)
	}

	// The cycling ability and the card's own Cycled trigger both stand on
	// the stack, the trigger above the ability.
	if got := len(e.G.Stack); got != 2 {
		t.Fatalf("stack after the redirected cycle = %d, want 2 (cycling ability + its Cycled trigger)", got)
	}
	if top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1]); top == nil || top.Source != id {
		t.Fatalf("stack top = %+v, want an ability sourced by the cycled card %d", top, id)
	}
}
