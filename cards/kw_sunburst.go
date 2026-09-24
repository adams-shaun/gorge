// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strconv"

// kwSunburst expands the Sunburst keyword (CR 702.47, task kw:Sunburst):
// "This object enters with a +1/+1 counter on it for each color of mana
// spent to cast it. If it isn't a creature, it instead enters with that many
// charge counters on it." The corpus carries only the bare K:Sunburst line
// (15 files, no parameter), so the expansion supplies the whole entry
// replacement: a Moved -> Battlefield Updated Repl whose body puts
// Count$Converge counters (CR 107.4f: the number of DISTINCT colours spent
// to cast the spell, captured at pay time by the FlagConverged CastInfo that
// rules/cast.go's faceWantsConverge / sunburstGrantOut gates arm for
// sunburst carriers) with the counter KIND decided on the printed face's
// types -- P1P1 for a creature, CHARGE otherwise, the same decision Forge's
// CardFactoryUtil expansion makes.
//
// The `DB$ Animate | Keywords$ Sunburst` grant shape (Solar Array, Lux
// Artillery) delivers the keyword only to the object's DERIVED keyword list
// -- the printed face never carries the line -- so it cannot route through
// this expander; rules/replacement.go's sunburstEntryMatch builds the
// identical synthetic Repl for the granted shape and SKIPS printed carriers
// (their face already carries this expansion, which the replacement scan
// collects), which is what keeps one entry from counting twice.
func kwSunburst(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("R", k) {
		return
	}
	kind := "CHARGE"
	if f.IsCreature() {
		kind = "P1P1"
	}
	sv := "__kwSunburst" + strconv.Itoa(i)
	f.setSVar(sv, "DB$ PutCounter | Defined$ Self | CounterType$ "+kind+" | CounterNum$ Count$Converge | ETB$ True")
	p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
		" | Keyword$ Sunburst")
	p["KeywordLine"] = k
	f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
}

func init() { registerKeyword(kwSunburst, "Sunburst") }
