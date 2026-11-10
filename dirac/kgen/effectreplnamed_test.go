package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLowerEffectReplNamed: Malicious Eclipse's Effect (a Moved replacement
// over every creature an opponent controls this turn) and Prairie Dog's
// activated Effect (an AddCounter replacement over +1/+1 counters on creatures
// its controller controls) each lower whole to one OpExt TurnRepl of their kind.
// Each out-of-shape variant of both still refuses by name, and Brad Boimler's
// shape still lowers to CounterPlusEOT.
func TestLowerEffectReplNamed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct {
		card string
		op   string
		kind int
	}{
		{"Malicious Eclipse", "TurnRepl", kop.TurnReplExileOpp},
		{"Prairie Dog", "TurnRepl", kop.TurnReplPlusCreatureCtr},
		{"Brad Boimler, Eager Ensign", "CounterPlusEOT", 0},
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
		n := 0
		for _, ab := range lc.Abilities {
			for _, in := range ab.Prog {
				if in.Op == c.op {
					n++
					if in.C != c.kind {
						t.Errorf("%s: %s C %d, want %d", c.card, c.op, in.C, c.kind)
					}
				}
			}
		}
		if n != 1 {
			t.Fatalf("%s: %d %s ops, want 1", c.card, n, c.op)
		}
	}

	const eclipse = "Name:Exile Bf Probe\nManaCost:1 B B\nTypes:Sorcery\n" +
		"A:SP$ Effect | ReplacementEffects$ DeathRep | SpellDescription$ probe\n" +
		"SVar:DeathRep:Event$ Moved | ValidLKI$ Creature.OppCtrl | Origin$ Battlefield | Destination$ Graveyard | ReplaceWith$ Exile | Description$ probe\n" +
		"SVar:Exile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ ReplacedCard\n"
	const dog = "Name:Exile Bf Probe\nManaCost:1 W\nTypes:Creature Squirrel\nPT:2/2\n" +
		"A:AB$ Effect | Cost$ 4 W | ReplacementEffects$ Plus | SpellDescription$ probe\n" +
		"SVar:Plus:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Creature.YouCtrl+inZoneBattlefield | ValidCounterType$ P1P1 | ReplaceWith$ More | Description$ probe\n" +
		"SVar:More:DB$ ReplaceCounter | ValidCounterType$ P1P1 | ChooseCounter$ True | Amount$ Y\n" +
		"SVar:Y:ReplaceCount$CounterNum/Plus.1\n"
	for _, base := range []string{eclipse, dog} {
		if r := exileBfSynth(t, base); len(r) != 0 {
			t.Fatalf("the probe script itself refuses: %q", keclReason(r))
		}
	}
	for _, c := range []struct{ name, base, from, to, want string }{
		{"eclipse mine", eclipse, "Creature.OppCtrl", "Creature.YouCtrl", `Effect replacement "DeathRep"`},
		{"eclipse any creature", eclipse, "Creature.OppCtrl", "Creature", `Effect replacement "DeathRep"`},
		{"eclipse permanent", eclipse, "Creature.OppCtrl", "Permanent.OppCtrl", `Effect replacement "DeathRep"`},
		{"eclipse to hand", eclipse, "Destination$ Graveyard", "Destination$ Hand", `Effect replacement "DeathRep"`},
		{"eclipse from anywhere", eclipse, "Origin$ Battlefield |", "Origin$ Any |", `Effect replacement "DeathRep"`},
		{"eclipse to the library", eclipse, "Destination$ Exile |", "Destination$ Library |", `Effect replacement "DeathRep"`},
		{"eclipse body extra key", eclipse, "Destination$ Exile |", "Destination$ Exile | Foo$ Bar |", `Effect replacement "DeathRep"`},
		{"eclipse active zone", eclipse, "Origin$ Battlefield | Destination$ Graveyard", "Origin$ Battlefield | Destination$ Graveyard | ActiveZones$ Command", `Effect replacement "DeathRep"`},
		{"eclipse duration", eclipse, "ReplacementEffects$ DeathRep", "ReplacementEffects$ DeathRep | Duration$ UntilHostLeavesPlay", "Effect replacement Duration$"},
		{"dog permanent", dog, "Creature.YouCtrl+inZoneBattlefield", "Permanent.YouCtrl+inZoneBattlefield", `Effect replacement "Plus"`},
		{"dog opponent's", dog, "Creature.YouCtrl+inZoneBattlefield", "Creature.OppCtrl+inZoneBattlefield", `Effect replacement "Plus"`},
		{"dog other kind", dog, "ValidCounterType$ P1P1 | ReplaceWith", "ValidCounterType$ M1M1 | ReplaceWith", `Effect replacement "Plus"`},
		{"dog no kind", dog, "ValidCounterType$ P1P1 | ReplaceWith", "ReplaceWith", `Effect replacement "Plus"`},
		{"dog body kind differs", dog, "DB$ ReplaceCounter | ValidCounterType$ P1P1", "DB$ ReplaceCounter | ValidCounterType$ M1M1", `Effect replacement "Plus"`},
		{"dog body no kind", dog, "DB$ ReplaceCounter | ValidCounterType$ P1P1 |", "DB$ ReplaceCounter |", `Effect replacement "Plus"`},
		{"dog plus two", dog, "CounterNum/Plus.1", "CounterNum/Plus.2", `Effect replacement "Plus"`},
		{"dog zone", dog, "ActiveZones$ Battlefield", "ActiveZones$ Graveyard", `Effect replacement "Plus"`},
		{"dog duration", dog, "ReplacementEffects$ Plus", "ReplacementEffects$ Plus | Duration$ UntilHostLeavesPlay", "Effect replacement Duration$"},
	} {
		src := strings.Replace(c.base, c.from, c.to, 1)
		if src == c.base {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}
