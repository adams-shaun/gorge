package cards

import (
	"path/filepath"
	"testing"
)

// mpOf parses a one-line script and returns the face's mana production, the
// way the view and the bot policy both read it (off the compiled face). It
// applies the intrinsic layer first, exactly as corpus load does, so a basic
// land's subtype-granted ability is present -- the projection itself does no
// subtype special-casing.
func mpOf(t *testing.T, src string) ManaProduction {
	t.Helper()
	c, err := ParseBytes("x/x.txt", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	f := c.Faces[0]
	f.ApplyIntrinsics()
	return f.ManaProduction()
}

// TestManaProductionBasicLand is the easy 60%: a basic land carries no
// mana ability in the script; the intrinsic layer grants it one from its
// subtype, and the projection picks it up the same way it would a scripted
// ability -- no subtype special-casing in the projection itself.
func TestManaProductionBasicLand(t *testing.T) {
	mp := mpOf(t, "Name:Plains\nManaCost:no cost\nTypes:Basic Land Plains\nOracle:x\n")
	if mp.IsZero() {
		t.Fatal("a basic Plains must project a production, not zero")
	}
	if !mp.ProducesColour(0) { // W
		t.Errorf("Plains should produce white; got %v", mp.Colour)
	}
	if mp.Colour[0] != 1 || mp.Any {
		t.Errorf("Plains = %v, want exactly one white, not flexible", mp)
	}
}

// TestManaProductionDual sums both intrinsic halves: a Tundra (Plains + Island)
// is granted a {W} ability and a {U} ability, and tapping it runs both, so the
// projection is {W}{U} -- the exact mana the pool receives.
func TestManaProductionDual(t *testing.T) {
	mp := mpOf(t, "Name:Tundra\nManaCost:no cost\nTypes:Land Plains Island\nOracle:x\n")
	if !mp.ProducesColour(0) || !mp.ProducesColour(1) {
		t.Fatalf("Tundra should produce white and blue; got %v", mp.Colour)
	}
	if mp.DistinctColours() != 2 || mp.Any {
		t.Errorf("Tundra = %v, want two distinct colours, not flexible", mp)
	}
}

// TestManaProductionAmount counts an Amount$ multiplier: a colourless rock
// that taps for {C}{C} projects two colourless, and a {G}{G}{G} producer
// three green.
func TestManaProductionAmount(t *testing.T) {
	c := mpOf(t, "Name:Rock\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | Oracle:x\n")
	if c.Colour[5] != 2 || c.Any {
		t.Errorf("colourless rock = %v, want two colourless, not flexible", c)
	}
	g := mpOf(t, "Name:Elf\nTypes:Creature\nA:AB$ Mana | Cost$ T | Produced$ G | Amount$ 3 | Oracle:x\n")
	if g.Colour[4] != 3 {
		t.Errorf("green producer = %v, want three green", g.Colour)
	}
}

// TestManaProductionAnyIsHonest is the conservative projection the brief
// demands. "Produced$ Any" means the card can add one mana of any colour,
// but this engine's executor (effects/misc.go's effMana) resolves it to
// colourless -- the only choice it can actually make. The projection must
// NOT assert a coloured pip the pool will never receive: it reports the
// colourless amount the pool reliably gets AND flags Any so a policy knows
// the coloured side is a stand-in, not a real any-colour source.
func TestManaProductionAnyIsHonest(t *testing.T) {
	mp := mpOf(t, "Name:Cavern\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Any | Oracle:x\n")
	if mp.Colour[5] != 1 {
		t.Errorf("Any production = %v, want one colourless (what the engine emits)", mp.Colour)
	}
	if !mp.Any {
		t.Error("Any production must be flagged as flexible/conditional")
	}
	for i := 0; i < 5; i++ {
		if mp.ProducesColour(i) {
			t.Errorf("Any production must not assert a coloured pip %d the engine will not produce", i)
		}
	}
}

// TestManaProductionComboMirrorsExecutor pins the executor's unusual but
// real rune-by-rune handling: Combo R G emits its two listed colours plus
// five colourless runes from "Combo". It remains Any because its script-level
// choice is not modelled.
func TestManaProductionComboMirrorsExecutor(t *testing.T) {
	mp := mpOf(t, "Name:Shock\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Combo R G | Oracle:x\n")
	if !mp.Any || mp.Colour[3] != 1 || mp.Colour[4] != 1 || mp.Colour[5] != 5 {
		t.Errorf("Combo R G = %v, want Any with 5C, R, and G", mp)
	}
}

// TestManaProductionChosenMirrorsExecutor ensures an unrecognised word is
// not silently dropped: every rune in Chosen maps to colourless in effMana.
func TestManaProductionChosenMirrorsExecutor(t *testing.T) {
	mp := mpOf(t, "Name:Chosen\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Chosen | Oracle:x\n")
	if !mp.Any || mp.Colour[5] != 6 {
		t.Errorf("Chosen = %v, want Any with 6 colourless", mp)
	}
}

// Both cache collections must reconstruct the unexported derived field.
func TestManaProductionDerivedRoundTrip(t *testing.T) {
	r := NewRegistry()
	for _, src := range []string{
		"Name:Dual\nTypes:Land Plains Island\n",
		"Name:Rock\nTypes:Artifact\nA:AB$ Mana | Produced$ Combo Any | Amount$ 2\n",
		"Name:Empty\nTypes:Creature\n",
	} {
		c, _ := ParseBytes("fixture", []byte(src))
		c.Link()
		f := c.Faces[0]
		f.ApplyIntrinsics()
		want := f.ManaProduction()
		f.ApplyIntrinsics()
		if got := f.ManaProduction(); got != want {
			t.Fatalf("repeated intrinsics: %v != %v", got, want)
		}
		r.Add(c)
		r.Tokens[f.Name] = c
	}
	path := filepath.Join(t.TempDir(), "ir.gob.gz")
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range r.Cards {
		f := c.Faces[0]
		var want ManaProduction
		for _, a := range f.ManaAbilities() {
			want.add(a)
		}
		for _, got := range []ManaProduction{f.ManaProduction(), back.Cards[i].Faces[0].ManaProduction(), back.Tokens[f.Name].Faces[0].ManaProduction()} {
			if got != want {
				t.Errorf("%s: got %v, want %v", f.Name, got, want)
			}
		}
		if n := testing.AllocsPerRun(100, func() {
			if f.ManaProduction() != want {
				panic("production changed")
			}
		}); n != 0 {
			t.Errorf("%s: getter allocates %g times", f.Name, n)
		}
	}
}

func TestManaProductionAnyIsMostFlexible(t *testing.T) {
	if got := (ManaProduction{Any: true}).DistinctColours(); got != 5 {
		t.Errorf("Any DistinctColours = %d, want 5", got)
	}
}
