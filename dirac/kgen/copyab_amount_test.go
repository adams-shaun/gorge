package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// copyAbSynth lowers and builds a synthetic creature holding one activated
// copy-an-ability ability whose extra params are the argument.
func copyAbSynth(t *testing.T, extra string) (*lCard, built) {
	t.Helper()
	src := "Name:Synthetic\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\n" +
		"A:AB$ CopySpellAbility | Cost$ T | TargetType$ Triggered.YouCtrl | ValidTgts$ Card | MayChooseTarget$ True" + extra + "\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	card, diags := cards.ParseBytes("synthetic.txt", []byte(src))
	if card == nil {
		t.Fatalf("unparsable: %v", diags)
	}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	if len(lc.Refusals) != 0 {
		return lc, built{}
	}
	return lc, buildCard(newPoolTables(&lowered{cards: []*lCard{lc}}, nil), lc)
}

// copyAbInstr is the built ExtCopyAbility instruction of b's one ability.
func copyAbInstr(t *testing.T, b built) (kop.Instr, kop.Ability) {
	t.Helper()
	if len(b.refusals) != 0 {
		t.Fatalf("build refused: %v", b.refusals)
	}
	for _, a := range b.def.Abilities {
		for _, in := range a.Prog {
			if in.Ext() == kop.ExtCopyAbility {
				return in, a
			}
		}
	}
	t.Fatalf("no ExtCopyAbility in %d abilities", len(b.def.Abilities))
	return kop.Instr{}, kop.Ability{}
}

// TestCopyAbilityAmountAndCantCopy: a copy-an-ability effect lowers its
// Amount$ into the instruction's B operand and its target slot into C>>1, and
// CantCopy$ True lands on the ability's table byte; any other CantCopy$
// value refuses.
func TestCopyAbilityAmountAndCantCopy(t *testing.T) {
	t.Run("constant amount", func(t *testing.T) {
		lc, b := copyAbSynth(t, " | Amount$ 2")
		if len(lc.Refusals) != 0 {
			t.Fatalf("refused: %v", lc.Refusals)
		}
		in, a := copyAbInstr(t, b)
		if in.B != kop.Const(2) || in.C&1 != 1 || in.C>>1 != 0 {
			t.Errorf("B %v C %d, want B Const(2), C&1 1, slot 0", in.B, in.C)
		}
		if a.CantCopy {
			t.Errorf("CantCopy set without CantCopy$")
		}
	})
	t.Run("no amount is one copy", func(t *testing.T) {
		lc, b := copyAbSynth(t, "")
		if len(lc.Refusals) != 0 {
			t.Fatalf("refused: %v", lc.Refusals)
		}
		if in, _ := copyAbInstr(t, b); in.B != kop.Const(1) {
			t.Errorf("B %v, want Const(1)", in.B)
		}
	})
	t.Run("announced X and CantCopy", func(t *testing.T) {
		lc, b := copyAbSynth(t, " | Amount$ X | CantCopy$ True")
		if len(lc.Refusals) != 0 {
			t.Fatalf("refused: %v", lc.Refusals)
		}
		in, a := copyAbInstr(t, b)
		if in.B.K == kop.VConst {
			t.Errorf("B %v is a constant, want the announced X", in.B)
		}
		if !a.CantCopy {
			t.Errorf("CantCopy$ True did not reach the ability table")
		}
	})
	t.Run("other CantCopy values refuse", func(t *testing.T) {
		lc, _ := copyAbSynth(t, " | CantCopy$ Maybe")
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		if !strings.Contains(strings.Join(got, "|"), `CantCopy$ "Maybe"`) {
			t.Errorf("refusals %v, want a CantCopy$ refusal", got)
		}
	})
}
