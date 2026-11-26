package kgen

import (
	"os"
	"testing"
)

// TestReadSupportPodRowEvidences pins OD-4 stage 2's support rule in the
// reader, where the evidence lists are read: a four-seat pod row
// (Name\t4) evidences its card, exactly as a plain two-seat row does, while a
// three-seat row stays report only (pod-evidenced) and a row with no seats field
// still evidences. A four-seat row is what credits a Commander-only key that
// two-seat games never use (Command Tower's colour-identity mana).
func TestReadSupportPodRowEvidences(t *testing.T) {
	root := writeEvidenceLists(t, []evidenceList{{"pod-cmd5", "Forest\t4\nIsland\t3\nMountain\n"}})
	// Precondition: the tabbed and plain rows are where readSupport's glob looks.
	if _, err := os.Stat(evidencePath(root, "pod-cmd5")); err != nil {
		t.Fatal(err)
	}
	s, err := readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if !s.podEvidencedFor("Forest") || !s.podEvidencedFor("Island") {
		t.Errorf("pod evidence lost: podEvidenced=%v", s.podEvidenced)
	}
	if !s.evidencedFor("Forest") {
		t.Errorf("a four-seat pod row must evidence Forest: evidenced=%v", s.evidenced)
	}
	if s.evidencedFor("Island") {
		t.Errorf("a three-seat row must stay report only: evidenced=%v", s.evidenced)
	}
	if !s.evidencedFor("Mountain") {
		t.Errorf("a plain row must still evidence Mountain: evidenced=%v", s.evidenced)
	}
}
