package kgen

import (
	"strings"
	"testing"
)

// rickScript is Rick, Steadfast Leader / Greymond's shape (renamed): a
// three-way as-enters GenericChoice whose branches each store a flag, three
// CheckSVar$ statics reading them, a four-Humans +2/+2 and a leaves reset.
const rickScript = `Name:Rick Probe
ManaCost:2 W W
Types:Legendary Creature Human Soldier
PT:3/4
K:ETBReplacement:Other:PickTwo
SVar:PickTwo:DB$ GenericChoice | Choices$ BrA,BrB,BrC | SpellDescription$ choose
SVar:BrA:DB$ StoreSVar | Type$ Number | SVar$ FlagA | Expression$ 1 | SpellDescription$ one
SVar:BrB:DB$ StoreSVar | Type$ Number | SVar$ FlagB | Expression$ 1 | SpellDescription$ two
SVar:BrC:DB$ StoreSVar | Type$ Number | SVar$ FlagC | Expression$ 1 | SpellDescription$ three
S:Mode$ Continuous | CheckSVar$ FlagA | SVarCompare$ EQ1 | Affected$ Human.YouCtrl | AddKeyword$ First Strike & Vigilance
S:Mode$ Continuous | CheckSVar$ FlagB | SVarCompare$ EQ1 | Affected$ Human.YouCtrl | AddKeyword$ Vigilance & Lifelink
S:Mode$ Continuous | CheckSVar$ FlagC | SVarCompare$ EQ1 | Affected$ Human.YouCtrl | AddKeyword$ First Strike & Lifelink
S:Mode$ Continuous | IsPresent$ Human.YouCtrl | PresentCompare$ GE4 | Affected$ Human.YouCtrl | AddPower$ 2 | AddToughness$ 2
T:Mode$ ChangesZone | Static$ True | ValidCard$ Card.Self | Origin$ Battlefield | Destination$ Any | Execute$ Reset1
SVar:Reset1:DB$ StoreSVar | Type$ Number | SVar$ FlagA | Expression$ 0 | SubAbility$ Reset2
SVar:Reset2:DB$ StoreSVar | Type$ Number | SVar$ FlagB | Expression$ 0 | SubAbility$ Reset3
SVar:Reset3:DB$ StoreSVar | Type$ Number | SVar$ FlagC | Expression$ 0
SVar:FlagA:Number$0
SVar:FlagB:Number$0
SVar:FlagC:Number$0
`

// TestRickLowers: the as-enters three-way choice, its three flag stores, the
// CheckSVar$ statics and the leaves reset lower with no refusal, in the shape
// the kernel reads: one mode ask over three branches, each storing its own
// flag; three statics gated on the stored number; the reset an off-stack
// static trigger of three zero writes.
func TestRickLowers(t *testing.T) {
	c := lowerScriptCard(t, "Rick Probe", rickScript)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	var ask, branches int
	var stored []string
	var resetStatic bool
	progs := [][]lInstr{}
	for _, r := range c.Repls {
		progs = append(progs, r.Prog)
	}
	for _, a := range c.Abilities {
		progs = append(progs, a.Prog)
	}
	for pi, prog := range progs {
		stores := 0
		for _, in := range prog {
			switch {
			case in.Op == "GenericChoice" && in.C == 2 && in.A.N == 3:
				ask++
			case in.Op == "Branch":
				branches++
			case in.Op == "ExtStoreNum":
				stores++
				stored = append(stored, in.CS)
				if in.C != 0<<1|1 {
					t.Errorf("write of %s has C %d, want the delete flag over the printed 0", in.CS, in.C)
				}
			}
		}
		if a := pi - len(c.Repls); a >= 0 && c.Abilities[a].Trigger != nil && c.Abilities[a].Trigger.Static && stores == 3 {
			resetStatic = true
		}
	}
	if ask != 1 || branches != 3 {
		t.Errorf("%d mode asks and %d branches, want 1 and 3", ask, branches)
	}
	if !resetStatic {
		t.Error("no off-stack static trigger writing the three flags back to 0")
	}
	if got := strings.Join(stored, ","); got != "FlagA,FlagB,FlagC,FlagA,FlagB,FlagC" {
		t.Errorf("writes %s, want each flag once as a choice and once as the reset", got)
	}
	reads := 0
	for _, st := range c.Statics {
		if st.Cond.A.Kind == vSVarNum && st.Cond.B.isConst() && st.Cond.B.N == 1 {
			reads++
		}
	}
	if reads != 3 {
		t.Errorf("%d statics read a stored flag, want 3", reads)
	}
}

// TestRickRefusals: the shapes the flag choice does not model refuse by name.
func TestRickRefusals(t *testing.T) {
	for _, tc := range []struct{ name, from, to, want string }{
		{"a branch beyond a flag", "SVar$ FlagA | Expression$ 1 | SpellDescription$ one", "SVar$ FlagA | Expression$ 1 | UnlessCost$ 1 | SpellDescription$ one", "with UnlessCost$"},
		{"a non-literal flag", "Type$ Number | SVar$ FlagB | Expression$ 1", "Type$ Calculate | SVar$ FlagB | Expression$ X", "beyond a Number flag"},
		{"a choice with a target player", "Choices$ BrA,BrB,BrC |", "Choices$ BrA,BrB,BrC | Defined$ Player.Opponent |", "with Defined$"},
	} {
		src := strings.Replace(rickScript, tc.from, tc.to, 1)
		if src == rickScript {
			t.Fatalf("%s: the script has no %q", tc.name, tc.from)
		}
		c := lowerScriptCard(t, "Rick Probe", src)
		var all []string
		for _, rf := range c.Refusals {
			all = append(all, rf.Reason)
		}
		if !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, all, tc.want)
		}
	}
}
