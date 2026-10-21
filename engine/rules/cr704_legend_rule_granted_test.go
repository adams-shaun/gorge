package rules

// CR 704.5j: "If two or more legendary permanents with the same name are
// controlled by the same player, that player chooses one of them, and the
// rest are put into their owners' graveyards." Legendary is a supertype the
// layer system can grant (CR 205.4, 613.1d), so a permanent made legendary by
// Leyline of Singularity ("All nonland permanents are legendary") counts.
// Forge checks the current type line (GameAction.handleLegendRule reads
// CardLists.getType(..., "Legendary") over the live card state), and so does
// XMage (StateBasedActions' legend check filters on SuperType.LEGENDARY of the
// permanent's current characteristics).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR704_5jGrantedLegendaryCountsForLegendRule: under Leyline of
// Singularity two Grizzly Bears are legendary permanents with the same name,
// so the legend rule asks seat 0 to keep one and the other goes to the
// graveyard.
func TestCR704_5jGrantedLegendaryCountsForLegendRule(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Leyline of Singularity"), bears, bears}, nil)
	moveCorpusCard(t, e, "Leyline of Singularity", 0, state.ZBattlefield)
	a := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	if !legendaryUnderLayers(e, a) {
		t.Fatalf("precondition: Grizzly Bears under Leyline of Singularity is not legendary in the derived type list")
	}
	b := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		submitChoices(t, e, d.Options[0].Index)
		e.Advance()
	}
	za, zb := e.G.Obj(a).Zone, e.G.Obj(b).Zone
	if (za == state.ZBattlefield) == (zb == state.ZBattlefield) {
		t.Errorf("CR 704.5j seq %d: two legendary Grizzly Bears (Leyline of Singularity) are in %s and %s, want exactly one kept on the battlefield and the other in the graveyard",
			len(e.L.Events), za, zb)
	}
	replayCheck(t, e, cfg)
}

// TestCR704_5jGrantedLegendaryGuards pins what stays right: without Leyline
// two Grizzly Bears are not legendary and both stay; under Leyline two
// legendary permanents with DIFFERENT names both stay.
func TestCR704_5jGrantedLegendaryGuards(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	t.Run("no Leyline", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{bears, bears}, nil)
		a := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
		b := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
		if e.G.Obj(a).Zone != state.ZBattlefield || e.G.Obj(b).Zone != state.ZBattlefield {
			t.Errorf("CR 704.5j seq %d: nonlegendary Grizzly Bears left the battlefield", len(e.L.Events))
		}
		replayCheck(t, e, cfg)
	})
	t.Run("Leyline, different names", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Leyline of Singularity"), bears, lookup(t, reg, "Raging Goblin")}, nil)
		moveCorpusCard(t, e, "Leyline of Singularity", 0, state.ZBattlefield)
		a := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
		b := moveCorpusCard(t, e, "Raging Goblin", 0, state.ZBattlefield)
		if e.G.Obj(a).Zone != state.ZBattlefield || e.G.Obj(b).Zone != state.ZBattlefield {
			t.Errorf("CR 704.5j seq %d: differently named legendary permanents left the battlefield", len(e.L.Events))
		}
		if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
			t.Errorf("CR 704.5j seq %d: pending %+v, want priority (no duplicate set)", len(e.L.Events), d)
		}
		replayCheck(t, e, cfg)
	})
}
