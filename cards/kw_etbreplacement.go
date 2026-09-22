// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwETBReplacement(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("R", k) {
		return
	}
	// param is "Copy:<SVar>" or "Other:<SVar>", optionally followed
	// by real Forge's trailing fields
	// "...:<Mandatory|Optional>:<Zone>:<ValidCard>".
	//
	// The Zone and ValidCard fields are the replacement's real
	// applicability: Zone is where the SOURCE must be for the static to
	// function (Forge's ActiveZones$) and ValidCard is the filter the
	// ENTERING object must match. The well-modelled self-shape
	// (`Other:ChooseCT`) carries neither and keeps the historical
	// defaults (Card.Self, active from anywhere). Without them a
	// graveyard static like Dearly Departed's fired on its OWN entry and
	// a lord like Bramblewood Paragon only ever pumped itself.
	//
	// The layer tag (Copy vs Other) and the Optional/Mandatory field are
	// still parsed past, not modeled (Ledger: replacement-semantics task
	// owns telling a Copy-layer or Optional replacement apart from a
	// mandatory Other one); the 47 corpus lines carrying a trailing zone
	// spec are the population this reads.
	_, rest, _ := strings.Cut(param, ":")
	sv, tail, _ := strings.Cut(rest, ":")
	validCard, activeZones := "Card.Self", ""
	if fields := strings.Split(tail, ":"); len(fields) >= 3 {
		if z := strings.TrimSpace(fields[1]); z != "" {
			activeZones = z
		}
		if vc := strings.TrimSpace(fields[2]); vc != "" {
			validCard = vc
		}
	}
	line := "Event$ Moved | Destination$ Battlefield | ValidCard$ " + validCard + " | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ ETBReplacement"
	if activeZones != "" {
		line += " | ActiveZones$ " + activeZones
	}
	p := parseParams(line)
	p["KeywordLine"] = k
	f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
}

func init() { registerKeyword(kwETBReplacement, "ETBReplacement") }
