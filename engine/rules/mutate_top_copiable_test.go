package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMutateTopCopiable pins CR 730.2a on a top merge: the merged permanent
// has only the characteristics of its topmost component, and that is itself a
// copiable effect timestamped at the merge, so it supersedes every copy effect
// applied to the pile before it (CR 613.2, 613.7). The old top's own copiable
// values (a token copy's copy basis, a clone layer, a spell copy's
// non-legendary rider) leave the pile with it and the new top's come with the
// mutating object; an earlier copy effect keeps applying (CR 730.2c) but can
// no longer win the layer-1 timestamp order.
func TestMutateTopCopiable(t *testing.T) {
	t.Parallel()
	const wurm = "Name:Copied Wurm\nManaCost:4 G\nTypes:Creature Wurm\nPT:6/6\nOracle:x\n"
	const mole = "Name:Fixture Mole\nManaCost:1 B\nTypes:Creature Mole\nPT:2/5\nOracle:x\n"

	// tokenCopy mints a token copy of a Mutate Bear that is a copy of the
	// Wurm: the token is minted on the Bear's card with the Wurm as its own
	// copy basis (CopyFace), as copyTokenBasis does.
	tokenCopy := func(t *testing.T, e *Engine) state.ObjID {
		t.Helper()
		wurmID := moveSeeded(t, e, 0, wurm, state.ZBattlefield)
		bear := moveSeeded(t, e, 0, mutateBearSrc, state.ZBattlefield)
		e.emit(events.Event{Kind: events.ClonePermanent, Obj: bear, IDs: []state.ObjID{wurmID}, Player: 0})
		tok := e.G.NextID
		e.emit(events.Event{Kind: events.CopyToken, Obj: bear, Player: 0})
		e.emit(events.Event{Kind: events.MoveZone, Obj: tok, From: state.ZLibrary, To: state.ZBattlefield})
		o := e.G.Obj(tok)
		if o == nil || !o.IsToken || !o.IsCopy || o.CopyFace == nil || o.Face().Name != "Copied Wurm" ||
			o.Card.Faces[0].Name != "Mutate Bear" || len(o.CloneLayers) != 0 {
			t.Fatalf("precondition: token %d = %+v, want a token copy of the Wurm minted on the Bear's card", tok, o)
		}
		return tok
	}

	t.Run("card on top of a token copy", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 403, card(t, mutateTesterSrc), card(t, wurm), card(t, mutateBearSrc))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		tok := tokenCopy(t, e)
		addMana(t, e, 0, "GG")
		mutateCastOnto(t, e, mutatedCastOption(t, e, mut), tok, true)
		pile := e.G.Obj(tok)
		if len(pile.MergedCards) != 1 {
			t.Fatalf("precondition: merged %+v, want the token copy under the Tester", pile.MergedCards)
		}
		if f := pile.Face(); f == nil || f.Name != "Apex Tester" || pile.CopyFace != nil {
			t.Fatalf("pile face %v (CopyFace %v), want the Tester's own face (CR 730.2a)", f, pile.CopyFace)
		}
		if d := e.Derived(tok); d.Power != 3 || d.Toughness != 3 {
			t.Fatalf("pile P/T %d/%d, want the Tester's 3/3", d.Power, d.Toughness)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("non-legendary spell copy on top", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 404, card(t, mutateTesterSrc))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
		addMana(t, e, 0, "GG")
		mutateCastOntoUndrained(t, e, mutatedCastOption(t, e, mut), bear, true)
		e.emit(events.Event{Kind: events.StackCopy, Obj: mut, Player: 0,
			Counter: events.StackCopyCounterString(events.StackCopyCounter{NonLegendary: true})})
		cp := e.G.Stack[len(e.G.Stack)-1]
		if c := e.G.Obj(cp); cp == mut || c == nil || !c.IsCopy || !c.CopyNonLegendary {
			t.Fatalf("precondition: stack top %d = %+v, want a non-legendary copy of the mutating spell", cp, e.G.Obj(cp))
		}
		for i := 0; i < 10 && len(e.G.Obj(bear).MergedCards) == 0; i++ {
			passPriorityOnce(t, e)
		}
		pile := e.G.Obj(bear)
		if len(pile.MergedCards) != 1 || pile.Face().Name != "Apex Tester" || !pile.Face().IsLegendary() {
			t.Fatalf("precondition: pile top %q merged %+v, want the legendary Tester's copy on top", pile.Face().Name, pile.MergedCards)
		}
		if !pile.CopyNonLegendary || legendaryUnderLayers(e, bear) {
			t.Fatalf("pile nonlegendary=%v legendary=%v, want the copy's non-legendary rider on the pile",
				pile.CopyNonLegendary, legendaryUnderLayers(e, bear))
		}
		// The original resolves on top of the copy: a real legendary card,
		// so the rider leaves with the copy and the pile is legendary again.
		passUntilStackEmpty(t, e, 40)
		pile = e.G.Obj(bear)
		if len(pile.MergedCards) != 2 || pile.CopyNonLegendary || !legendaryUnderLayers(e, bear) {
			t.Fatalf("pile merged %d nonlegendary=%v legendary=%v after the card went on top, want a legendary pile",
				len(pile.MergedCards), pile.CopyNonLegendary, legendaryUnderLayers(e, bear))
		}
		replayCheck(t, e, cfg)
	})

	t.Run("merge supersedes a clone effect on the pile", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 405, card(t, mutateTesterSrc), card(t, wurm), card(t, mutateBearSrc), card(t, mole))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		tok := tokenCopy(t, e)
		moleID := moveSeeded(t, e, 0, mole, state.ZBattlefield)
		e.emit(events.Event{Kind: events.ClonePermanent, Obj: tok, IDs: []state.ObjID{moleID}, Player: 0})
		if o := e.G.Obj(tok); o.Face().Name != "Fixture Mole" || len(o.CloneLayers) != 1 || !o.TokenBasisSaved {
			t.Fatalf("precondition: token %+v, want a Mole copy layer with its Wurm basis saved", o)
		}
		addMana(t, e, 0, "GG")
		mutateCastOnto(t, e, mutatedCastOption(t, e, mut), tok, true)
		pile := e.G.Obj(tok)
		if len(pile.MergedCards) != 1 {
			t.Fatalf("precondition: merged %+v, want one under-card", pile.MergedCards)
		}
		// CR 730.2a's timestamp sentence: the merge is the newest layer-1
		// copiable effect, so the pile shows its top, not the Mole.
		if f := pile.Face(); f == nil || f.Name != "Apex Tester" || pile.CopyFace != nil {
			t.Fatalf("pile face %v (CopyFace %v) right after the merge, want the Tester (CR 730.2a)", f, pile.CopyFace)
		}
		// The superseded effect still ends; that must not bring its face
		// back over the merge's later timestamp.
		e.emit(events.Event{Kind: events.ClonePermanent, Obj: tok, Player: 0})
		if f := e.G.Obj(tok).Face(); f == nil || f.Name != "Apex Tester" {
			t.Fatalf("pile face %v after the copy effect ended, want the Tester (CR 730.2a)", f)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("merge supersedes an as-enters clone", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 406, card(t, mutateTesterSrc), card(t, mole), card(t, mutateBearSrc))
		mut := moveSeededCard(t, e, 0, card(t, mutateTesterSrc), state.ZHand)
		bear := moveSeededCard(t, e, 0, card(t, mutateBearSrc), state.ZBattlefield)
		moleID := moveSeeded(t, e, 0, mole, state.ZBattlefield)
		e.emit(events.Event{Kind: events.ClonePermanent, Obj: bear, IDs: []state.ObjID{moleID},
			Player: 0, Counter: "gain-this-ability"})
		if o := e.G.Obj(bear); o.Face().Name != "Fixture Mole" || len(o.CloneLayers) != 1 || o.IsToken ||
			!o.CopyGainThisAbility {
			t.Fatalf("precondition: permanent %+v, want a Mole copy layer (GainThisAbility) on a real card", o)
		}
		addMana(t, e, 0, "GG")
		mutateCastOnto(t, e, mutatedCastOption(t, e, mut), bear, true)
		pile := e.G.Obj(bear)
		if len(pile.MergedCards) != 1 {
			t.Fatalf("precondition: merged %+v, want one under-card", pile.MergedCards)
		}
		if f := pile.Face(); f == nil || f.Name != "Apex Tester" || pile.CopyFace != nil {
			t.Fatalf("pile face %v (CopyFace %v) after merging onto a clone, want the Tester (CR 730.2a)", f, pile.CopyFace)
		}
		// The demoted top's GainThisAbility$ rider is its own copiable value:
		// it leaves with the component, like its copy basis.
		if pile.CopyGainThisAbility {
			t.Fatal("pile kept the demoted clone's GainThisAbility$ rider")
		}
		e.emit(events.Event{Kind: events.ClonePermanent, Obj: bear, Player: 0})
		if p := e.G.Obj(bear); p.Face() == nil || p.Face().Name != "Apex Tester" || p.CopyFace != nil ||
			p.CopyGainThisAbility {
			t.Fatalf("pile face %v (CopyFace %v, GainThisAbility %v) after the clone ended, want the Tester (CR 730.2a)",
				p.Face(), p.CopyFace, p.CopyGainThisAbility)
		}
		replayCheck(t, e, cfg)
	})
}
