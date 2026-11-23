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

// chooseColorOrColors reads a mid-resolution ChooseColor's OrColors$ True
// ("becomes the color or colors of your choice", Quickchange, Shyft). gorge
// effChooseColor (chooseColorOptions) cannot ask the shape: it notes it and
// records the first-WUBRG fallback "W" with no decision, so the lowering is
// the resolution's no-ask white answer (ExtChooseColorRes C = 1). Any other
// value refuses.
func chooseColorOrColors(ps *params) (bool, error) {
	v, ok := ps.get("OrColors")
	if !ok {
		return false, nil
	}
	if !strings.EqualFold(strings.TrimSpace(v), "True") {
		return false, refuse("ChooseColor OrColors$ %q", v)
	}
	return true, nil
}
