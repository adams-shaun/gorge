package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// flipProg returns the lowered program of the probe's first ability, failing
// the test on any refusal.
func flipProg(t *testing.T, name, src string) []lInstr {
	t.Helper()
	c := lowerScriptCard(t, name, src)
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	if len(c.Abilities) == 0 {
		t.Fatal("no lowered ability")
	}
	return c.Abilities[0].Prog
}

// flipRefusal returns the probe's refusals joined, failing when there is none.
func flipRefusal(t *testing.T, name, src string) string {
	t.Helper()
	c := lowerScriptCard(t, name, src)
	if len(c.Refusals) == 0 {
		t.Fatalf("lowered with no refusal, want one")
	}
	return fmt.Sprint(c.Refusals)
}

// TestFlipCoinAmountLowers: a multi-coin FlipCoin (gorge effFlipCoin's
// Amount$ loop) lowers to Repeat over a batch flip and its branch blocks,
// closed by the Mark that sets the turn bit; the per-flip Wins/Losses
// publications are 1/0 by block; Amount$ X is clamped to >= 1; NoCall$ with
// several coins refuses, and so does Count$RememberedNumber after a
// RememberNumber$ tally (its lowering would be the remembered-object count).
func TestFlipCoinAmountLowers(t *testing.T) {
	t.Run("three coins, a token per win", func(t *testing.T) {
		prog := flipProg(t, "Probe Coin Runner", "Name:Probe Coin Runner\nManaCost:3 R\nTypes:Creature Goblin\nPT:2/2\n"+
			"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ ProbeFlips | TriggerDescription$ probe\n"+
			"SVar:ProbeFlips:DB$ FlipCoin | RememberNumber$ Wins | Amount$ 3 | WinSubAbility$ ProbeMake\n"+
			"SVar:ProbeMake:DB$ Token | TokenScript$ r_1_1_goblin | TokenAmount$ Wins | TokenOwner$ You\nOracle:x\n")
		want := []string{"Repeat", "RollDie", "SkipIf", "CreateToken", "RollDie"}
		var got []string
		for _, in := range prog {
			got = append(got, in.Op)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Fatalf("program %v, want %v", got, want)
		}
		if !prog[0].A.isConst() || prog[0].A.N != 3 || prog[0].Skip != 3 {
			t.Errorf("Repeat A %+v skip %d, want const 3 over 3 instructions", prog[0].A, prog[0].Skip)
		}
		if prog[1].C != int(kop.RollDieCoin|kop.RollDieCoinBatch) {
			t.Errorf("batch flip C %d, want RollDieCoin|RollDieCoinBatch", prog[1].C)
		}
		if !prog[3].A.isConst() || prog[3].A.N != 1 {
			t.Errorf("Token A %+v, want const 1 (the win block's Wins)", prog[3].A)
		}
		if prog[4].C != int(kop.RollDieCoin|kop.RollDieCoinMark) {
			t.Errorf("closing flip C %d, want RollDieCoin|RollDieCoinMark", prog[4].C)
		}
	})
	t.Run("chosen number of coins is clamped", func(t *testing.T) {
		prog := flipProg(t, "Probe Coin Chooser", "Name:Probe Coin Chooser\nManaCost:1 R\nTypes:Sorcery\n"+
			"A:SP$ ChooseNumber | Defined$ You | Min$ 1 | Max$ 4 | SubAbility$ ProbeFlips\n"+
			"SVar:ProbeFlips:DB$ FlipCoin | Amount$ N | WinSubAbility$ ProbeDraw\n"+
			"SVar:ProbeDraw:DB$ Draw | NumCards$ Wins | Defined$ You\n"+
			"SVar:N:Count$ChosenNumber\nOracle:x\n")
		for _, in := range prog {
			if in.Op != "Repeat" {
				continue
			}
			a := in.A
			if a.Kind != vCompare || a.Op != opGE || a.N != 1 || len(a.Sub) != 2 || a.Sub[0].Kind != vChosenNumber {
				t.Fatalf("Repeat A %+v, want max(ChosenNumber, 1)", a)
			}
			return
		}
		t.Fatal("no Repeat for Amount$ N")
	})
	t.Run("two coins with NoCall refuses", func(t *testing.T) {
		got := flipRefusal(t, "Probe Coin Caller", "Name:Probe Coin Caller\nManaCost:2 R\nTypes:Sorcery\n"+
			"A:SP$ FlipCoin | Amount$ 2 | NoCall$ True | HeadsSubAbility$ ProbeDraw\n"+
			"SVar:ProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You\nOracle:x\n")
		if !strings.Contains(got, "NoCall") {
			t.Fatalf("refusal %s, want the NoCall$ refusal", got)
		}
	})
	t.Run("remembered tally refuses", func(t *testing.T) {
		got := flipRefusal(t, "Probe Coin Tally", "Name:Probe Coin Tally\nManaCost:2 R\nTypes:Sorcery\n"+
			"A:SP$ FlipCoin | Amount$ 2 | RememberNumber$ Wins | WinSubAbility$ ProbeDraw | SubAbility$ ProbeGain\n"+
			"SVar:ProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You\n"+
			"SVar:ProbeGain:DB$ GainLife | LifeAmount$ T | Defined$ You\n"+
			"SVar:T:Count$RememberedNumber\nOracle:x\n")
		if !strings.Contains(got, "RememberedNumber") {
			t.Fatalf("refusal %s, want the Count$RememberedNumber refusal", got)
		}
	})
	t.Run("lose block reads Losses times two", func(t *testing.T) {
		prog := flipProg(t, "Probe Coin Burn", "Name:Probe Coin Burn\nManaCost:1 R\nTypes:Sorcery\n"+
			"A:SP$ FlipCoin | Amount$ 2 | WinSubAbility$ ProbeDraw | LoseSubAbility$ ProbeBurn\n"+
			"SVar:ProbeDraw:DB$ Draw | NumCards$ Wins | Defined$ You\n"+
			"SVar:ProbeBurn:DB$ DealDamage | NumDmg$ Burn | Defined$ You\n"+
			"SVar:Burn:SVar$Losses/Times.2\nOracle:x\n")
		blocks := 0
		for i, in := range prog {
			if in.Op != "SkipIf" || i+1 >= len(prog) {
				continue
			}
			blocks++
			body := prog[i+1]
			switch {
			case in.Cond.B.N == 2 && body.Op == "DealDamage":
				a := body.A
				if a.Kind != vConst || a.Add != 0 || a.Div != 0 || a.Neg || a.N*max(a.Mul, 1) != 2 {
					t.Fatalf("lose block damage %+v, want 2", a)
				}
				return
			}
		}
		t.Fatalf("no lose-block DealDamage among %d blocks: %v", blocks, prog)
	})
}
