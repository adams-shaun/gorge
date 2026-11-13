package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// joinForcesProg is every instruction of every ability of c, in order.
func joinForcesProg(c *lCard) []lInstr {
	var out []lInstr
	for _, ab := range c.Abilities {
		out = append(out, ab.Prog...)
	}
	return out
}

// joinForcesShape asserts the accumulator's lowering: the loop, an
// ExtJoinForces adding the chosen number inside it, and a vJoinForces read
// after it. It returns the program for further checks.
func joinForcesShape(t *testing.T, c *lCard) []lInstr {
	t.Helper()
	prog := joinForcesProg(c)
	loop, add, read := -1, -1, -1
	for i, in := range prog {
		switch {
		case in.Op == "RepeatEach" && loop < 0:
			loop = i
		case in.Op == "ExtJoinForces" && in.B.Kind == vChosenNumber && add < 0:
			add = i
		}
		for _, v := range []lValue{in.A, in.B} {
			if v.Kind == vJoinForces && read < 0 {
				read = i
			}
		}
	}
	if loop < 0 || add < loop || read < add {
		t.Fatalf("%s: RepeatEach@%d, ExtJoinForces(vChosenNumber)@%d, vJoinForces read@%d: want ascending", c.Name, loop, add, read)
	}
	return prog
}

// TestJoinForcesAccumulatorLowers: the join-forces cards lower with no
// refusal (Collective Voyage keeps only its each-player library search).
func TestJoinForcesAccumulatorLowers(t *testing.T) {
	for _, name := range []string{"Minds Aglow", "Alliance of Arms", "Shared Trauma"} {
		c := lowerCorpusCard(t, name)
		for _, rf := range c.Refusals {
			t.Errorf("%s refused: %s", name, rf.Reason)
		}
		joinForcesShape(t, c)
	}
	// the reset after the read lowers as ExtJoinForces 0
	prog := joinForcesProg(lowerCorpusCard(t, "Shared Trauma"))
	if last := prog[len(prog)-1]; last.Op != "ExtJoinForces" || last.C != kop.JoinForcesReset {
		t.Errorf("Shared Trauma's last op = %s C=%d, want the ExtJoinForces reset (C %d)", last.Op, last.C, kop.JoinForcesReset)
	}
	cv := lowerCorpusCard(t, "Collective Voyage")
	if len(cv.Refusals) != 1 || !strings.Contains(cv.Refusals[0].Reason, "ChangeZone DefinedPlayer$ Player") {
		t.Fatalf("Collective Voyage refusals = %v, want only ChangeZone DefinedPlayer$ Player", cv.Refusals)
	}
}

// TestJoinForcesBlocksTriggerLowers: Mana-Charged Dragon lowers both its
// Attacks and Blocks triggers: the RepeatEach's Player.IsRemembered binding
// is not a Blocks trigger's remembered attacker.
func TestJoinForcesBlocksTriggerLowers(t *testing.T) {
	c := lowerCorpusCard(t, "Mana-Charged Dragon")
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s %s", rf.Node, rf.Reason)
	}
	joinForcesShape(t, c)
	nAdd := 0
	for _, in := range joinForcesProg(c) {
		if in.Op == "ExtJoinForces" && in.B.Kind == vChosenNumber {
			nAdd++
		}
	}
	if nAdd != 2 {
		t.Errorf("%d accumulator writes, want 2 (one per trigger)", nAdd)
	}
	// an object Remembered read on a Blocks trigger still refuses
	got := lowerScript(t, "Probe Blocker", `Name:Probe Blocker
ManaCost:1
Types:Creature Bear
PT:2/2
T:Mode$ Blocks | ValidCard$ Card.Self | Execute$ TrigX | TriggerDescription$ x
SVar:TrigX:DB$ Pump | Defined$ Remembered | NumAtt$ +1
`)
	if len(got) == 0 {
		t.Errorf("a Blocks trigger reading Remembered lowered; want a refusal")
	}
}

const joinForcesMinds = `Name:Probe Aglow
ManaCost:U
Types:Sorcery
A:SP$ RepeatEach | RepeatPlayers$ %s | StartingWith$ You | RepeatSubAbility$ DBPay | SubAbility$ DBCards | SpellDescription$ x
SVar:DBPay:DB$ ChooseNumber | ChooseAnyNumber$ True | Defined$ Player.IsRemembered | SubAbility$ DBAdd
SVar:DBAdd:DB$ StoreSVar | UnlessSwitched$ True | UnlessPayer$ %s | UnlessCost$ Y | Type$ CountSVar | SVar$ PoolTotal | Expression$ PoolTotal/Plus.Y
SVar:DBCards:DB$ Draw | NumCards$ PoolTotal | Defined$ Player | SubAbility$ DBReset
SVar:DBReset:DB$ StoreSVar | Type$ Number | SVar$ PoolTotal | Expression$ 0
SVar:Y:%s
SVar:PoolTotal:Number$0
`

