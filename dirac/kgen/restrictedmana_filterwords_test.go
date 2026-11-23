package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kfwHead is a probe Equipment, so the synthetic pool knows the Equipment
// subtype as the corpus pool does.
const kfwHead = "Name:Cast Probe\nManaCost:4\nTypes:Artifact Equipment\n"

func kfwLine(rv string) string {
	return "A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ " + rv
}

// TestRestrictedManaFilterWordsLower: Spell.withDevoid, Spell.wasCastFromYourGraveyard
// and Spell.Equipment,Activated.Equip, the filters of Corrupted Crossroads,
// Lord of the Forsaken and Ronin, Shadow Stalker, lower to a kernel
// restriction on a plain {T} source: the word's own predicate, not a refusal.
func TestRestrictedManaFilterWordsLower(t *testing.T) {
	// the kernel predicate each word builds to
	for _, c := range []struct {
		rv   string
		pred kop.Pred
		n    int16
	}{
		{"Spell.withDevoid", kop.PredWord7, kop.W7WithDevoid},
		{"Spell.wasCastFromYourGraveyard", kop.PredWasCastSpell, spellYourGrave},
	} {
		refs, preds := kwascastLowerHead(t, kfwHead, kfwLine(c.rv))
		if len(refs) != 0 {
			t.Errorf("%s refused: %v", c.rv, refs)
			continue
		}
		if n, ok := preds[c.pred]; !ok || n != c.n {
			t.Errorf("%s built to preds %v, want pred %v N %d", c.rv, preds, c.pred, c.n)
		}
	}

	// the lowered clauses
	if cl := kwrRestrOf(t, kwrLower(t, kfwHead, kfwLine("Spell.withDevoid"))); cl.CmpStat != cmpWord7 || cl.CmpN != int(kop.W7WithDevoid) {
		t.Errorf("withDevoid clause CmpStat %v CmpN %d, want cmpWord7 %d", cl.CmpStat, cl.CmpN, kop.W7WithDevoid)
	}
	if cl := kwrRestrOf(t, kwrLower(t, kfwHead, kfwLine("Spell.wasCastFromYourGraveyard"))); cl.CmpStat != cmpWasCastSpell || cl.CmpN != spellYourGrave {
		t.Errorf("wasCastFromYourGraveyard clause CmpStat %v CmpN %d, want cmpWasCastSpell %d", cl.CmpStat, cl.CmpN, spellYourGrave)
	}

	// Ronin: the spell half is the Equipment subtype, the activated half
	// matches no card (gorge reads Card.Equip as an unknown word)
	ronin := kwrLower(t, kfwHead, kfwLine("Spell.Equipment,Activated.Equip"))
	if len(ronin.Refusals) != 0 {
		t.Fatalf("Spell.Equipment,Activated.Equip refused: %v", ronin.Refusals)
	}
	var m *lMana
	for _, ab := range ronin.Abilities {
		if ab.Kind == akMana && ab.Mana != nil {
			m = ab.Mana
		}
	}
	if m == nil || m.Restrict == nil || m.RestrictAb == nil {
		t.Fatalf("no restricted mana ability with both halves: %+v", m)
	}
	if cl := m.Restrict.Clauses; len(cl) != 1 || cl[0].Subtype != "Equipment" {
		t.Errorf("spell half %+v, want the Equipment subtype", cl)
	}
	if cl := m.RestrictAb.Clauses; len(cl) != 1 || cl[0].NotTypes != tAnyCard || cl[0].Types != 0 {
		t.Errorf("activated half %+v, want one clause that matches no card", cl)
	}
	if len(m.RestrictAbZones) != 1 || m.RestrictAbZones[0] != kop.ZoneAny {
		t.Errorf("activated half zones %v, want one zone-free alternative", m.RestrictAbZones)
	}
}

// TestRestrictedManaFilterWordsCards: the two no-{T} carriers (Lord of the
// Forsaken, Ronin) stay refused for the cost of their ability (a life payment), and the refusal names no filter
// word.
func TestRestrictedManaFilterWordsCards(t *testing.T) {
	// a sole {T}, Pay 1 life any-colour restricted ability lowers (the kernel's
	// restricted-life unit), so its filter word is lowered with it
	if lc := kwrLower(t, "Name:Corrupted Crossroads\nManaCost:no cost\nTypes:Land\n",
		"A:AB$ Mana | Cost$ T PayLife<1> | Produced$ Any | Amount$ 1 | RestrictValid$ Spell.withDevoid"); len(lc.Refusals) != 0 {
		t.Errorf("Corrupted Crossroads refused: %v", lc.Refusals)
	}
	for _, c := range []struct{ name, head, line string }{
		{"Lord of the Forsaken", "Name:Lord of the Forsaken\nManaCost:2 B\nTypes:Creature Vampire\nPT:3/3\n",
			"A:AB$ Mana | Cost$ PayLife<1> | Produced$ C | RestrictValid$ Spell.wasCastFromYourGraveyard"},
		{"Ronin, Shadow Stalker", "Name:Ronin, Shadow Stalker\nManaCost:3\nTypes:Artifact Equipment\n",
			"A:AB$ Mana | Cost$ PayLife<2> | Produced$ Any | Amount$ 2 | RestrictValid$ Spell.Equipment,Activated.Equip | ActivationLimit$ 1"},
	} {
		lc := kwrLower(t, c.head, c.line)
		if len(lc.Refusals) == 0 {
			t.Errorf("%s lowered, want a refusal for its life payment", c.name)
			continue
		}
		for _, r := range lc.Refusals {
			if !strings.Contains(r.Reason, "a life payment") {
				t.Errorf("%s refusal %q does not name the cost part", c.name, r.Reason)
			}
			for _, w := range []string{"withDevoid", "wasCast", "Equip", "unknown property"} {
				if strings.Contains(r.Reason, w) {
					t.Errorf("%s refusal %q names the filter word %q", c.name, r.Reason, w)
				}
			}
		}
	}
}
