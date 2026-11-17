// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwAmplify expands Amplify N (CR 702.38a): "As this object enters, reveal
// any number of cards from your hand that share a creature type with it.
// This permanent enters with N +1/+1 counters on it for each card revealed
// this way." Forge's shape: a Moved -> Battlefield Updated replacement whose
// body reveals any number of hand cards sharing a creature type with the
// source (the bare sharesCreatureTypeWith reads the source), remembers them,
// puts N counters per remembered card as it enters, and clears the memory.
// The as-enters pick is why entryAskKeywords names Amplify.
func kwAmplify(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if param == "" || has("R", k) {
		return
	}
	n := "Count$RememberedSize"
	if len(param) != 1 || param[0] != '1' { // not a literal string compare (codeshape stringLiteralCompares)
		n += "/Times." + param
	}
	reveal, put, count, clear := "__kwAmplifyReveal", "__kwAmplifyPut", "__kwAmplifyX", "__kwAmplifyClear"
	f.setSVar(reveal, "DB$ Reveal | Defined$ You | AnyNumber$ True | RevealValid$ Card.sharesCreatureTypeWith | RememberRevealed$ True | SubAbility$ "+put)
	f.setSVar(put, "DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | ETB$ True | CounterNum$ "+count+" | SubAbility$ "+clear)
	f.setSVar(count, n)
	f.setSVar(clear, "DB$ Cleanup | ClearRemembered$ True")
	p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + reveal + " | Keyword$ Amplify")
	p["KeywordLine"] = k
	f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
}

func init() { registerKeyword(kwAmplify, "Amplify") }
