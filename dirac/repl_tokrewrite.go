package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplRewriteTokens: "If one or more tokens would be created under your
// control, that many <script> tokens are created instead" (Holo-Engineer; a
// ReplaceToken body with Type$ ReplaceToken and one fixed TokenScript$; gorge
// replacement_token.go applyTokenReplacementToPlan, mint by mint). The count
// is kept and the card becomes Repl.C (a pool token index the matchup
// remaps). Only the tokens the replacement's controller creates are
// rewritten: replTokens is asked for that player's tokens and walks that
// player's battlefield.
//
// A rewriter does not commute with a multiplier or with another rewriter
// (gorge tokenReplacementsCommute: a rewriter composed after a multiplier
// rewrites every duplicated mint, before it only the original), so two
// replacements matching one token make gorge ask the CR 616.1 order. A
// CreateToken op poses that ask to the creator, token by token
// (repl_tokorder.go); every other mint site reads the rewritten card (amass,
// incubate, empower, endure and recruit mint it with their riders, the copy
// sites mint it as a plain untapped token, copyTokenRewrite, whatever riders
// the copy instruction carries) but cannot suspend, so a creation contested
// there stalls (StallTokRewrite). A Role
// mint (TokAttach) stalls the same way: gorge's applyTokenReplacementToPlan
// turns it into a plain script mint of the rewriter's card (the Illusion is no
// Aura, so auraTokenWithheld withholds nothing), which the kernel's attach path
// (createAttachedToken, a host for the card) does not follow.

// replTokensCard is replTokens for the mint sites: the card the tokens are
// (tok, or the rewriter's) and how many. ok is false when the game stalled
// (the creation is contested: the CreateToken ops pose the order ask first,
// replTokensOp).
func (s *State) replTokensCard(k *prep, p uint8, tok CardID, n int) (CardID, int, bool) {
	if k.gen != nil && k.gen.anyTokRew && k.gen.anyRepl&rbDblTok != 0 && n > 0 && p <= 1 {
		return s.replTokensRew(k, p, tok, n)
	}
	return tok, s.replTokens(k, p, tok, n), true
}

// DebugTokRewriteOff is a mutation hook for TestHoloEngineerMutation: the
// rewriters are ignored, so every token keeps its own script.
var DebugTokRewriteOff bool

// StallTokRewrite: a CreateToken rewriter beside another token replacement of
// the same token at a mint site that cannot pose gorge's CR 616.1 order ask (a
// CreateToken op can, repl_tokorder.go), or a Role mint a rewriter replaces.
const StallTokRewrite = 135

func init() { stallNames[StallTokRewrite] = "tokrewrite" }

// replTokensRew is replTokens for a table with a rewriter: the card and the
// number of tokens an effect creating n tokens of card tok under p's control
// creates. ok is false when the game stalled on the order ask the site cannot
// pose (the other results are then tok and 0), or on a ReplAddToken the site
// does not mint the extra token of (addsOK: the site does, mintTokAdds, and
// only a lone adder is left to it).
func (s *State) replTokensRew(k *prep, p uint8, tok CardID, n int) (CardID, int, bool) {
	card, m, _, ok := s.replTokensRewApplied(k, p, tok, n, false)
	return card, m, ok
}

// replTokensRewApplied is replTokensRew that also reports whether a rewriter
// applied: a copy of the rewriter's own card (an Illusion token) keeps its
// card but is still rewritten to a plain, untapped, non-copy mint.
func (s *State) replTokensRewApplied(k *prep, p uint8, tok CardID, n int, addsOK bool) (CardID, int, bool, bool) {
	g := k.gen
	if n <= 0 || p > 1 {
		return tok, n, false, true
	}
	card, cands, rew, adds := tok, 0, 0, 0
	mult := n
	for m := s.tokSrcM(p); m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbDblTok == 0 || s.genLost(k, b) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(b)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement {
				continue
			}
			switch a.Repl.Kind {
			case kop.ReplRewriteTokens:
				if DebugTokRewriteOff {
					continue
				}
				cands++
				rew++
				card = CardID(a.Repl.C)
			case kop.ReplAddToken:
				// the extra token is minted by a CreateToken op and the
				// single copy site (mintTokAdds); any other mint site stalls
				if a.Repl.Filter != 0 && !s.matchCardGen(k, a.Repl.Filter, tok, p, p, NoObj) {
					continue
				}
				cands++
				adds++
			case kop.ReplDoubleTokens:
				if a.Repl.Filter != 0 && !s.matchCardGen(k, a.Repl.Filter, tok, p, p, NoObj) {
					continue
				}
				cands++
				x := 2
				if a.Repl.A > 2 {
					x = int(a.Repl.A)
				}
				if mult < 1<<8 {
					mult *= x
				}
			}
		}
	}
	if adds > 0 && (!addsOK || cands > adds) {
		s.capFull(StallTokRewrite)
		return tok, 0, false, false
	}
	if rew > 0 && cands > 1 {
		s.capFull(StallTokRewrite)
		return tok, 0, false, false
	}
	if rew > 0 {
		return card, n, true, true
	}
	return tok, mult, false, true
}

// copyTokenRewrite is the copy sites' read of the rewriter (execCopyToken):
// the number of copies and, when a CreateToken rewriter applies, the card
// that replaces them (gorge continueCopyTokenProposal: each copy mint becomes
// a plain script mint of the rewriter's card, untapped and no copy). The
// effect is then left with no copy, so its per-copy riders (TokenTapped$,
// PumpKeywords$, RememberTokens$, AtEOTTrig$, AtEOT$ Exile) have nothing to
// act on, and the one AtEOT$ Sacrifice group is registered over no object
// (execCopyToken's delayEmpty; gorge effCopyPermanent, CR 603.7). ok is false
// when the game stalled. addsOK: the site mints a lone ReplAddToken's extra
// token itself (mintTokAdds), else the adder stalls.
func (s *State) copyTokenRewrite(k *prep, p uint8, tok CardID, n int, addsOK bool) (card CardID, cnt int, rewritten, ok bool) {
	if k.gen == nil || !k.gen.anyTokRew || k.gen.anyRepl&rbDblTok == 0 || n <= 0 || p > 1 {
		return tok, s.replTokens(k, p, tok, n), false, true
	}
	return s.replTokensRewApplied(k, p, tok, n, addsOK)
}

// mintRewritten creates n plain untapped non-copy tokens of the rewriter's
// card tc under p (a copy mint a CreateToken rewriter replaced), as the
// CreateToken op does. True when the board is full (a stall).
func (s *State) mintRewritten(k *prep, p uint8, tc CardID, n int) bool {
	in := kop.Instr{Op: kop.OpCreateToken, C: int16(tc)}
	if n > 1 {
		return s.createTokensHeld(k, &in, p, n, nil)
	}
	for i := 0; i < n; i++ {
		if s.nearCapacity(k) {
			s.stall(StallBoard)
			return true
		}
		o := s.newToken(tc, p)
		sl := s.enter(k, o, p, false, kop.ZoneLibrary)
		if k.gen.anyTokCr {
			s.fireTokenCreated(k, sl)
		}
		s.res.lastTok = o
	}
	return false
}
