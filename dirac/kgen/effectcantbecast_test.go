package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEffectCantBeCastEmblem: a no-target Effect whose StaticAbilities$ body
// is Mode$ CantBeCast lowers to a CantBeCast emblem static under the key
// whitelist gorge's effEffect registers (CantBeCastParamsReadable); a key the
// whitelist does not name, or one the lowering does not read, refuses by
// name. Bilbo's Gambit is the "players can't cast spells this turn" emblem;
// Avatar's Wrath's "from anywhere other than their hands" needs the cast
// origin of the spell being cast, which the kernel's cast gate cannot read,
// so it stays refused by that word.
func TestEffectCantBeCastEmblem(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Bilbo's Gambit\nAvatar's Wrath\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		byName[c.Name] = c
	}

	b := byName["Bilbo's Gambit"]
	if b == nil || len(b.Refusals) != 0 {
		t.Fatalf("Bilbo's Gambit: %+v, want lowered clean", b)
	}
	if len(b.Emblems) != 1 {
		t.Fatalf("Bilbo's Gambit: %d emblems, want 1", len(b.Emblems))
	}
	if e := b.Emblems[0]; !e.UntilEOT || e.UntilNextTurn || e.WhileHost || len(e.Statics) != 1 ||
		e.Statics[0].Kind != "CantBeCast" || len(e.Statics[0].Flags) != 0 {
		t.Errorf("Bilbo's Gambit emblem = %+v, want one unflagged CantBeCast until end of turn", e)
	}

	a := byName["Avatar's Wrath"]
	if a == nil {
		t.Fatal("Avatar's Wrath: not lowered")
	}
	var got []string
	for _, rf := range a.Refusals {
		got = append(got, rf.Reason)
	}
	if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), "wasCastFromTheirHand") {
		t.Errorf("Avatar's Wrath refusals = %q, want one naming wasCastFromTheirHand", got)
	} else {
		t.Logf("Avatar's Wrath: %q", got)
	}

	// The same carrier with a caster scope and the next-turn lifetime lowers
	// with the flag and the next-turn emblem.
	root := "AB$ Effect | Cost$ 1 | Duration$ UntilYourNextTurn | StaticAbilities$ ST"
	if got := effectSourceSynthetic(t, root, map[string]string{"ST": "Mode$ CantBeCast | ValidCard$ Card | Caster$ Opponent"}); len(got) != 0 {
		t.Errorf("opponent-scoped CantBeCast emblem refused: %q", got)
	}
	// A CantBeCast param gorge's whitelist does not name refuses by name
	// (gorge registers nothing for it).
	got = effectSourceSynthetic(t, root, map[string]string{"ST": "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player | ValidCause$ Spell"})
	if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), "ValidCause") {
		t.Errorf("unread ValidCause$ refusals = %q, want one naming ValidCause", got)
	}
	// A whitelisted param the lowering does not read refuses by name too.
	got = effectSourceSynthetic(t, root, map[string]string{"ST": "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player | Origin$ Hand"})
	if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), "Origin") {
		t.Errorf("unread Origin$ refusals = %q, want one naming Origin", got)
	}
	// The node condition is read after the emblem; a Condition key it does
	// not read still refuses.
	got = effectSourceSynthetic(t, "AB$ Effect | Cost$ 1 | ConditionBogus$ x | StaticAbilities$ ST",
		map[string]string{"ST": "Mode$ CantBeCast | ValidCard$ Card | Caster$ Player"})
	if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), "ConditionBogus") {
		t.Errorf("unread ConditionBogus$ refusals = %q, want one naming it", got)
	}
}
