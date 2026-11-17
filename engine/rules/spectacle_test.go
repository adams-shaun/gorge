package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Spectacle (CR 702.137a): "You may pay [cost] rather than pay this spell's
// mana cost" if an opponent lost life this turn. Before any opponent loses
// life the spectacle cast is not offered; after a 1-damage hit it is, and it
// is charged the spectacle cost in place of the mana cost.
func TestSpectacleNeedsAnOpponentToHaveLostLife(t *testing.T) {
	t.Parallel()
	src := "Name:Stage\nManaCost:2 R\nTypes:Sorcery\nK:Spectacle:R\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	pingSrc := "Name:Ping\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | Defined$ Opponent | NumDmg$ 1\nOracle:x\n"
	e, cfg, stage := newFixtureDeck(t, 26, src, pingSrc)
	ping := addToHand(t, e, 0, pingSrc)
	addMana(t, e, 0, "RR")
	for _, o := range castOptions(t, e) {
		if o.Mode == "spectacled" {
			t.Fatal("spectacle offered before any opponent lost life")
		}
	}
	castObj(t, e, ping)
	if e.LifeLostThisTurn(1) == 0 {
		t.Fatal("the opponent lost no life to Ping")
	}
	var spec *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "spectacled" && o.Obj == stage {
			spec = &o
		}
	}
	if spec == nil {
		t.Fatal("spectacle not offered after an opponent lost life")
	}
	pool := e.G.Players[0].Pool.Total()
	submitChoices(t, e, spec.Index)
	if got := e.G.Players[0].Pool.Total(); got != pool-1 {
		t.Fatalf("spectacle cost {R} not what was paid: pool %d -> %d", pool, got)
	}
	if o := e.G.Obj(stage); o.Zone != state.ZStack {
		t.Fatalf("Stage not on the stack after the spectacle cast: %s", o.Zone)
	}
	replayCheck(t, e, cfg)
}

// The real-corpus spectacle carriers whose only gap was the keyword are
// supported. Rix Maadi Reveler and Rafter Demon also read Card.spectacle
// (the cost was paid), a provenance that needs a CastFlags bit: they stay
// unsupported by prop:spectacle.
func TestSpectacleCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Light Up the Stage", "Skewer the Critics", "Blade Juggler", "Body Count",
		"Dead Revels", "Drill Bit", "Hackrobat", "Spawn of Mayhem", "Spikewheel Acrobat"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
