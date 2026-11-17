// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwIngest expands Ingest (CR 702.115a): "Whenever this creature deals
// combat damage to a player, that player exiles the top card of their
// library." The trigger is the Renown shape (a combat DamageDone to a
// player); the body digs the damaged player's top card into exile.
func kwIngest(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.addKeywordTrigger(head, k,
		"Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | TriggerDescription$ Ingest",
		"DB$ Dig | Defined$ TriggeredTarget | DigNum$ 1 | ChangeNum$ All | DestinationZone$ Exile", has)
}

func init() { registerKeyword(kwIngest, "Ingest") }
