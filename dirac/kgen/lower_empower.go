package kgen

import (
	"strings"
)

// empower lowers api:Empower (gorge effEmpower, CR 701.71a): the resolving
// controller puts Num$ (default 1) loyalty counters on one Jace token they
// control; with none, they first create the u_empower token (a blue 0-loyalty
// Jace planeswalker token), which gets the Type$ subtype and the
// "<Type$> Token" name as permanent grants. The corpus is entirely
// Type$ Jace; any other Type$ refuses (the per-type token script
// u_empower_<type> is not in the corpus, so the exact grant is unverified).
// Defined$ refuses: gorge's default is You and no carrier names another
// player. The token is created even when Num$ is 0 (it then dies to the
// planeswalker SBA), unlike amass's early return.
func (lc *lowerCtx) empower(ps *params) error {
	if d, ok := ps.get("Defined"); ok {
		return refuse("Empower Defined$ %q", d)
	}
	typ := strings.TrimSpace(ps.str("Type"))
	if typ != "Jace" {
		return refuse("Empower Type$ %q", typ)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	key := "u_empower"
	if _, ok := lc.p.reg.Token(key); !ok {
		return refuse("token script %q not in the corpus", key)
	}
	name := typ + " Token"
	lc.out.overrides = append(lc.out.overrides, &lOverride{AddSubtypes: []string{typ}, Name: name})
	in := lInstr{
		Op:   "Empower",
		Obj:  lRef{Kind: rYou},
		A:    n,
		C:    len(lc.out.overrides) - 1,
		CS:   key,
		Note: "also a " + typ + " named " + name,
	}
	lc.out.Tokens = append(lc.out.Tokens, key, nameCardKey+name)
	lc.emit(in)
	return nil
}
