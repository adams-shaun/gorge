package kgen

import (
	"strconv"
	"strings"
)

// Granted toxic N (CR 702.164; gorge ToxicValue sums every Toxic:<N> entry of
// the derived keywords, 702.164c). Toxic has a CardDef field, not a KwMask
// bit, so a grant is carried by amount, beside the printed N:
//
//   - a Pump / PumpAll KW$ term: OpExt ExtKwMark KwMarkToxic, N the high byte
//     of C (splitKwMarks), until end of turn (Aspirant's Ascent, Plague Nurse);
//   - a Continuous static's AddKeyword$ term: kop.Static.Toxic (Karumonix,
//     Necrogen Communion, Prosthetic Injector);
//   - an Animate's Keywords$ term: kop.Override.Toxic (Monument to Perfection).
//
// Terms add. A term whose N is not a positive integer, or a Pump's sum past
// maxToxicMark, stays in the keyword list, where pumpKeywords refuses it.

// maxToxicMark is the most toxic one ExtKwMark carries: the amount sits in
// C's high byte under the int16 sign bit.
const maxToxicMark = 127

// toxicTerm reads a "Toxic:N" keyword term, N a positive integer.
func toxicTerm(k string) (n int, ok bool) {
	v, found := strings.CutPrefix(k, "Toxic:")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	return n, err == nil && n >= 1 && n <= 255
}
