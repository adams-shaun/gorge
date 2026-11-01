package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kSpellTargetedForms: the SpellTargeted$CardManaCostLKI value head -- the
// spell the resolving spell targeted, read where it now is (gorge
// refTargetsSpellTargeted through the shared CardManaCostLKI property arm).
// The lowered read is the kernel's existing ExMVOf/targetMV: the live
// object's printed mana value, plus X only while it is still on the stack
// (CR 202.3e). Sound the Trumpets counters the spell and then recruits if
// its mana value was 2 or less.
//
// Real `kgen -lir` output pinned here:
//
//	CARD Sound the Trumpets ...
//	  ab[0] Spell ...
//	    target[0] mode=-1 zone=6 min=1 max=1 F={types=0x0} ... spell=true
//	     0 Counter obj=Target0
//	     1 SkipIf if=Value(op2,MVOf(Target0),2) skip=1
//	     2 Recruit CS=w_1_1_human_soldier
var kSpellTargetedForms = []formCase{
	{card: "Sound the Trumpets", form: "SpellTargeted$CardManaCostLKI (ExMVOf over the countered spell's target slot)", lir: "0 Counter obj=Target0\n     1 SkipIf if=Value(op2,MVOf(Target0),2) skip=1\n     2 Recruit CS=w_1_1_human_soldier"},
	// Scope pins: both carriers keep their ChangeZone refusal, which comes
	// BEFORE the value head -- the new case must not move them.
	{card: "Press the Enemy", form: "ChangeZone Origin$ Battlefield,Stack refuses before its SpellTargeted SVar", refused: `Origin$ Battlefield,Stack ValidTgts$`},
	{card: "Gale's Redirection", form: "ChangeZone Origin$ Stack shape refuses before its SpellTargeted SVar", refused: "ChangeZone Origin$ Stack shape"},
}

func TestSpellTargetedMVLKI(t *testing.T) {
	checkForms(t, kSpellTargetedForms)
	// The ChangeZone refusals must not mention the value head: if the head
	// slid in front of them their refusal chains would carry SpellTargeted$.
	t.Run("scope", func(t *testing.T) {
		cardsDir := ratchetCards(t)
		dir := t.TempDir()
		list := filepath.Join(dir, "cards.txt")
		if err := os.WriteFile(list, []byte("Press the Enemy\nGale's Redirection\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := loadCardList(list, cardsDir)
		if err != nil {
			t.Fatal(err)
		}
		lw := lowerPool(p)
		if len(lw.cards) != 2 {
			t.Fatalf("scope: %d lowered cards, want 2", len(lw.cards))
		}
		for _, c := range lw.cards {
			var reasons []string
			for _, r := range c.Refusals {
				reasons = append(reasons, r.Reason)
			}
			got := strings.Join(reasons, "; ")
			if !strings.Contains(got, "ChangeZone") {
				t.Errorf("%s: want the ChangeZone refusal, got %q", c.Name, got)
			}
			if strings.Contains(got, "SpellTargeted") || strings.Contains(got, "value head") {
				t.Errorf("%s: refusal chain reaches the value head: %q", c.Name, got)
			}
		}
	})
}
