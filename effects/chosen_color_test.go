package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestChosenColorPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	sourceID := corpusObject(t, reg, g, "Wash Out").ID
	blueID := corpusObject(t, reg, g, "Air Elemental").ID
	redID := corpusObject(t, reg, g, "Shivan Dragon").ID
	source := g.Obj(sourceID)
	blueObj, redObj := g.Obj(blueID), g.Obj(redID)
	source.ChosenColor = "U"
	blueColors, redColors := ColorsOf(blueObj), ColorsOf(redObj)
	if blueColors == redColors || !strings.Contains(blueColors, "U") || strings.Contains(redColors, "U") {
		t.Fatalf("precondition: candidate colors must differ (blue=%q red=%q)", blueColors, redColors)
	}
	if !MatchesObjectCtx(g, "Card.ChosenColor", blueObj, SpecContext{You: 0, Source: sourceID}) {
		t.Fatal("Card.ChosenColor did not match the source's recorded blue choice")
	}
	if MatchesObjectCtx(g, "Card.ChosenColor", redObj, SpecContext{You: 0, Source: sourceID}) {
		t.Fatal("Card.ChosenColor matched a candidate outside the recorded color")
	}
	for _, spec := range []string{"Card.ChosenColor", "Card.!ChosenColor"} {
		if MatchesObjectCtx(g, spec, blueObj, SpecContext{You: 0}) {
			t.Errorf("unbound %s matched; context-bound predicate must fail closed", spec)
		}
	}
	if got := UnknownPredicates("Card.ChosenColor"); len(got) != 0 {
		t.Fatalf("UnknownPredicates(Card.ChosenColor) = %v", got)
	}
	if got := resolveGains("ChosenColor", "", source); got != "blue" {
		t.Fatalf("Protection Gains$ ChosenColor resolved to %q, want blue", got)
	}
	boardGame, ids := board(t)
	protectionSource := boardGame.Obj(ids["myBear"])
	protectionSource.ChosenColor = "U"
	protectionHost := &fakeHost{g: boardGame}
	Resolve(protectionHost, &Ctx{Source: protectionSource.ID, Controller: 0,
		Targets: []state.Target{{Obj: ids["myFlier"]}}},
		sa(t, "AB$ Protection | ValidTgts$ Creature | Gains$ ChosenColor"))
	if len(protectionHost.continuous) != 1 || len(protectionHost.continuous[0].AddKeywords) != 1 ||
		protectionHost.continuous[0].AddKeywords[0] != "Protection from blue" {
		t.Fatalf("Gains$ ChosenColor protection = %+v, want actual protection from blue", protectionHost.continuous)
	}
	source.ChosenColor = ""
	for _, spec := range []string{"Card.ChosenColor", "Card.!ChosenColor"} {
		if MatchesObjectCtx(g, spec, blueObj, SpecContext{You: 0, Source: sourceID}) {
			t.Errorf("empty recorded choice made %s match", spec)
		}
	}
	source.ChosenColor = "not-a-color"
	if got := resolveGains("ChosenColor", "", source); got != "" {
		t.Fatalf("invalid protection choice resolved to %q, want fail-closed empty", got)
	}
	for _, spec := range []string{"Card.ChosenColor", "Card.!ChosenColor"} {
		if MatchesObjectCtx(g, spec, blueObj, SpecContext{You: 0, Source: sourceID}) {
			t.Errorf("invalid recorded choice made %s match", spec)
		}
	}
}
