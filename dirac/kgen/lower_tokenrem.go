package kgen

import "strings"

// tokRemSrcBit is a CreateToken lInstr.C bit: each minted token remembers the
// creating source (RememberSource$ True; kop.TokRememberSrc).
const tokRemSrcBit = 8192

// tokSacYourUpkeepBit is a Token lInstr's C bit for AtEOT$ YourSacrificeUpkeep
// (kop.TokAtEOTSacYourUpkeep in B.N's high byte).
const tokSacYourUpkeepBit = 32768 // 16384 is origin/main's tokAtkPlayerBit

// srcRemValue lowers the SVar body `Remembered$CardCounters.<KIND>` in a token's
// characteristic-defining static (Saproling Burst's token: "power and toughness
// are each equal to the number of fade counters on CARDNAME"). gorge reads the
// counters over the token's own Remembered list, which RememberSource$ fills
// with the creating source (effects/token.go tokenMemoryOf); the kernel reads
// the first live smRem record keyed by the token (kop.ExSrcRemCounters). Only the
// one-counter-kind read inside the token's own CDA static qualifies; every
// other Remembered$ spelling keeps its refusal.
func (lc *lowerCtx) srcRemValue(body string) (lValue, bool) {
	kind, ok := strings.CutPrefix(strings.TrimSpace(body), "Remembered$CardCounters.")
	if !ok || !lc.srcRemStatic || strings.EqualFold(kind, "ALL") {
		return lValue{}, false
	}
	return lValue{Kind: vSrcRemCounters, S: kind}, true
}

// tokenReadsSrcRem reports whether the token script's CDA reads the creator it
// remembers (a face SVar of the Remembered$CardCounters head).
func tokenReadsSrcRem(p *pool, script string) bool {
	c, ok := p.reg.Token(script)
	if !ok || len(c.Faces) == 0 {
		return false
	}
	for _, body := range c.Faces[0].SVars {
		if strings.HasPrefix(strings.TrimSpace(body), "Remembered$CardCounters.") {
			return true
		}
	}
	return false
}

// tokenMemory lowers a Token's memory riders: ImprintTokens$ (the imprinted
// set), then RememberSource$ (the token's own memory of its creator).
func (lc *lowerCtx) tokenMemory(ps *params, script string, in *lInstr) error {
	if err := lc.tokenImprint(ps, in); err != nil { // lower_imprintzone.go
		return err
	}
	return lc.tokenRememberSource(ps, script, in)
}

// tokenRememberSource lowers a Token's RememberSource$ True into in.C. Only a
// script whose CDA reads the remembered creator takes it, and a script that
// reads one needs it (without the memory gorge's read is 0); the rider shares
// the destination byte with AttachedTo$ and the AtEOT kinds, so those refuse.
func (lc *lowerCtx) tokenRememberSource(ps *params, script string, in *lInstr) error {
	reads := tokenReadsSrcRem(lc.p, script)
	v, has := ps.get("RememberSource")
	if !has {
		if reads {
			return refuse("Token script %q reads its remembered creator without RememberSource$", script)
		}
		return nil
	}
	if strings.TrimSpace(v) != "True" || !reads {
		return refuse("Token RememberSource$ %q on script %q", v, script)
	}
	if lc.repeatCard > 0 {
		// The RepeatEach Role body mints from a static site (dirac's
		// static_roletokens.go), where the kernel's tokenMemos reads the
		// creator from the resolving effect, which is not the token's
		// creator there: the record it would write is wrong or absent
		// (s.res inactive) while this bit tells the static read to expect
		// one. No card remembers a source on a static-created token.
		return refuse("Token RememberSource$ on a static-created token (RepeatEach Role body)")
	}
	if _, tok := ps.get("TokenRemembered"); tok {
		// gorge's tokenMemoryOf appends the TokenRemembered$ group AND the
		// source to each minted token's memory: the same source twice
		// counts its counters twice in the CDA's Remembered$CardCounters
		// read, while the kernel keeps one smRem record (counts once). No
		// card needs both.
		return refuse("Token RememberSource$ beside TokenRemembered$")
	}
	if in.C&(32|512|1024) != 0 {
		return refuse("Token RememberSource$ beside AttachedTo$ or a destination AtEOT$")
	}
	ps.consume("RememberSource")
	in.C |= tokRemSrcBit
	lc.out.SrcMem = true
	return nil
}
