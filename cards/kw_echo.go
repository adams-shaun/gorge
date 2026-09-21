// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwEcho(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.35a: "At the beginning of your upkeep, if this permanent
	// came under your control since the beginning of your most recent
	// upkeep, you may pay {cost}. If you don't, sacrifice it." Expanded
	// into the ordinary Phase-trigger pipeline like Cumulative upkeep
	// (normal APNAP ordering, stack interaction, response windows). The
	// intervening-if rides the Echo$ True marker the same way
	// Annihilator$ rides its generated trigger: rules/trigger_match.go
	// checks it against the object's control-acquisition tuple
	// (Object.AcqTurn/AcqStep vs Player.LastUpkeepTurn) and suppresses
	// the trigger before it stacks when the gate is false. The election
	// itself (pay-or-sacrifice) is rules/echo.go's resolution-time flow.
	// param may include Forge's trailing display text after a colon;
	// only the first field is the echo cost.
	cost, _, _ := strings.Cut(param, ":")
	f.addKeywordTrigger(head, k,
		"Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | TriggerDescription$ Echo | Echo$ True",
		"DB$ Echo | Cost$ "+cost, has)
}

func init() { registerKeyword(kwEcho, "Echo") }
