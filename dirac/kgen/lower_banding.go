package kgen

// Banding's grant path (CR 702.22, Battering Ram). KwMask is full
// (kop/keywords.go), so a granted Banding is not a keyword bit: pumpKeywords
// lowers the head into the override record's AddKw list, and takeBanding
// moves it onto the record's Banding bool (kop.Override.Banding) before the
// record is interned. protKwInto calls it, so every override-record keyword
// grant (a Pump of any duration, an Animate's, a landwalk's companion list)
// carries it the same way; the record's own duration ends it, and the
// manifest's keyword slot (dirac/manifest/key/keyBanding) stays reserved.

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
