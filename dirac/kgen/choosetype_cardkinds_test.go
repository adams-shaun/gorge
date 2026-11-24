package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// cardKindMask is the kop mask of gorge's offered card-type labels.
func cardKindMask(t *testing.T, valid, invalid string) kop.TypeMask {
	t.Helper()
	var m kop.TypeMask
	for _, l := range effects.TypeChoiceLabels("Card", valid, invalid) {
		b, ok := cardTypeChoice[l]
		if !ok {
			t.Fatalf("label %q is no card type", l)
		}
		m |= b
	}
	return m
}

// TestChooseTypeCardKinds pins the as-enters card-type ask: ChooseType | Type$
// Card lowers to the enters program's OpChooseType with C = the offered
// kop.TypeMask (gorge effChooseType over TypeChoiceLabels("Card", ValidTypes$,
// InvalidTypes$): ValidTypes$ narrows, InvalidTypes$ removes). The creature
// spelling keeps C = 0, and the spellings the ask cannot answer refuse.
func TestChooseTypeCardKinds(t *testing.T) {
	const head = "Name:Kinds Probe\nManaCost:3\nTypes:Artifact\nK:ETBReplacement:Other:ChooseCT\n"
	const tail = "S:Mode$ ReduceCost | ValidCard$ Card.ChosenType | Type$ Spell | Activator$ You | Amount$ 1 | Description$ Spells you cast of the chosen type cost {1} less to cast.\n"
	probe := func(db string) string {
		return head + "SVar:ChooseCT:DB$ ChooseType | " + db + " | SpellDescription$ As CARDNAME enters, choose a card type.\n" + tail
	}
	for _, c := range []struct{ name, db, valid, invalid string }{
		{"Archon", "Type$ Card | ValidTypes$ Artifact,Enchantment,Instant,Sorcery,Planeswalker", "Artifact,Enchantment,Instant,Sorcery,Planeswalker", ""},
		{"Cloud Key", "Defined$ You | Type$ Card | ValidTypes$ Artifact,Creature,Enchantment,Instant,Sorcery", "Artifact,Creature,Enchantment,Instant,Sorcery", ""},
		{"Stenn", "Defined$ You | Type$ Card | InvalidTypes$ Creature,Land", "", "Creature,Land"},
		{"Nonland", "Type$ Card | ValidTypes$ Nonland", "Nonland", ""},
	} {
		lc := lowerSynth(t, probe(c.db))
		if r := lc.Refusals; len(r) != 0 {
			t.Errorf("%s: the as-enters card-type ask refuses: %q", c.name, keclReason(r))
			continue
		}
		want := cardKindMask(t, c.valid, c.invalid)
		ins := chooseTypeInstrs(lc)
		if len(ins) != 1 || ins[0].C != int(want) || want == 0 {
			t.Errorf("%s: lowers to %+v, want one ChooseType with C = %#x", c.name, ins, want)
		}
	}
	if m := cardKindMask(t, "", "Creature,Land"); m&(kop.TCreature|kop.TLand) != 0 {
		t.Fatalf("Stenn's mask %#x keeps Creature or Land: the control cannot tell InvalidTypes$ was read", m)
	}

	// The control: the creature-type spelling keeps its unmarked ask.
	ins := chooseTypeInstrs(lowerSynth(t, probe("Type$ Creature")))
	if len(ins) != 1 || ins[0].C != 0 || ins[0].B.N != 0 {
		t.Errorf("as-enters creature-type choice lowers to %+v, want one ChooseType with C = 0", ins)
	}

	// What the ask cannot answer keeps refusing.
	for name, db := range map[string]string{
		"AtRandom":     "Type$ Card | ValidTypes$ Artifact,Instant | AtRandom$ True",
		"Secretly":     "Type$ Card | ValidTypes$ Artifact,Instant | Secretly$ True",
		"Defined":      "Type$ Card | ValidTypes$ Artifact,Instant | Defined$ Player.Opponent",
		"one type":     "Type$ Card | ValidTypes$ Artifact",
		"no type left": "Type$ Card | ValidTypes$ Artifact | InvalidTypes$ Artifact",
	} {
		if len(lowerSynth(t, probe(db)).Refusals) == 0 {
			t.Errorf("as-enters card-type ask with %s lowers, want a refusal", name)
		}
	}

	// The corpus cards: the three lower, and the other two keep only their own
	// refusals.
	lw := lowerNamedPool(t, []string{"Archon of Valor's Reach", "Cloud Key", "Stenn, Paranoid Partisan", "Serra's Emissary", "Umori, the Collector"})
	for _, lc := range lw.cards {
		got := keclReason(lc.Refusals)
		switch lc.Name {
		case "Archon of Valor's Reach", "Cloud Key", "Stenn, Paranoid Partisan":
			if got != "" {
				t.Errorf("%s still refuses: %s", lc.Name, got)
			}
		case "Serra's Emissary", "Umori, the Collector":
			if got == "" || strings.Contains(got, "ChooseType shape") {
				t.Errorf("%s refusals %q: want its other refusal without ChooseType shape", lc.Name, got)
			}
		}
	}
}
