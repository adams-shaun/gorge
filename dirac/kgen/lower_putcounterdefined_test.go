package kgen

import (
	"strings"
	"testing"
)

// TestPutCounterChoicesDefinedUnread: gorge's Choices$ PutCounter pick ignores
// Defined$ (putCounterChooseTitled takes the recipient from the pick alone;
// Forge's CountersPutEffect Choices$ branch never reads it either), so kgen
// lowers the pick with Defined$ present and unread. The pick is asserted on
// the prog: a ChooseCard over the Choices$ pool followed by the counters on
// the chosen card.
//
// Gluntch, the Bestower's real block is the second subtest: with Defined$
// accepted the block now reaches its next unmodelable parameter, its
// Chooser$/Placer$ ChosenPlayer (the mid-resolution chosen player the
// kernel does not hold yet), and refuses there -- never on Defined$ again.
func TestPutCounterChoicesDefinedUnread(t *testing.T) {
	svars := func(put string) map[string]string {
		return map[string]string{
			"R":     "SP$ Draw | Defined$ You | NumCards$ 1 | SubAbility$ DBPut",
			"DBPut": "DB$ PutCounter | " + put,
		}
	}

	t.Run("defined beside a plain pick lowers", func(t *testing.T) {
		rs, prog := kfxLowered(t, svars(
			"Defined$ ChosenCard | Choices$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 2"))
		if len(rs) != 0 {
			t.Fatalf("refused: %v", rs)
		}
		all := strings.Join(prog, "\n")
		if !kfxHas(prog, "ChooseCard") || !kfxHas(prog, "PutCounter obj=Chosen A=2 CS=P1P1") {
			t.Fatalf("want a recipient pick and the counters on it, got:\n%s", all)
		}
		if !kfxHas(prog, "CS=PutCounterPick") {
			t.Errorf("the pick is not a counter recipient pick:\n%s", all)
		}
	})

	t.Run("the chosen-player block refuses at its chooser", func(t *testing.T) {
		// Gluntch, the Bestower's Counters SVar verbatim.
		rs, _ := kfxLowered(t, svars(
			"Defined$ ChosenPlayer | Choices$ Creature.ControlledBy ChosenPlayer | Chooser$ ChosenPlayer | "+
				"Placer$ ChosenPlayer | CounterType$ P1P1 | CounterNum$ 2"))
		all := strings.Join(rs, " | ")
		if !kfxHas(rs, "PutCounter Choices$ with Chooser$") {
			t.Fatalf("want the block refused at its Chooser$, got %q", all)
		}
		if strings.Contains(all, "with Defined$") {
			t.Errorf("the Defined$ spelling refused again: %q", all)
		}
	})
}
