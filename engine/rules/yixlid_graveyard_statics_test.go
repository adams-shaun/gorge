package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestYixlidJailerSilencesGraveyardStaticsOnBothWalks pins CR 613.1f for a
// card whose own static functions from the graveyard (a ReduceCost and a ManaConvert
// with EffectZone$ Graveyard): under "cards in graveyards
// lose all abilities" neither the fresh scan (scanCostStatics,
// scanManaConvSources) nor the fused walk cache (boardStaticsRef, the
// printed-list scan) may keep it. Forge removes the static through the
// card's trait changes whatever its zone (CardTraitChanges.applyStaticAbility
// via Card.updateStaticAbilities). The walk cache used to gate the loss on
// the battlefield alone, so the cached walk kept the static the fresh one
// dropped (walkCacheVerify then panics on the divergence).
func TestYixlidJailerSilencesGraveyardStaticsOnBothWalks(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	jailer, ok := reg.Lookup("Yixlid Jailer")
	if !ok {
		t.Fatal("Yixlid Jailer missing from corpus")
	}
	tax := card(t, "Name:Grave tax\nTypes:Creature Imp\nPT:1/1\n"+
		"S:Mode$ ReduceCost | ValidCard$ Creature | EffectZone$ Graveyard | Amount$ 1\nOracle:x\n")
	conv := card(t, "Name:Grave conv\nTypes:Creature Imp\nPT:1/1\n"+
		"S:Mode$ ManaConvert | ValidPlayer$ You | EffectZone$ Graveyard | ManaConversion$ AnyType->AnyColor\nOracle:x\n")

	count := func(e *Engine, name string, views []staticView) (n int) {
		for _, v := range views {
			if o := e.G.Obj(v.Source); o != nil && o.Face().Name == name {
				n++
			}
		}
		return n
	}
	run := func(t *testing.T, withJailer bool) (taxFresh, taxCached, convFresh, convCached int) {
		e := handEngine(t, tax, conv, jailer)
		ids := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
		e.emit(events.Event{Kind: events.MoveZone, Obj: ids[0], From: state.ZHand, To: state.ZGraveyard})
		e.emit(events.Event{Kind: events.MoveZone, Obj: ids[1], From: state.ZHand, To: state.ZGraveyard})
		if withJailer {
			e.emit(events.Event{Kind: events.MoveZone, Obj: ids[2], From: state.ZHand, To: state.ZBattlefield})
		}
		taxFresh = count(e, "Grave tax", e.scanCostStatics().reduce)
		convFresh = count(e, "Grave conv", viewsOf(e.scanManaConvSources(nil)))
		e.BeginDerivedReads()
		taxCached = 0
		convCached = count(e, "Grave conv", viewsOf(e.manaConvPrintedSources()))
		taxCached = count(e, "Grave tax", e.collectCostStatics().reduce)
		e.EndDerivedReads()
		return
	}
	t.Run("control-no-jailer", func(t *testing.T) {
		af, ac, cf, cc := run(t, false)
		if af == 0 || ac == 0 || cf == 0 || cc == 0 {
			t.Fatalf("control: graveyard statics must function without the Jailer: tax fresh/cached %d/%d, conv fresh/cached %d/%d", af, ac, cf, cc)
		}
	})
	t.Run("jailer", func(t *testing.T) {
		af, ac, cf, cc := run(t, true)
		if af != 0 || ac != 0 || cf != 0 || cc != 0 {
			t.Fatalf("under Yixlid Jailer: tax fresh/cached %d/%d, conv fresh/cached %d/%d, want all 0", af, ac, cf, cc)
		}
	})
}

func viewsOf(src []manaConvSource) []staticView {
	out := make([]staticView, len(src))
	for i := range src {
		out[i] = src[i].sv
	}
	return out
}
