package kgen

import (
	"strings"
	"testing"
)

// numStoreProbe is the synthetic card: an ETB write, an activated read, an
// increment (the CountSVar form), and a write of the printed default.
const numStoreProbe = `Name:Num Store Probe
ManaCost:2
Types:Artifact
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigStore | TriggerDescription$ probe
SVar:TrigStore:DB$ StoreSVar | SVar$ N | Type$ Number | Expression$ 3
SVar:N:Number$0
A:AB$ GainLife | Cost$ T | LifeAmount$ N | SpellDescription$ probe
A:AB$ StoreSVar | Cost$ 1 | SVar$ N | Type$ CountSVar | Expression$ N/Plus.1 | SpellDescription$ probe
A:AB$ StoreSVar | Cost$ 2 | SVar$ N | Type$ Number | Expression$ 0 | SpellDescription$ probe
`

// TestNumStoreLowers: the probe lowers with no refusal to two writes of a
// literal and an increment, and a read of the stored name.
func TestNumStoreLowers(t *testing.T) {
	c := lowerScriptCard(t, "Num Store Probe", numStoreProbe)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	var stores []lInstr
	var read bool
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "ExtStoreNum" {
				stores = append(stores, in)
			}
			for _, v := range []lValue{in.A, in.B} {
				if v.Kind == vSVarNum {
					read = true
				}
			}
		}
	}
	if len(stores) != 3 {
		t.Fatalf("%d ExtStoreNum, want 3", len(stores))
	}
	if !read {
		t.Error("no read of the stored name")
	}
	var lit, inc, del *lInstr
	for i := range stores {
		switch b := stores[i].B; {
		case b.Kind == vSVarNum:
			inc = &stores[i]
		case b.isConst() && b.N == 3:
			lit = &stores[i]
		case b.isConst() && b.N == 0:
			del = &stores[i]
		}
	}
	if lit == nil || inc == nil || del == nil {
		t.Fatalf("stores %v: want a literal 3, an increment and a literal 0", stores)
	}
	if inc.B.Add != 1 || inc.B.Sub[0].N != 0 || inc.CS != "N" {
		t.Errorf("increment = %v, want ExSVarNum N + 1 over the printed 0", inc)
	}
	if del.C != 0<<1|1 || lit.C != 0<<1|1 {
		t.Errorf("writes have C %d and %d, want the delete flag over the printed 0", del.C, lit.C)
	}
}

// TestNumStoreRefusals: the shapes the store does not model refuse by name.
func TestNumStoreRefusals(t *testing.T) {
	for _, tc := range []struct{ name, from, to, want string }{
		{"triggered type", "Type$ Number | Expression$ 3", "Type$ Triggered | Expression$ 3", "Type$ Triggered"},
		{"no printed body", "SVar:N:Number$0\n", "", "no printed body"},
		{"condition read", "Cost$ 2 | SVar$ N", "Cost$ 2 | ConditionCheckSVar$ N | ConditionSVarCompare$ GE2 | SVar$ N", "CheckSVar"},
	} {
		src := strings.Replace(numStoreProbe, tc.from, tc.to, 1)
		c := lowerScriptCard(t, "Num Store Probe", src)
		var all []string
		for _, rf := range c.Refusals {
			all = append(all, rf.Reason)
		}
		if !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, all, tc.want)
		}
	}
	// a spell's write
	spell := `Name:Num Store Spell
ManaCost:1
Types:Sorcery
A:SP$ StoreSVar | Cost$ 1 | SVar$ N | Type$ Number | Expression$ 3 | SubAbility$ DBGain
SVar:DBGain:DB$ GainLife | LifeAmount$ N
SVar:N:Number$0
`
	c := lowerScriptCard(t, "Num Store Spell", spell)
	if len(c.Refusals) == 0 || !strings.Contains(c.Refusals[0].Reason, "in a spell") {
		t.Errorf("spell write: refusals %v, want one naming the spell", c.Refusals)
	}
}
