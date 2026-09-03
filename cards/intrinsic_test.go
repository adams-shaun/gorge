package cards

import "testing"

// Basic lands carry no mana ability in the corpus: Forge grants abilities from
// land subtypes. Without this layer every basic land is a blank card, which is
// the single most load-bearing intrinsic in the whole engine.
func TestIntrinsicBasicLandMana(t *testing.T) {
	src := "Name:Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:({T}: Add {R}.)\n"
	c, _ := ParseBytes("m/mountain.txt", []byte(src))
	f := c.Faces[0]
	if len(f.ManaAbilities()) != 0 {
		t.Fatal("script should not define a mana ability")
	}
	f.ApplyIntrinsics()
	ma := f.ManaAbilities()
	if len(ma) != 1 {
		t.Fatalf("mana abilities = %d, want 1", len(ma))
	}
	if ma[0].Params["Produced"] != "R" {
		t.Errorf("Produced = %q, want R", ma[0].Params["Produced"])
	}
	if ma[0].Params["Cost"] != "T" {
		t.Errorf("Cost = %q, want T", ma[0].Params["Cost"])
	}
}

func TestIntrinsicIsIdempotent(t *testing.T) {
	src := "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
	c, _ := ParseBytes("f/forest.txt", []byte(src))
	f := c.Faces[0]
	f.ApplyIntrinsics()
	f.ApplyIntrinsics()
	if got := len(f.ManaAbilities()); got != 1 {
		t.Fatalf("mana abilities = %d after two passes, want 1", got)
	}
}

func TestIntrinsicDualLandGetsBothColors(t *testing.T) {
	src := "Name:Tundra\nTypes:Land Plains Island\nOracle:x\n"
	c, _ := ParseBytes("t/tundra.txt", []byte(src))
	f := c.Faces[0]
	f.ApplyIntrinsics()
	got := map[string]bool{}
	for _, a := range f.ManaAbilities() {
		got[a.Params["Produced"]] = true
	}
	if !got["W"] || !got["U"] || len(got) != 2 {
		t.Fatalf("produced = %v, want {W,U}", got)
	}
}

func TestFaceCharacteristics(t *testing.T) {
	src := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n"
	c, _ := ParseBytes("b/bear.txt", []byte(src))
	f := c.Faces[0]
	if !f.IsCreature() || f.IsLand() || !f.IsPermanent() {
		t.Error("type predicates wrong")
	}
	if !f.HasKeyword("Trample") || f.HasKeyword("Flying") {
		t.Error("keyword predicates wrong")
	}
	if f.Power() != 2 || f.Toughness() != 2 {
		t.Errorf("P/T = %d/%d", f.Power(), f.Toughness())
	}
}

func TestKeywordHeadStripsParameters(t *testing.T) {
	for src, want := range map[string]string{
		"Flying":               "Flying",
		"Equip:2":              "Equip",
		"Protection from blue": "Protection from blue",
		"etbCounter:P1P1:1":    "etbCounter",
	} {
		if got := KeywordHead(src); got != want {
			t.Errorf("KeywordHead(%q) = %q, want %q", src, got, want)
		}
	}
}
