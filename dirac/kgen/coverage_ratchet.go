package kgen

import (
	"bufio"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

const (
	coverageKeysGlob  = "dirac/support/evidence-keys/*.tsv"
	coverageLegacy    = "dirac/support/evidence-keys/LEGACY.txt"
	coverageFloorPath = "dirac/support/coverage-floor.txt"
)

// coverageFloor is dirac/support/coverage-floor.txt: the totals over every sidecar row the
// tree has reached, which only rise, and the count of rows whose recorded lir16 no longer
// matches the card's IR, which only falls.
type coverageFloor struct {
	keysUsed, keysTotal, branchesHit, branchesTotal, full, staleMax int
}

func (f *coverageFloor) fields() []struct {
	name string
	p    *int
} {
	return []struct {
		name string
		p    *int
	}{{"keys_used", &f.keysUsed}, {"keys_total", &f.keysTotal}, {"branches_hit", &f.branchesHit},
		{"branches_total", &f.branchesTotal}, {"full", &f.full}, {"stale_max", &f.staleMax}}
}

func (f coverageFloor) String() string {
	var b strings.Builder
	for _, fl := range f.fields() {
		fmt.Fprintf(&b, "%s=%d\n", fl.name, *fl.p)
	}
	return b.String()
}

func readCoverageFloor(src fs.FS) (coverageFloor, error) {
	var f coverageFloor
	b, err := fs.ReadFile(src, coverageFloorPath)
	if err != nil {
		return f, err
	}
	fields := f.fields()
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		i := slices.IndexFunc(fields, func(fl struct {
			name string
			p    *int
		}) bool {
			return fl.name == k
		})
		n, err := strconv.Atoi(v)
		if !ok || i < 0 || err != nil || n < 0 {
			return f, fmt.Errorf("%s: bad line %q", coverageFloorPath, line)
		}
		*fields[i].p = n
		seen[k] = true
	}
	for _, fl := range fields {
		if !seen[fl.name] {
			return f, fmt.Errorf("%s: %s missing", coverageFloorPath, fl.name)
		}
	}
	return f, nil
}

// coverageRow is one sidecar row.
type coverageRow struct {
	card, lir16                         string
	keysUsed, keysTotal, brHit, brTotal int
}

func parseFrac(s string) (int, int, error) {
	a, b, ok := strings.Cut(s, "/")
	n, err1 := strconv.Atoi(a)
	m, err2 := strconv.Atoi(b)
	if !ok || err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("bad n/m %q", s)
	}
	return n, m, nil
}

// readCoverageRows reads every sidecar, one row per card (the first sidecar in name order
// wins), and the card names in row order.
func readCoverageRows(src fs.FS) (map[string]coverageRow, []string, error) {
	files, err := fs.Glob(src, coverageKeysGlob)
	if err != nil {
		return nil, nil, err
	}
	rows := map[string]coverageRow{}
	var order []string
	for _, f := range files {
		b, err := fs.ReadFile(src, f)
		if err != nil {
			return nil, nil, err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if line == "" || (i == 0 && strings.HasPrefix(line, "card\t")) {
				continue
			}
			col := strings.Split(line, "\t")
			if len(col) < 6 {
				return nil, nil, fmt.Errorf("%s:%d: %d columns, want 7", f, i+1, len(col))
			}
			r := coverageRow{card: col[0], lir16: col[1]}
			if r.keysUsed, r.keysTotal, err = parseFrac(col[3]); err != nil {
				return nil, nil, fmt.Errorf("%s:%d: %w", f, i+1, err)
			}
			if r.brHit, r.brTotal, err = parseFrac(col[5]); err != nil {
				return nil, nil, fmt.Errorf("%s:%d: %w", f, i+1, err)
			}
			if _, dup := rows[r.card]; !dup {
				rows[r.card] = r
				order = append(order, r.card)
			}
		}
	}
	return rows, order, nil
}

