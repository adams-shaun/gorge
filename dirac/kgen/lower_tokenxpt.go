package kgen

import "strings"

// DB$ Token with TokenPower$/TokenToughness$ ("create an X/X
// token"). gorge (effects/token.go) resolves both sides once before the mint
// loop and, after each mint enters, adds one permanent layer-7b set
// (SetPower/SetToughness, sourced to the token) on it. The kernel mints the
// tokens, then OpExt ExtTokenPT applies the same set from override C, built
// exactly as Endure's (lower_endure.go), to each of the TokenAmount$ mints
// (Gelatinous Genesis's X). Only the exact shape lowers: both sides, one
// creature script, no AttachedTo$; everything else refuses by name.

// tokenXPTGate refuses the token shapes the TokenPower$/TokenToughness$
// rider cannot carry; it reports whether the rider is present. script is the
// raw TokenScript$.
func (lc *lowerCtx) tokenXPTGate(ps *params, script string) (bool, error) {
	_, hp := ps.m["TokenPower"]
	_, ht := ps.m["TokenToughness"]
	if !hp && !ht {
		return false, nil
	}
	if hp != ht {
		return false, refuse("Token TokenPower$/TokenToughness$ with only one side")
	}
	if strings.Contains(script, ",") {
		return false, refuse("Token TokenPower$ over a token list")
	}
	if _, ok := ps.m["AttachedTo"]; ok {
		return false, refuse("Token TokenPower$ beside AttachedTo$")
	}
	c, ok := lc.p.reg.Token(script)
	if !ok || len(c.Faces) == 0 {
		return false, refuse("token script %q not in the corpus", script)
	}
	creature := false
	for _, t := range c.Faces[0].Types {
		if t == "Creature" {
			creature = true
		}
	}
	if !creature {
		return false, refuse("Token TokenPower$ on a non-creature token %q", script)
	}
	return true, nil
}

// tokenXPTOverride reads both sides through lc.value and records the
// layer-7b set as an override, returning its index.
func (lc *lowerCtx) tokenXPTOverride(ps *params) (int, error) {
	p, err := lc.value(ps.str("TokenPower"))
	if err != nil {
		return 0, err
	}
	t, err := lc.value(ps.str("TokenToughness"))
	if err != nil {
		return 0, err
	}
	lc.out.overrides = append(lc.out.overrides, &lOverride{HasSetPT: true, PVal: p, TVal: t})
	return len(lc.out.overrides) - 1, nil
}
