package kgen

import "testing"

// manaSpentCondSpells read ConditionManaSpent$ in their own resolution chain
// ("if {R} was spent to cast this spell"); each lowers to the resolving
// spell's per-colour spend read (spellManaSpentCondition), Mythos of Brokkos's
// two-colour list to the conjunction.
var manaSpentCondSpells = []string{
	"Boros Fury-Shield", "Dryad's Caress", "Firespout", "Flash Conscription", "Induce Paranoia",
	"Invert the Skies", "Mythos of Brokkos", "Repel Intruders", "Ribbons of Night", "River's Grasp",
	"Rolling Spoil", "Seed Spark", "Torrent of Souls", "Unnerving Assault",
}

func TestManaSpentCondSlice(t *testing.T) {
	for _, name := range manaSpentCondSpells {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
