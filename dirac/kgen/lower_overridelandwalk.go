package kgen

// An override static's landwalk grant (Halimar Wavewatch's level band: "set
// base power and toughness 6/6, has islandwalk"). A landwalk has no override
// field a static's record carries; it is the Continuous static's own
// kop.Static.Landwalk (lower_landwalkgrant.go). So the Landwalk terms leave the
// override's AddKeyword$ list and ride a companion Continuous static over the
// same filter and condition: the layer-6 grant and the layer-7b set are
// separate effects of one source (CR 613.1), applied as gorge's layer walk
// applies them, so splitting them is exact.

// overrideAddKeyword reads an override static's AddKeyword$ into o.AddKw and
// returns the landwalk filter its Landwalk terms name (nil: none).
func (lc *lowerCtx) overrideAddKeyword(sp *params, o *lOverride) (*lFilter, error) {
	k, ok := sp.get("AddKeyword")
	if !ok {
		return nil, nil
	}
	rest, landwalk, err := lc.splitLandwalkGrant(k, sp.str("Affected"))
	if err != nil {
		return nil, err
	}
	if rest != "" {
		// the protection terms ride o.ProtFrom (lower_protgrant.go)
		if err := lc.protKwInto(o, rest); err != nil {
			return nil, err
		}
	}
	return landwalk, nil
}

// landwalkCompanion appends the Continuous static carrying an override
// static's landwalk grant (nil landwalk: out unchanged). A part of the
// override that begins before layer 6 keeps the whole effect past a loss of
// abilities (CR 613.6, beginsBeforeLayer6), which the companion would not
// share, so that combination refuses.
func (lc *lowerCtx) landwalkCompanion(landwalk *lFilter, st *lStatic, o *lOverride, out []lStatic) ([]lStatic, error) {
	if landwalk == nil {
		return out, nil
	}
	if o.beginsBeforeLayer6() {
		return nil, refuse("override static landwalk grant beside a layer 1-5 part")
	}
	return append(out, lStatic{Kind: "Continuous", F: st.F, Cond: st.Cond, Landwalk: landwalk}), nil
}
