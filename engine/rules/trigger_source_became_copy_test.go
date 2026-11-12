package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A trigger on the stack keeps the text it triggered with (CR 113.7a) when its source
// becomes a copy in response: Outlaw Stitcher's X is its own SVar, and two spells this
// turn make it two counters whatever Fleeting Reflection copied onto the Stitcher.
func TestTriggerKeepsItsSVarsWhenItsSourceBecomesACopy(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Outlaw Stitcher"), lookup(t, reg, "Fleeting Reflection"),
		lookup(t, reg, "Grizzly Bears")}, nil)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	stitcher := moveCorpusCard(t, e, "Outlaw Stitcher", 0, state.ZHand)
	reflection := moveCorpusCard(t, e, "Fleeting Reflection", 0, state.ZHand)
	addMana(t, e, 0, "CCCU")
	submitChoices(t, e, castOption(t, e, stitcher))
	// Stitcher on the stack: pass until it has resolved and its trigger waits.
	for i := 0; i < 8 && e.G.Obj(stitcher).Zone != state.ZBattlefield; i++ {
		passOnce(t, e)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("the stack holds %d objects, want the Stitcher's trigger", len(e.G.Stack))
	}
	addMana(t, e, 0, "CU")
	submitChoices(t, e, castOption(t, e, reflection))
	for _, target := range []state.ObjID{stitcher, bears} {
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("no target decision for %d: %+v", target, d)
		}
		submitChoices(t, e, indexOfObjOption(d, target))
	}
	resolveStack(t, e)
	if f := e.G.Obj(stitcher).Face(); f == nil || f.Name != "Grizzly Bears" {
		t.Fatalf("the Stitcher did not become the Bears: %+v", f)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.IsToken {
			if got := counterCount(o, "P1P1"); got != 2 {
				t.Fatalf("the token has %d +1/+1 counters, want 2", got)
			}
			replayCheck(t, e, cfg)
			return
		}
	}
	t.Fatal("no token was created")
}
