package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DB$ Token TokenPower$/TokenToughness$ (gorge
// effects/token.go: both sides resolve once BEFORE the mint loop, then one
// permanent layer-7b set per mint, sourced to the token). Cold and out of
// line: only an ExtTokenPT instruction reaches it. The Ext sits right
// BEFORE the CreateToken (Ex form): it reads override C's P/T first, so a
// count over the creatures the token itself would match sees the board
// without it, then runs the CreateToken in place (skipping it in the
// program), and sets the token the mint left in res.lastTok inside the same
// resolution, before any SBA check (CR 704.3), like Endure's Spirit.

// StallTokenPT: a TokenPower$ mint whose count is not the op's amount (a
// CreateToken replacement multiplied or cut it; no pool card has a token
// doubler).
const StallTokenPT = 111

// DebugTokenXPTSkip is the mutation switch of TestTokenxptLockstep: the set
// is not applied.
var DebugTokenXPTSkip bool

func init() { stallNames[StallTokenPT] = "tokenpt" }

// tokenPT is ExtTokenPT: read override C's P/T, run the CreateToken that
// follows, then set the token it minted to those values.
//
//go:noinline
func (s *State) tokenPT(k *prep, in *kop.Instr) {
	p, t := s.overridePT(k, int(in.C)) // before the mint (gorge resolves once)
	s.res.pc++                         // the CreateToken; runProg steps past it
	next := &s.prog(k)[s.res.pc]
	want := 0 // the mint's amount: every token of it takes the set (gorge sets each mint)
	if next.Op == kop.OpCreateTokenEx {
		want = s.value(k, next.A)
	}
	s.g.tokPT.on, s.g.tokPT.idx, s.g.tokPT.p, s.g.tokPT.t = true, uint16(in.C), p, t
	s.g.tokPT.left = uint8(min(max(want, 1), 255) - 1)
	ok := next.Op == kop.OpCreateTokenEx && s.exec(k, next)
	applied := !s.g.tokPT.on // every mint's entry took it (enterGen)
	s.g.tokPT.on, s.g.tokPT.left = false, 0
	if !ok {
		s.capFull(StallTokenPT) // kgen emits the pair and refuses an asking rider
		return
	}
	if s.over {
		return
	}
	n := int(s.g.tokMade)
	switch {
	case n == 0:
		return
	case n != want && (n > 1 || want > 1):
		s.capFull(StallTokenPT)
		return
	case n > 1:
		if !applied {
			s.capFull(StallTokenPT)
		}
		return
	}
	if DebugTokenXPTSkip || applied {
		return
	}
	o := s.res.lastTok
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return
	}
	s.applyOverridePT(k, s.objs[o].slot, int(in.C), s.res.e.card, 0, p, t)
}
