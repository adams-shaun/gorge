package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Ram Through (DealDamage DamageSource$ ParentTarget | ExcessDamage$
// ThisTargetedController | ExcessDamageCondition$ Card.withTrample; CR 120.4a,
// 120.10): a trampling biter whose power exceeds what is lethal to the bitten
// creature deals it exactly the lethal amount and the rest to its controller,
// in the one damage batch (gorge emitObjectDamageExcess; kernel
// kop.BiteExcessTrample). The mutation (DebugBiteNoExcess) keeps the whole
// hit on the creature.
var ramThroughDeck = []cardset.Entry{
	{Name: "Ram Through", Count: 8}, {Name: "Colossal Dreadmaw", Count: 3}, {Name: "Charging Monstrosaur", Count: 3},
	{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 6},
	{Name: "Forest", Count: 14}, {Name: "Mountain", Count: 2},
}

// TestRamThroughExcessLockstep: four-seat games of the Ram Through deck play in
// lockstep, and the excess-free kernel diverges.
func TestRamThroughExcessLockstep(t *testing.T) {
	atkShapeLockstep(t, ramThroughDeck, 24, 0, 0, nil, nil, &dirac.DebugBiteNoExcess)
}
