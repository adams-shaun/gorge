package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fixtureFS is the Repro's input: one evidence list, one refusal, one
// sidecar with a row for an evidenced card and one for a card in no list,
// and a LEGACY list.
func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"dirac/support/evidence/a.txt": {
			Data: []byte("Sol Ring\nLost Card\n"),
		},
		"dirac/support/refused/push.json": {
			Data: []byte(`{"card":"Push","class":"test","why":"fixture"}`),
		},
		"dirac/support/evidence-keys/a.tsv": {
			Data: []byte("card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\n" +
				"Sol Ring\t1111222233334444\t12\t3/4\tapi:TestUnused\t2/3\tapi:TestBranch\n" +
				"Gone Card\taaaabbbbccccdddd\t2\t1/1\t\t0/0\t\n"),
		},
		"dirac/support/evidence-keys/LEGACY.txt": {
			Data: []byte("Lost Card\n"),
		},
	}
}

// twoCardCorpus is the two-card corpus the audit resolves: Sol Ring and
// Push are real cards, while Gone Card and Lost Card exist only in the
// lists the audit joins.
func twoCardCorpus(t *testing.T) *cards.Registry {
	t.Helper()
	corpus := testutil.CorpusRegistry(t)
	reg := cards.NewRegistry()
	for _, name := range []string{"Sol Ring", "Push"} {
		c, ok := corpus.Lookup(name)
		if !ok {
			t.Fatalf("corpus lacks %q", name)
		}
		reg.Add(c)
	}
	if _, ok := reg.Lookup("Gone Card"); ok {
		t.Fatal("Gone Card must not be in the two-card corpus")
	}
	if _, ok := reg.Lookup("Lost Card"); ok {
		t.Fatal("Lost Card must not be in the two-card corpus")
	}
	return reg
}

// fixtureLIR stands in for kgen.LIRKeysFor: the lowered cards' evidence
// keys, with Sol Ring's key matching its sidecar row.
func fixtureLIR([]string) map[string]string {
	return map[string]string{
		"Sol Ring":  "1111222233334444",
		"Push":      "aaaabbbbccccdddd",
		"Gone Card": "aaaabbbbccccdddd",
		"Lost Card": "deadbeefdeadbeef",
	}
}

func TestAuditRowsGolden(t *testing.T) {
	reg := twoCardCorpus(t)
	rows := audit(fixtureFS(), reg, fixtureLIR, opts{})
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4: %v", len(rows), rows)
	}
	by := map[string]row{}
	for _, r := range rows {
		by[r.Card] = r
	}
	if got := by["Sol Ring"].DClaim; got != "supported" {
		t.Errorf("Sol Ring d_claim = %q, want supported", got)
	}
	if got := by["Sol Ring"].Keys; got != "3/4" {
		t.Errorf("Sol Ring keys = %q, want 3/4", got)
	}
	if got := by["Push"].DClaim; got != "refused" {
		t.Errorf("Push d_claim = %q, want refused", got)
	}
	if got := by["Gone Card"].Lost; got != 1 {
		t.Errorf("Gone Card lost = %d, want 1", got)
	}
	if got := by["Gone Card"].DClaim; got != "unverified" {
		t.Errorf("Gone Card d_claim = %q, want unverified", got)
	}
	if got := by["Lost Card"].Legacy; got != 1 {
		t.Errorf("Lost Card legacy = %d, want 1", got)
	}
	if by["Sol Ring"].Stale != 0 {
		t.Errorf("Sol Ring stale = %d, want 0 (its key matches the sidecar)", by["Sol Ring"].Stale)
	}

	var got bytes.Buffer
	if err := writeRows(&got, rows); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("rows differ from testdata/golden.tsv\n--- got ---\n%s--- want ---\n%s", got.Bytes(), want)
	}
}

func TestAuditDeterministic(t *testing.T) {
	fsys := fixtureFS()
	// Enough names that an un-sorted map range is likely to change the bytes.
	var names, sidecar strings.Builder
	for i := 0; i < 24; i++ {
		name := fmt.Sprintf("Card %02d", i)
		fmt.Fprintf(&names, "%s\n", name)
		fmt.Fprintf(&sidecar, "%s\t%016x\t%d\t1/1\t\t0/0\t\n", name, i, i)
	}
	fsys["dirac/support/evidence/b.txt"] = &fstest.MapFile{Data: []byte(names.String())}
	fsys["dirac/support/evidence-keys/b.tsv"] = &fstest.MapFile{Data: []byte(
		"card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\n" + sidecar.String())}

	reg := twoCardCorpus(t)
	var first []byte
	for run := 0; run < 3; run++ {
		rows := audit(fsys, reg, fixtureLIR, opts{})
		var b bytes.Buffer
		if err := writeRows(&b, rows); err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			first = append([]byte(nil), b.Bytes()...)
			continue
		}
		if !bytes.Equal(first, b.Bytes()) {
			t.Fatalf("run %d differs from run 0:\n--- run 0 ---\n%s--- run %d ---\n%s", run, first, run, b.Bytes())
		}
	}
}

func TestNeedFileFormat(t *testing.T) {
	rows := []row{
		{Card: "Alpha", DClaim: "supported", Missing: "k1,k2", Plays: 9},
		{Card: "Beta", DClaim: "supported", Missing: "k3", Plays: 1, FourSeat: true},
		{Card: "Gamma", DClaim: "supported", Missing: "k4", Plays: 2},
	}
	var b bytes.Buffer
	if err := writeNeed(&b, rows); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want a header plus 3 rows:\n%s", len(lines), b.String())
	}
	if want := "card\towed_keys\towed_uses\tpriority"; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	for i, line := range lines[1:] {
		if got := len(strings.Split(line, "\t")); got != 4 {
			t.Errorf("row %d has %d columns, want 4: %q", i, got, line)
		}
	}
	// 4-seat first, then fewer plays: Beta (4-seat), Gamma (2 plays), Alpha (9).
	wantCards := []string{"Beta", "Gamma", "Alpha"}
	for i, want := range wantCards {
		if got := strings.Split(lines[i+1], "\t")[0]; got != want {
			t.Errorf("row %d card = %q, want %q", i, got, want)
		}
	}
}

func TestSummaryCounts(t *testing.T) {
	rows := []row{
		{Card: "Alpha", GClaim: "supported", DLists: "a", Keys: "3/4", Branches: "2/3", Missing: "k", XBucket: "HAS-DIFFERENCES"},
		{Card: "Beta", GClaim: "missing", DLists: "a,b", Keys: "1/1", Branches: "0/0", XBucket: "AGREEING"},
	}
	var b bytes.Buffer
	if err := writeSummary(&b, rows); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d summary lines, want header + separator + 2 lists + ALL:\n%s", len(lines), s)
	}
	for _, want := range []string{"| list:a |", "| list:b |", "| ALL |"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "| ALL | 2 | 1 | 2 | 0 | 0 | 0 | 4/5 | 2/3 | 0 | 1 | 0 | 1 |") {
		t.Errorf("ALL row counts are wrong:\n%s", s)
	}
}
