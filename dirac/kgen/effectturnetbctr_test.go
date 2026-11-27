package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLowerEffectTurnEtbCounter: Zameck Guildmage's activated Effect (a Moved
// Updated replacement over every creature its controller controls) lowers whole
// to one OpExt TurnRepl TurnReplEtbCreatureCtr. Combine Guildmage's
// Creature.YouCtrl+Other line refuses by name, and every out-of-shape body,
// zone and replacement variant still refuses.
func TestLowerEffectTurnEtbCounter(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Zameck Guildmage")
	if !ok || card == nil {
		t.Fatal("Zameck Guildmage not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("Zameck Guildmage: refusals %q, want none", got)
	}
	n := 0
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "TurnRepl" {
				n++
				if in.C != kop.TurnReplEtbCreatureCtr {
					t.Errorf("Zameck Guildmage: TurnRepl C %d, want %d", in.C, kop.TurnReplEtbCreatureCtr)
				}
			}
		}
	}
	if n != 1 {
		t.Fatalf("Zameck Guildmage: %d TurnRepl ops, want 1", n)
	}

	const turnEtb = "Name:Exile Bf Probe\nManaCost:G U\nTypes:Creature Elf Wizard\nPT:2/2\n" +
		"A:AB$ Effect | Cost$ G U | ReplacementEffects$ ExtraETBCounter | SpellDescription$ probe\n" +
		"SVar:ExtraETBCounter:Event$ Moved | ReplacementResult$ Updated | ActiveZones$ Command | Destination$ Battlefield | ValidCard$ Creature.YouCtrl | ReplaceWith$ AddExtraCounter | Description$ probe\n" +
		"SVar:AddExtraCounter:DB$ PutCounter | ETB$ True | CounterType$ P1P1 | Defined$ ReplacedCard | CounterNum$ 1\n"
	if r := exileBfSynth(t, turnEtb); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	lc = turnEtbBfLower(t, turnEtb)
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("the probe script itself refuses: %q", got)
	}
	// The probe lowers to the same one-op shape the carrier does.
	n = 0
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "TurnRepl" && in.C == kop.TurnReplEtbCreatureCtr {
				n++
			}
		}
	}
	if n != 1 {
		t.Fatalf("the probe script: %d TurnRepl TurnReplEtbCreatureCtr ops, want 1", n)
	}

	for _, c := range []struct{ name, from, to, want string }{
		{"combine +Other", "Creature.YouCtrl | ReplaceWith", "Creature.YouCtrl+Other | ReplaceWith", `Effect ETB-counter turn replacement ValidCard$ "Creature.YouCtrl+Other"`},
		{"other plane", "ValidCard$ Creature.YouCtrl", "ValidCard$ Permanent.YouCtrl", `Effect replacement "ExtraETBCounter"`},
		{"other event", "Event$ Moved", "Event$ AddCounter", `Effect replacement "ExtraETBCounter"`},
		{"destination graveyard", "Destination$ Battlefield", "Destination$ Graveyard", `Effect replacement "ExtraETBCounter"`},
		{"replacement not Updated", "ReplacementResult$ Updated |", "ReplacementResult$ Replaced |", `Effect replacement "ExtraETBCounter"`},
		{"zone battlefield", "ActiveZones$ Command", "ActiveZones$ Battlefield", `Effect replacement "ExtraETBCounter"`},
		{"body other kind", "CounterType$ P1P1", "CounterType$ M1M1", "Effect ETB-counter turn replacement body"},
		{"body no ETB", "ETB$ True | ", "", "Effect ETB-counter turn replacement body"},
		{"body two counters", "CounterNum$ 1", "CounterNum$ 2", "Effect ETB-counter turn replacement body"},
		{"replacement extra key", "ReplaceWith$ AddExtraCounter", "ReplaceWith$ AddExtraCounter | Optional$ True", "Effect ETB-counter turn replacement unread"},
		{"effect extra key", "SpellDescription$ probe", "SpellDescription$ probe | Foo$ Bar", "Effect ETB-counter turn replacement unread"},
	} {
		src := strings.Replace(turnEtb, c.from, c.to, 1)
		if src == turnEtb {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}

// turnEtbBfLower lowers one synthetic turn-long ETB-counter script.
func turnEtbBfLower(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("turnetb_probe.txt", []byte(src))
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
