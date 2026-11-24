package kgen

import (
	"strings"
	"testing"
)

// TestChangeZoneAllShuffleFromLibraryRefuses: gorge's ChangeZoneAll with
// Shuffle$ True shuffles each library a moved card left when the destination
// is not the library (CR 701.24a, Intuition's "... into your graveyard. Then
// shuffle."); kgen refuses that shape rather than lowering a sweep with no
// shuffle. A graveyard-to-hand sweep (no library origin) still lowers.
func TestChangeZoneAllShuffleFromLibraryRefuses(t *testing.T) {
	probe := func(origin string) string {
		return strings.Join([]string{
			"Name:Probe",
			"ManaCost:U",
			"Types:Sorcery",
			"A:SP$ ChangeZoneAll | Origin$ " + origin + " | Destination$ Hand | ChangeType$ Card.YouOwn+namedIsland | Shuffle$ True | SpellDescription$ Probe.",
			"Oracle:Probe.",
		}, "\n")
	}
	const want = "ChangeZoneAll Shuffle$ from the library to"
	if got := abReasons(abLower(t, probe("Library"), false)); !strings.Contains(got, want) {
		t.Fatalf("library-origin sweep: refusals %q, want %q", got, want)
	}
	if got := abReasons(abLower(t, probe("Graveyard"), false)); strings.Contains(got, want) {
		t.Fatalf("graveyard-origin sweep: refusals %q, must not carry %q", got, want)
	}
}
