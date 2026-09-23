// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

func kwVanishing(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// Vanishing N: the entry replacement and both later actions use the
	// ordinary replacement/trigger pipelines, so counter changes, priority
	// and response windows remain replayable.
	n, _, _ := strings.Cut(param, ":")
	if n == "" {
		return // Bare K:Vanishing has no counter count and is outside this expansion's script shape.
	}
	if !has("R", k) {
		sv := "__kwVanishing" + strconv.Itoa(i)
		f.setSVar(sv, "DB$ PutCounter | Defined$ Self | CounterType$ TIME | CounterNum$ "+n+" | ETB$ True")
		p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
			" | Keyword$ Vanishing | Description$ CARDNAME enters with " + n + " time counters.")
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
	}
	f.addKeywordTrigger("Vanishing", k+"#upkeep",
		"Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | IsPresent$ Card.Self+counters_GE1_TIME | PresentCompare$ GE1 | PresentDefined$ Self | TriggerZones$ Battlefield | TriggerDescription$ At the beginning of your upkeep, remove a time counter from CARDNAME.",
		"DB$ RemoveCounter | Defined$ Self | CounterType$ TIME | CounterNum$ 1", has)
	f.addKeywordTrigger("Vanishing", k+"#last-counter",
		"Mode$ CounterRemoved | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ TIME | NewCounterAmount$ 0 | TriggerDescription$ When the last time counter is removed from CARDNAME, sacrifice it.",
		"DB$ Sacrifice | Defined$ Self", has)
}

func init() { registerKeyword(kwVanishing, "Vanishing") }
