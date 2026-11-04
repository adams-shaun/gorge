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
// replacements matching one token make gorge ask the CR 616.1 order. The
// kernel poses no such ask mid-resolution: the game stalls
// (StallTokRewrite). A mint site that makes the card it was given (amass,
// incubate, empower, endure, recruit, the copy sites) stalls the same way
// when a rewriter applies; the CreateToken ops read the rewritten card. A Role
// mint (TokAttach) stalls the same way: gorge's applyTokenReplacementToPlan
// turns it into a plain script mint of the rewriter's card (the Illusion is no
// Aura, so auraTokenWithheld withholds nothing), which the kernel's attach path
// (createAttachedToken, a host for the card) does not follow.

// replTokensCard is replTokens for the CreateToken ops: the card the tokens
// are (tok, or the rewriter's) and how many. ok is false when the game
// stalled.
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
// the same token (gorge asks the CR 616.1 order), or at a mint site that
// does not read the rewritten card.
const StallTokRewrite = 135

func init() { stallNames[StallTokRewrite] = "tokrewrite" }

// replTokensRew is replTokens for a table with a rewriter: the card and the
// number of tokens an effect creating n tokens of card tok under p's control
// creates. ok is false when the game stalled on the order ask (the other
// results are then tok and 0).
func (s *State) replTokensRew(k *prep, p uint8, tok CardID, n int) (CardID, int, bool) {
	g := k.gen
	if n <= 0 || p > 1 {
		return tok, n, true
	}
	card, cands, rew := tok, 0, 0
	mult := n
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
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
	if rew > 0 && cands > 1 {
		s.capFull(StallTokRewrite)
		return tok, 0, false
	}
	if rew > 0 {
		return card, n, true
	}
	return tok, mult, true
}
