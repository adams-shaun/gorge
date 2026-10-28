package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMayAskSeesGrantedOptionalReplacement: Pulmonic Sliver GRANTS every
// Sliver an optional Moved replacement through a Mode$ Continuous static
// (AddReplacementEffect$ PulmonicMoveToLibrary, Optional$ True). The granted
// line is not in any face's Repls, so a DestroyAll resolution whose text is
// ask-free used to be exempted from its checkpoint and then panicked at the
// replacement ask ("the ask-free predicate missed an ask"). The predicate
// must see the grant, and the resolution must pose the ask.
func TestMayAskSeesGrantedOptionalReplacement(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	pulmonic := mustCorpusCard(t, reg, "Pulmonic Sliver")
	muscle := mustCorpusCard(t, reg, "Muscle Sliver")
	wrath := card(t, "Name:Gate Wrath\nManaCost:0\nTypes:Sorcery\n"+
		"A:SP$ DestroyAll | ValidCards$ Sliver | SpellDescription$ Destroy all Slivers.\nOracle:x\n")

	// Precondition: the grant is exactly the shape this test is about, and
	// the control card carries none.
	if got := cards.FaceGrantedReplAskEvents(pulmonic.Faces[0]); !got.Has(cards.ReplMoved) {
		t.Fatalf("Pulmonic Sliver carries no granted elective Moved replacement: %#x", got)
	}
	if got := cards.FaceGrantedReplAskEvents(muscle.Faces[0]); got.Has(cards.ReplMoved) {
		t.Fatalf("Muscle Sliver unexpectedly carries a granted Moved replacement: %#x", got)
	}

	e, cfg := tokenReplGame(t, 7555083076110424585, pulmonic, muscle, wrath)
	moveSeededCard(t, e, 0, wrath, state.ZHand)
	pul := moveSeededCard(t, e, 0, pulmonic, state.ZBattlefield)
	mus := moveSeededCard(t, e, 0, muscle, state.ZBattlefield)
	if got := e.G.Obj(pul).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: Pulmonic Sliver zone = %v, want battlefield", got)
	}
	if got := e.G.Obj(mus).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: Muscle Sliver zone = %v, want battlefield", got)
	}

	addMana(t, e, 0, "")
	castSpellOption(t, e, wrath.Faces[0].Name)
	// The spell's own text is ask-free; the grant is what makes it ask.
	if sa := wrath.Faces[0].Abilities[0]; cards.SAChainMayAsk(sa, wrath.Faces[0].SVars, wrath.Faces[0], true) {
		t.Fatal("precondition: DestroyAll's text alone is not ask-free")
	}
	if !tapeMayAsk(e) {
		t.Fatal("the ask-free predicate exempted DestroyAll despite the granted optional replacement")
	}
	if asks := driveCountingReplacementAsks(t, e, 0); asks == 0 {
		t.Fatal("the granted optional replacement ask was not posed")
	}
	replayCheck(t, e, cfg)
}
