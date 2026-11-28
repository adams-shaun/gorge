package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins which of CR 702.140e ("A mutated permanent has all abilities
// of each card and token that represents it") and CR 707.2 (a copy takes the
// copied rules text) decides the keywords of a merged permanent under a copy
// effect: they are copiable effects applied in timestamp order (CR 730.2a,
// 613.2a), so the later of the merge and the copy effect wins.

const (
	copiedPileWurmSrc  = "Name:Copied Wurm\nManaCost:4 G\nTypes:Creature Wurm\nPT:6/6\nK:Trample\nOracle:x\n"
	copiedPileUnderSrc = "K:Flying\nK:Ward:2\n"
)

// wantPileKeywords fails unless the pile derives exactly the keyword lines
// want, in any order, and the filter and single-head keyword reads agree on
// flying and ward.
func wantPileKeywords(t *testing.T, e *Engine, pile state.ObjID, want ...string) {
	t.Helper()
	got := slices.Clone(e.Derived(pile).Keywords)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("derived keywords %q, want %q", got, want)
	}
	if m, w := e.matchesSpecFrom("Creature.withFlying", pile, 0, 0), slices.Contains(want, "Flying"); m != w {
		t.Fatalf("Creature.withFlying matches the pile = %v, want %v", m, w)
	}
	if h, w := e.hasKeywordH(pile, kwHeadOf("Ward")), slices.Contains(want, "Ward:2"); h != w {
		t.Fatalf("hasKeywordH(Ward) = %v, want %v", h, w)
	}
}

// becomeCopyOf applies one copy effect to id and requires it to have shown the
// source's face as a new layer.
func becomeCopyOf(t *testing.T, e *Engine, id, src state.ObjID) {
	t.Helper()
	before := len(e.G.Obj(id).CloneLayers)
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: id, IDs: []state.ObjID{src}, Player: 0})
	o := e.G.Obj(id)
	if o.CopyFace == nil || o.Face().Name != e.G.Obj(src).Face().Name || len(o.CloneLayers) != before+1 {
		t.Fatalf("precondition: %d did not become a copy of %d: %+v", id, src, o)
	}
}

// TestMutatedPileThatBecomesACopyHasOnlyTheCopiedKeywords: a copy effect newer
// than the merge replaces the under cards' keyword lines with the copied
// face's, and its end gives them back.
func TestMutatedPileThatBecomesACopyHasOnlyTheCopiedKeywords(t *testing.T) {
	t.Parallel()
	e, cfg := tokenReplGame(t, 461, card(t, mutateKeywordCrownSrc), card(t, copiedPileWurmSrc),
		card(t, "Name:Keyword Bearer\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"+copiedPileUnderSrc+"Oracle:x\n"))
	crown := moveSeededCard(t, e, 0, card(t, mutateKeywordCrownSrc), state.ZHand)
	pile := moveSeeded(t, e, 0, "Name:Keyword Bearer\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"+copiedPileUnderSrc+"Oracle:x\n", state.ZBattlefield)
	wurm := moveSeeded(t, e, 0, copiedPileWurmSrc, state.ZBattlefield)
	addMana(t, e, 0, "GG")
	mutateCastOnto(t, e, mutatedCastOption(t, e, crown), pile, true)
	mutateDrain(t, e, 40)
	if o := e.G.Obj(pile); o.Face().Name != "Keyword Crown" || len(o.MergedCards) != 1 {
		t.Fatalf("precondition: want the Crown on top of the Bearer, got %+v", o)
	}
	wantPileKeywords(t, e, pile, "Mutate:1 G", "Flying", "Ward:2")

	becomeCopyOf(t, e, pile, wurm)
	if n := len(e.G.Obj(pile).MergedCards); n != 1 {
		t.Fatalf("precondition: the copy effect left %d under cards, want 1", n)
	}
	wantPileKeywords(t, e, pile, "Trample")

	e.emit(events.Event{Kind: events.ClonePermanent, Obj: pile, Player: 0})
	if o := e.G.Obj(pile); o.CopyFace != nil || o.Face().Name != "Keyword Crown" {
		t.Fatalf("precondition: the copy effect did not end: %+v", o)
	}
	wantPileKeywords(t, e, pile, "Mutate:1 G", "Flying", "Ward:2")
	replayCheck(t, e, cfg)
}

