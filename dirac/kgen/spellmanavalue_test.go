package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spellmanavalueForms: the mana value of the spell that triggered a
// SpellCast ability, as gorge's last known information
// (TriggeredSpellAbility$CardManaCostLKI -> ExTriggerSpellMVLKI). The
// carrier's SpellCast trigger reads the pre-cast card: a split card sums its
// halves, an announced {X} is not counted, and a copied spell reads its
// copied face plus X.
var spellmanavalueForms = []formCase{
	{card: "Manaplasm", form: "TriggeredSpellAbility$CardManaCostLKI (both pump halves)", lir: "0 PumpUntilEOT obj=Self A=TriggerSpellMVLKI B=TriggerSpellMVLKI"},
	{card: "Erratic Cyclops", form: "TriggeredSpellAbility$CardManaCostLKI (power only)", lir: "0 PumpUntilEOT obj=Self A=TriggerSpellMVLKI B=0"},
	{card: "Kaervek the Merciless", form: "TriggeredSpellAbility$CardManaCostLKI (damage)", lir: "0 DealDamage obj=Target0 A=TriggerSpellMVLKI"},
	{card: "Skittering Cicada", form: "TriggeredSpellAbility$CardManaCostLKI beside a keyword grant", lir: "0 PumpUntilEOT obj=Self A=TriggerSpellMVLKI B=TriggerSpellMVLKI\n     1 GrantKeyword obj=Self CS=Trample"},
	{card: "Hammerhead Tyrant", form: "TriggeredSpellAbility$CardManaCostLKI in the target filter (the X SVar)", lir: "cmp(2 2 0 X )"},
	{card: "Prismari Apprentice", form: "SpellCastOrCopy TriggeredSpellAbility$CardManaCostLKI (or-copy gate)", lir: "0 GrantKeyword obj=Self CS=Unblockable\n     1 SkipIf if=Value(op1,TriggerSpellMVLKI,5) skip=1\n     2 PutCounter obj=Self A=1 CS=P1P1"},
	// scope pin: the value alone does not lift Narci (the plain
	// CardManaCost spelling on a non-SpellCast trigger stays refused)
	{card: "Narci, Fable Singer", refused: "AbilityResolves"},
}

func TestTriggeredSpellManaValueForms(t *testing.T) {
	checkForms(t, spellmanavalueForms)
	// Dovescape scope pin: its SpellCast body counters the triggering spell
	// and then creates X tokens, so the value lowers. The card itself stays
	// blocked at the token script on a corpus that lacks it (the pinned
	// corpus); a corpus that carries the script lowers the whole card. What
	// must never change is the refusal: the value head is gone either way.
	t.Run("Dovescape scope", func(t *testing.T) {
		cardsDir := ratchetCards(t)
		dir := t.TempDir()
		list := filepath.Join(dir, "cards.txt")
		if err := os.WriteFile(list, []byte("Dovescape\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := loadCardList(list, cardsDir)
		if err != nil {
			t.Fatal(err)
		}
		lw := lowerPool(p)
		if len(lw.cards) != 1 {
			t.Fatalf("Dovescape: %d lowered cards", len(lw.cards))
		}
		c := lw.cards[0]
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if got == "" {
			var sb strings.Builder
			dumpLIR(&sb, c)
			if !strings.Contains(sb.String(), "1 CreateToken obj=TriggeredPlayer A=TriggerSpellMVLKI CS=wu_1_1_bird_flying") {
				t.Errorf("Dovescape: lowered IR lacks the token count:\n%s", sb.String())
			}
			return
		}
		if !strings.Contains(got, "token script") {
			t.Errorf("Dovescape: want the token script refusal (the value must lower), got %q", got)
		}
	})
}