// coverageRowNames is the card names that have a sidecar row: the cards whose IR a test
// has to lower to compare against the row's lir16.
func coverageRowNames(src fs.FS) ([]string, error) {
	_, order, err := readCoverageRows(src)
	slices.Sort(order)
	return order, err
}

func readLegacy(src fs.FS) (map[string]bool, error) {
	out := map[string]bool{}
	f, err := src.Open(coverageLegacy)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if n := strings.TrimSpace(sc.Text()); n != "" {
			out[n] = true
		}
	}
	return out, sc.Err()
}

// coverageNow is the floor's fields as the tree stands: stale in staleMax.
func coverageNow(src fs.FS, lirNow map[string]string) (coverageFloor, error) {
	rows, order, err := readCoverageRows(src)
	if err != nil {
		return coverageFloor{}, err
	}
	byFace := map[string]string{}
	for name, k := range lirNow {
		if f := firstFace(name); f != "" {
			byFace[f] = k
		}
	}
	var now coverageFloor
	for _, card := range order {
		r := rows[card]
		now.keysUsed += r.keysUsed
		now.keysTotal += r.keysTotal
		now.branchesHit += r.brHit
		now.branchesTotal += r.brTotal
		if r.keysUsed == r.keysTotal && r.brHit == r.brTotal {
			now.full++
		}
		k, ok := lirNow[card]
		if !ok {
			k, ok = byFace[card]
		}
		if ok && r.lir16 != "" && r.lir16 != k {
			now.staleMax++
		}
	}
	return now, nil
}

// coverageFalls is every way the evidence-keys sidecar and the floor disagree with the
// tree, sorted: a row whose card left the lists, an evidenced card with neither a row nor a
// LEGACY.txt entry, a LEGACY.txt entry that has a row, and a floor total that fell (or
// stale count that rose). lirNow is each card's current LIRKeys; a row whose card is absent
// from it, or has no recorded lir16, is not counted stale. A card refused by name is
// skipped: its refusal's change deletes the row.
func coverageFalls(src fs.FS, lirNow map[string]string) ([]string, error) {
	sup, err := readSupport(src)
	if err != nil {
		return nil, err
	}
	rows, _, err := readCoverageRows(src)
	if err != nil {
		return nil, err
	}
	legacy, err := readLegacy(src)
	if err != nil {
		return nil, err
	}
	floor, err := readCoverageFloor(src)
	if err != nil {
		return nil, err
	}
	now, err := coverageNow(src, lirNow)
	if err != nil {
		return nil, err
	}
	refused := func(card string) bool {
		if sup.refusedFor(card) != nil {
			return true
		}
		for name := range sup.refused {
			if firstFace(name) == card {
				return true
			}
		}
		return false
	}
	var falls []string
	for card := range rows {
		if !sup.evidenced[card] && !refused(card) {
			falls = append(falls, "lost evidence: "+card)
		}
	}
	for card := range sup.evidenced {
		if _, ok := rows[card]; !ok && !legacy[card] && !refused(card) {
			falls = append(falls, "no coverage record: "+card)
		}
	}
	for card := range legacy {
		if _, ok := rows[card]; ok {
			falls = append(falls, "stale legacy entry: "+card)
		}
	}
	if now.keysUsed < floor.keysUsed {
		falls = append(falls, fmt.Sprintf("keys fell: %d used, floor %d", now.keysUsed, floor.keysUsed))
	}
	if now.branchesHit < floor.branchesHit {
		falls = append(falls, fmt.Sprintf("branches fell: %d hit, floor %d", now.branchesHit, floor.branchesHit))
	}
	if now.full < floor.full {
		falls = append(falls, fmt.Sprintf("full fell: %d cards fully covered, floor %d", now.full, floor.full))
	}
	if now.staleMax > floor.staleMax {
		falls = append(falls, fmt.Sprintf("stale rose: %d rows at an old IR, stale_max %d", now.staleMax, floor.staleMax))
	}
	slices.Sort(falls)
	return falls, nil
}
