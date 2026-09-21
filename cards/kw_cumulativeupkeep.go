// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwCumulativeUpkeep(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.46a is a triggered ability, not an upkeep turn action.
	// Expanding it into the ordinary Phase-trigger pipeline gives it
	// normal APNAP ordering, stack interaction and response windows.
	// param may include Forge's trailing display text after a colon;
	// only the first field is the actual upkeep cost.
	cost, _, _ := strings.Cut(param, ":")
	f.addKeywordTrigger(head, k,
		"Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | TriggerDescription$ Cumulative upkeep",
		"DB$ CumulativeUpkeep | Cost$ "+cost, has)
}

func init() { registerKeyword(kwCumulativeUpkeep, "Cumulative upkeep") }
