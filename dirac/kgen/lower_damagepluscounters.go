package kgen

import (
	"strconv"
	"strings"
)

// dmgPlusCtrOperand parses the operand of a
// ReplaceCount$DamageAmount/Plus.<op> replacement amount.
// A positive integer is the constant Valley Flamecaller path and leaves ok
// false. Otherwise op must name a face SVar whose trimmed body is exactly
// Count$CardCounters.<KIND> with a known counter kind: Fated Firepower's
// Plus.Y reads the fire counters on the replacement source itself, which
// gorge rewriteReplaceCountOperand resolves in that source's context at
// application time (effects/replacement.go:54-71). Any other operand is a
// named refusal.
func (lc *lowerCtx) dmgPlusCtrOperand(op string) (kind string, ok bool, err error) {
	if n, e := strconv.Atoi(op); e == nil && n > 0 {
		return "", false, nil
	}
	body, found := lc.face.SVars[op]
	if !found {
		return "", false, refuse("repl-hard: DamageDone plus %q", op)
	}
	body = strings.TrimSpace(body)
	k, cut := strings.CutPrefix(body, "Count$CardCounters.")
	if !cut {
		return "", false, refuse("repl-hard: DamageDone plus %q", body)
	}
	if _, e := counterKind(k); e != nil {
		return "", false, refuse("repl-hard: DamageDone plus %q", body)
	}
	return k, true, nil
}