// TestMergeUnderACopyKeepsTheUnderKeywords: a merge newer than the copy effect
// (CR 730.2a's timestamp sentence) keeps the copy's face on top and adds the
// under cards' keywords. A later copy effect removes them, and when it ends
// leaving the older layer, they are back.
func TestMergeUnderACopyKeepsTheUnderKeywords(t *testing.T) {
	t.Parallel()
	const under = "Name:Keyword Burrower\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\n" +
		"K:Mutate:1 G\n" + copiedPileUnderSrc + "Oracle:x\n"
	const mole = "Name:Fixture Mole\nManaCost:1 B\nTypes:Creature Mole\nPT:2/5\nK:Deathtouch\nOracle:x\n"
	e, cfg := tokenReplGame(t, 462, card(t, under), card(t, copiedPileWurmSrc), card(t, mutateBearSrc), card(t, mole))
	mut := moveSeededCard(t, e, 0, card(t, under), state.ZHand)
	pile := moveSeeded(t, e, 0, mutateBearSrc, state.ZBattlefield)
	wurm := moveSeeded(t, e, 0, copiedPileWurmSrc, state.ZBattlefield)
	moleID := moveSeeded(t, e, 0, mole, state.ZBattlefield)
	becomeCopyOf(t, e, pile, wurm)
	older := e.G.Obj(pile).CloneSeq

	addMana(t, e, 0, "GG")
	mutateCastOnto(t, e, mutatedCastOption(t, e, mut), pile, false)
	mutateDrain(t, e, 40)
	if o := e.G.Obj(pile); o.Face().Name != "Copied Wurm" || len(o.MergedCards) != 1 {
		t.Fatalf("precondition: want the Burrower under the Wurm copy, got %+v", o)
	}
	merged := []string{"Trample", "Mutate:1 G", "Flying", "Ward:2"}
	wantPileKeywords(t, e, pile, slices.Clone(merged)...)

	becomeCopyOf(t, e, pile, moleID)
	wantPileKeywords(t, e, pile, "Deathtouch")

	e.emit(events.Event{Kind: events.ClonePermanent, Obj: pile, Player: 0,
		Text: events.CloneLiveText([]int32{older})})
	if o := e.G.Obj(pile); o.Face().Name != "Copied Wurm" || len(o.CloneLayers) != 1 {
		t.Fatalf("precondition: want the older Wurm layer alone, got %+v", o)
	}
	wantPileKeywords(t, e, pile, slices.Clone(merged)...)
	replayCheck(t, e, cfg)
}

// TestMergeUnderATokenCopyKeepsTheUnderKeywords: a token copy's own copiable
// values are older than the merge and are no copy effect over it.
func TestMergeUnderATokenCopyKeepsTheUnderKeywords(t *testing.T) {
	t.Parallel()
	const under = "Name:Keyword Burrower\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\n" +
		"K:Mutate:1 G\n" + copiedPileUnderSrc + "Oracle:x\n"
	e, cfg := tokenReplGame(t, 463, card(t, under), card(t, copiedPileWurmSrc), card(t, mutateBearSrc))
	mut := moveSeededCard(t, e, 0, card(t, under), state.ZHand)
	wurm := moveSeeded(t, e, 0, copiedPileWurmSrc, state.ZBattlefield)
	bear := moveSeeded(t, e, 0, mutateBearSrc, state.ZBattlefield)
	becomeCopyOf(t, e, bear, wurm)
	tok := e.G.NextID
	e.emit(events.Event{Kind: events.CopyToken, Obj: bear, Player: 0})
	e.emit(events.Event{Kind: events.MoveZone, Obj: tok, From: state.ZLibrary, To: state.ZBattlefield})
	if o := e.G.Obj(tok); o == nil || !o.IsToken || o.CopyFace == nil || len(o.CloneLayers) != 0 {
		t.Fatalf("precondition: want a token copy of the Wurm, got %+v", o)
	}
	addMana(t, e, 0, "GG")
	mutateCastOnto(t, e, mutatedCastOption(t, e, mut), tok, false)
	mutateDrain(t, e, 40)
	if o := e.G.Obj(tok); o.Face().Name != "Copied Wurm" || len(o.MergedCards) != 1 {
		t.Fatalf("precondition: want the Burrower under the token copy, got %+v", o)
	}
	wantPileKeywords(t, e, tok, "Trample", "Mutate:1 G", "Flying", "Ward:2")
	replayCheck(t, e, cfg)
}
