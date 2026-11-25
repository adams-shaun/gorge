package kgen

import "strings"

// urzaLandsValue lowers Count$UrzaLands.<assembled>.<not> (gorge
// count_body_heads.go, controlsAllUrzaLands in count_zone_fold.go): <assembled>
// when the resolving controller controls a permanent of each Urza land
// subtype (Urza's Mine, Urza's Tower, Urza's Power-Plant), else <not>; each
// arm a literal or an SVar (Urza's Mine's Count$UrzaLands.2.1). A type line
// splits on spaces, so each subtype is the pair of words "Urza's" and Mine /
// Tower / Power-Plant (gorge hasSubtype matches the consecutive words). The
// three presence counts multiply: the product is at least one exactly when
// every count is.
func (lc *lowerCtx) urzaLandsValue(arms string, depth int) (lValue, error) {
	parts := strings.Split(arms, ".")
	if len(parts) != 2 {
		return lValue{}, refuse("Count$UrzaLands.%s", arms)
	}
	var have lValue
	for i, sub := range [...]string{"Mine", "Tower", "Power-Plant"} {
		f, err := lc.filter("Permanent.YouCtrl+Urza's+" + sub)
		if err != nil {
			return lValue{}, err
		}
		c := lValue{Kind: vCount, F: f}
		if i == 0 {
			have = c
			continue
		}
		have = lValue{Kind: vProduct, Sub: []lValue{have, c}}
	}
	var br [2]lValue
	for i, t := range parts {
		var err error
		if br[i], err = lc.headArm("UrzaLands", t, depth); err != nil {
			return lValue{}, err
		}
	}
	return lValue{Kind: vCompare, N: 1, Op: opGE, Sub: []lValue{have, {Kind: vBranches, Sub: br[:]}}}, nil
}
