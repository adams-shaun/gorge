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
		"SVar:Exile:DB$ ChangeZone | Destination$ Exile | Origin$ Battlefield | Defined$ ReplacedCard\n"
	const dog = "Name:Exile Bf Probe\nManaCost:1 W\nTypes:Creature Squirrel\nPT:2/2\n" +
		"A:AB$ Effect | Cost$ 4 W | ReplacementEffects$ Plus | SpellDescription$ probe\n" +
		"SVar:Plus:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Creature.YouCtrl+inZoneBattlefield | ValidCounterType$ P1P1 | ReplaceWith$ More | Description$ probe\n" +
		"SVar:More:DB$ ReplaceCounter | ValidCounterType$ P1P1 | ChooseCounter$ True | Amount$ Z\n" +
		"SVar:Z:ReplaceCount$CounterNum/Plus.1\n"
	for _, base := range []string{eclipse, dog} {
		if r := exileBfSynth(t, base); len(r) != 0 {
			t.Fatalf("the probe script itself refuses: %q", keclReason(r))
		}
	}
	// Malicious Eclipse's shape neighbours over another creature or permanent
	// spec (no other keys, no Duration$) no longer refuse: since T15b the
	// no-Duration dies-to-exile Effect lowers as the until-end-of-turn emblem
	// (effectReplEmblem); only ValidLKI$ Creature.OppCtrl keeps the
	// specialized TurnRepl record above.
	for _, spec := range []string{"Creature.YouCtrl", "Creature", "Permanent.OppCtrl"} {
		ec := eclipseBfLower(t, strings.Replace(eclipse, "Creature.OppCtrl", spec, 1))
		if got := keclReason(ec.Refusals); got != "" {
			t.Errorf("eclipse %s: refusals %q, want the emblem", spec, got)
		}
		if len(ec.Emblems) != 1 || len(ec.Emblems[0].Repls) != 1 || ec.Emblems[0].Repls[0].Kind != "ExileOnLeave" {
			t.Errorf("eclipse %s: emblems %+v, want one ExileOnLeave emblem", spec, ec.Emblems)
		}
	}
	for _, c := range []struct{ name, base, from, to, want string }{
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

// eclipseBfLower lowers one synthetic eclipse-neighbour script the way
// exileBfSynth does, and returns the whole lowered card.
func eclipseBfLower(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("eclipse_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Exile Bf Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false)
}
