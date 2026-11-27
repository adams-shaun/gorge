package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
)

// cwfValidSA reads a CastWithFlash static's ValidSA$ tail, the spell-side
// words gorge's spellMatchesValidSA grants on (rules/statics_cast.go):
// bare `Spell`; `Spell.XCostLE<N>` (Silver Scrutiny), returned as xmax =
// 1+N, the kernel's X bound (kop.Static.A, dirac/cast_flashxcap.go); and
// `Spell.<creature type>` (Blur of Heroism's Hero), returned as the subtype
// the static's card filter also requires. Every other word refuses.
func cwfValidSA(v string) (xmax int, subtype string, err error) {
	kind, tail, dotted := strings.Cut(strings.TrimSpace(v), ".")
	if kind != "Spell" {
		return 0, "", refuse("CastWithFlash shape")
	}
	if !dotted {
		return 0, "", nil
	}
	if num, ok := strings.CutPrefix(tail, "XCostLE"); ok {
		if n, err := strconv.Atoi(num); err == nil && n >= 0 && n < 255 {
			return n + 1, "", nil
		}
		return 0, "", refuse("CastWithFlash shape")
	}
	if effects.CreatureTypeWords(tail) {
		return 0, tail, nil
	}
	return 0, "", refuse("CastWithFlash shape")
}

// cwfValidCard is the static's affected-card filter: ValidCard$, narrowed to
// the ValidSA$ subtype when there is one (gorge matches both, ValidCard$ and
// `Card.<type>`, against the cast card). A ValidCard$ with alternatives or a
// conjunction refuses with a subtype, the narrowing being one more qualifier.
func (sc *staticCtx) cwfValidCard(subtype string) (*lFilter, error) {
	if subtype == "" {
		return sc.readF("ValidCard")
	}
	vc := sc.sp.str("ValidCard")
	if vc == "" || strings.ContainsAny(vc, ",+ ") {
		return nil, refuse("CastWithFlash shape")
	}
	return sc.lc.filter(vc + "." + subtype)
}
