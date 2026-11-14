package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterProbe is Wash Away's printed front half with the ValidTgts$ spelling
// swapped in.
func counterProbe(vt string) string {
	return "Name:Counter Probe\nManaCost:U\nTypes:Instant\n" +
		"A:SP$ Counter | TargetType$ Spell | ValidTgts$ " + vt + " | TgtPrompt$ Select target spell | SpellDescription$ Counter target spell.\n" +
		"Oracle:Counter target spell.\n"
}

// TestCounterTargetCastProvenance: a plain Counter slot's ValidTgts$ admits the
// cast-provenance origin words, read off the candidate spell's stack entry
// (Wash Away's Card.!wasCastFromTheirHand), and nowhere else outside the
// SpellCast/CantBeCast carriers: a CopySpellAbility slot keeps refusing.
func TestCounterTargetCastProvenance(t *testing.T) {
	for _, tc := range []struct {
		vt   string
		want int // CmpN of the lowered clause
	}{
		{"Card.!wasCastFromTheirHand", spellHand | spellNot},
		{"Card.wasCastFromTheirHand", spellHand},
		{"Card.!wasCastFromYourHand", spellHand | spellNot},
		{"Card.wasCastFromExile", spellExile},
	} {
		c := lowerScriptCard(t, "Counter Probe", counterProbe(tc.vt))
		if len(c.Refusals) != 0 {
			t.Errorf("Counter ValidTgts$ %s refused: %v", tc.vt, c.Refusals)
			continue
		}
		var found bool
		for _, a := range c.Abilities {
			for _, tg := range a.Targets {
				if tg.F == nil || len(tg.F.Clauses) != 1 {
					continue
				}
				cl := tg.F.Clauses[0]
				found = cl.CmpStat == cmpWasCastSpell && cl.CmpN == tc.want && tg.Zone == zStack && tg.Spell
			}
		}
		if !found {
			t.Errorf("Counter ValidTgts$ %s: no TgtSpell slot with a cast-provenance clause of CmpN %d", tc.vt, tc.want)
		}
	}

	// the families that read the chooser or more state stay refused
	for _, vt := range []string{"Card.wasCastFromYourHandByYou", "Card.wasCast"} {
		got := lowerScript(t, "Counter Probe", counterProbe(vt))
		if len(got) == 0 {
			t.Errorf("Counter ValidTgts$ %s lowered, want a refusal", vt)
		}
	}

	// the target gate is the Counter slot's own: CopySpellAbility reads the same
	// stack but keeps the CantBeCast-only refusal
	copySrc := "Name:Copy Probe\nManaCost:U\nTypes:Instant\n" +
		"A:SP$ CopySpellAbility | TargetType$ Spell | ValidTgts$ Instant.!wasCastFromTheirHand | TgtPrompt$ Select target spell | SpellDescription$ Copy target spell.\n" +
		"Oracle:Copy target instant spell.\n"
	got := lowerScript(t, "Copy Probe", copySrc)
	if !strings.Contains(strings.Join(got, "; "), "wasCastFromTheirHand outside a CantBeCast") {
		t.Errorf("CopySpellAbility ValidTgts$ with wasCastFromTheirHand: refusals = %q, want the CantBeCast-only refusal", got)
	}
}

// TestEnchantedPlayerCtrlLowers: Trespasser's Curse's ChangesZone ValidCard$
// Creature.EnchantedPlayerCtrl lowers to PredWord7 W7EnchPlayerCtrl on the
// battlefield; read off it (a hand ChangeZone's ChangeType$), kgen refuses.
func TestEnchantedPlayerCtrlLowers(t *testing.T) {
	c := lowerCorpusCard(t, "Trespasser's Curse")
	if len(c.Refusals) != 0 {
		t.Fatalf("Trespasser's Curse refused: %v", c.Refusals)
	}
	var found bool
	for _, a := range c.Abilities {
		if a.Trigger == nil {
			continue
		}
		for _, cl := range a.Trigger.F.Clauses {
			found = found || cl.CmpStat == cmpWord7 && cl.CmpN == int(kop.W7EnchPlayerCtrl)
		}
	}
	if !found {
		t.Errorf("Trespasser's Curse: no trigger clause reads W7EnchPlayerCtrl")
	}

	known := func(string) bool { return false }
	f, err := parseFilter("Creature.EnchantedPlayerCtrl", known, known)
	if err != nil || len(f.Clauses) != 1 || f.Clauses[0].CmpStat != cmpWord7 || f.Clauses[0].CmpN != int(kop.W7EnchPlayerCtrl) {
		t.Fatalf("parse: got %s, %v", f, err)
	}
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
	if err != nil || kf.Pred != kop.PredWord7 || kf.PredN&0xff != kop.W7EnchPlayerCtrl {
		t.Errorf("battlefield read: got %+v, %v", kf, err)
	}
	if _, err := cb.clause(f.Clauses[0], kop.ZoneGraveyard); err == nil || !strings.Contains(err.Error(), "EnchantedPlayerCtrl off the battlefield") {
		t.Errorf("graveyard read: err %v, want the named refusal", err)
	}
}
