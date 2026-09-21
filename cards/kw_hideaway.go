// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

func kwHideaway(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("R", k) {
		return
	}
	// Hideaway is an enters-the-battlefield replacement. Keep the
	// keyword parameter as data so its varying N is not lost.
	n := strings.TrimSpace(param)
	if n == "" {
		n = "4"
	}
	sv := "__kwHideaway" + strconv.Itoa(i)
	f.setSVar(sv, "DB$ Hideaway | Amount$ "+n)
	p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ Hideaway")
	p["KeywordLine"] = k
	f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
}

func init() { registerKeyword(kwHideaway, "Hideaway") }
