package kgen

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// evidenceList is one evidence file's name (without .txt) and sorted card names.
type evidenceList struct {
	name, body string
}

// writeEvidenceLists makes a support root holding lists under cardset.EvidenceDir,
// the place both refuseCard and the TestEvidenceAndRefusedAreDisjoint guard look.
func writeEvidenceLists(t *testing.T, lists []evidenceList) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(cardset.EvidenceDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, l := range lists {
		if err := os.WriteFile(filepath.Join(dir, l.name+".txt"), []byte(l.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func evidencePath(root, name string) string {
	return filepath.Join(root, filepath.FromSlash(cardset.EvidenceDir), name+".txt")
}

// TestRefuseCardStripsEveryEvidenceList pins the data invariant behind
// TestEvidenceAndRefusedAreDisjoint: refusing a card by name must take it out of
// every evidence list that names it and record each of those lists, so -unrefuse
// restores exactly the rows it took and a later evidence run that writes a refused
// name back is still caught by the guard.
func TestRefuseCardStripsEveryEvidenceList(t *testing.T) {
	const card = "Rockface Village"
	lists := []evidenceList{
		{"aa-first", card + "\nZebra\n"},
		{"bb-middle", "Plains\n" + card + "\nSwamp\n"},
		{"cc-last", "Aaa\nBbb\n" + card + "\n"},
		{"dd-absent", "Aaa\nZzz\n"},
		{"ee-absent", "Plains\nSwamp\n"},
	}
	wantEvidence := []string{"aa-first", "bb-middle", "cc-last"}
	root := writeEvidenceLists(t, lists)

	// Precondition: the card is where refuseCard's glob looks, and only in the
	// lists whose removal the assertions below measure.
	var in []string
	for _, l := range lists {
		if slices.Contains(parseCardList([]byte(l.body)), card) {
			in = append(in, l.name)
		}
	}
	if !slices.Equal(in, wantEvidence) {
		t.Fatalf("fixture has the card in %v, want %v", in, wantEvidence)
	}

	repro, err := parseRefusedRepro("blb:autopay:710")
	if err != nil {
		t.Fatal(err)
	}
	path, err := refuseCard(root, card, "lockstep", "it differs", repro)
	if err != nil {
		t.Fatal(err)
	}

	// Every list lost the card, in place, and every list that never held it is
	// unchanged.
	for _, l := range lists {
		b, err := os.ReadFile(evidencePath(root, l.name))
		if err != nil {
			t.Fatal(err)
		}
		got := parseCardList(b)
		if slices.Contains(got, card) {
			t.Errorf("%s still names %s: %q", l.name, card, b)
		}
		want := parseCardList([]byte(l.body))
		if i := slices.Index(want, card); i >= 0 {
			want = slices.Delete(want, i, i+1)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s = %q, want %q", l.name, got, want)
		}
	}

	entries, err := cardset.ReadRefused(os.DirFS(root))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries %v, %v", entries, err)
	}
	if !slices.Equal(entries[0].Evidence, wantEvidence) {
		t.Errorf("entry records evidence %v, want %v", entries[0].Evidence, wantEvidence)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("entry %s: %v", path, err)
	}

	// The guard reads the same two maps: the card is refused and in no list.
	s, err := readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if s.refused[card] == nil || s.evidencedFor(card) {
		t.Fatalf("support: refused %v, evidenced %v", s.refused[card] != nil, s.evidencedFor(card))
	}

	// A re-added row is the violation the guard reports; readSupport must see it.
	if err := os.WriteFile(evidencePath(root, "dd-absent"), []byte("Aaa\n"+card+"\nZzz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if s.refused[card] == nil || !s.evidenced[card] {
		t.Fatalf("re-added row: refused %v, evidenced %v; the guard would miss it", s.refused[card] != nil, s.evidenced[card])
	}

	// -unrefuse restores exactly the rows the entry recorded, and nothing else.
	if err := unrefuseCard(root, card); err != nil {
		t.Fatal(err)
	}
	for _, l := range lists {
		b, err := os.ReadFile(evidencePath(root, l.name))
		if err != nil {
			t.Fatal(err)
		}
		want := l.body
		if l.name == "dd-absent" {
			want = "Aaa\n" + card + "\nZzz\n"
		}
		if string(b) != want {
			t.Errorf("%s after unrefuse = %q, want %q", l.name, b, want)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("entry stayed: %v", err)
	}
}
