package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func repeatTestZone(g *state.Game, o *state.Object, z state.Zone, p state.PlayerID) {
	o.Zone = z
	g.SetZone(z, p, append(g.Zone(z, p), o.ID))
}

func TestRepeatDefinedRememberedUsesPresentAndFallbackCompare(t *testing.T) {
	h, c := fixtureHost(t)
	good := h.g.AddObject(mkCard(t, "Name:Good\nTypes:Creature\nOracle:x\n"), 0)
	repeatTestZone(h.g, good, state.ZBattlefield, 0)
	if good.Zone != state.ZBattlefield || !MatchesObjectCtx(h.Game(), "Card", good, c.SpecContext(c.Controller)) {
		t.Fatal("good fixture is not a matching battlefield Card")
	}
	var player state.Target
	player = state.Target{IsPlayer: true, Player: 1}
	runs := 0
	Register("TestDefinedTick", func(h Host, c *Ctx, _ *cards.SA) {
		runs++
		if runs < 3 {
			c.Remembered = []state.Target{{Obj: good.ID}}
		} else {
			c.Remembered = []state.Target{player}
		}
	})
	t.Cleanup(func() { unregister("TestDefinedTick") })
	c.SVars = map[string]string{"Loop": "DB$ TestDefinedTick"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{
		"RepeatSubAbility": "Loop", "RepeatDefined": "Remembered",
		"RepeatPresent": "Card", "RepeatSVarCompare": "EQ1",
	}})
	if runs != 3 {
		t.Fatalf("defined gate ran %d times, want 3 (do-while then first nonmatch)", runs)
	}
}

func TestRepeatDefinedDedicatedCompareFiltersPresent(t *testing.T) {
	h, c := fixtureHost(t)
	high := h.g.AddObject(mkCard(t, "Name:High\nManaCost:4\nTypes:Creature\nOracle:x\n"), 0)
	low := h.g.AddObject(mkCard(t, "Name:Low\nManaCost:1\nTypes:Creature\nOracle:x\n"), 0)
	repeatTestZone(h.g, high, state.ZBattlefield, 0)
	repeatTestZone(h.g, low, state.ZBattlefield, 0)
	if !MatchesObjectCtx(h.Game(), "Card.cmcGE4", high, c.SpecContext(c.Controller)) || MatchesObjectCtx(h.Game(), "Card.cmcGE4", low, c.SpecContext(c.Controller)) {
		t.Fatalf("cmc fixtures do not differ as required: high=%v low=%v", MatchesObjectCtx(h.Game(), "Card.cmcGE4", high, c.SpecContext(c.Controller)), MatchesObjectCtx(h.Game(), "Card.cmcGE4", low, c.SpecContext(c.Controller)))
	}
	runs := 0
	Register("TestDefinedTick", func(h Host, c *Ctx, _ *cards.SA) {
		runs++
		if runs == 1 {
			c.Remembered = []state.Target{{Obj: high.ID}, {Obj: low.ID}}
		} else {
			c.Remembered = []state.Target{{Obj: low.ID}}
		}
	})
	t.Cleanup(func() { unregister("TestDefinedTick") })
	c.SVars = map[string]string{"Loop": "DB$ TestDefinedTick"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{"RepeatSubAbility": "Loop", "RepeatDefined": "Remembered", "RepeatPresent": "Card.cmcGE4", "RepeatCompare": "EQ1"}})
	if runs != 2 {
		t.Fatalf("dedicated defined compare ran %d times, want 2", runs)
	}
}

func TestRepeatDefinedAbsentCompareMeansPresence(t *testing.T) {
	h, c := fixtureHost(t)
	obj := h.g.AddObject(mkCard(t, "Name:Present\nTypes:Creature\nOracle:x\n"), 0)
	repeatTestZone(h.g, obj, state.ZBattlefield, 0)
	if !MatchesObjectCtx(h.Game(), "Card", obj, c.SpecContext(c.Controller)) {
		t.Fatal("presence fixture is not a Card")
	}
	runs := 0
	Register("TestDefinedTick", func(h Host, c *Ctx, _ *cards.SA) {
		runs++
		if runs == 1 {
			c.Remembered = []state.Target{{Obj: obj.ID}}
		} else {
			c.Remembered = nil
		}
	})
	t.Cleanup(func() { unregister("TestDefinedTick") })
	c.SVars = map[string]string{"Loop": "DB$ TestDefinedTick"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{"RepeatSubAbility": "Loop", "RepeatDefined": "Remembered", "RepeatPresent": "Card"}})
	if runs != 2 {
		t.Fatalf("presence-default gate ran %d times, want 2", runs)
	}
}

func TestRepeatDefinedAndCheckSVarAreBothRequired(t *testing.T) {
	h, c := fixtureHost(t)
	obj := h.g.AddObject(mkCard(t, "Name:Both\nTypes:Creature\nOracle:x\n"), 0)
	repeatTestZone(h.g, obj, state.ZBattlefield, 0)
	runs := 0
	Register("TestDefinedTick", func(h Host, c *Ctx, _ *cards.SA) {
		runs++
		c.Remembered = []state.Target{{Obj: obj.ID}}
		c.SVars["Check"] = "Count$ValidHand Card"
	})
	t.Cleanup(func() { unregister("TestDefinedTick") })
	c.SVars = map[string]string{"Loop": "DB$ TestDefinedTick", "Check": "Remembered$Amount"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{"RepeatSubAbility": "Loop", "RepeatDefined": "Remembered", "RepeatPresent": "Card", "RepeatCompare": "EQ1", "RepeatCheckSVar": "Check", "RepeatSVarCompare": "GE1"}})
	if runs != 1 {
		t.Fatalf("AND gate ran %d times, want 1", runs)
	}
}

func TestRepeatDefinedUsesPersistedImprinted(t *testing.T) {
	h, c := fixtureHost(t)
	imprint := h.g.AddObject(mkCard(t, "Name:Imprint\nTypes:Artifact\nOracle:x\n"), 0)
	repeatTestZone(h.g, imprint, state.ZExile, 0)
	source := h.g.Obj(c.Source)
	source.Imprinted = []state.ObjID{imprint.ID}
	runs := 0
	Register("TestDefinedTick", func(h Host, c *Ctx, _ *cards.SA) {
		runs++
		if runs > 1 {
			h.Game().Obj(c.Source).Imprinted = nil
		}
	})
	t.Cleanup(func() { unregister("TestDefinedTick") })
	c.SVars = map[string]string{"Loop": "DB$ TestDefinedTick"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{"RepeatSubAbility": "Loop", "RepeatDefined": "Imprinted", "RepeatPresent": "Card"}})
	if runs != 2 {
		t.Fatalf("imprinted gate ran %d times, want 2", runs)
	}
}

func TestRepeatDefinedCorpusPinCountrysideCrusher(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Countryside Crusher")
	if !ok {
		t.Fatal("Countryside Crusher missing from corpus")
	}
	repeat := cards.ResolveSVar(card.Faces[0].SVars, "TrigRepeat")
	if repeat == nil {
		t.Fatal("Countryside Crusher has no compiled TrigRepeat SVar")
	}
	if repeat.Params["RepeatDefined"] != "Remembered" || repeat.Params["RepeatPresent"] != "Card" || repeat.Params["RepeatSVarCompare"] != "EQ1" {
		t.Fatalf("compiled TrigRepeat params = %#v", repeat.Params)
	}
}
