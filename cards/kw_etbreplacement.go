// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwETBReplacement(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("R", k) {
		return
	}
	// param is "Copy:<SVar>" or "Other:<SVar>", occasionally
	// followed by further colon-separated fields real Forge reads
	// for its own bookkeeping (Mandatory/Optional, a valid-zone
	// spec, a filter): those are not part of the SVar name, so only
	// the field right after the layer tag is taken. The layer tag
	// itself (Copy vs Other) and the Optional/Mandatory field are
	// parsed past, not modeled: both are expanded identically here
	// (Ledger: replacement-semantics task owns telling a Copy-layer
	// or Optional replacement apart from a mandatory Other one).
	_, rest, _ := strings.Cut(param, ":")
	sv, _, _ := strings.Cut(rest, ":")
	p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ ETBReplacement")
	p["KeywordLine"] = k
	f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
}

func init() { registerKeyword(kwETBReplacement, "ETBReplacement") }
