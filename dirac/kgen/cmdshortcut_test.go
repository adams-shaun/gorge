package kgen

import (
	"os"
	"strings"
	"testing"
)

// TestCmd5PoolNoCommanderShortcuts: no card in the five Commander precons
// reaches a kgen lowering that assumes no seat has a commander. Those
// lowerings stay as they are, but a future Commander pool card that reaches
// one would diverge silently.
func TestCmd5PoolNoCommanderShortcuts(t *testing.T) {
	cards := tableCorpus(t)
	p, _, _, err := readPool(os.DirFS("../.."), "cmd5", cards)
	if err != nil {
		t.Fatal(err)
	}
	var hits [numCmdShortcuts][]string
	for _, name := range p.names {
		cmdShortcutHits = [numCmdShortcuts]int{}
		lowerCard(p, name, p.byName[name], false)
		for k := range cmdShortcutHits {
			if cmdShortcutHits[k] > 0 {
				hits[k] = append(hits[k], name)
			}
		}
	}
	shortcuts := [numCmdShortcuts]string{
		cmdShortcutNegatedWord:   "a negated IsCommander lowered to always holds",
		cmdShortcutCommandBeacon: "a command-zone IsCommander pick lowered to nothing",
		cmdShortcutCommanderMana: "a commander-only mana filter dropped",
		cmdShortcutCommandSweep:  "a command-zone and graveyard IsCommander sweep lowered to nothing",
	}
	for k, names := range hits {
		if len(names) > 0 {
			t.Errorf("cmd5 card lowering took a no-commander shortcut (%s): %s",
				shortcuts[k], strings.Join(names, ", "))
		}
	}
}
