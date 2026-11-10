package kgen

import (
	"strings"
	"testing"
)

// repeatCardsProbe is Asinine Antics' script with its RepeatEach line and
// body replaced, for the out-of-shape variants.
func repeatCardsProbe(each, body string) string {
	return "Name:Probe Antics\nManaCost:2 U U\nTypes:Sorcery\n" +
		"A:SP$ RepeatEach | " + each + " | RepeatSubAbility$ DBToken\n" +
		"SVar:DBToken:" + body + "\n" +
		"Oracle:Probe.\n"
}

const probeRole = "DB$ Token | TokenScript$ role_cursed | AttachedTo$ Remembered"

// TestLowerRepeateachCards: the WOE Role trio (RepeatEach over cards, a Role
// token attached to each) lowers as OpRepeatEach with a one-instruction
// body, and each out-of-shape variant refuses by name.
func TestLowerRepeateachCards(t *testing.T) {
	for _, tc := range []struct {
		card   string
		filter bool // a RepeatCards$ filter, else the chain's target slot
	}{
		{"Asinine Antics", true},
		{"Twisted Sewer-Witch", true},
		{"Become Brutes", false},
	} {
		c := lowerCorpusCard(t, tc.card)
		for _, rf := range c.Refusals {
			t.Errorf("%s refused: %s", tc.card, rf.Reason)
		}
		saw := false
		for _, a := range c.Abilities {
			for i, in := range a.Prog {
				if in.Op != "RepeatEachCard" {
					continue
				}
				saw = true
				if in.Skip != 1 || i+1 >= len(a.Prog) {
					t.Fatalf("%s: RepeatEachCard Skip %d, want a one-instruction body", tc.card, in.Skip)
				}
				if tc.filter != (in.F != nil) || (tc.filter && in.Obj.Kind != rNone) || (!tc.filter && (in.Obj.Kind != rTarget || in.Obj.N != 0)) {
					t.Errorf("%s: RepeatEachCard Obj %v F %v, want filter=%v", tc.card, in.Obj, in.F, tc.filter)
				}
				tok := a.Prog[i+1]
				if tok.Op != "CreateToken" || tok.C&32 == 0 || tok.Obj2.Kind != rEach {
					t.Errorf("%s: body %v, want a Role token attached to the repeated card (RefEach)", tc.card, tok)
				}
			}
		}
		if !saw {
			t.Errorf("%s lowered no RepeatEachCard", tc.card)
		}
	}

	if got := lowerScript(t, "Probe Antics", repeatCardsProbe("RepeatCards$ Creature.OppCtrl | Zone$ Battlefield | ChangeZoneTable$ True", probeRole)); len(got) != 0 {
		t.Fatalf("the in-shape probe refused: %v", got)
	}
	for _, tc := range []struct{ name, each, body, want string }{
		{"other zone", "RepeatCards$ Creature.OppCtrl | Zone$ Graveyard | ChangeZoneTable$ True", probeRole, "RepeatEach cards Zone$"},
		{"no table", "RepeatCards$ Creature.OppCtrl | Zone$ Battlefield", probeRole, "without ChangeZoneTable$ True"},
		{"both selectors", "RepeatCards$ Creature.OppCtrl | DefinedCards$ Targeted | ChangeZoneTable$ True", probeRole, "RepeatCards$ beside DefinedCards$"},
		{"remembered cards", "DefinedCards$ Remembered | ChangeZoneTable$ True", probeRole, "RepeatEach DefinedCards$"},
		{"targeted, no target", "DefinedCards$ Targeted | ChangeZoneTable$ True", probeRole, "chain target slots"},
		{"filter reads the remembered set", "RepeatCards$ Card.IsRemembered | Zone$ Battlefield | ChangeZoneTable$ True", probeRole, "reads Remember"},
		{"another body", "RepeatCards$ Creature.OppCtrl | ChangeZoneTable$ True", "DB$ GainLife | LifeAmount$ 1", "RepeatEach cards body GainLife"},
		{"order ask", "RepeatCards$ Creature.OppCtrl | ChangeZoneTable$ True | ChooseOrder$ True", probeRole, "ChooseOrder"},
		{"role outside a loop", "RepeatCards$ Creature.OppCtrl | ChangeZoneTable$ True", "DB$ Token | TokenScript$ role_cursed | AttachedTo$ Targeted", "Token AttachedTo"},
	} {
		got := strings.Join(lowerScript(t, "Probe Antics", repeatCardsProbe(tc.each, tc.body)), "; ")
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want one naming %q", tc.name, got, tc.want)
		}
	}
	// the Role destination outside a card loop still refuses
	plain := "Name:Probe Role\nManaCost:1 W\nTypes:Sorcery\nA:SP$ Token | TokenScript$ role_cursed | AttachedTo$ Remembered\nOracle:Probe.\n"
	if got := strings.Join(lowerScript(t, "Probe Role", plain), "; "); !strings.Contains(got, "Token AttachedTo") {
		t.Errorf("a Role attached to Remembered outside a loop: refusals %q, want Token AttachedTo", got)
	}
}
