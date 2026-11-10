package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// WithCountersType$ with a comma list is EVERY listed kind, in script order,
// each of WithCountersAmount$, folded into the entering permanent's entry
// (gorge entryWithCounters, CR 122.6, 614.1c). Only a ChangeZone to the
// battlefield (OpReturnEx) with exactly two kinds and a constant amount is
// read: the second kind rides the constant count's high byte
// (kop.RetCounterPairShift). Every other move, a third kind, or a variable
// amount refuses by name.

// withCountersList checks a WithCountersType$ value at lowering. A single kind
// passes through; a list needs the Return-to-battlefield shape.
func withCountersList(kinds, op string, dest int) error {
	if !strings.Contains(kinds, ",") {
		return nil
	}
	ks := strings.Split(kinds, ",")
	if len(ks) != 2 {
		return refuse("WithCountersType$ %q: only a pair of kinds is read", kinds)
	}
	for _, k := range ks {
		if _, err := counterKind(strings.TrimSpace(k)); err != nil {
			return refuse("WithCountersType$ %q: %v", kinds, err)
		}
	}
	if op != "Return" || dest != zBattlefield {
		return refuse("WithCountersType$ %q on a %s move: a list of kinds is read for a battlefield entry only", kinds, op)
	}
	return nil
}

// counterPair packs a two-kind entry counter rider into ki (OpReturnEx):
// B the first kind, A the constant count | (second kind + 1)<<8.
func (cb *cardBuilder) counterPair(ki *kop.Instr, in lInstr, kinds string) error {
	if in.Op != "Return" {
		return fmt.Errorf("WithCountersType$ %q on a %s", kinds, in.Op)
	}
	ks := strings.Split(kinds, ",")
	if len(ks) != 2 {
		return fmt.Errorf("WithCountersType$ %q: not a pair", kinds)
	}
	var ck [2]kop.CounterKind
	for i, k := range ks {
		c, err := counterKind(strings.TrimSpace(k))
		if err != nil {
			return err
		}
		ck[i] = c
	}
	n := 1
	if in.A.Kind != vNone {
		if !in.A.isConst() || in.A.N < 1 || in.A.N > 255 {
			return fmt.Errorf("WithCountersType$ %q with a count that is not a constant 1..255", kinds)
		}
		n = in.A.N
	}
	ki.A = kop.Const(n | (int(ck[1])+1)<<kop.RetCounterPairShift)
	ki.B = kop.Const(int(ck[0]))
	return nil
}
