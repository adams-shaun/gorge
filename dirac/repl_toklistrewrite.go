package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A CreateToken rewriter with a comma-listed TokenScript$ (Academy Manufactor:
// "If you would create a Clue, Food, or Treasure token, instead create one of
// each"; gorge replacement_token.go applyTokenReplacementToPlan, ReplaceToken
// arm): every matching planned mint becomes one mint per listed script, in
// list order, so n tokens the effect would create make n runs of the whole
// list, token by token. The kernel reads it as a kop.ReplRewriteTokens whose
// Repl.C is the first card, Repl.Filter the ValidToken$ filter on the token
// created, and whose ability program holds one OpCreateToken per further card
// (the matchup remaps each as it does any CreateToken's).
//
// Only a CreateToken op mints it, and only alone: a second matching token
// replacement (a multiplier, another rewriter, an adder or a copy election) is
// gorge's CR 616.1 order ask, which composes with a list in ways the ask's
// walk (repl_tokorder.go) does not carry, so that creation stalls
// (StallTokRewrite). The other mint sites stall on a matching list rewriter
// (replTokensRewApplied).

// DebugTokListRewriteOff is a mutation hook for TestDemoTokenscriptListMutation:
// the list rewriters are ignored, so every token keeps its own script.
var DebugTokListRewriteOff bool

// TokListStats counts what the demo tokenscript-list lockstep proves fired:
// Rewrites is the CreateToken ops a list rewriter replaced, EnchantedMints the
// CreateToken ops that named the player the source Aura enchants
// (kop.RefEnchantedPlayer) and made tokens for them.
type TokListStats struct{ Rewrites, EnchantedMints int }

// DebugTokListStats, when set, receives the counts (a test hook; nil costs a
// compare).
var DebugTokListStats *TokListStats

// maxTokList is how many cards a list rewriter holds (kgen refuses longer).
const maxTokList = 8

// tokRewMatches reports whether rewriter r applies to a token of card tok
// created under p: its ValidToken$ filter (none: every token) accepts the card
// (DebugTokListRewriteOff: no list rewriter does).
func (s *State) tokRewMatches(k *prep, a *kop.Ability, tok CardID, p uint8) bool {
	if DebugTokListRewriteOff && len(a.Prog) > 0 {
		return false
	}
	return a.Repl.Filter == 0 || s.matchCardGen(k, a.Repl.Filter, tok, p, p, NoObj)
}

// tokListRewrite is a CreateToken op's read of a list rewriter: handled is
// true when one matches the op's token (the op made its tokens, or the game
// stalled: ok is the exec result). Not handled: no list rewriter matches, and
// the ordinary walk (replTokensOp) takes the op.
func (s *State) tokListRewrite(k *prep, in *kop.Instr, p uint8, n int) (handled, ok bool) {
	if n <= 0 || p >= MaxSeats {
		return false, false
	}
	tok := CardID(in.C)
	var t tokOrd
	_, ncopy, over := s.tokCands(k, p, tok, &t)
	var list *kop.Ability
	for j := 0; j < int(t.nc); j++ {
		if a := s.tokAbilityOf(k, &t, j); a.Repl.Kind == kop.ReplRewriteTokens && len(a.Prog) > 0 {
			list = a
			break
		}
	}
	if list == nil {
		return false, false
	}
	if over {
		s.capFull(StallReplCands) // loud: never truncate the candidates
		return true, true
	}
	_, na, addOver := s.tokAddCards(k, p, tok)
	if t.nc > 1 || ncopy > 0 || na > 0 || addOver || len(list.Prog)+1 > maxTokList || s.g.tokPT.on ||
		in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttach != 0 {
		s.capFull(StallTokRewrite)
		return true, true
	}
	var cs [maxTokList]CardID
	cs[0] = CardID(list.Repl.C)
	for i := range list.Prog {
		cs[i+1] = CardID(list.Prog[i].C)
	}
	if in.Op == kop.OpCreateTokenEx {
		s.g.tokMade = 0
	}
	if DebugTokListStats != nil {
		DebugTokListStats.Rewrites++
	}
	if s.createTokensSeq(k, in, p, n*(len(list.Prog)+1), cs[:len(list.Prog)+1], true) {
		return true, true
	}
	s.g.tokDefSet = false
	return true, true
}
