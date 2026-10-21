package cards

import "testing"

// TestUniverseFaceByNameFindsTheNamedFace pins the lookup a chosen-name copy
// (CopyFromChosenName$) resolves through: a back-face name names that face of
// its card (CR 712.19), a front name face 0, and an unknown name nothing.
func TestUniverseFaceByNameFindsTheNamedFace(t *testing.T) {
	parse := func(src string) *Card {
		c, d := ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		return c
	}
	u := UniverseOf([]*Card{
		parse("Name:Bear Test\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		parse("Name:Delver Test\nManaCost:U\nTypes:Creature Human Wizard\nPT:1/1\nAlternateMode:DoubleFaced\nOracle:x\n" +
			"ALTERNATE\nName:Aberration Test\nTypes:Creature Human Insect\nPT:3/2\nOracle:x\n"),
	})
	if i, f, ok := u.FaceByName("Aberration Test"); !ok || i != 1 || f != 1 {
		t.Fatalf("FaceByName(back face) = %d, %d, %v; want 1, 1, true", i, f, ok)
	}
	if i, f, ok := u.FaceByName("Delver Test"); !ok || i != 1 || f != 0 {
		t.Fatalf("FaceByName(front face) = %d, %d, %v; want 1, 0, true", i, f, ok)
	}
	if _, _, ok := u.FaceByName("Nope"); ok {
		t.Fatal("FaceByName found an unknown name")
	}
}
