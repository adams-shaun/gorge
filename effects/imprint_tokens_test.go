package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestImprintTokensBindsCreatedTokensToTheSource pins the ImprintTokens$ True
// class end to end at the effects boundary: Forge's TokenEffect writes the
// created TOKENS into the SOURCE's imprintedCards, so a following
// `Defined$ Imprinted` names the token. All 14 corpus carriers of
// `ImprintTokens$ True` (measured with GNU grep over .cards/cardsfolder at
// the current FORGE_REF) read the association back in that same direction:
// 11 through `Defined$ Imprinted`/`ImprintCards$ Imprinted` (Timothar's
// DBAnimate grant, Intrude on the Mind's DBPutCounters, Ugin the
// Ineffable's DBEffect, ...) and 3 through the sibling spellings
// `RememberObjects$ ImprintedLKI` (Kharasha Foothills, Shredder, Shadow
// Master) and `AttachedTo$ Imprinted` (Stangg, Echo Warrior). So pinning the
// association on one authored shape covers the class rather than one card.
// The three sibling spellings are NOT covered by this test, and the two
// CopyPermanent carriers among them (Kharasha, Shredder) do not reach this
// code at all -- effCopyPermanent reads no ImprintTokens$ (the
// (copyperm-grants) row in AGENTS.md's Known approximations).
//
// This test can fail two ways and each is the point:
//   - the source carries no token imprint (the old behaviour, which imprinted
//     the TOKEN with the resolution's remembered cards instead), and
//   - `Defined$ Imprinted` resolves to a remembered card rather than the
//     token.
func TestImprintTokensBindsCreatedTokensToTheSource(t *testing.T) {
	h, c := fixtureHostWithTokens(t) // Game.Tokens: r_1_1_goblin, ...
	src := h.Game().Obj(c.Source)
	if src == nil {
		t.Fatalf("precondition: source %d must exist, got %+v", c.Source, src)
	}
	// A remembered card the association must NOT resolve `Imprinted` to: the
	// pre-rework code imprinted this onto the TOKEN. It is a distinct object
	// from the source, so the assertions below cannot pass vacuously.
	remembered := h.Game().AddObject(mkCard(t, "Name:Exiled Card\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0).ID
	if remembered == c.Source || remembered == 0 {
		t.Fatalf("precondition: remembered object %d must differ from the source %d", remembered, c.Source)
	}
	c.Remembered = []state.Target{{Obj: remembered}}

	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{
		"TokenScript": "r_1_1_goblin", "ImprintTokens": "True",
	}})

	bf := h.Game().Zone(state.ZBattlefield, c.Controller)
	if len(bf) != 1 {
		t.Fatalf("battlefield = %v, want exactly one minted token", bf)
	}
	token := bf[0]
	if o := h.Game().Obj(token); o == nil || !o.IsToken {
		t.Fatalf("setup did not mint a token: %+v", o)
	}

	// The SOURCE carries the token imprint (Forge's imprintedCards).
	if got := h.Game().Obj(c.Source).ImprintTokens; len(got) != 1 || got[0] != token {
		t.Fatalf("source ImprintTokens = %v, want [%d] (the created token)", got, token)
	}
	// And Defined$ Imprinted resolves to that token, never the remembered card.
	got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Imprinted"}})
	foundToken := false
	for _, tgt := range got {
		if tgt.IsPlayer {
			continue
		}
		if tgt.Obj == remembered {
			t.Fatalf("Defined$ Imprinted named the remembered card %d; want the created token %d (got %+v)",
				remembered, token, got)
		}
		if tgt.Obj == token {
			foundToken = true
		}
	}
	if !foundToken {
		t.Fatalf("Defined$ Imprinted = %+v, want the created token %d", got, token)
	}
}
