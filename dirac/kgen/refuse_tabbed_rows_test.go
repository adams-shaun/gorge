package kgen

import (
	"os"
	"strings"
	"testing"
)

// refuseTestCard refuses card under root with the standard test repro.
func refuseTestCard(t *testing.T, root, card string) {
	t.Helper()
	repro, err := parseRefusedRepro("blb:autopay:710")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refuseCard(root, card, "lockstep", "it differs", repro); err != nil {
		t.Fatal(err)
	}
}

// TestRefuseCardCutsATabbedEvidenceRow pins the row, not just the name: an
// evidence row carries a seats column (Name\t<seats>), and refusing a card has
// to remove that whole row. Cutting only "\n<name>\t" leaves the seats value
// behind as a phantom card name, which the coverage ratchet then reports as a
// card with no record.
func TestRefuseCardCutsATabbedEvidenceRow(t *testing.T) {
	const card = "Rockface Village"
	lists := []evidenceList{
		{"aa-first", card + "\t4\nAlpha\t3\n"},
		{"bb-middle", "Alpha\t4\n" + card + "\t4\nCharlie\t3\n"},
		{"cc-last", "Alpha\t4\n" + card + "\t3\n"},
	}
	want := map[string]string{
		"aa-first":  "Alpha\t3\n",
		"bb-middle": "Alpha\t4\nCharlie\t3\n",
		"cc-last":   "Alpha\t4\n",
	}
	root := writeEvidenceLists(t, lists)
	// Precondition: each target row carries the seats column refuseCard must
	// consume with the name.
	for _, l := range lists {
		if !strings.Contains(l.body, card+"\t") {
			t.Fatalf("fixture %s has no tabbed row for %s: %q", l.name, card, l.body)
		}
	}

	refuseTestCard(t, root, card)

	for _, l := range lists {
		b, err := os.ReadFile(evidencePath(root, l.name))
		if err != nil {
			t.Fatal(err)
		}
		if got := string(b); got != want[l.name] {
			t.Errorf("%s = %q, want %q", l.name, got, want[l.name])
		}
	}
	s, err := readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	// The refusal must not turn a seats value into a card name, and the rows
	// that stay must keep their seats columns.
	if s.evidenced["4"] || s.evidenced["3"] {
		t.Errorf("a seats value became a card name: %v", s.evidenced)
	}
	if !s.podEvidenced["Alpha"] || !s.podEvidenced["Charlie"] {
		t.Errorf("sibling pod rows lost their seats field: %v", s.podEvidenced)
	}
	if s.podEvidenced[card] {
		t.Errorf("refused card %s still has pod evidence", card)
	}
}

// TestUnrefuseCardKeepsTabbedSiblingRows pins that restoring a card from a
// refusal rewrites each recorded list from its rows, not its names: every other
// row keeps its seats column, and only the restored card is inserted plain.
func TestUnrefuseCardKeepsTabbedSiblingRows(t *testing.T) {
	const card = "Bravo"
	const body = "Alpha\t4\nBravo\nCharlie\t3\n"
	root := writeEvidenceLists(t, []evidenceList{{"aa-tabbed", body}})
	// Precondition: the tabbed sibling rows are there to lose.
	if !strings.Contains(body, "Alpha\t4") || !strings.Contains(body, "Charlie\t3") {
		t.Fatalf("fixture has no tabbed sibling rows: %q", body)
	}

	refuseTestCard(t, root, card)
	b, err := os.ReadFile(evidencePath(root, "aa-tabbed"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), "Alpha\t4\nCharlie\t3\n"; got != want {
		t.Fatalf("after refuse = %q, want %q", got, want)
	}

	if err := unrefuseCard(root, card); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(evidencePath(root, "aa-tabbed"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != body {
		t.Errorf("after unrefuse = %q, want %q", got, body)
	}
}

// TestUnrefuseCardRestoresAPlainName pins the rest of the rewrite: a restored card is
// inserted as a plain name even when its row carried a seats column before the refusal,
// and its tabbed siblings keep theirs.
func TestUnrefuseCardRestoresAPlainName(t *testing.T) {
	const card = "Alpha"
	root := writeEvidenceLists(t, []evidenceList{{"aa-tabbed", card + "\t4\nBravo\nCharlie\t3\n"}})
	refuseTestCard(t, root, card)
	if err := unrefuseCard(root, card); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(evidencePath(root, "aa-tabbed"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), card+"\nBravo\nCharlie\t3\n"; got != want {
		t.Errorf("after unrefuse = %q, want %q", got, want)
	}
}
