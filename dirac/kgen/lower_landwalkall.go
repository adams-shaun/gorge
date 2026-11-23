package kgen

// splitLandwalkAll takes the "Landwalk:<spec>" terms out of a PumpAll's KW$
// list (Nature's Cloak: "green creatures you control gain forestwalk until end
// of turn"): the rest of the list and one filter holding every term's clauses.
// gorge's PumpAll registers the grant on each match as it resolves (CR 611.2c),
// so the kernel reads it as the sweep's end-of-turn override record
// (grantLandwalkAll), beside a printed or statically granted landwalk. A
// ValidTgts$ player scope refuses, as the other PumpAll grants do.
func (lc *lowerCtx) splitLandwalkAll(kw string, scope lRef) (string, *lFilter, error) {
	rest, f, err := lc.splitLandwalkPump(kw)
	if err != nil {
		return "", nil, err
	}
	if f != nil && scope.Kind != rNone {
		return "", nil, refuse("PumpAll KW$ landwalk with a ValidTgts$ player scope")
	}
	return rest, f, nil
}

// grantLandwalkAll emits the end-of-turn OverrideAll record carrying the land
// filter lw (kop.Override.Landwalk) over every permanent matching f.
func (lc *lowerCtx) grantLandwalkAll(f, lw *lFilter) {
	o := &lOverride{Landwalk: lw}
	in := lInstr{Op: "OverrideAll", F: f, CS: "EOT", Note: overrideString(o)}
	lc.out.overrides = append(lc.out.overrides, o)
	in.C = len(lc.out.overrides) - 1
	lc.emit(in)
}
