package kgen

import "strings"

// multiZoneZones maps a Count$Valid<Z1>,<Z2>,... zone word to the count
// that reads that zone (gorge count_zone_fold.go zoneWords): N 1 on
// vCountHand is the library walk (build.go's Count$ValidLibrary zone).
var multiZoneZones = map[string]lValue{
	"Battlefield": {Kind: vCount},
	"Graveyard":   {Kind: vCountGraveyard},
	"Exile":       {Kind: vCountExile},
	"Hand":        {Kind: vCountHand},
	"Library":     {Kind: vCountHand, N: 1},
}

// multiZoneCount lowers Count$Valid<Z1>,<Z2>[,...] <spec> with no $
// aggregate, the zone pairs the fixed table in countValue does not name
// (Machinist's Dismissal's Battlefield,Hand; Frenzied Geistblaster's
// Graveyard,Library,Hand): gorge multiCountZones folds every named zone's
// matching objects, each zone once, so the value is the sum of one count per
// zone over the same spec. A repeated or unknown zone word is no multi-zone
// head in gorge (multiCountZones returns nil) and is left to the caller. ok
// is false when rest is not this shape.
func (lc *lowerCtx) multiZoneCount(rest string) (lValue, bool, error) {
	head, spec, cut := strings.Cut(rest, " ")
	zs, isValid := strings.CutPrefix(head, "Valid")
	if !cut || !isValid || !strings.Contains(zs, ",") || strings.Contains(spec, "$") || strings.Contains(spec, "ActivePlayer") {
		return lValue{}, false, nil
	}
	words := strings.Split(zs, ",")
	seen := map[string]bool{}
	for _, w := range words {
		if _, known := multiZoneZones[w]; !known || seen[w] {
			return lValue{}, false, nil
		}
		seen[w] = true
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, true, err
	}
	var v lValue
	for i, w := range words {
		z := multiZoneZones[w]
		z.F = f
		if i == 0 {
			v = z
			continue
		}
		v = lValue{Kind: vSum, Sub: []lValue{v, z}}
	}
	return v, true, nil
}
