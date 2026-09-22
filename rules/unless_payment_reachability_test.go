package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func corpusUnlessCost(c *cards.Card) string {
	var walk func(*cards.SA) string
	walk = func(sa *cards.SA) string {
		if sa == nil {
			return ""
		}
		if raw := sa.Params["UnlessCost"]; raw != "" {
			return raw
		}
		return walk(sa.Sub)
	}
	for _, f := range c.Faces {
		for _, sa := range f.Abilities {
			if raw := walk(sa); raw != "" {
				return raw
			}
		}
	}
	return ""
}

func TestUnlessCostPayableNeedsEnoughAndRightColourFromRealCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	leak := mustCorpusCard(t, reg, "Mana Leak")
	chain := mustCorpusCard(t, reg, "Chain Lightning")
	leakCost := corpusUnlessCost(leak)
	chainCost := corpusUnlessCost(chain)
	if leakCost == "" || chainCost == "" {
		t.Fatalf("real cards lost UnlessCost$: Mana Leak=%q Chain Lightning=%q", leakCost, chainCost)
	}

	e := handEngine(t, leak, mustCorpusCard(t, reg, "Grizzly Bears"))
	e.G.Players[0].Pool = state.Mana{}
	island := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Island"))
	if e.G.Obj(island).Tapped {
		t.Fatal("Island precondition failed: source is tapped")
	}
	if e.UnlessCostPayable(0, leakCost) {
		t.Fatalf("one Island incorrectly makes %s payable", leakCost)
	}
	if e.UnlessCostPayable(0, chainCost) && strings.Contains(chainCost, "R") {
		t.Fatalf("Island incorrectly makes red cost %s payable", chainCost)
	}
}
