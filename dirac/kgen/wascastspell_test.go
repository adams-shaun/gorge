package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const kwsTrig = "T:Mode$ SpellCast | ValidCard$ %s | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 1"

// TestWascastspellLowering: a cast-provenance word in a SpellCast ValidCard$
// lowers to PredWasCastSpell with the origin and negation in PredN, wasCast
// beside a word folds into it, and a spell ability's Self ConditionPresent$
// reads the same predicate.
func TestWascastspellLowering(t *testing.T) {
	for _, c := range []struct {
		vc string
		n  int16
	}{
		{"Card.!wasCastFromYourHand", spellHand | spellNot},
		{"Card.wasCastFromYourHand", spellHand},
		{"Card.wasCastFromExile", spellExile},
		{"Card.wasCast+!wasCastFromYourHand", spellNotHand},
		{"Card.!wasCastFromYourHand+wasCast", spellNotHand},
		{"Card.wasCast+wasCastFromExile", spellExile},
		{"Card.wasCast", spellCast},
		{"Card.!wasCast", spellNot},
	} {
		refs, preds := kwascastLower(t, strings.Replace(kwsTrig, "%s", c.vc, 1))
		if len(refs) != 0 {
			t.Errorf("%s refused: %v", c.vc, refs)
			continue
		}
		if n, ok := preds[kop.PredWasCastSpell]; !ok || n != c.n {
			t.Errorf("%s lowered to preds %v, want PredWasCastSpell %d", c.vc, preds, c.n)
		}
	}

	cond := "A:SP$ Draw | Cost$ 2 B | NumCards$ 1 | SubAbility$ DBDraw\nSVar:DBDraw:DB$ Draw | NumCards$ 1 | ConditionDefined$ Self | ConditionPresent$ Card.wasCast+!wasCastFromYourHand | ConditionCompare$ EQ1"
	refs, preds := kwascastLowerHead(t, "Name:Cast Probe\nManaCost:2 B\nTypes:Sorcery\n", cond)
	if len(refs) != 0 {
		t.Errorf("spell condition refused: %v", refs)
	}
	if n, ok := preds[kop.PredWasCastSpell]; !ok || n != spellNotHand {
		t.Errorf("spell condition lowered to preds %v, want PredWasCastSpell %d", preds, spellNotHand)
	}
}

// TestWascastspellRefusals: the word reads a spell on the stack, so any
// other filter naming it refuses by name: a static, a creature's ETB
// condition, and two provenance words that disagree. A mana ability's spend
// restriction lowers it (TestWascastrestrictLowering).
func TestWascastspellRefusals(t *testing.T) {
	for name, lines := range map[string]string{
		"static":          "S:Mode$ Continuous | Affected$ Creature.YouCtrl+wasCastFromExile | AddPower$ 1",
		"not cast + from": strings.Replace(kwsTrig, "%s", "Card.!wasCast+wasCastFromExile", 1),
		"graveyard word":  strings.Replace(kwsTrig, "%s", "Card.wasCastFromGraveyard", 1),
		"trigger body":    "T:Mode$ Phase | Phase$ Upkeep | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1 | ConditionDefined$ Self | ConditionPresent$ Card.wasCastFromExile",
	} {
		refs, preds := kwascastLower(t, lines)
		if len(refs) == 0 {
			t.Errorf("%s lowered (preds %v), want a refusal", name, preds)
		}
	}
}
