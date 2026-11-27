package kgen

import (
	"strings"
	"testing"
)

// Monk Class's phase ability animates the card it exiled, granting it a
// Mode$ Continuous body (StPlay: MayPlay$ True from exile, gated on the
// turn's spell count). The kernel reads an OvGate Continuous body only on the
// source slot's own record (statCond -> ovGranted(st.sl)), so a record on a
// non-Self holder never switched it on and the lowering was refused. The
// lowering now keeps the body as a source static gated by the override record
// the Animate applies. The exiled holder's record is off the battlefield, so
// the grant stays inert in both engines (gorge refuses the CheckSVar-gated
// may-play Animate route, d572f27986): the accepted card is the lowering's
// contract.
func TestAnimateContinuousGrantOnNonSelfHolder(t *testing.T) {
	c := lowerNamedCard(t, "Monk Class")
	if len(c.Refusals) != 0 {
		t.Fatalf("Monk Class refusals = %v, want none", c.Refusals)
	}
	if got := overrideLIR(c); !strings.Contains(got, "grantstatics=1") {
		t.Errorf("Monk Class's phase Animate lacks a granted static:\n%s", got)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	if got := sb.String(); !strings.Contains(got, "static MayPlayFrom") || !strings.Contains(got, "zone=5 ovgate=1") {
		t.Errorf("Monk Class's granted StPlay is not the OvGate-gated exile permission:\n%s", got)
	}
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
