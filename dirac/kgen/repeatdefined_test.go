package kgen

import (
	"strings"
	"testing"
)

// TestRepeatDefinedCorpusCards lowers the two RepeatDefined$ cards that
// block decks (Tainted Pact, Demonlord Belzenlok) and checks the shape the
// kernel plays: the defined gate over the remembered/imprinted set, the
// sibling check gate, and the body writers (Pump RememberObjects$ Imprinted,
// DigUntil ImprintFound$+RememberFound$, Imprinted$Amount).
func TestRepeatDefinedCorpusCards(t *testing.T) {
	t.Run("Tainted Pact", func(t *testing.T) {
		lc := lowerCorpusCard(t, "Tainted Pact")
		for _, rf := range lc.Refusals {
			t.Errorf("refused: %s", rf.Reason)
		}
		ab := lc.Abilities[0]
		var loop *lInstr
		for i := range ab.Prog {
			if ab.Prog[i].Op == "RepeatWhile" {
				loop = &ab.Prog[i]
			}
		}
		if loop == nil {
			t.Fatal("no RepeatWhile")
		}
		if loop.Cond.Kind != cAnd || len(loop.Cond.Sub) != 2 {
			t.Fatalf("gate %v, want an AND of two conditions", loop.Cond)
		}
		defined := loop.Cond.Sub[0]
		if defined.Kind != cAnd || len(defined.Sub) != 2 || defined.Sub[0].Ref.Kind != rImprinted {
			t.Fatalf("defined gate %v, want an AND over Imprinted", defined)
		}
		if f := defined.Sub[0].F; f == nil || len(f.Clauses) != 1 || f.Clauses[0].CmpStat != cmpSameNameRem {
			t.Fatalf("defined filter %v, want sharesNameWith Remembered", f)
		}
		if check := defined.Sub[1]; check.Kind != cValue || check.A.Kind != vCountHand {
			t.Fatalf("check gate %v, want Count$ValidHand", check)
		}
		if guard := loop.Cond.Sub[1]; guard.Kind != cValue || guard.A.Kind != vLibrarySizeOf {
			t.Fatalf("stored guard %v, want the library-nonempty read", guard)
		}
		want := []string{"RememberImprinted", "ClearImprinted", "Dig", "DigImprint", "SkipIf", "MayAsk", "Bounce"}
		got := progOps(lc)[1:] // progOps includes the RepeatWhile header
		if strings.Join(got[:len(want)], " ") != strings.Join(want, " ") {
			t.Fatalf("body %v, want prefix %v", got, want)
		}
		if ab.Prog[1].Op != "RememberImprinted" {
			t.Fatalf("first instruction A %v, want ExtRememberImprinted", ab.Prog[0].A)
		}
	})
	t.Run("Demonlord Belzenlok", func(t *testing.T) {
		lc := lowerCorpusCard(t, "Demonlord Belzenlok")
		for _, rf := range lc.Refusals {
			t.Errorf("refused: %s", rf.Reason)
		}
		ab := lc.Abilities[0]
		if ab.Prog[0].Op != "RepeatWhile" {
			t.Fatalf("first op %s, want RepeatWhile", ab.Prog[0].Op)
		}
		cond := ab.Prog[0].Cond
		if cond.Kind != cRefMatches || cond.Ref.Kind != rRemembered || cond.F == nil || len(cond.F.Clauses) != 1 ||
			cond.F.Clauses[0].CmpStat != cmpCMC || cond.F.Clauses[0].CmpOp != opGE || cond.F.Clauses[0].CmpN != 4 {
			t.Fatalf("gate %v, want Remembered cmcGE4", cond)
		}
		var dig, ret, dmg *lInstr
		for i := range ab.Prog {
			switch ab.Prog[i].Op {
			case "DigUntil":
				dig = &ab.Prog[i]
			case "Return":
				ret = &ab.Prog[i]
			case "DealDamage":
				dmg = &ab.Prog[i]
			}
		}
		if dig == nil || !strings.Contains(dig.CS, "ImprintFound") || !strings.Contains(dig.CS, "RememberFound") {
			t.Fatalf("DigUntil %v, want ImprintFound|RememberFound", dig)
		}
		if ret == nil || ret.Obj.Kind != rRemembered || ret.C != 2 {
			t.Fatalf("Return %v, want Remembered -> hand", ret)
		}
		if dmg == nil || dmg.A.Kind != vRefCount || dmg.A.Ref.Kind != rChosen {
			t.Fatalf("damage %v, want Imprinted$Amount over the chosen set", dmg)
		}
	})
}
