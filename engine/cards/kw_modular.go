// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwModular expands Modular N (CR 702.43a): "This permanent enters with N
// +1/+1 counters on it" and "When this permanent is put into a graveyard
// from the battlefield, you may put a +1/+1 counter on target artifact
// creature for each +1/+1 counter on this permanent."
//
// The entry half is the Graft/Sunburst shape: a Moved -> Battlefield Updated
// replacement whose body puts the counters. N is a literal, or the word
// Sunburst (Arcbound Wanderer), which counts the distinct colours spent to
// cast it (Count$Converge, the same read kwSunburst makes). The dies half is
// an optional ChangesZone trigger that reads the dying permanent's own
// last-known +1/+1 count (TriggeredCard$CardCounters.P1P1). Both lines carry
// Keyword$ Modular: the rules package's Triggered.Modular cause qualifier
// (Zabaz, the Glimmerwasp) recognises the trigger's source by the printed
// keyword.
func kwModular(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	n := param
	if n == "Sunburst" {
		n = "Count$Converge"
	}
	if n == "" {
		return
	}
	if !has("R", k) {
		put := "__kwModularPut"
		f.setSVar(put, "DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ "+n+" | ETB$ True")
		p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + put + " | Keyword$ Modular")
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
	}
	if !has("T", k) {
		count := "__kwModularCount"
		f.setSVar(count, "TriggeredCard$CardCounters.P1P1")
		f.addKeywordTrigger(head, k,
			"Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | OptionalDecider$ You | TriggerDescription$ Modular "+param,
			"DB$ PutCounter | ValidTgts$ Artifact.Creature | TgtPrompt$ Select target artifact creature | CounterType$ P1P1 | CounterNum$ "+count, has)
	}
}

func init() { registerKeyword(kwModular, "Modular") }
