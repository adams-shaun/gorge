package kgen

import (
	"os"
	"testing"
)

// TestReadSupportPodRowDoesNotEvidence pins stage 1's support rule in the
// reader, not just in the coverage ratchet: a pod row (Name\t<seats>) feeds
// pod-evidenced, but it must not make the card supported the way a plain
// two-seat row does.
func TestReadSupportPodRowDoesNotEvidence(t *testing.T) {
	root := writeEvidenceLists(t, []evidenceList{{"pod-cmd5", "Forest\t4\nMountain\n"}})
	// Precondition: one tabbed pod row and one plain row are where readSupport's
	// glob looks.
	if _, err := os.Stat(evidencePath(root, "pod-cmd5")); err != nil {
		t.Fatal(err)
	}
	s, err := readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if !s.podEvidencedFor("Forest") {
		t.Errorf("pod evidence lost: podEvidenced=%v", s.podEvidenced)
	}
	if s.evidencedFor("Forest") {
		t.Errorf("a pod row must not evidence Forest: evidenced=%v", s.evidenced)
	}
	if !s.evidencedFor("Mountain") {
		t.Errorf("a plain row must still evidence Mountain: evidenced=%v", s.evidenced)
	}
}
