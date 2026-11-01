package kgen

import (
	"strings"
)

// chooseColorExclude reads an as-enters ChooseColor's Exclude$ (gorge
// chooseColorOptions drops the named colours from the ask; Citadel Gate,
// Sea Gate): the excluded colours as bits W U B R G = 0..4 (the
// OpChooseColor C). An unknown colour word refuses.
func chooseColorExclude(ps *params) (int, error) {
	v, ok := ps.get("Exclude")
	if !ok {
		return 0, nil
	}
	m := 0
	for w := range strings.SplitSeq(v, ",") {
		switch strings.ToLower(strings.TrimSpace(w)) {
		case "white":
			m |= 1
		case "blue":
			m |= 2
		case "black":
			m |= 4
		case "red":
			m |= 8
		case "green":
			m |= 16
		default:
			return 0, refuse("ChooseColor Exclude$ %q", v)
		}
	}
	if m == 31 {
		return 0, refuse("ChooseColor Exclude$ every colour")
	}
	return m, nil
}