// TestJoinForcesAccumulatorRefusesOtherShapes: only the one exact shape
// lowers; each deviation refuses by name.
func TestJoinForcesAccumulatorRefusesOtherShapes(t *testing.T) {
	probe := func(rp, payer, y string) string {
		return sprintf3(joinForcesMinds, rp, payer, y)
	}
	// the control: the exact shape lowers, so each refusal below is its edit's
	if got := lowerScript(t, "Probe Aglow", probe("Player", "Player.IsRemembered", "Count$ChosenNumber")); len(got) != 0 {
		t.Fatalf("control shape refused: %v", got)
	}
	cases := []struct {
		name, src, want string
	}{
		{"wrong Y body", probe("Player", "Player.IsRemembered", "Count$YourLifeTotal"), "api StoreSVar"},
		{"outside a player RepeatEach", `Name:Probe Lone
ManaCost:U
Types:Sorcery
A:SP$ StoreSVar | UnlessSwitched$ True | UnlessPayer$ Player.IsRemembered | UnlessCost$ Y | Type$ CountSVar | SVar$ PoolTotal | Expression$ PoolTotal/Plus.Y | SubAbility$ DBCards
SVar:DBCards:DB$ Draw | Defined$ You | NumCards$ PoolTotal
SVar:Y:Count$ChosenNumber
SVar:PoolTotal:Number$0
`, "api StoreSVar"},
		{"opponents only", probe("Player.Opponent", "Player.IsRemembered", "Count$ChosenNumber"), "api StoreSVar"},
		{"UnlessPayer You", probe("Player", "You", "Count$ChosenNumber"), "api StoreSVar"},
		{"read from another top-level ability", `Name:Probe Two
ManaCost:U
Types:Creature Bear
PT:1/1
A:AB$ RepeatEach | Cost$ T | RepeatPlayers$ Player | StartingWith$ You | RepeatSubAbility$ DBPay | SubAbility$ DBCards | SpellDescription$ x
SVar:DBPay:DB$ ChooseNumber | ChooseAnyNumber$ True | Defined$ Player.IsRemembered | SubAbility$ DBAdd
SVar:DBAdd:DB$ StoreSVar | UnlessSwitched$ True | UnlessPayer$ Player.IsRemembered | UnlessCost$ Y | Type$ CountSVar | SVar$ PoolTotal | Expression$ PoolTotal/Plus.Y
SVar:DBCards:DB$ Draw | NumCards$ PoolTotal | Defined$ Player | SubAbility$ DBReset
SVar:DBReset:DB$ StoreSVar | Type$ Number | SVar$ PoolTotal | Expression$ 0
A:AB$ Draw | Cost$ 1 | Defined$ You | NumCards$ PoolTotal | SpellDescription$ y
SVar:Y:Count$ChosenNumber
SVar:PoolTotal:Number$0
`, "outside the chain that writes it"},
		{"read after the reset", strings.Replace(probe("Player", "Player.IsRemembered", "Count$ChosenNumber"),
			"SVar:DBReset:DB$ StoreSVar | Type$ Number | SVar$ PoolTotal | Expression$ 0",
			"SVar:DBReset:DB$ StoreSVar | Type$ Number | SVar$ PoolTotal | Expression$ 0 | SubAbility$ DBAgain\nSVar:DBAgain:DB$ Draw | Defined$ You | NumCards$ PoolTotal", 1), "after its reset"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lowerScript(t, "Probe", tc.src)
			if len(got) == 0 {
				t.Fatalf("lowered; want a refusal containing %q", tc.want)
			}
			for _, r := range got {
				if strings.Contains(r, tc.want) {
					return
				}
			}
			t.Fatalf("refusals %v do not name %q", got, tc.want)
		})
	}
}

// sprintf3 fills joinForcesMinds' three %s slots.
func sprintf3(f, a, b, c string) string {
	f = strings.Replace(f, "%s", a, 1)
	f = strings.Replace(f, "%s", b, 1)
	return strings.Replace(f, "%s", c, 1)
}
