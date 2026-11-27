package kgen

// playImprint reads ImprintPlayed$ True (gorge rules/play_tape.go: every card
// the play actually begins joins the resolution source's Imprinted list), the
// kernel's freeImprint bit (dirac/cast_playimprint.go). A non-True value
// refuses by name. The read opens the resolution's imprinted set so the rest
// of this chain sees it (the ConditionDefined$ Imprinted gate and the
// MayPlay's RefImprinted read, then the Cleanup's clear).
func (lc *lowerCtx) playImprint(ps *params) (bool, error) {
	v, ok := ps.get("ImprintPlayed")
	if !ok {
		return false, nil
	}
	if v != "True" {
		return false, refuse("Play ImprintPlayed %q", v)
	}
	if err := lc.openImprint("Play ImprintPlayed$"); err != nil {
		return false, err
	}
	return true, nil
}

// playImprintShapeOK refuses the shapes whose cast path has no commit point to
// hang the imprint write on: the Hideaway play (lower_hideaway.go) and the
// land-capable play (dirac/cast_freeland.go).
func playImprintShapeOK(imprint, hideaway, landPlay bool) error {
	if !imprint {
		return nil
	}
	if hideaway {
		return refuse("Play ImprintPlayed on Defined$ ExiledWith")
	}
	if landPlay {
		return refuse("Play ImprintPlayed on a land-capable Play")
	}
	return nil
}

// playImprintFlag is OpCastFree's Imprint flag (C bit 15, dirac/cast_freecast.go).
func playImprintFlag(imprint bool) string {
	if imprint {
		return "|Imprint"
	}
	return ""
}
