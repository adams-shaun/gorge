package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestGorgeKwMapping(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		kw   string
		want kop.KwMask
	}{
		{"Flying", kop.KwFlying}, {"First Strike", kop.KwFirstStrike}, {"Ward:2", kop.KwWard},
		{"Kicker:1 G", kop.KwKicker}, {"Flashback:3 R", kop.KwFlashback}, {"Hexproof:Instant", kop.KwHexproofFromInstants},
		{"Protection from red", kop.KwProtRed}, {"Equip:1", 0}, {"Enchant:Creature", 0}, {"Cycling:2", 0}, {"TypeCycling:Basic:1 R", 0},
		{"Job select", 0}, {"Battle cry", 0}, {"Gift", 0}, {"Annihilator:2", 0},
		// the expandedKeywords heads on an MBC battlefield object: Transmute
		// (Tolaria West, Drift of Phantasms), Madness (Arrogant Wurm, Reckless
		// Wurm) and Fabricate (Marionette Apprentice) are printed keywords in
		// gorge's derived list with no kernel bit
		{"Transmute:1 U U", 0}, {"Madness:2 G", 0}, {"Madness:2 R", 0}, {"Fabricate:1", 0},
	} {
		got, ok := gorgeKw(c.kw)
		if !ok || got != c.want {
			t.Errorf("gorgeKw(%q) = %v, %v; want %v, true", c.kw, got, ok, c.want)
		}
	}
	// a gorge keyword with no kernel bit is a projection error, never a
	// silent drop
	for _, kw := range []string{"Rampage:2", "Flanking"} {
		if _, ok := gorgeKw(kw); ok {
			t.Errorf("gorgeKw(%q) mapped; want an error", kw)
		}
	}
}

func TestStaticKwBitsMasked(t *testing.T) {
	t.Parallel()

	// Unblockable is a CantBlockBy static in gorge: never compared as a keyword
	if b, _ := gorgeKw("Unblockable"); b&^kwStaticBits != 0 {
		t.Fatalf("Unblockable survives the mask: %v", b)
	}
}

func TestSubtypeVocab(t *testing.T) {
	t.Parallel()

	v := newCharVocab(&kop.Table{Subtypes: []string{"Elf", "Rabbit", "Equipment"}})
	if v.sub["Rabbit"] != 1 || v.sub["Equipment"] != 2 {
		t.Fatalf("vocab %v", v.sub)
	}
	if _, ok := v.sub["Bird"]; ok {
		t.Fatal("a subtype outside the table is mapped")
	}
}

func TestCharsOfChangeling(t *testing.T) {
	t.Parallel()

	v := newCharVocab(&kop.Table{Subtypes: []string{"Elf", "Rabbit", "Shapeshifter", "Forest"}})
	// gorge materialises a changeling's creature types into its type words
	// and marks AllCreatureTypes
	c, bad := charsOf(v, []string{"Creature", "Shapeshifter", "Elf", "Rabbit", "Bird"}, true, "UG", []string{"Changeling", "Flying"})
	if bad != "" {
		t.Fatalf("Changeling is modelled as the subtype set, got bad=%q", bad)
	}
	want := kop.SubMask{}.With(0).With(1).With(2)
	if c.Subtypes != want || !c.AllTypes || c.Types != uint16(kop.TCreature) || c.Colors != uint8(kop.ColU|kop.ColG) || c.Keywords != uint32(kop.KwFlying) {
		t.Fatalf("charsOf = %+v", c)
	}
}
