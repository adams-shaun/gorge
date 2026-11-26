package kgen

// RememberPlayed$ True on api:Play (gorge effects/play.go: the PlayDone pass
// adds the object whose play began to the source's remembered state, through
// rememberTarget). The lowering rides OpCastFree's C bit 13, beside the
// ForgetPlayed$ bit 4; dirac/cast_playremember.go acts on it where the cast
// commits.

// playForgetRemember reads ForgetPlayed$ and RememberPlayed$. Each is read
// for the literal True only; any other value refuses by name.
func playForgetRemember(ps *params) (forget, remember bool, err error) {
	if v, ok := ps.get("ForgetPlayed"); ok {
		if v != "True" {
			return false, false, refuse("Play ForgetPlayed %q", v)
		}
		forget = true
	}
	if v, ok := ps.get("RememberPlayed"); ok {
		if v != "True" {
			return false, false, refuse("Play RememberPlayed %q", v)
		}
		remember = true
	}
	return forget, remember, nil
}

// playRememberFlags is the CastFree flag spelling of the two riders.
func playRememberFlags(forget, remember bool) string {
	cs := ""
	if forget {
		cs += "|Forget"
	}
	if remember {
		cs += "|Remember"
	}
	return cs
}

// castFreeRemember sets OpCastFree's C bit 13 (RememberPlayed$ True).
func (ic *instrCtx) castFreeRemember() {
	if _, ok := ic.fl["Remember"]; ok {
		ic.ki.C |= 1 << 13
	}
}
