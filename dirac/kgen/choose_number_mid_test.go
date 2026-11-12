package kgen

import (
	"strings"
	"testing"
)

// A resolution-time ChooseNumber (api:ChooseNumber outside the as-enters
// replacement: Well of Lost Dreams, Yusri, the join forces cards, the
// "pay one or more {E}" family) lowers to ChooseNumberMid: the chooser, the
// literal Min$ floor, the Max$ value and the Random$ flag. Count$ChosenNumber
// and Count$YourCountersEnergy resolve to the value heads the answer is read
// through, and PayEnergy<X> carries the chosen amount into the unless cost.
// The secret election (MatchedAbility$/Guesser$) is a different question and
// stays refused by name.
func TestChooseNumberMidLowers(t *testing.T) {
	c := lowerScriptCard(t, "Probe Number", "Name:Probe Number\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Defined$ You | Min$ 1 | Max$ 5 | SubAbility$ DBDraw\n"+
		"SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ X\n"+
		"SVar:X:Count$ChosenNumber\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	var ask, draw bool
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "ChooseNumberMid" {
				ask = true
				if in.Obj.Kind != rYou || in.C&0xff != 1 || in.C&0x100 == 0 || in.C&0x200 != 0 {
					t.Errorf("ChooseNumberMid Obj %v C %#x, want You, Min 1, Max present, not random", in.Obj, in.C)
				}
				if !in.B.isConst() || in.B.N != 5 {
					t.Errorf("ChooseNumberMid B %+v, want const 5", in.B)
				}
			}
			if in.Op == "Draw" && in.A.Kind == vChosenNumber {
				draw = true
			}
		}
	}
	if !ask || !draw {
		t.Fatalf("ChooseNumberMid %v, Count$ChosenNumber read by Draw %v", ask, draw)
	}
}

func TestChooseNumberMidRandom(t *testing.T) {
	c := lowerScriptCard(t, "Probe Random Number", "Name:Probe Random Number\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Defined$ You | Min$ 1 | Max$ 3 | Random$ True\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "ChooseNumberMid" {
				if in.C&0x200 == 0 {
					t.Fatalf("Random$ True lowered without the random flag: C %#x", in.C)
				}
				return
			}
		}
	}
	t.Fatal("no ChooseNumberMid")
}

func TestChooseNumberMidEnergyBound(t *testing.T) {
	c := lowerScriptCard(t, "Probe Energy Number", "Name:Probe Energy Number\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Max$ Count$YourCountersEnergy | SubAbility$ DBDraw\n"+
		"SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ X\n"+
		"SVar:X:Count$ChosenNumber\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "ChooseNumberMid" {
				if in.B.Kind != vEnergyOf || in.C&0x100 == 0 {
					t.Fatalf("energy bound B %+v C %#x, want vEnergyOf with Max present", in.B, in.C)
				}
				return
			}
		}
	}
	t.Fatal("no ChooseNumberMid")
}

func TestChooseNumberMidPayEnergy(t *testing.T) {
	c := lowerScriptCard(t, "Probe Energy Pay", "Name:Probe Energy Pay\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Max$ Count$YourCountersEnergy | SubAbility$ DBPay\n"+
		"SVar:DBPay:DB$ Draw | Defined$ You | NumCards$ X | UnlessCost$ Mandatory PayEnergy<X> | UnlessPayer$ You | UnlessSwitched$ True\n"+
		"SVar:X:Count$ChosenNumber\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if (in.Op == "UnlessSwitched" || in.Op == "IfPaid") && in.CS == "PayEnergy" {
				if in.A.Kind != vChosenNumber {
					t.Fatalf("PayEnergy amount %+v, want vChosenNumber", in.A)
				}
				return
			}
		}
	}
	t.Fatal("no dynamic PayEnergy unless instruction")
}

func TestChooseNumberMidJoinForcesRepeat(t *testing.T) {
	c := lowerScriptCard(t, "Probe Join Forces", "Name:Probe Join Forces\nManaCost:G\nTypes:Sorcery\n"+
		"A:SP$ RepeatEach | RepeatPlayers$ Player | StartingWith$ You | RepeatSubAbility$ DBPay\n"+
		"SVar:DBPay:DB$ ChooseNumber | Defined$ Player.IsRemembered | ChooseAnyNumber$ True\n"+
		"Oracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "ChooseNumberMid" {
				if in.Obj.Kind != rEach {
					t.Fatalf("repeated chooser Obj %v, want RefEach", in.Obj)
				}
				return
			}
		}
	}
	t.Fatal("no ChooseNumberMid inside the RepeatEach body")
}

func TestChooseNumberMidRefusesTheSecretElection(t *testing.T) {
	r := lowerScript(t, "Probe Election", "Name:Probe Election\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Defined$ TargetedAndYou | Min$ 1 | Max$ 3 | MatchedAbility$ DBDraw | UnmatchedAbility$ DBDraw | Secretly$ True\n"+
		"SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
	got := ""
	for _, s := range r {
		if s != "" {
			got = s
		}
	}
	if got == "" || !strings.Contains(got, "ChooseNumber") || !strings.Contains(got, "secret election") {
		t.Fatalf("refusals %v, want the secret-election reason", r)
	}
}
