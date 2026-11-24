package cards

import (
	"slices"
	"testing"
)

// TestTimeLordIsOneCreatureType pins CR 205.3m: "One creature type is two
// words long: Time Lord." The Types: line spells it as two space-separated
// words, but the parsed face must carry the one subtype "Time Lord" (Forge and
// XMage both report a single Time Lord subtype for Romana II / The Thirteenth
// Doctor), not the two tokens Time and Lord.
func TestTimeLordIsOneCreatureType(t *testing.T) {
	src := []byte("Name:Test Doctor\nManaCost:2 U\nTypes:Legendary Creature Time Lord Doctor\nPT:2/2\nOracle:\n")
	c, diags := ParseBytes("test_doctor.txt", src)
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	if len(c.Faces) != 1 {
		t.Fatalf("faces = %d, want 1", len(c.Faces))
	}
	got := c.Faces[0].Types
	want := []string{"Legendary", "Creature", "Time Lord", "Doctor"}
	if !slices.Equal(got, want) {
		t.Fatalf("Types = %q, want %q", got, want)
	}
	// A Time or Lord word on its own is not CR 205.3m's subtype: a line that
	// carries only one of them keeps it as written.
	c2, _ := ParseBytes("t2.txt", []byte("Name:X\nTypes:Creature Lord Time\nPT:1/1\n"))
	if got := c2.Faces[0].Types; !slices.Equal(got, []string{"Creature", "Lord", "Time"}) {
		t.Fatalf("non-adjacent Lord Time = %q, want unchanged", got)
	}
}
