package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestHorsemanshipWithWithoutPredicates is the effects leaf for the
// with<Keyword>/without<Keyword> object predicates when the keyword is
// Horsemanship (CR 702.31). The keyword list that init() expands into the
// predicates map did not name Horsemanship, so every spec carrying
// withHorsemanship / withoutHorsemanship failed closed: a horsemanship-
// targeting spell had no legal targets and a withoutHorsemanship sweep
// matched nothing. Because the matcher and UnknownPredicates both read the
// same predicates map, the empty census is asserted alongside the matches --
// the two cannot disagree.
func TestHorsemanshipWithWithoutPredicates(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	mk := func(owner state.PlayerID, src string) *state.Object {
		t.Helper()
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, owner)
		o.Zone = state.ZBattlefield
		return o
	}
	rider := mk(0, "Name:Rider\nManaCost:2 W\nTypes:Creature Human Soldier\nPT:2/2\nK:Horsemanship\nOracle:x\n")
	bear := mk(0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	if !MatchesObjectCtx(g, "Creature.withHorsemanship", rider, SpecContext{You: 0}) {
		t.Error("a creature with printed K:Horsemanship must match Creature.withHorsemanship")
	}
	if MatchesObjectCtx(g, "Creature.withoutHorsemanship", rider, SpecContext{You: 0}) {
		t.Error("a creature with printed K:Horsemanship must not match Creature.withoutHorsemanship")
	}
	if !MatchesObjectCtx(g, "Creature.withoutHorsemanship", bear, SpecContext{You: 0}) {
		t.Error("a plain creature must match Creature.withoutHorsemanship")
	}
	if MatchesObjectCtx(g, "Creature.withHorsemanship", bear, SpecContext{You: 0}) {
		t.Error("a plain creature must not match Creature.withHorsemanship")
	}
	if got := UnknownPredicates("Creature.withHorsemanship"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.withHorsemanship) = %v, want empty", got)
	}
	if got := UnknownPredicates("Creature.withoutHorsemanship"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.withoutHorsemanship) = %v, want empty", got)
	}
}

// TestHorsemanshipCorpusCarriersMatchTheirSpecs pins the predicate on the
// real corpus cards whose scripts carry the specs, so the fix is tied to the
// exact filter text the engine will be asked to evaluate -- never a synthetic
// spelling. Zhang Fei, Fierce Warrior carries printed K:Horsemanship; Grizzly
// Bears is a plain creature.
func TestHorsemanshipCorpusCarriersMatchTheirSpecs(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	rider := corpusObject(t, reg, g, "Zhang Fei, Fierce Warrior")
	bear := corpusObject(t, reg, g, "Grizzly Bears")

	// Trip Wire: SP$ Destroy | ValidTgts$ Creature.withHorsemanship
	// Zuo Ci, the Mocking Sage / Taoist Mystic: CantBlockBy ValidBlocker$
	if !MatchesSpec(g, "Creature.withHorsemanship", rider.ID, 0) {
		t.Error("Zhang Fei must match Creature.withHorsemanship (Trip Wire / Zuo Ci / Taoist Mystic specs)")
	}
	if MatchesSpec(g, "Creature.withHorsemanship", bear.ID, 0) {
		t.Error("Grizzly Bears must not match Creature.withHorsemanship")
	}
	// Broken Dam: ValidTgts$ Creature.withoutHorsemanship
	// Rolling Earthquake: ValidCards$ Creature.withoutHorsemanship
	if !MatchesSpec(g, "Creature.withoutHorsemanship", bear.ID, 0) {
		t.Error("Grizzly Bears must match Creature.withoutHorsemanship (Broken Dam / Rolling Earthquake specs)")
	}
	if MatchesSpec(g, "Creature.withoutHorsemanship", rider.ID, 0) {
		t.Error("Zhang Fei must not match Creature.withoutHorsemanship")
	}
}
