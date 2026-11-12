// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwFading(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.32a: "This permanent enters with N fade counters on it. At the
	// beginning of your upkeep, remove a fade counter from it. If you can't,
	// sacrifice it." Both halves ride the ordinary replacement and trigger
	// pipelines (Vanishing's shape), so the counter changes, the upkeep
	// trigger's APNAP placement and a response window all behave like any
	// other. The trigger is ONE ability: the sacrifice reads the fade count
	// as the ability resolves, and is chained BEFORE the removal so that
	// "can't remove" means no counter was there to remove (a last counter
	// removed leaves the permanent until the next upkeep).
	// (the corpus prints only K:Fading:<N>, so param is the count itself)
	n := param
	if n != "" && !has("R", k) {
		sv := "__kwFading#" + k
		f.setSVar(sv, "DB$ PutCounter | Defined$ Self | CounterType$ FADE | CounterNum$ "+n+" | ETB$ True")
		p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
			" | Keyword$ Fading | Description$ CARDNAME enters with " + n + " fade counters.")
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
	}
	if has("T", k) {
		return
	}
	rm := "__kwFadingRm#" + k
	f.setSVar(rm, "DB$ RemoveCounter | Defined$ Self | CounterType$ FADE | CounterNum$ 1")
	f.addKeywordTrigger("Fading", k,
		"Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | TriggerDescription$ Fading",
		"DB$ Sacrifice | Defined$ Self | ConditionDefined$ Self | ConditionPresent$ Card.counters_EQ0_FADE | SubAbility$ "+rm, has)
}

func init() { registerKeyword(kwFading, "Fading") }
