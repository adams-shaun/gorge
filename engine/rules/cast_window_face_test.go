package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	cwFaceSpellBG = "Name:Face BG\nManaCost:BG\nTypes:Instant\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw one.\nOracle:x\n"
	cwSwamp = "Name:Face Swamp\nTypes:Basic Land Swamp\n" +
		"A:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Add {B}.\nOracle:x\n"
	cwFaceSpellTwoBR = "Name:Face Two BR\nManaCost:2 BR\nTypes:Instant\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw one.\nOracle:x\n"
	cwChosenLand = "Name:Chosen Land\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Chosen | SpellDescription$ Add one mana of the chosen color.\nOracle:x\n"
	cwFilterLand = "Name:Filter Land\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\n" +
		"A:AB$ Mana | Cost$ 1 T | Produced$ Any | SpellDescription$ Add one mana of any color.\nOracle:x\n"
	cwMountain = "Name:Face Mountain\nTypes:Basic Land Mountain\n" +
		"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add {R}.\nOracle:x\n"
)

// cwFaceAsk begins the cast of spell and returns the option kinds of its pip
// face ask.
func cwFaceAsk(t *testing.T, e *Engine, spell state.ObjID) map[string]int {
	t.Helper()
	e.pending = nil
	e.beginCast(0, decision.Option{Kind: "cast", Obj: spell})
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("after beginning the cast = %+v, want the pip face ask", d)
	}
	faces := map[string]int{}
	for _, o := range d.Options {
		faces[o.Kind] = o.Index
	}
	return faces
}

// A land that makes its recorded colour pays that half of a hybrid pip
// (CR 106.1, 107.4e), even though the recorded colour is no choice at activation.
func TestCastWindowChosenColourPaysHybridFace(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 884, cwFaceSpellBG, cwChosenLand, cwSwamp)
	land := putCreature(t, e, 0, cwChosenLand)
	putCreature(t, e, 0, cwSwamp)
	recordChosenGreen(t, e, land)
	e.pending = nil
	e.priorityRound()

	faces := cwFaceAsk(t, e, spell)
	if _, ok := faces["pay_B"]; !ok {
		t.Fatalf("the {B} face is withheld from a Swamp: %v", faces)
	}
	green, ok := faces["pay_G"]
	if !ok {
		t.Fatalf("the {G} face is withheld from a land that makes {G}: %v", faces)
	}
	submitChoices(t, e, green)
	if !cwActivateInWindow(t, e, land) {
		t.Fatalf("the mana window did not offer the land: %+v", e.Pending())
	}
	if !nlPutOnStack(e, spell) {
		t.Fatal("the spell never reached the stack")
	}
}

// A filter's activation cost is paid before its mana is added (CR 602.2b, 605.3b),
// so two Mountains and a filter land cannot pay the {B} face of {2}{B/R}: its fee
// and the {2} want three mana besides the {B}. A third Mountain funds it.
func TestCastWindowFaceChargesActivationFee(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		mountains int
		wantBlack bool
	}{
		{"two mountains cannot fund the fee", 2, false},
		{"three mountains can", 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extras := []string{cwFilterLand}
			for range tc.mountains {
				extras = append(extras, cwMountain)
			}
			e, _, spell := newFixtureDeck(t, 885, cwFaceSpellTwoBR, extras...)
			putCreature(t, e, 0, cwFilterLand)
			for range tc.mountains {
				putCreature(t, e, 0, cwMountain)
			}
			e.pending = nil
			e.priorityRound()

			faces := cwFaceAsk(t, e, spell)
			if _, ok := faces["pay_R"]; !ok {
				t.Fatalf("the {R} face is withheld: %v", faces)
			}
			if _, got := faces["pay_B"]; got != tc.wantBlack {
				t.Fatalf("the {B} face offered = %v, want %v: %v", got, tc.wantBlack, faces)
			}
		})
	}
}

// An attack tax is a cost like any other (CR 508.1h), so a source of a recorded
// colour is among the attack window's sources.
func TestAttackWindowCountsChosenColour(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 886, cwFaceSpellBG, cwChosenLand)
	land := putCreature(t, e, 0, cwChosenLand)
	recordChosenGreen(t, e, land)

	sources := e.attackManaSources(0)
	if len(sources) != 1 || sources[0].id != land {
		t.Fatalf("attack window sources = %+v, want only the chosen-colour land %d", sources, land)
	}
	if got := sources[0].ma.Params["Produced"]; got != "G" {
		t.Fatalf("pinned production = %q, want the recorded G", got)
	}
}
