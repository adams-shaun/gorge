package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestCopiedSpellIsNotWasCastFromGraveyard pins the never-cast guard on the
// graveyard-origin cast provenance: a stack copy that inherited its source's
// FlagFlashback (events.Apply's StackCopy leaves the graveyard-origin bits
// inherited -- state.CastProvenanceFlags does not strip them) must NOT
// satisfy Card.wasCastFromGraveyard, because a copy is put on the stack and
// never cast (CR 707.10). The original flashback-cast spell still reads true.
//
// Real corpus carrier: Sevinne's Reclamation gates its CopySpellAbility sub
// on ConditionPresent$ Card.wasCastFromGraveyard; without the guard the copy
// poses its own may-copy election and recurses.
func TestCopiedSpellIsNotWasCastFromGraveyard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	original := corpusObject(t, reg, g, "Sevinne's Reclamation")
	original.CastFlags |= state.FlagFlashback

	// A real copy of a graveyard-origin cast: the StackCopy mint's two
	// defining bits, IsCopy and the inherited FlagFlashback.
	copied := corpusObject(t, reg, g, "Sevinne's Reclamation")
	copied.IsCopy = true
	copied.CastFlags |= state.FlagFlashback

	// PRECONDITION: both objects carry the flashback bit, so the ONLY
	// difference the predicate reads is IsCopy. A vacuous setup (the copy
	// flagged but the bit absent, or vice versa) must fail loudly here.
	if original.CastFlags&state.FlagFlashback == 0 {
		t.Fatalf("precondition: original CastFlags %#x lost FlagFlashback", original.CastFlags)
	}
	if copied.CastFlags&state.FlagFlashback == 0 {
		t.Fatalf("precondition: copy CastFlags %#x lost FlagFlashback", copied.CastFlags)
	}
	if !copied.IsCopy || original.IsCopy {
		t.Fatalf("precondition: IsCopy original=%v copy=%v, want false/true",
			original.IsCopy, copied.IsCopy)
	}

	// The ONE home, read directly.
	if !state.ObjectWasCastFromGraveyard(original) {
		t.Error("the original flashback cast must read as cast from a graveyard")
	}
	if state.ObjectWasCastFromGraveyard(copied) {
		t.Error("a stack copy must NOT read as cast from a graveyard (CR 707.10: a copy is never cast)")
	}
	if state.ObjectWasCastFromGraveyard(nil) {
		t.Error("a nil object must read false")
	}

	// The predicate as a card script sees it: the compiled predicate path
	// (Card.wasCastFromGraveyard) the Sevinne condition gate evaluates.
	if !MatchesObjectCtx(g, "Card.wasCastFromGraveyard", original, SpecContext{You: 0}) {
		t.Error("Card.wasCastFromGraveyard must match the original flashback cast")
	}
	if MatchesObjectCtx(g, "Card.wasCastFromGraveyard", copied, SpecContext{You: 0}) {
		t.Error("Card.wasCastFromGraveyard must not match a stack copy")
	}
	// The negation is the read the condition's false branch needs: the copy
	// satisfies it, the original does not.
	if !MatchesObjectCtx(g, "Card.!wasCastFromGraveyard", copied, SpecContext{You: 0}) {
		t.Error("Card.!wasCastFromGraveyard must match a stack copy")
	}
}
