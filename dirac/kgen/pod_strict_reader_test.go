package kgen

import (
	"testing"
	"testing/fstest"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestPodStrictReadsSeatsFour: the strict pod column reads only a seats field of
// podCreditSeats or more. A plain row and a three-seat row are not strict; a four-seat
// row is. A split card is found under either spelling: its full-name row, or (as coverage
// writes it) its first face's row, the fallback podEvidencedFor has.
func TestPodStrictReadsSeatsFour(t *testing.T) {
	src := fstest.MapFS{cardset.EvidenceDir + "/pod-strict.txt": {Data: []byte("A\nB\t3\nC\t4\nD // E\t4\nF\t4\n")}}
	s, err := readSupport(src)
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: the rows were read, and the three-seat row is pod-evidenced, so the
	// strict column is the narrower reading, not an unread list.
	if !s.evidencedFor("A") || !s.podEvidencedFor("B") {
		t.Fatalf("rows not read: evidenced=%v podEvidenced=%v", s.evidenced, s.podEvidenced)
	}
	for name, want := range map[string]bool{"A": false, "B": false, "C": true, "D // E": true, "F // G": true, "B // C2": false} {
		if got := s.podStrictFor(name); got != want {
			t.Errorf("podStrictFor(%q) = %v, want %v (podStrict=%v)", name, got, want, s.podStrict)
		}
	}
	// Precondition for the fallback: "F // G" itself has no row, only its first face.
	if s.podStrict["F // G"] || !s.podStrict["F"] {
		t.Errorf("split row read wrong: podStrict=%v", s.podStrict)
	}
}
