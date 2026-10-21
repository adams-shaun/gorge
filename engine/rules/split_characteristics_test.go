package rules

// CR 709.4b-d split-card colours and card types (cr-g13-face-characteristics-mvd).
// Off the stack a split card has the colours of its combined mana cost and
// each card type on either half; on the stack only the half being cast
// exists (CR 709.3b) unless the spell is fused (CR 709.4d). A cast
// restriction or cost modifier reads the half being cast (CR 601.3a/709.3a).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSplitCardColoursCombineOffTheStack: Fire // Ice ({1}{R} // {1}{U}) is
// red AND blue in the hand (CR 709.4b), only blue on the stack cast as Ice
// (CR 709.3b), and both again when fused (CR 709.4d).
func TestSplitCardColoursCombineOffTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	fi := handCard(e, mustCorpusCard(t, reg, "Fire // Ice"), 0)
	sc := e.specCtx(0, 0)
	if !e.matchesSpec("Card.Blue", fi, sc) || !e.matchesSpec("Card.Red", fi, sc) || !e.matchesSpec("Card.MultiColor", fi, sc) {
		t.Errorf("Fire // Ice in hand: Blue=%v Red=%v MultiColor=%v, want all true (CR 709.4b)",
			e.matchesSpec("Card.Blue", fi, sc), e.matchesSpec("Card.Red", fi, sc), e.matchesSpec("Card.MultiColor", fi, sc))
	}
	if got := e.Colors(fi); got != "UR" {
		t.Errorf("Fire // Ice in hand: Colors = %q, want UR", got)
	}
	if e.matchesSpec("Card.Blue", fi, e.spellSpecCtx(sc, fi)) {
		t.Error("Fire // Ice read as the Fire spell matches Card.Blue (CR 601.3a/709.3a)")
	}
	// Direct placement (no cast): the read needs only zone, face and the
	// fuse flag.
	o := e.G.Obj(fi)
	o.Zone = state.ZStack
	o.SetFaceIdx(1)
	if e.matchesSpec("Card.Red", fi, sc) || !e.matchesSpec("Card.Blue", fi, sc) {
		t.Errorf("Ice on the stack: Red=%v Blue=%v, want only blue (CR 709.3b)",
			e.matchesSpec("Card.Red", fi, sc), e.matchesSpec("Card.Blue", fi, sc))
	}
	o.SetFaceIdx(0)
	o.CastFlags |= state.FlagFused
	if !e.matchesSpec("Card.Blue", fi, sc) || !e.matchesSpec("Card.Red", fi, sc) {
		t.Error("fused Fire // Ice on the stack is not both red and blue (CR 709.4d)")
	}
}

// TestSplitCardTypesCombineOffTheStack: Commit // Memory (instant // sorcery)
// is an instant card and a sorcery card in the hand (CR 709.4c), and only an
// instant on the stack cast as Commit.
func TestSplitCardTypesCombineOffTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	cm := handCard(e, mustCorpusCard(t, reg, "Commit // Memory"), 0)
	sc := e.specCtx(0, 0)
	if !e.matchesSpec("Card.Sorcery", cm, sc) || !e.matchesSpec("Card.Instant", cm, sc) {
		t.Errorf("Commit // Memory in hand: Sorcery=%v Instant=%v, want both (CR 709.4c)",
			e.matchesSpec("Card.Sorcery", cm, sc), e.matchesSpec("Card.Instant", cm, sc))
	}
	// Read as the spell being cast (a cast restriction's context), the hand
	// card is only Commit, an instant (CR 601.3a/709.3a).
	if e.matchesSpec("Card.Sorcery", cm, e.spellSpecCtx(sc, cm)) {
		t.Error("Commit // Memory read as the Commit spell matches Card.Sorcery")
	}
	e.G.Obj(cm).Zone = state.ZStack
	if e.matchesSpec("Card.Sorcery", cm, sc) {
		t.Error("Commit on the stack matches Card.Sorcery (CR 709.3b: only the half cast exists)")
	}
}

// TestColourCostModifierReadsTheHalfBeingCast guards CR 601.3a/709.3a:
// Sapphire Medallion ("Blue spells you cast cost {1} less") does not discount
// Fire ({1}{R}) although Fire // Ice is blue in the hand. Passes at the base
// (which reads the left half) and must keep passing once the hand card reads
// both colours.
func TestColourCostModifierReadsTheHalfBeingCast(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Sapphire Medallion"))
	fi := handCard(e, mustCorpusCard(t, reg, "Fire // Ice"), 0)
	if got := e.offerCostFor(0, fi, ParseCost("1 R"), spellScope("")); got.Generic != 1 {
		t.Fatalf("Fire under Sapphire Medallion offers generic %d, want 1 (Fire is red, not blue)", got.Generic)
	}
}
