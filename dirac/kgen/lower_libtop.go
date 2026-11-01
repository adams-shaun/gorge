package kgen

import "strings"

// "you may play/cast <cards> from the top of your
// library" (MayPlay$ True | AffectedZone$ Library, TopLibrary filters).
//
// gorge's may-play walk offers exactly one library card: lib[0] of the
// PRIORITY player's own library (engine/rules/legal_mayplay.go), then
// matches it against Affected$ like any other zone, where TopLibrary is
// always true. So the word is implied by the zone and is stripped before
// lc.filter; the kernel offers only the top card (tryLibTop). Every comma
// alternative must carry TopLibrary (else a deeper library card would
// match in gorge's static walk) and a YouCtrl / YouOwn word (an opponent's
// top card is never offered, and the shape stays refused).
//
// MayPlayLimit$ 1 (Assemble the Players, Johann) lowers WITH
// StOncePerTurn: gorge caps a limited static once per turn PER STATIC
// (engine/rules/mayplay.go mayPlayKeyLimitReached), the same as
// Forge and XMage, so a second, different top card is not played through
// the same static that turn.

// libTopAffected strips TopLibrary from each alternative of a library
// permission's Affected$.
func libTopAffected(aff string) (string, error) {
	alts := strings.Split(aff, ",")
	for i, alt := range alts {
		dot := strings.IndexByte(alt, '.')
		if dot < 0 {
			return "", refuse("MayPlay Library %q", aff)
		}
		words := strings.Split(alt[dot+1:], "+")
		kept := words[:0:0]
		top, mine := false, false
		for _, w := range words {
			switch w {
			case "TopLibrary":
				top = true
				continue
			case "YouCtrl", "YouOwn":
				mine = true
			}
			kept = append(kept, w)
		}
		if !top || !mine {
			return "", refuse("MayPlay Library %q", aff)
		}
		alts[i] = alt[:dot]
		if len(kept) > 0 {
			alts[i] += "." + strings.Join(kept, "+")
		}
	}
	return strings.Join(alts, ","), nil
}

// libTopRefusals refuses the riders a library permission does not model.
func libTopRefusals(sp *params) error {
	for _, k := range []string{"ValidAfterStack", "MayPlayIgnoreType", "MayPlayWithoutManaCost", "MayPlayIgnoreColor", "RaiseCost"} {
		if _, ok := sp.get(k); ok {
			return refuse("MayPlay Library with %s$", k)
		}
	}
	return nil
}
