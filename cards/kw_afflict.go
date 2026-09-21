// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwAfflict(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.130: "Whenever this creature becomes blocked, defending
	// player loses N life." The parameter is the life amount; every
	// corpus K:Afflict line carries one (measured 10/10). The engine's
	// become-blocked hook is trig:AttackerBlocked
	// (checkAttackerBlockedTriggers), which queues one instance per
	// blocked attacker and captures the defender as the trigger
	// context's DefendingPlayer -- exactly the Defined$ the body reads.
	// A keyword granted in a layer (AddKeyword$ Afflict:N, e.g. Lost
	// Monarch of Ifnir's Zombie grant) needs no expansion here: rules'
	// checkGrantedAfflictTriggers synthesizes the same trigger from the
	// derived keyword list.
	if strings.TrimSpace(param) == "" {
		return
	}
	f.addKeywordTrigger(head, k, "Mode$ AttackerBlocked | ValidCard$ Card.Self | TriggerDescription$ Afflict",
		"DB$ LoseLife | Defined$ TriggeredDefendingPlayer | LifeAmount$ "+strings.TrimSpace(param), has)
}

func init() { registerKeyword(kwAfflict, "Afflict") }
