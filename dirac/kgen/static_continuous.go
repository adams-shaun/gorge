package kgen

// The bodies of the Continuous statics (static_mode.go).
func init() {
	staticMode("Continuous", (*staticCtx).modeContinuous)
}

func (sc *staticCtx) modeContinuous() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	lc.grantInner = nil
	if err := extraBlockRefusal(sp); err != nil {
		return nil, true, err // lower_extrablock.go
	}
	goad := sp.is("Goad", "True")
	sp.consume("Goad")
	lure, lureOnly, err := lc.lureGrant(sp, &sc.st) // lower_lure.go
	if err != nil {
		return nil, true, err
	}
	var sts []lStatic
	took, err := lc.cdaFlash(sp, &sc.st) // lower_cdaflash.go
	if err != nil {
		return nil, true, err
	}
	if lureOnly {
		sc.st = lure
	} else if took {
		// the conditional CDA flash static: sc.st already carries it
	} else if sts, err = lc.continuous(sp, &sc.st); err != nil {
		return nil, true, err
	}
	if !took && sts == nil && !lureOnly { // the one plain Continuous static: nothing else carries the grant
		if err := lc.grantedStaticParam(sp, &sc.st); err != nil {
			return nil, true, err // lower_grantedstatic.go
		}
	}
	if err := lc.selfCondCheck(&sc.st); err != nil {
		return nil, true, err // lower_selfcondition.go
	}
	if err := affectedKwCheck(&sc.st); err != nil {
		return nil, true, err // lower_selfkwaffected.go
	}
	sc.extra = sts
	sc.allZoneTwin() // lower_offzone.go
	if lure.Kind != "" && !lureOnly {
		// the lure rides beside the other parts (Gift of the Deity's
		// +1/+1): a second static over the same filter and condition
		if sc.extra == nil {
			sc.extra = []lStatic{sc.st} // extra replaces the static itself
		}
		sc.extra = append(sc.extra, lure)
	}
	if goad {
		// Goad$ True: the affected creatures are goaded while the
		// static applies (gorge's static goad route; kop.StatGoad), a
		// second static over the same filter and condition
		if sc.st.Kind != "Continuous" || sc.st.F == nil || sc.st.F.Players != 0 {
			return nil, true, refuse("Goad$ on a %s static", sc.st.Kind)
		}
		if sc.extra == nil {
			sc.extra = []lStatic{sc.st} // extra replaces the static itself
		}
		sc.extra = append(sc.extra, lStatic{Kind: "Goad", F: sc.st.F, Cond: sc.st.Cond})
	}
	return nil, false, nil
}
