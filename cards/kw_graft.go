// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

func kwGraft(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	n := strings.TrimSpace(param)
	if !has("R", k) {
		put := "__kwGraftPut" + strconv.Itoa(i)
		f.setSVar(put, "DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ "+n+" | ETB$ True")
		p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + put + " | Keyword$ Graft")
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
	}
	if !has("T", k) {
		move := "__kwGraftMove" + strconv.Itoa(i)
		f.setSVar(move, "DB$ MoveCounter | Source$ Self | Defined$ TriggeredCard | CounterType$ P1P1 | CounterNum$ 1")
		p := parseParams("Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other | TriggerZones$ Battlefield | OptionalDecider$ You | Execute$ " + move + " | Keyword$ Graft | TriggerDescription$ Graft " + n)
		p["KeywordLine"] = k
		f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
	}
}

func init() { registerKeyword(kwGraft, "Graft") }
