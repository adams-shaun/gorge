// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwLevelUp(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	// CR 702.87a: "Level up [cost]" means "[cost]: Put a level counter
	// on this permanent. Activate only as a sorcery." It is an
	// ordinary activated ability, so the counter placement goes through
	// the existing PutCounter primitive and the level bands (ordinary
	// layer-7 SetPower$/SetToughness$/AddKeyword$ statics gated on
	// IsPresent$ Card.Self+counters_GE<n>_LEVEL) read the counter with
	// no further machinery. SorcerySpeed$ True is the CR 702.87a
	// sorcery-window restriction. param is the cost; any trailing
	// fields after a second colon are display text (none in the
	// measured 26-line corpus), exactly the trailing-field strip the
	// etbCounter and Affinity cases do.
	cost, _, _ := strings.Cut(param, ":")
	cost = strings.TrimSpace(cost)
	sa, _ := parseSA("", "AB$ PutCounter | Cost$ "+cost+" | Defined$ Self | CounterType$ LEVEL | CounterNum$ 1 | SorcerySpeed$ True | Keyword$ Level up | SpellDescription$ Level up "+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwLevelUp, "Level up") }
