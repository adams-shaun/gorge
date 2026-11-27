package kgen

import (
	"os"
	"testing"
)

// TestReadSupportXproofRowEvidences pins the reader's side of scripts/dirac-xproof-evidence.py:
// a row Name\t<seats>\txproof evidences its card from two seats (dirac's lockstep plays two),
// where a two-seat row of any other source, or none, does not; a four-seat row still evidences
// whatever its third column says, a three-seat xproof row as well, and the third column does not hide the seats field.
func TestReadSupportXproofRowEvidences(t *testing.T) {
	root := writeEvidenceLists(t, []evidenceList{{"pod-xproof",
		"Abrade\t2\txproof\nBorrowed Hostility\t2\tother\nCommand Tower\t4\txproof\nDismember\t3\txproof\nEvolving Wilds\t2\nFlusterstorm\t2\t\n"}})
	// Precondition: the list is where readSupport's glob looks.
	if _, err := os.Stat(evidencePath(root, "pod-xproof")); err != nil {
		t.Fatal(err)
	}
	s, err := readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	for card, want := range map[string]bool{
		"Abrade": true, "Command Tower": true, "Dismember": true,
		"Borrowed Hostility": false, "Evolving Wilds": false, "Flusterstorm": false,
	} {
		if got := s.evidencedFor(card); got != want {
			t.Errorf("evidencedFor(%q) = %v, want %v: evidenced=%v", card, got, want, s.evidenced)
		}
	}
	if !s.podEvidencedFor("Command Tower") || !s.podEvidencedFor("Dismember") || s.podEvidencedFor("Abrade") {
		t.Errorf("pod-evidenced column wrong: %v", s.podEvidenced)
	}
}
