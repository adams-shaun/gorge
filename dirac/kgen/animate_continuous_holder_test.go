package kgen

import "testing"

// Monk Class's phase ability animates another creature with a Continuous
// static body (StPlay). The kernel reads an OvGate Continuous body only on the
// source slot's own record (statCond -> ovGranted(st.sl)), so a record on
// another holder never switches it on: the lowering is refused until a chars
// arm reads the record's source on the holder. This was the one corpus carrier
// the fail-closed check took out of the acceptance pin.
var animateContinuousHolderForms = []formCase{
	{card: "Monk Class", form: "Animate staticAbilities$ Continuous on a non-Self holder", refused: "a Continuous grant on a non-Self holder"},
}

func TestAnimateContinuousGrantRefusesNonSelfHolder(t *testing.T) {
	checkForms(t, animateContinuousHolderForms)
}

// cardTypeBit is the RemoveTypes$ word reader: only the card types whose
// correlated subtypes the kernel strips (Creature, Artifact) are accepted, so
// a Land/Enchantment/Planeswalker word cannot leave its subtypes behind.
func TestAnimateRemoveTypesOnlyStripsCorrelatedCardTypes(t *testing.T) {
	for _, w := range []string{"Creature", "Artifact"} {
		if _, ok := cardTypeBit(w); !ok {
			t.Errorf("cardTypeBit(%q) refused a card type the kernel strips", w)
		}
	}
	for _, w := range []string{"Land", "Enchantment", "Planeswalker", "Instant", "Sorcery", "Equipment", "Legendary", "Permanent"} {
		if _, ok := cardTypeBit(w); ok {
			t.Errorf("cardTypeBit(%q) accepted a word without a kernel strip", w)
		}
	}
}
