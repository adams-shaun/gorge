package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLowerEffectUncounterScope: Veil of Summer (spells you control can't be
// countered this turn, and you and your permanents have hexproof from blue and
// from black) and Domri, Anarch of Bolas (creature spells you cast can't be
// countered) lower whole: one TurnRepl of the uncounter kind each, Veil's
// beside its hexproof grant. Each out-of-shape variant keeps a named refusal.
func TestLowerEffectUncounterScope(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct {
		card  string
		kinds []int
	}{
		{"Veil of Summer", []int{kop.TurnReplUncounter, kop.TurnReplHexUB}},
		{"Domri, Anarch of Bolas", []int{kop.TurnReplUncounterCreature}},
	} {
		card, ok := reg.Lookup(c.card)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", c.card)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if got := keclReason(lc.Refusals); got != "" {
			t.Fatalf("%s: refusals %q, want none", c.card, got)
		}
		var kinds []int
		for _, ab := range lc.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "TurnRepl" {
					kinds = append(kinds, in.C)
				}
			}
		}
		if len(kinds) != len(c.kinds) {
			t.Fatalf("%s: TurnRepl kinds %v, want %v", c.card, kinds, c.kinds)
		}
		for i := range kinds {
			if kinds[i] != c.kinds[i] {
				t.Errorf("%s: TurnRepl kinds %v, want %v", c.card, kinds, c.kinds)
			}
		}
	}

	const veil = "Name:Uncounter Probe\nManaCost:G\nTypes:Instant\n" +
		"A:SP$ Effect | ReplacementEffects$ AntiMagic | SubAbility$ DBPump | SpellDescription$ probe\n" +
		"SVar:AntiMagic:Event$ Counter | ValidSA$ Spell.YouCtrl | Layer$ CantHappen | Description$ probe\n" +
		"SVar:DBPump:DB$ Pump | KW$ Hexproof:Black & Hexproof:Blue | Defined$ You & Valid Permanent.YouCtrl | SpellDescription$ probe\n"
	if r := exileBfSynth(t, veil); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	for _, c := range []struct{ name, from, to, want string }{
		{"opponent's spells", "Spell.YouCtrl", "Spell.OppCtrl", `Effect replacement "AntiMagic"`},
		{"cause scope", "Layer$ CantHappen", "ValidCause$ Spell.Blue | Layer$ CantHappen", `Effect replacement "AntiMagic"`},
		{"abilities too", "Spell.YouCtrl", "Spell.YouCtrl,Activated.YouCtrl", `Effect replacement "AntiMagic"`},
		{"not stopping", "Layer$ CantHappen", "Layer$ Other", `Effect replacement "AntiMagic"`},
		{"a duration", "ReplacementEffects$ AntiMagic", "ReplacementEffects$ AntiMagic | Duration$ UntilHostLeavesPlay", "Effect replacement Duration$"},
		{"one colour", "KW$ Hexproof:Black & Hexproof:Blue", "KW$ Hexproof:Black & Hexproof:Red", `keyword "Hexproof:Black"`},
		{"all colours of one", "KW$ Hexproof:Black & Hexproof:Blue", "KW$ Hexproof:Black", `keyword "Hexproof:Black"`},
		{"other permanents", "Valid Permanent.YouCtrl", "Valid Creature.YouCtrl", "hexproof from colours over"},
		{"opponent's permanents", "Valid Permanent.YouCtrl", "Valid Permanent.OppCtrl", "hexproof from colours over"},
	} {
		src := strings.Replace(veil, c.from, c.to, 1)
		if src == veil {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}

// TestLowerPumpYouPlayerGrant: the player half of a Pump Defined$ You & Valid
// grant is the kernel's TurnRepl player hexproof (Dawn's Truce); shroud or
// protection, which gorge also gives the player, stay refused by name.
func TestLowerPumpYouPlayerGrant(t *testing.T) {
	const truce = "Name:Player Grant Probe\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ Pump | Defined$ You & Valid Permanent.YouCtrl | KW$ Hexproof | SpellDescription$ probe\n"
	if r := exileBfSynth(t, truce); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	for _, c := range []struct{ kw, want string }{
		{"Shroud", `a player Shroud grant`},
		{"Protection from red", `a player ProtectionFromRed grant`},
	} {
		src := strings.Replace(truce, "KW$ Hexproof", "KW$ "+c.kw, 1)
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.kw, got, c.want)
		}
	}
}
