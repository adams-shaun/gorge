package kgen

import "strings"

// cantHaveKeyword reads a Continuous static's CantHaveKeyword$ beside its
// RemoveKeyword$ (st.Override already carries the removal): "creatures your
// opponents control lose flying and can't have or gain flying" (the
// Archetype cycle). gorge chars.derive applies the prohibition twice: in the
// effect's own layer-6 step (a removal there, which RemoveKeyword$ already
// is) and as a final filter after every layer-6 grant, printed keywords,
// keyword counters and until-end-of-turn grants alike; the kernel's layer-6
// walk drops Override.CantHaveKw from its result (dirac/static_layer6.go).
//
// Lowered only when every prohibited keyword is also removed (so the in-step
// filter is the removal), none is added by the same static (gorge filters
// after the add; the walk adds after the removal), and the affected filter
// does not read keywords (the final filter would need the walk's per-step
// match). Hexproof refuses (see below).
func cantHaveKeyword(sp *params, st *lStatic) error {
	v, ok := sp.get("CantHaveKeyword")
	if !ok {
		return nil
	}
	ks, err := removeKeywords(v)
	if err != nil {
		return refuse("CantHaveKeyword %q", v)
	}
	added := map[string]bool{}
	for _, a := range strings.Split(sp.m["AddKeyword"], "&") {
		added[strings.TrimSpace(a)] = true
	}
	for _, k := range ks {
		if k == "Hexproof" {
			// gorge matches the prohibition by keyword head, so it takes
			// "Hexproof:<valid>" too, a separate kernel bit
			// (KwHexproofFromInstants) this mask would leave: Archetype of
			// Endurance stays refused
			return refuse("CantHaveKeyword %q (its head covers the kernel's hexproof-from bits)", k)
		}
		removed := false
		for _, r := range st.Override.RemoveKw {
			removed = removed || r == k
		}
		if !removed {
			return refuse("CantHaveKeyword %q without its RemoveKeyword$", k)
		}
		if added[k] {
			return refuse("CantHaveKeyword %q beside its own AddKeyword$", k)
		}
	}
	if readsFlying(st.F) {
		return refuse("CantHaveKeyword on a keyword-gated static")
	}
	st.Override.CantHaveKw = ks
	return nil
}
