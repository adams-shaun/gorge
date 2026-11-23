package kgen

// Banding's grant path (CR 702.22, Battering Ram). KwMask is full
// (kop/keywords.go), so a granted Banding is not a keyword bit: pumpKeywords
// lowers the head into the override record's AddKw list, and takeBanding
// moves it onto the record's Banding bool (kop.Override.Banding) before the
// record is interned. protKwInto calls it, so every override-record keyword
// grant (a Pump of any duration, an Animate's, a landwalk's companion list)
// carries it the same way; the record's own duration ends it, and the
// manifest's keyword slot (dirac/manifest/key/keyBanding) stays reserved.
//
// The two grants that do not pass through protKwInto land on a record of
// their own: a Pump's default (until end of turn) Banding is an "EOT"
// override record (splitBandingGrant, called by pumpGrantKw), and a
// Continuous static's AddKeyword$ Banding is the static's Override record
// (staticBandingGrant), read while the static applies by the kernel's
// hasBanding (dirac/kw_banding.go) with the static's filter and condition.

// takeBanding moves the granted Banding out of o's AddKw list onto o.Banding.
// A KW$ list cannot repeat a keyword, so at most one head matches.
func takeBanding(o *lOverride) {
	for i, k := range o.AddKw {
		if k != "Banding" {
			continue
		}
		o.Banding = true
		o.AddKw = append(o.AddKw[:i], o.AddKw[i+1:]...)
		return
	}
}

// splitBandingGrant removes a granted Banding from the KW$ list kw and sets
// o.Banding. perm is whether the Pump's subject is a battlefield permanent:
// the override record rides one, so any other subject refuses.
func (lc *lowerCtx) splitBandingGrant(kw string, o *lOverride, perm bool) (string, error) {
	rest, found := splitKWSentence(kw, "Banding")
	if !found {
		return kw, nil
	}
	if !perm {
		return "", refuse("granted Banding to a non-permanent subject")
	}
	o.Banding = true
	return rest, nil
}

// staticBandingGrant removes a granted Banding from a Continuous static's
// AddKeyword$ list and carries it on the static's Override record (the one a
// RemoveKeyword$ beside it already made, else a new one), the grant the
// kernel reads from the Continuous static's own filter and condition. Only a
// battlefield static reaches that read: an AffectedZone$ naming any other
// zone refuses.
func (lc *lowerCtx) staticBandingGrant(sp *params, st *lStatic, kw string) (string, error) {
	rest, found := splitKWSentence(kw, "Banding")
	if !found {
		return kw, nil
	}
	if az := sp.str("AffectedZone"); az != "" && az != "Battlefield" {
		return "", refuse("granted Banding to a %s object", az)
	}
	if st.Override == nil {
		st.Override = &lOverride{}
		lc.out.overrides = append(lc.out.overrides, st.Override)
	}
	st.Override.Banding = true
	return rest, nil
}
