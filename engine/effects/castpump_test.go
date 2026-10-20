package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The CastPumpBonus contract at the read boundary: a literal, non-negative
// creature pump comes back as the compiled NumAtt$/NumDef$ pair, and every
// unknown answers 0/0 (a curse, a mixed-sign pump, a dynamic amount, a
// non-creature target, a no-op, and any other ability). The bot's A6 pump
// timing rule reads this fact, so a reverted reader cannot silently turn an
// instant pump into a no-op.
func TestCastPumpBonusReadsTheCompiledPump(t *testing.T) {
	cases := []struct {
		name         string
		sa           *cards.SA
		power, tough int32
	}{
		{
			name:  "literal +3/+3",
			sa:    &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "+3", "NumDef": "+3"}},
			power: 3, tough: 3,
		},
		{
			name:  "power only",
			sa:    &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "+2"}},
			power: 2,
		},
		{
			name: "negative curse is unknown",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "-2", "NumDef": "-2"}},
		},
		{
			name: "mixed signs are unknown",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "+2", "NumDef": "-2"}},
		},
		{
			name: "dynamic amount is unknown",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "+X", "NumDef": "+X"}},
		},
		{
			name: "zero pump is a no-op",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "0", "NumDef": "0"}},
		},
		{
			name: "non-creature target is not read",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Artifact", "NumAtt": "+1"}},
		},
		{
			name: "opponent-only target is not read",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature.OppCtrl", "NumAtt": "+3", "NumDef": "+3"}},
		},
		{
			name: "you-dont-control target is not read",
			sa:   &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Creature.YouDontCtrl", "NumAtt": "+2"}},
		},
		{
			name:  "a creature alternative still names a creature",
			sa:    &cards.SA{API: "Pump", Params: map[string]string{"ValidTgts": "Artifact,Creature.YouCtrl", "NumAtt": "+1"}},
			power: 1,
		},
		{
			name: "unheld API pairs with nothing",
			sa:   &cards.SA{API: "DealDamage", Params: map[string]string{"ValidTgts": "Creature", "NumAtt": "+3", "NumDef": "+3"}},
		},
		{
			name: "nil ability",
		},
	}
	for _, c := range cases {
		power, tough := CastPumpBonus(c.sa)
		if power != c.power || tough != c.tough {
			t.Errorf("%s: CastPumpBonus = %d/%d, want %d/%d", c.name, power, tough, c.power, c.tough)
		}
	}
}
