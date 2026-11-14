package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
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
	if err := os.WriteFile(list, []byte("Bilbo's Gambit\nAvatar's Wrath\nDrannith Magistrate\nKnowledge Pool\nAerial Extortionist\nGhostly Pilferer\nPossibility Storm\nRiver Song's Diary\nSwindler's Scheme\nWash Away\n"), 0o644); err != nil {
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
	if a == nil || len(a.Refusals) != 0 {
		t.Fatalf("Avatar's Wrath: %+v, want lowered clean", a)
	}
	if len(a.Emblems) != 1 {
		t.Fatalf("Avatar's Wrath: %d emblems, want 1", len(a.Emblems))
	}
	if e := a.Emblems[0]; !e.UntilNextTurn || len(e.Statics) != 1 || e.Statics[0].Kind != "CantBeCast" ||
		len(e.Statics[0].Flags) != 1 || e.Statics[0].Flags[0] != "ActivatorOpp" {
		t.Errorf("Avatar's Wrath emblem = %+v, want one ActivatorOpp CantBeCast until the next turn", e)
	} else if f := e.Statics[0].F; f == nil || len(f.Clauses) != 1 ||
		f.Clauses[0].CmpStat != cmpWasCastSpell || f.Clauses[0].CmpN != spellHand|spellNot {
		t.Errorf("Avatar's Wrath filter = %+v, want one !wasCastFromYourHand clause (PredN 17)", f)
	}

	// Drannith Magistrate: the same filter on a printed static.
	d := byName["Drannith Magistrate"]
	if d == nil || len(d.Refusals) != 0 {
		t.Fatalf("Drannith Magistrate: %+v, want lowered clean", d)
	}
	var cbc int
	for _, st := range d.Statics {
		if st.Kind == "CantBeCast" {
			cbc++
			if f := st.F; f == nil || len(f.Clauses) != 1 || f.Clauses[0].CmpStat != cmpWasCastSpell ||
				f.Clauses[0].CmpN != spellHand|spellNot {
				t.Errorf("Drannith Magistrate filter = %+v, want PredN 17", f)
			}
		}
	}
	if cbc != 1 {
		t.Errorf("Drannith Magistrate: %d CantBeCast statics, want 1", cbc)
	}

	// The word stays refused outside a CantBeCast ValidCard$: the SpellCast
	// triggers read a stack entry (Wash Away's Counter target lowers it: countertgtprov_test.go).
	for _, n := range []string{"Knowledge Pool", "Aerial Extortionist", "Ghostly Pilferer", "Possibility Storm", "River Song's Diary", "Swindler's Scheme"} {
		c := byName[n]
		if c == nil || !refusalsMention(c, "wasCastFromTheirHand") {
			t.Errorf("%s: want a refusal naming wasCastFromTheirHand, got %+v", n, c)
		}
	}
	// A CantBeCast origin the kernel's offer read does not cover refuses by name.
	root0 := "AB$ Effect | Cost$ 1 | Duration$ UntilYourNextTurn | StaticAbilities$ ST"
	for _, w := range []string{"Card.wasCastFromYourHandByYou", "Card.wasCast", "Card.wasCast+!wasCastFromYourHand"} {
		got := effectSourceSynthetic(t, root0, map[string]string{"ST": "Mode$ CantBeCast | ValidCard$ " + w + " | Caster$ Opponent"})
		if w == "Card.wasCast+!wasCastFromYourHand" {
			if len(got) != 0 {
				t.Errorf("%s: refused %q, want the cast-not-from-hand family lowered", w, got)
			}
			continue
		}
		if len(got) == 0 || !strings.Contains(strings.Join(got, "; "), "outside the origin families") {
			t.Errorf("%s: refusals = %q, want one naming the origin families", w, got)
		}
	}
	// A SpellCast trigger: wasCastFromTheirHand refuses (the new spelling is
	// admitted by a CantBeCast ValidCard$ only); wasCastFromYourHandByYou
	// lowers there as it did before this change.
	if got := spellCastTriggerSynthetic(t, "Card.wasCastFromTheirHand"); len(got) == 0 ||
		!strings.Contains(strings.Join(got, "; "), "wasCastFromTheirHand outside a CantBeCast") {
		t.Errorf("SpellCast ValidCard$ Card.wasCastFromTheirHand: refusals = %q, want the CantBeCast-only refusal", got)
	}
	if got := spellCastTriggerSynthetic(t, "Card.wasCastFromYourHandByYou"); len(got) != 0 {
		t.Errorf("SpellCast ValidCard$ Card.wasCastFromYourHandByYou: refusals = %q, want none (unchanged)", got)
	}

	// The same carrier with a caster scope and the next-turn lifetime lowers
	// with the flag and the next-turn emblem.
	root := "AB$ Effect | Cost$ 1 | Duration$ UntilYourNextTurn | StaticAbilities$ ST"
	got := effectSourceSynthetic(t, root, map[string]string{"ST": "Mode$ CantBeCast | ValidCard$ Card | Caster$ Opponent"})
	if len(got) != 0 {
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

func refusalsMention(c *lCard, w string) bool {
	for _, rf := range c.Refusals {
		if strings.Contains(rf.Reason, w) {
			return true
		}
	}
	return false
}

// spellCastTriggerSynthetic lowers a card whose SpellCast trigger reads
// ValidCard$ <vc> (the Knowledge Pool shape) and returns its refusals.
func spellCastTriggerSynthetic(t *testing.T, vc string) []string {
	t.Helper()
	src := "Name:Synthetic\nManaCost:6\nTypes:Artifact\n" +
		"T:Mode$ SpellCast | ValidCard$ " + vc + " | Execute$ X | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
		"SVar:X:DB$ Draw | NumCards$ 1\n"
	card, diags := cards.ParseBytes("synthetic.txt", []byte(src))
	if card == nil {
		t.Fatalf("synthetic card did not parse: %v", diags)
	}
	for _, f := range card.Faces {
		for i := range f.Triggers {
			if f.Triggers[i].Effect == nil {
				f.Triggers[i].Effect = cards.ResolveSVar(f.SVars, f.Triggers[i].Params["Execute"])
			}
		}
	}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, rf := range lowerCard(p, "Synthetic", card, false).Refusals {
		out = append(out, rf.Reason)
	}
	return out
}
