package kgen

import "strings"

// inertKeywordGrant reports whether an AddKeyword$ grant is one gorge never
// reads, which lowers to nothing (st.Kind is cleared): a LimitSearchLibrary
// grant to a player spec (view-only, see lower_continuous.go) or
// Kodama's soulshift grant (inertSoulshiftGrant).
func inertKeywordGrant(sp *params, st *lStatic, k string) bool {
	if strings.TrimSpace(k) == "LimitSearchLibrary" && st.F != nil && st.F.Players > 0 || inertSoulshiftGrant(sp, st, k) {
		st.Kind = ""
		return true
	}
	return false
}

// inertSoulshiftGrant reports whether a Continuous static is Kodama of the
// Center Tree's "has soulshift X, where X is the number of Spirits you
// control" grant, which lowers to nothing because gorge's grant is inert:
//   - CalcKeywordN$ is read nowhere in engine/;
//   - a granted keyword becomes a trigger only through
//     scriptfacts.GrantedKeywordTrigger (GrantedTriggerHeads), and Soulshift
//     is not a head;
//   - Soulshift is not in engine/cards grantedKeywordHeads either (those mint
//     activated abilities);
//   - Soulshift's printed expansion (cards/kw_soulshift.go) mints a trigger
//     only from a printed K: line, and the card has none.
//
// So the card plays with no soulshift. The deviation from the rules (it
// should have soulshift X) is deliberate lockstep fidelity and is recorded in
// the commit message, not in the known-approximations register.
//
// It consumes CalcKeywordN and returns true only when the grant is the
// static's whole effect: no condition and no parameter beyond Mode, the
// affected spec, AddKeyword and CalcKeywordN (ignored keys such as
// Description aside). Anything else is left for the usual grant chain.
func inertSoulshiftGrant(sp *params, st *lStatic, kw string) bool {
	if !strings.HasPrefix(strings.TrimSpace(kw), "Soulshift:") || st.Cond.Kind != cNone {
		return false
	}
	if _, ok := sp.m["CalcKeywordN"]; !ok {
		return false
	}
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedDefined", "AddKeyword", "CalcKeywordN":
		default:
			if !ignoredParams[k] {
				return false
			}
		}
	}
	sp.consume("CalcKeywordN")
	return true
}
