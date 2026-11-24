package kgen

import "testing"

// A rewritten Soulgorger Orgg: the enters trigger stores all but one life and
// loses it, the leaves trigger gains the stored number back. Both read the
// stored name through ExSVarNum (the number store, lower_storenum.go).
const payLifeOrgg = `Name:Pay Life Orgg
ManaCost:3 R R
Types:Creature Nightmare Orgg
PT:6/6
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDrain | TriggerDescription$ drain
SVar:TrigDrain:DB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ AllButOne | SubAbility$ DBLose
SVar:DBLose:DB$ LoseLife | LifeAmount$ LifePaidOnETB
SVar:LifePaidOnETB:Number$0
SVar:AllButOne:Count$YourLifeTotal/Minus.1
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ TrigRefund | TriggerDescription$ refund
SVar:TrigRefund:DB$ GainLife | LifeAmount$ LifePaidOnETB
`

// TestPayLifeOrggLowers: the Orgg lowers with no refusal.
func TestPayLifeOrggLowers(t *testing.T) {
	c := lowerScriptCard(t, "Pay Life Orgg", payLifeOrgg)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
}
