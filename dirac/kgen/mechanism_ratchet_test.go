package kgen

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// mechanismTSV pins the ALL carrier rows per mechanism family. The table is a
// ratchet: a family that grows fails, so a D-phase PR shows its yield as a fall
// in its row. A fall passes unless DIRAC_PIN_STRICT=1 (the nightly) holds the
// table exact; KGEN_REPIN=1 rewrites it.
const mechanismTSV = "testdata/mechanism.tsv"

// mechanismTable renders the pin: one family<TAB>carrier_rows row per family,
// in family order.
func mechanismTable(rows []stdTSVRow) string {
	var b strings.Builder
	b.WriteString("# TestMechanismCensusOnlyFalls: ALL carrier rows per mechanism family (family<TAB>carrier_rows)\n")
	for _, r := range mechanismCensus(rows) {
		fmt.Fprintf(&b, "%s\t%d\n", r.fam, r.carriers)
	}
	return b.String()
}

// parseMechanismTable reads family<TAB>carrier_rows lines, ignoring comments.
func parseMechanismTable(src string) (map[string]int, error) {
	m := map[string]int{}
	for _, line := range strings.Split(strings.TrimSuffix(src, "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fam, count, ok := strings.Cut(line, "\t")
		n, err := strconv.Atoi(count)
		if !ok || err != nil || fam == "" || n < 0 {
			return nil, fmt.Errorf("bad line %q", line)
		}
		m[fam] = n
	}
	return m, nil
}

// TestMechanismCensusOnlyFalls lowers the ALL list with every blocker and holds
// the per-family carrier rows against testdata/mechanism.tsv.
func TestMechanismCensusOnlyFalls(t *testing.T) {
	heavyParallel(t)
	names, err := readCardList(testSetlistPath(t, "ALL"))
	if err != nil {
		t.Fatal(err)
	}
	lr, err := lowerStdList(stdList{set: "ALL", names: names}, ratchetCards(t), true)
	if err != nil {
		t.Fatal(err)
	}
	table := mechanismTable(lr.rows)
	t.Logf("\n%s", table)
	if os.Getenv("KGEN_REPIN") == "1" {
		if err := os.WriteFile(mechanismTSV, []byte(table), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(mechanismTSV)
	if err != nil {
		t.Fatal(err)
	}
	want, err := parseMechanismTable(string(b))
	if err != nil {
		t.Fatal(err)
	}
	census := mechanismCensus(lr.rows)
	if len(census) != len(mechFamilies) {
		t.Fatalf("census has %d families, want %d", len(census), len(mechFamilies))
	}
	total := 0
	for _, r := range census {
		total += r.carriers
	}
	if total == 0 {
		t.Fatal("ALL census found no carrier rows")
	}
	var moved []string
	for _, r := range census {
		pin := want[r.fam]
		if r.carriers > pin || (pinStrict() && r.carriers < pin) {
			moved = append(moved, fmt.Sprintf("%s %d -> %d", r.fam, pin, r.carriers))
		}
	}
	seen := map[string]bool{}
	for _, r := range census {
		seen[r.fam] = true
	}
	for fam, pin := range want {
		if !seen[fam] && pinStrict() {
			moved = append(moved, fmt.Sprintf("%s %d -> 0", fam, pin))
		}
	}
	slices.Sort(moved)
	if len(moved) > 0 {
		t.Errorf("mechanism census moved from %s (re-pin with KGEN_REPIN=1):\n%s", mechanismTSV, strings.Join(moved, "\n"))
	}
}
