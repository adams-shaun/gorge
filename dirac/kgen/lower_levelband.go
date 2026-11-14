package kgen

import (
	"regexp"
)

// Level up bands (CR 702.87): gorge kwLevelUp is a sorcery-speed
// `AB$ PutCounter | CounterType$ LEVEL`, and each level band is an ordinary
// Continuous static gated `IsPresent$ Card.Self+counters_GE<a>_LEVEL
// +counters_LE<b>_LEVEL` (or `counters_LT<b>`). A kernel filter clause
// carries one counter comparison, so the band is two presence tests of the
// source itself, ANDed: Card.Self names one object, so "it satisfies both"
// is "it satisfies each" (gorge's present count is 1 or 0 either way).

var levelBandRe = regexp.MustCompile(`^Card\.Self\+counters_GE(\d+)_([A-Z]+)\+counters_(LE|LT)(\d+)_([A-Z]+)$`)

// levelBand splits a two-sided Self counter band into its lower and upper
// single-comparison filters; ok is false for any other spelling.
func levelBand(ip string) (lo, hi string, ok bool) {
	m := levelBandRe.FindStringSubmatch(ip)
	if m == nil || m[2] != m[5] {
		return "", "", false
	}
	return "Card.Self+counters_GE" + m[1] + "_" + m[2], "Card.Self+counters_" + m[3] + m[4] + "_" + m[5], true
}
