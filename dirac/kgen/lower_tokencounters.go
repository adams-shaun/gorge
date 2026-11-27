package kgen

// tokenEnter is a Token's WithCountersType$/WithCountersAmount$ read: the
// token enters with the counters (CR 122.6 / 614.1c).
type tokenEnter struct {
	kind   string
	amount lValue
	has    bool
}

// tokenEnterCounters reads WithCountersType$ on a Token. The kernel arms
// genState.enterCtr before the mint (ExtTokenCounters) and clears it after;
// one token per mint is the exact shape (Printlifter Ooze).
func (lc *lowerCtx) tokenEnterCounters(ps *params, n lValue, andYou bool) (tokenEnter, error) {
	kind, has := ps.get("WithCountersType")
	if !has {
		return tokenEnter{}, nil
	}
	if andYou || !n.isConst() || n.N != 1 {
		return tokenEnter{}, refuse("Token WithCountersType$ on %v tokens", n)
	}
	if _, err := counterKind(kind); err != nil {
		return tokenEnter{}, refuse("Token WithCountersType$ %q", kind)
	}
	amt, err := lc.amount(ps, "WithCountersAmount", 1)
	if err != nil {
		return tokenEnter{}, err
	}
	return tokenEnter{kind: kind, amount: amt, has: true}, nil
}

// emitEnterCounterMints emits the mint, bracketed by arming and clearing the
// token's entering counters when it has any.
func (lc *lowerCtx) emitEnterCounterMints(in lInstr, andYou bool, e tokenEnter) {
	if e.has {
		lc.emit(lInstr{Op: "TokenCounters", A: e.amount, CS: e.kind})
	}
	lc.emitMints(in, andYou)
	if e.has {
		lc.emit(lInstr{Op: "TokenCounters", A: cst(0), CS: e.kind})
	}
}
