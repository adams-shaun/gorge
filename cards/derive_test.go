package cards

import (
	"path/filepath"
	"testing"
)

// TestDerivedConstructionRoutesAgree is the enforcement point for the derived-|-
// field design. Face's parsed fields (power/toughness, the characteristic-|-
// defining flag, converted mana cost) are unexported, so gob never encodes
// them and a stale cache silently decodes them as zero. LoadRegistry repairs
// that by re-deriving from the printed text on decode. This test proves the
// two construction routes — direct (ParseBytes) and the gob round-trip
// (Save -> LoadRegistry) — end with byte-identical derived values, and that
// they are the values the printed text actually implies, not all zeros.
//
// The fixture deliberately mixes plain P/T (2/2), a characteristic-defining
// P/T (*), a hybrid-ish "1+*" P/T, a space-form and a brace-form cost, and a
// "no cost" face, so every derive branch is exercised on both routes.
func TestDerivedConstructionRoutesAgree(t *testing.T) {
	direct, _ := ParseBytes("s.txt", []byte(
		"Name:Sample\n"+
			"ManaCost:1 W W\n"+
			"Types:Creature\n"+
			"PT:2/2\n"+
			"\n"+
			"ALTERNATE\n"+
			"Name:Other\n"+
			"ManaCost:{2}{U}{U}\n"+
			"Types:Creature\n"+
			"PT:*/*\n"))

	// The non-trivial expectations are asserted explicitly so a regression
	// that derives the *same wrong value on both routes* still fails: a face
	// whose every derived field is zero is not a fix.
	if f := direct.Faces[0]; f.Power() != 2 || f.Toughness() != 2 || f.Cmc() != 3 || f.CharacteristicDefining() {
		t.Fatalf("front face derived wrong: power=%d tough=%d cmc=%d cd=%v", f.Power(), f.Toughness(), f.Cmc(), f.CharacteristicDefining())
	}
	if f := direct.Faces[1]; !f.CharacteristicDefining() || f.Power() != 0 || f.Toughness() != 0 || f.Cmc() != 4 {
		t.Fatalf("alternate face derived wrong: power=%d tough=%d cmc=%d cd=%v", f.Power(), f.Toughness(), f.Cmc(), f.CharacteristicDefining())
	}

	// Round-trip the same card plus a token through the gob cache path.
	r := NewRegistry()
	r.Add(direct)
	r.Tokens["tk"], _ = ParseBytes("t.txt", []byte("Name:Token\nManaCost:no cost\nTypes:Creature\nPT:1+*/0\n"))
	dir := t.TempDir()
	p := filepath.Join(dir, "ir.gob.gz")
	if err := r.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRegistry(p)
	if err != nil {
		t.Fatal(err)
	}

	compare := func(want, got []*Face) {
		if len(want) != len(got) {
			t.Fatalf("face count %d != %d", len(want), len(got))
		}
		for i, w := range want {
			if w.Power() != got[i].Power() || w.Toughness() != got[i].Toughness() {
				t.Errorf("face %d: power/toughness %d/%d (direct) != %d/%d (gob)", i, w.Power(), w.Toughness(), got[i].Power(), got[i].Toughness())
			}
			if w.CharacteristicDefining() != got[i].CharacteristicDefining() {
				t.Errorf("face %d: characteristicDefining %v != %v", i, w.CharacteristicDefining(), got[i].CharacteristicDefining())
			}
			if w.Cmc() != got[i].Cmc() {
				t.Errorf("face %d: cmc %d != %d", i, w.Cmc(), got[i].Cmc())
			}
		}
	}
	compare(direct.Faces, loaded.Cards[0].Faces)
	compare(r.Tokens["tk"].Faces, loaded.Tokens["tk"].Faces)
}
