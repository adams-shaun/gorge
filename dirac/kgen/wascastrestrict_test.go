package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kwrLower lowers a synthetic card (header, script lines) and returns its
// lowered form.
func kwrLower(t *testing.T, head, lines string) *lCard {
	t.Helper()
	src := head + lines + "\n"
	card, diags := cards.ParseBytes("cast_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Cast Probe"}, byName: map[string]*cards.Card{"Cast Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerPool(p).cards[0]
}

const (
	kwrArtifact = "Name:Cast Probe\nManaCost:4\nTypes:Artifact\n"
	kwrGrantee  = "Name:Cast Probe\nManaCost:3 U U\nTypes:Legendary Creature Jellyfish Advisor\nPT:3/4\n"
	kwrGrant    = "S:Mode$ Continuous | Affected$ Artifact.YouCtrl | AddAbility$ AnyMana\nSVar:AnyMana:AB$ Mana | Cost$ T | Produced$ U | RestrictValid$ %s"
)

// kwrRestrOf is the one clause of a printed mana ability's spend restriction.
func kwrRestrOf(t *testing.T, c *lCard) lClause {
	t.Helper()
	for _, ab := range c.Abilities {
		if ab.Kind == akMana && ab.Mana != nil && ab.Mana.Restrict != nil {
			if len(ab.Mana.Restrict.Clauses) != 1 {
				t.Fatalf("restriction has %d clauses", len(ab.Mana.Restrict.Clauses))
			}
			return ab.Mana.Restrict.Clauses[0]
		}
	}
	t.Fatalf("no restricted mana ability in %v", c.Refusals)
	return lClause{}
}

// TestWascastrestrictLowering: a spell cast-provenance word in a mana
// ability's RestrictValid$ lowers to PredWasCastSpell on its Restrict
// (printed and granted); every other provenance spelling, the Activated term
// and the other granted restrictions still refuse.
func TestWascastrestrictLowering(t *testing.T) {
	for _, c := range []struct {
		word string
		n    int
	}{
		{"wasCastFromExile", spellExile},
		{"!wasCastFromYourHand", spellHand | spellNot},
		{"wasCastFromYourHand", spellHand},
		{"wasCastFromYourHandByYou", spellHandBy},
		{"wasCast+!wasCastFromYourHand", spellNotHand},
	} {
		printed := kwrLower(t, kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Spell."+c.word)
		if len(printed.Refusals) != 0 {
			t.Errorf("printed %s refused: %v", c.word, printed.Refusals)
			continue
		}
		if cl := kwrRestrOf(t, printed); cl.CmpStat != cmpWasCastSpell || cl.CmpN != c.n {
			t.Errorf("printed %s: CmpStat %v CmpN %d, want cmpWasCastSpell %d", c.word, cl.CmpStat, cl.CmpN, c.n)
		}
	}

	// Interdimensional Web Watch's Combo Any x2 production lowers too: the
	// kernel follows gorge's prefix search for it
	ww := kwrLower(t, kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ Combo Any | Amount$ 2 | RestrictValid$ Spell.wasCastFromExile")
	if len(ww.Refusals) != 0 {
		t.Fatalf("combo any refused: %v", ww.Refusals)
	}
	if cl := kwrRestrOf(t, ww); cl.CmpStat != cmpWasCastSpell || cl.CmpN != spellExile || cl.CmpN&15 != 3 {
		t.Errorf("combo any: CmpStat %v CmpN %d, want cmpWasCastSpell %d", cl.CmpStat, cl.CmpN, spellExile)
	}

	// Mm'menon's granted ability: Latent, with the restriction
	g := kwrLower(t, kwrGrantee, strings.Replace(kwrGrant, "%s", "Spell.!wasCastFromYourHand", 1))
	if len(g.Refusals) != 0 {
		t.Fatalf("granted !wasCastFromYourHand refused: %v", g.Refusals)
	}
	n := 0
	for _, ab := range g.Abilities {
		if ab.Latent && ab.Kind == akMana {
			n++
			if ab.Mana == nil || ab.Mana.Restrict == nil || len(ab.Mana.Restrict.Clauses) != 1 ||
				ab.Mana.Restrict.Clauses[0].CmpStat != cmpWasCastSpell || ab.Mana.Restrict.Clauses[0].CmpN != spellHand|spellNot {
				t.Errorf("granted ability restriction %+v, want cmpWasCastSpell %d", ab.Mana, spellHand|spellNot)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d latent mana abilities, want 1", n)
	}
}

// TestWascastrestrictRefusals: the spellings that are not gorge's
// pending-guarded families stay refused by name.
func TestWascastrestrictRefusals(t *testing.T) {
	for name, c := range map[string]struct{ head, lines, want string }{
		"bare wasCast":      {kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Spell.wasCast", "bare wasCast"},
		"not wasCast":       {kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Spell.!wasCast", "bare wasCast"},
		"graveyard":         {kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Spell.wasCastFromGraveyard", "wasCastFromGraveyard on a spell"},
		"activated":         {kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Activated.wasCastFromExile", "cast-provenance word outside"},
		"spell and bare":    {kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Spell.wasCastFromExile,Spell.wasCast", "bare wasCast"},
		"granted graveyard": {kwrGrantee, strings.Replace(kwrGrant, "%s", "Spell.wasCastFromGraveyard", 1), "wasCastFromGraveyard on a spell"},
		"granted activated": {kwrGrantee, strings.Replace(kwrGrant, "%s", "Activated.wasCastFromExile", 1), "cast-provenance word outside"},
		"granted ability":   {kwrGrantee, strings.Replace(kwrGrant, "%s", "Activated.Artifact", 1), "granted mana ability with a spend restriction"},
	} {
		lc := kwrLower(t, c.head, c.lines)
		found := false
		for _, r := range lc.Refusals {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if !found {
			t.Errorf("%s: refusals %v, want one containing %q", name, lc.Refusals, c.want)
		}
	}
}

// TestCantCastSpellFromHandLowering: the bare CantCastSpellFromHand term is a
// spell restriction on the hand-provenance word with every non-spell payment
// admitted.
func TestCantCastSpellFromHandLowering(t *testing.T) {
	lc := kwrLower(t, kwrArtifact, "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ CantCastSpellFromHand")
	if len(lc.Refusals) != 0 {
		t.Fatalf("refused: %v", lc.Refusals)
	}
	if cl := kwrRestrOf(t, lc); cl.CmpStat != cmpWasCastSpell || cl.CmpN != spellHand|spellNot {
		t.Errorf("restriction CmpStat %v CmpN %d, want cmpWasCastSpell %d", cl.CmpStat, cl.CmpN, spellHand|spellNot)
	}
	for _, ab := range lc.Abilities {
		if ab.Kind == akMana && !ab.Mana.NonSpell {
			t.Error("the non-spell payments are not admitted")
		}
	}
}
