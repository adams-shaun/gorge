package state

import "strings"

// caseVariantCounterKinds maps the corpus's minority spellings of a counter
// kind to the one every reader uses. Forge resolves counter kinds through
// CounterEnumType case-insensitively, so `CounterType$ Stun` (72 PutCounter
// lines, e.g. Fear of Sleep Paralysis) IS the STUN counter that the untap
// replacement (engine/effects/untap.go) and `ValidCounterType$ STUN` read; likewise
// Overseer of Vault 76's `Quest` (paid back as RemoveAnyCounter<3/QUEST>),
// Lost Isle Calling's `Verse` (read back as CardCounters.VERSE), Shadows'
// Lair's `SubCounter<1/Dread>` (the one COST-side minority spelling: its
// front face places `CounterType$ DREAD`), and Elspeth Resplendent's
// `WithCounters$ Shield` -- the REAL shield counter (CR 122.1c), never the
// engine's mixed-case "Shield" regeneration marker.
//
// The fold is not a blanket upper-casing: keyword counters (Flying,
// Deathtouch, ...) are stored and read title-cased, and the engine's own
// "Shield"/"Deathtouched" status markers are written directly by
// effRegenerate/combat, never through this fold. The entries below are the
// complete set of script spellings that differ from the engine's kind.
//
// caseVariantCounterKinds is keyed by the UPPER-CASE spelling: the lookup
// upper-cases the script kind first, matching Forge's case-insensitive
// CounterEnumType.getType. A kind outside this table (a title-cased keyword
// counter, a player counter such as "Experience", a grammar token such as
// "All"/"Any"/"EachFromSource") is returned unchanged.
var caseVariantCounterKinds = map[string]string{
	"STUN":   "STUN",
	"QUEST":  "QUEST",
	"VERSE":  "VERSE",
	"DREAD":  "DREAD",
	"SHIELD": "SHIELD",
}

// CanonicalCounterKind folds a counter kind (or each entry of a comma list)
// onto its canonical spelling; anything else is returned unchanged. It is the
// ONE home of the fold: the PutCounter parameter reader (effects), the
// ChangeZone/token entry-counter helpers (effects), and the SubCounter /
// RemoveAnyCounter cost parser (engine/rules/cost) all call it, so a kind
// cannot be placed under one spelling and read back under another.
func CanonicalCounterKind(kind string) string {
	if !strings.Contains(kind, ",") {
		return canonicalCounterKindOne(kind)
	}
	parts := strings.Split(kind, ",")
	for i, p := range parts {
		parts[i] = canonicalCounterKindOne(p)
	}
	return strings.Join(parts, ",")
}

// canonicalCounterKindOne folds one script spelling. strings.ToUpper is the
// case-insensitive half of Forge's lookup; only the table's entries are
// re-spelled, so upper-casing cannot turn a title-cased keyword counter
// ("Flying") or a grammar token into a different kind.
func canonicalCounterKindOne(kind string) string {
	if k, ok := caseVariantCounterKinds[strings.ToUpper(strings.TrimSpace(kind))]; ok {
		return k
	}
	return strings.TrimSpace(kind)
}
