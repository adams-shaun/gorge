package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// castSpentCount lowers a Count$ body: Count$CastTotalManaSpent, its Snow form
// (gorge Object.ManaSnowSpent, CR 107.4h; the snow units the cast's payment
// took) and its producer-tag forms Treasure / Cave / Desert (gorge
// Object.ManaTreasureSpent / ManaCaveSpent / ManaDesertSpent). All read the
// same vCastManaSpent: the kernel records the one form the card names as the
// cast's spend (the same contexts), so the plain total stays the total. One
// card has one meaning for the read: a face that reads two forms refuses, and
// so does any filtered form in the clone ask's bound (another spell's spend).
// Any other body is countValue's.
func (lc *lowerCtx) castSpentCount(rest string, depth int) (lValue, error) {
	if rest == voteTallyName {
		return lc.voteTallyValue()
	}
	if strings.HasPrefix(rest, "Adamant_") || strings.HasPrefix(rest, "Adamant.") {
		return lc.adamantValue(rest) // lower_evoke.go
	}
	tag, ok := spentTagOf(rest)
	if !ok {
		return lc.countValue(rest, depth)
	}
	if lc.face != nil {
		if _, multi := lc.faceSpentForm(); multi {
			return lValue{}, refuse("Count$CastTotalManaSpent read in more than one form")
		}
	}
	if tag == kop.SpentTagSnow {
		if lc.cloneRepl {
			return lValue{}, refuse("Count$CastTotalManaSpent Snow in an as-enters copy")
		}
	} else if tag != kop.SpentTagNone && lc.cloneRepl {
		return lValue{}, refuse("Count$CastTotalManaSpent %s in an as-enters copy", spentTagName(tag))
	}
	v, err := lc.countValue("CastTotalManaSpent", depth)
	if err == nil {
		switch tag {
		case kop.SpentTagSnow:
			lc.out.SpentSnow = true
		case kop.SpentTagNone:
		default:
			lc.out.SpentTag = tag
		}
	}
	return v, err
}

// spentTagOf parses the producer-tag word after Count$CastTotalManaSpent.
// SpentTagNone is the plain total; ok is false for a form this package does
// not lower (countValue keeps its refusal).
func spentTagOf(rest string) (tag uint8, ok bool) {
	switch rest {
	case "CastTotalManaSpent":
		return kop.SpentTagNone, true
	case "CastTotalManaSpent Snow":
		return kop.SpentTagSnow, true
	case "CastTotalManaSpent Treasure":
		return kop.SpentTagTreasure, true
	case "CastTotalManaSpent Cave":
		return kop.SpentTagCave, true
	case "CastTotalManaSpent Desert":
		return kop.SpentTagDesert, true
	}
	return 0, false
}

func spentTagName(tag uint8) string {
	switch tag {
	case kop.SpentTagSnow:
		return "Snow"
	case kop.SpentTagTreasure:
		return "Treasure"
	case kop.SpentTagCave:
		return "Cave"
	case kop.SpentTagDesert:
		return "Desert"
	}
	return "?"
}

// faceSpentForm reads the form each Count$CastTotalManaSpent occurrence in the
// face's SVars names. multi is true when two forms appear: the kernel records
// one spend per cast, so the read has one meaning.
func (lc *lowerCtx) faceSpentForm() (form uint8, multi bool) {
	const read = "Count$CastTotalManaSpent"
	seen := false
	add := func(f uint8) {
		if seen && form != f {
			multi = true
		}
		if !seen {
			form, seen = f, true
		}
	}
	for _, body := range lc.face.SVars {
		for rest := body; ; {
			i := strings.Index(rest, read)
			if i < 0 {
				break
			}
			rest = rest[i+len(read):]
			switch {
			case strings.HasPrefix(rest, " Snow"):
				add(kop.SpentTagSnow)
			case strings.HasPrefix(rest, " Treasure"):
				add(kop.SpentTagTreasure)
			case strings.HasPrefix(rest, " Cave"):
				add(kop.SpentTagCave)
			case strings.HasPrefix(rest, " Desert"):
				add(kop.SpentTagDesert)
			default:
				add(kop.SpentTagNone)
			}
		}
	}
	return form, multi
}
