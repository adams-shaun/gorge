package rules

// CR 709.4b/202.3d split-card mana value (cr-g13-face-characteristics-mvb).
// A split card's mana value is its two halves' combined value in every zone
// but the stack; on the stack only the half being cast counts, unless the
// spell was fused (CR 709.4d). A cast restriction or cost modifier evaluates
// the half being cast (CR 601.3a/709.3a), never the combined card. wantCmc
// is face_mana_value_test.go's.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestManaValueSplitCardCombinesHalvesOffTheStack: CR 709.4b/202.3d. Fire //
// Ice ({1}{R} // {1}{U}) has mana value 4 in the hand, library and graveyard;
// on the stack cast as one half it is that half's 2, and fused (CR 709.4d)
// it is 4 again.
func TestManaValueSplitCardCombinesHalvesOffTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	fi := handCard(e, mustCorpusCard(t, reg, "Fire // Ice"), 0)
	wantCmc(t, e, fi, 4, "Fire // Ice in hand (CR 709.4b)")

	// Direct placement (no cast): the stack read only needs zone, face and
	// the fuse flag, the three facts state.ManaValue reads.
	o := e.G.Obj(fi)
	o.Zone = state.ZStack
	o.SetFaceIdx(1)
	wantCmc(t, e, fi, 2, "Ice on the stack (CR 709.3b)")
	o.SetFaceIdx(0)
	o.CastFlags |= state.FlagFused
	wantCmc(t, e, fi, 4, "fused Fire // Ice on the stack (CR 709.4d)")
}

// TestCascadeSkipsSplitCardByCombinedManaValue is the audit repro: Bloodbraid
// Elf (mana value 4) cascading over Fire // Ice (4 in exile, CR 709.4b) does
// not stop there; it stops at the Grizzly Bears behind it.
func TestCascadeSkipsSplitCardByCombinedManaValue(t *testing.T) {
	t.Parallel()
	e, cfg := cascadeTestEngineFiller(t, 9214, "Bloodbraid Elf", []string{"Fire", "Grizzly Bears"}, nil, "Mountain")
	elfID := searchMoveByName(t, e, "Bloodbraid Elf", state.ZHand)
	addMana(t, e, 0, "GGRR")
	idx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == elfID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Bloodbraid Elf: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	passOnce(t, e)
	passOnce(t, e)
	_ = cascadeElection(t, e, "Grizzly Bears")
	_ = cfg
}

// TestDarkConfidantLosesSplitCardsCombinedManaValue is the second audit
// repro, on Dark Confidant's own count (SVar:X:Remembered$CardManaCost): a
// revealed Fire // Ice costs 4 life, not 2.
func TestDarkConfidantLosesSplitCardsCombinedManaValue(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	fi := handCard(e, mustCorpusCard(t, reg, "Fire // Ice"), 0)
	got, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0,
		Remembered: []state.Target{{Obj: fi}}}, "Remembered$CardManaCost")
	if !ok || got != 4 {
		t.Fatalf("Remembered$CardManaCost over Fire // Ice = %d (ok=%v), want 4", got, ok)
	}
}

// TestSpellCostModifierReadsTheHalfBeingCast guards CR 601.3a/709.3a: a cost
// modifier evaluates only the half being cast, not the split card's combined
// value. Spectacle Mage ("instant and sorcery spells you cast with mana value
// 5 or greater cost {1} less") does not discount Order ({3}{W}, mana value 4)
// although Order // Chaos is mana value 7 in the hand. Passes at the base
// (which reads the face) and must keep passing once the hand card reads 7.
func TestSpellCostModifierReadsTheHalfBeingCast(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spectacle Mage"))
	oc := handCard(e, mustCorpusCard(t, reg, "Order // Chaos"), 0)
	if got := e.offerCostFor(0, oc, ParseCost("3 W"), spellScope("")); got.Generic != 3 {
		t.Fatalf("Order under Spectacle Mage offers generic %d, want 3 (mana value 4 is below 5)", got.Generic)
	}
}
