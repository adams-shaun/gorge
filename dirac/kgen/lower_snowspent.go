package kgen

import "strings"

// castSpentCount lowers a Count$ body: Count$CastTotalManaSpent Snow (gorge
// Object.ManaSnowSpent, CR 107.4h; the snow units the cast's payment took) and
// the plain total are the spend read (vCastManaSpent, the same contexts) and
// the Snow form marks the card SpentSnow, so the kernel's cast records its
// snow spend in place of the mana value. One card has one meaning for the
// read: a face that reads both refuses, and so does the Snow form in the clone
// ask's bound (another spell's spend). Any other body is countValue's.
func (lc *lowerCtx) castSpentCount(rest string, depth int) (lValue, error) {
	if rest == voteTallyName {
		return lc.voteTallyValue()
	}
	snow := rest == "CastTotalManaSpent Snow"
	if strings.HasPrefix(rest, "Adamant_") || strings.HasPrefix(rest, "Adamant.") {
		return lc.adamantValue(rest) // lower_evoke.go
	}
	if !snow && rest != "CastTotalManaSpent" {
		return lc.countValue(rest, depth)
	}
	if lc.face != nil && lc.faceReadsBothSpends() {
		return lValue{}, refuse("Count$CastTotalManaSpent beside its Snow form")
	}
	if snow && lc.cloneRepl {
		return lValue{}, refuse("Count$CastTotalManaSpent Snow in an as-enters copy")
	}
	v, err := lc.countValue("CastTotalManaSpent", depth)
	if err == nil && snow {
		lc.out.SpentSnow = true
	}
	return v, err
}

// faceReadsBothSpends: some SVar of the face reads the Snow form and some
// reads the plain total.
func (lc *lowerCtx) faceReadsBothSpends() bool {
	const read = "Count$CastTotalManaSpent"
	var snow, plain bool
	for _, body := range lc.face.SVars {
		for rest := body; ; {
			i := strings.Index(rest, read)
			if i < 0 {
				break
			}
			rest = rest[i+len(read):]
			if strings.HasPrefix(rest, " Snow") {
				snow = true
			} else {
				plain = true
			}
		}
	}
	return snow && plain
}
