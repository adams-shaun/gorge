package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func TestCastCombatTrick(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "pump",
			src:  "Name:Giant Growth\nManaCost:G\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "curse",
			src:  "Name:Fleeting Distraction\nManaCost:U\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ -1 | NumDef$ -1 | SpellDescription$ x\n",
		},
		{
			name: "dynamic amount",
			src:  "Name:Bulk Up\nManaCost:1 R\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ Double | SpellDescription$ x\n",
		},
		{
			name: "permanent pump",
			src:  "Name:Enduring Growth\nManaCost:G\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | Duration$ Permanent | SpellDescription$ x\n",
		},
		{
			name: "keyword only",
			src:  "Name:Divine Resilience\nManaCost:W\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature.YouCtrl | KW$ Indestructible | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "pump all own",
			src:  "Name:Claws Out\nManaCost:3 W W\nTypes:Instant\nOracle:x\nA:SP$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ +2 | NumDef$ +2 | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "pump all opponent",
			src:  "Name:Wrong Mass Pump\nManaCost:1 B\nTypes:Instant\nOracle:x\nA:SP$ PumpAll | ValidCards$ Creature.OppCtrl | NumAtt$ +2 | NumDef$ +2 | SpellDescription$ x\n",
		},
		{
			name: "put counter",
			src:  "Name:Snakeskin Veil\nManaCost:G\nTypes:Instant\nOracle:x\nA:SP$ PutCounter | ValidTgts$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "minus counter",
			src:  "Name:Shrink Counter\nManaCost:B\nTypes:Instant\nOracle:x\nA:SP$ PutCounter | ValidTgts$ Creature | CounterType$ M1M1 | CounterNum$ 1 | SpellDescription$ x\n",
		},
		{
			name: "protection",
			src:  "Name:Shield\nManaCost:W\nTypes:Instant\nOracle:x\nA:SP$ Protection | ValidTgts$ Creature | Gains$ red | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "protection without gains",
			src:  "Name:Empty Shield\nManaCost:W\nTypes:Instant\nOracle:x\nA:SP$ Protection | ValidTgts$ Creature | SpellDescription$ x\n",
		},
		{
			name: "counter",
			src:  "Name:Refute\nManaCost:1 U\nTypes:Instant\nOracle:x\nA:SP$ Counter | TargetType$ Spell | ValidTgts$ Card | SpellDescription$ x\n",
		},
		{
			name: "charm",
			src:  "Name:Charm\nManaCost:1 R\nTypes:Instant\nOracle:x\nA:SP$ Charm | Choices$ Dmg,Pump | SpellDescription$ x\nSVar:Dmg:DB$ DealDamage | NumDmg$ 3 | ValidTgts$ Any | SpellDescription$ x\nSVar:Pump:DB$ Pump | ValidTgts$ Creature | NumAtt$ +3 | SpellDescription$ x\n",
		},
		{
			name: "pump plus fight",
			src:  "Name:Bite Down\nManaCost:1 G\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature.YouCtrl | SubAbility$ Dmg | SpellDescription$ x\nSVar:Dmg:DB$ DealDamage | NumDmg$ 3 | Defined$ Targeted | SpellDescription$ x\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card, diags := cards.ParseBytes("castcombat_test.go", []byte(tc.src))
			if len(diags) != 0 {
				t.Fatalf("parse: %v", diags)
			}
			card.Link()
			if got := CastCombatTrick(card.Faces[0].SpellAbility()); got != tc.want {
				t.Fatalf("CastCombatTrick = %v, want %v", got, tc.want)
			}
		})
	}
}
