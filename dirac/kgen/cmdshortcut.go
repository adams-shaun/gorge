package kgen

// The kernel now reads IsCommander on the battlefield, so a Commander game
// where a card reaches one of these lowerings would diverge: each assumes no
// seat has a commander. The lowering is left as it is because the cmd5 pool
// reaches none of them; TestCmd5PoolNoCommanderShortcuts reads the counters
// and fails when a future Commander card does.
const (
	cmdShortcutNegatedWord = iota
	cmdShortcutCommandBeacon
	cmdShortcutCommanderMana
	cmdShortcutCommandSweep
	numCmdShortcuts
)

var cmdShortcutHits [numCmdShortcuts]int

// noteCmdShortcut records that one card's lowering took a no-commander
// shortcut. Only the cmd5 pool guard reads the counters; a table build does
// not.
func noteCmdShortcut(which int) { cmdShortcutHits[which]++ }
