package kgen

import (
	"strings"
)

// tokListBit is a CreateToken lInstr.C bit: the token is one of a TokenScript$
// list (kop.TrList).
const tokListBit = 4096

// tokenList lowers a comma-listed TokenScript$ (Reckoner Bankbuster's
// "c_a_treasure_sac,c_1_1_pilot_crewbuff"): gorge's runTokenMints creates
// each listed script in order, TokenAmount$ of each, inside one zone batch,
// which the kernel's per-resolution token batch already is. Only the plain
// shape is read: every rider beside the list (owner, amount, tapped,
// attacking, attached, remembered, AtEOT) refuses, except TokenOwner$ You: the
// resolving controller is gorge's default owner (tokenOwnerPlayers), so the
// spelling is the plain list (Specimen Collector, Fae Offering).
func (lc *lowerCtx) tokenList(ps *params, list string) error {
	if o, ok := ps.get("TokenOwner"); ok && o == "You" {
		ps.consume("TokenOwner")
	} else if ok {
		return refuse("TokenScript list %q with TokenOwner$", list)
	}
	for _, k := range []string{"TokenAmount", "TokenTapped", "TokenAttacking",
		"AttachedTo", "RememberTokens", "RememberOriginalTokens", "AtEOT", "ImprintTokens"} {
		if _, ok := ps.m[k]; ok {
			return refuse("TokenScript list %q with %s$", list, k)
		}
	}
	ps.consume("TokenScript", "LockTokenScript")
	var scripts []string
	for _, s := range strings.Split(list, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			return refuse("TokenScript %q", list)
		}
		if _, ok := lc.p.reg.Token(s); !ok {
			return refuse("token script %q not in the corpus", s)
		}
		if tokenIsAura(lc.p, s) {
			return refuse("Aura token %q in a TokenScript list", s)
		}
		scripts = append(scripts, s)
	}
	for _, s := range scripts {
		lc.out.Tokens = append(lc.out.Tokens, s)
		lc.emit(lInstr{Op: "CreateToken", Obj: lRef{Kind: rYou}, A: cst(1), CS: s, C: tokListBit})
	}
	return nil
}
