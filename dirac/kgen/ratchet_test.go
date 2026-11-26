package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The acceptance ratchets: kgen lowers every registered card list in
// dirac/cardset/setlists and counts the accepted cards (cards that lower,
// whether or not refused by name). The pins are data, one row per list in
// testdata/acceptance.tsv (`list<TAB>accepted<TAB>cards`, lists in byte order);
// git log -p on that file is the pin history, and a commit says which cards it
// moves.
//
// By default a fall fails and a rise passes, so a pull request or a merge-queue
// batch need not commit the table (two PRs each +1 failed a batch on an exact
// pin, 11-17). DIRAC_PIN_STRICT=1, which the nightly CI run sets, makes every pin
// exact; KGEN_REPIN=1 re-measures them.
//
// The count is LOWER + BUILD (checkAcceptance runs buildPool): a card the LIR
// pass accepts but `generate` refuses (counter kinds, cost shapes, filter
// compares) is not accepted here, so the pins equal what can be emitted.
// MBC is the Mystery Booster Commander draft pool's dealt names, and CMM, MH3,
// MH2 and CM2 are Commander and Modern Horizons sets, not Standard sets; the
// operator tracks them.
const (
	acceptanceTSV      = "testdata/acceptance.tsv"
	buildAcceptanceTSV = "testdata/build_acceptance.tsv"
	pinTSVHeader       = "list\taccepted\tcards"
)

// pinStrict reports whether DIRAC_PIN_STRICT=1 asks for exact pins.
func pinStrict() bool { return os.Getenv("DIRAC_PIN_STRICT") == "1" }

// parsePinTSV reads a pin table: the header, then one
// `list<TAB>accepted<TAB>cards` row per list, lists in strictly increasing byte
// order and accepted never above cards.
func parsePinTSV(src string) (map[string][2]int, error) {
	body, ok := strings.CutSuffix(src, "\n")
	if !ok {
		return nil, fmt.Errorf("pin table does not end in a newline")
	}
	lines := strings.Split(body, "\n")
	if lines[0] != pinTSVHeader {
		return nil, fmt.Errorf("pin table header is %q, want %q", lines[0], pinTSVHeader)
	}
	pins := make(map[string][2]int, len(lines)-1)
	prev := ""
	for i, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 3 || f[0] == "" {
			return nil, fmt.Errorf("pin table line %d: %q is not list<TAB>accepted<TAB>cards", i+2, line)
		}
		if f[0] <= prev {
			return nil, fmt.Errorf("pin table line %d: %s is not byte-after %s", i+2, f[0], prev)
		}
		prev = f[0]
		var pin [2]int
		for j, s := range f[1:] {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 || strconv.Itoa(n) != s {
				return nil, fmt.Errorf("pin table line %d: %q is not a count", i+2, s)
			}
			pin[j] = n
		}
		if pin[0] > pin[1] {
			return nil, fmt.Errorf("pin table line %d: %s accepts %d of %d", i+2, f[0], pin[0], pin[1])
		}
		pins[f[0]] = pin
	}
	return pins, nil
}

// readPinTSV is parsePinTSV over the file at path.
func readPinTSV(t *testing.T, path string) map[string][2]int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := parsePinTSV(string(b))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return pins
}

// acceptancePins is the per-list pin table, read afresh on each call.
func acceptancePins(t *testing.T) map[string][2]int {
	t.Helper()
	return readPinTSV(t, acceptanceTSV)
}

// judgePin says whether got satisfies pin: equal always does; a fall (fewer
// accepted) never does; any other difference does only when strict is false.
func judgePin(name string, pin, got [2]int, strict bool) error {
	switch {
	case got == pin:
		return nil
	case got[0] < pin[0]:
		return fmt.Errorf("%s: accepted %d of %d, pinned %d of %d: a fall is a lowering regression; fix it, or re-pin with KGEN_REPIN=1 KGEN_REPIN_FALL=<reason>",
			name, got[0], got[1], pin[0], pin[1])
	case strict:
		return fmt.Errorf("%s: accepted %d of %d, pinned %d of %d: DIRAC_PIN_STRICT=1 holds the pin exact; re-pin with KGEN_REPIN=1",
			name, got[0], got[1], pin[0], pin[1])
	}
	return nil
}

// checkPin holds got against list's pin in the table at path: KGEN_REPIN=1
// rewrites a moved row, and otherwise judgePin decides under DIRAC_PIN_STRICT.
func checkPin(t *testing.T, path, list, name string, pin, got [2]int) {
	t.Helper()
	if got != pin && os.Getenv("KGEN_REPIN") == "1" {
		msg, err := repinTSV(path, list, pin, got, os.Getenv("KGEN_REPIN_FALL"))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s", path, msg)
		return
	}
	if err := judgePin(name, pin, got, pinStrict()); err != nil {
		t.Error(err)
	} else if got != pin {
		t.Logf("%s: %d of %d, not the pin %d of %d but no fall: land re-pins it (dirac/kgen/%s)",
			name, got[0], got[1], pin[0], pin[1], path)
	}
}

// testSetlistDir is setlistDir relative to the test's working directory
// (dirac/kgen), not the module root the -census-std flag runs from.
const testSetlistDir = "../../" + setlistDir

func ratchetCards(t *testing.T) string {
	t.Helper()
	cardsDir := os.Getenv("KGEN_CARDS")
	if cardsDir == "" {
		var err error
		if cardsDir, err = cardset.CorpusDir(); err != nil {
			t.Skipf("SKIP: acceptance ratchet NOT RUN: %v; link the corpus as .cards here", err)
		}
	}
	if _, err := os.Stat(cardsDir); err != nil {
		t.Skipf("SKIP: acceptance ratchet NOT RUN: corpus %s missing (%v); link it as .cards here", cardsDir, err)
	}
	return cardsDir
}

func checkAcceptance(t *testing.T, cardsDir, list string) {
	t.Helper()
	pin, ok := acceptancePins(t)[list]
	if !ok {
		t.Fatalf("%s.txt has no row in %s", list, acceptanceTSV)
	}
	p, err := loadCardList(testSetlistPath(t, list), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	// Lower AND build: buildPool appends the build-level refusals (counter
	// kinds, cost shapes, filter compares) to each card, so the pin equals
	// what `generate` can emit, not just what the LIR pass accepts.
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("%s: build: %v", list, err)
	}
	cv := computeCoverage(p, lw)
	t.Logf("%s: %d of %d accepted", list, cv.lowered, cv.cards)
	checkPin(t, acceptanceTSV, list, list, pin, [2]int{cv.lowered, cv.cards})
}

// The ALL list's lower+build pass, shared by the two corpus ratchets: they
// lower the same corpus, so two full passes per package run became one. The
// result is read-only — the callers hold its coverage against their own pins
// (ratchet_test.go's ALL row and build_acceptance.tsv's).
var (
	corpusAccOnce sync.Once
	corpusAccCv   coverage
	corpusAccTook time.Duration
	corpusAccErr  error
)

// corpusAcceptance runs that pass once per package run, in process and writing
// nothing, and returns its coverage.
func corpusAcceptance(t *testing.T) coverage {
	t.Helper()
	cardsDir := ratchetCards(t)
	corpusAccOnce.Do(func() {
		start := time.Now()
		allPath, err := corpusAllList(cardsDir)
		if err != nil {
			corpusAccErr = err
			return
		}
		p, err := loadCardList(allPath, cardsDir)
		if err != nil {
			corpusAccErr = err
			return
		}
		// Lower AND build: buildPool appends the build-level refusals (counter
		// kinds, cost shapes, filter compares) to each card, so the coverage
		// equals what `generate` can emit, not just what the LIR pass accepts.
		lw := lowerPool(p)
		if _, _, _, err := buildPool(lw); err != nil {
			corpusAccErr = fmt.Errorf("ALL: build: %v", err)
			return
		}
		corpusAccCv = computeCoverage(p, lw)
		corpusAccTook = time.Since(start)
	})
	if corpusAccErr != nil {
		t.Fatal(corpusAccErr)
	}
	return corpusAccCv
}

// TestCorpusAcceptance is the corpus ratchet (the generated ALL list, ~3 s).
func TestCorpusAcceptance(t *testing.T) {
	heavyParallel(t)
	pin, ok := acceptancePins(t)["ALL"]
	if !ok {
		t.Fatalf("ALL.txt has no row in %s", acceptanceTSV)
	}
	cv := corpusAcceptance(t)
	t.Logf("ALL: %d of %d accepted", cv.lowered, cv.cards)
	checkPin(t, acceptanceTSV, "ALL", "ALL", pin, [2]int{cv.lowered, cv.cards})
}

// TestSetAcceptance is the per-set ratchet, one subtest per list.
func TestSetAcceptance(t *testing.T) {
	cardsDir := ratchetCards(t)
	ents, err := os.ReadDir(testSetlistDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range ents {
		list, ok := strings.CutSuffix(e.Name(), ".txt")
		if !ok {
			continue
		}
		seen++
		t.Run(list, func(t *testing.T) { checkAcceptance(t, cardsDir, list) })
	}
	if n := len(acceptancePins(t)) - 1; seen != n {
		t.Errorf("%d set lists, %d set pins: a list and its row in %s are added together", seen, n, acceptanceTSV)
	}
}

// repinMu serialises the read-modify-write of a pin file.
var repinMu sync.Mutex

// repinFall checks a re-pin from pin to got and returns the line to print. A
// lower accepted count is refused unless fallReason says why it is acceptable.
func repinFall(list string, pin, got [2]int, fallReason string) (string, error) {
	msg := fmt.Sprintf("%s: repinned to %d of %d", list, got[0], got[1])
	if got[0] < pin[0] {
		if fallReason == "" {
			return "", fmt.Errorf("%s: accepted fell from %d to %d: a fall is a regression; fix it, or set KGEN_REPIN_FALL=<reason> to accept it",
				list, pin[0], got[0])
		}
		msg += fmt.Sprintf(", accepted fell from %d: %s", pin[0], fallReason)
	}
	return msg, nil
}

// setPinTSVRow rewrites list's row in the pin table src to got; every other
// byte is kept.
func setPinTSVRow(src, list string, got [2]int) (string, bool) {
	lines := strings.SplitAfter(src, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, list+"\t") {
			lines[i] = fmt.Sprintf("%s\t%d\t%d\n", list, got[0], got[1])
			return strings.Join(lines, ""), true
		}
	}
	return src, false
}

// repinTSV moves list's pin in the pin table at path from pin to got, under
// repinFall's rule, and returns the line to print.
func repinTSV(path, list string, pin, got [2]int, fallReason string) (string, error) {
	msg, err := repinFall(list, pin, got, fallReason)
	if err != nil {
		return "", err
	}
	repinMu.Lock()
	defer repinMu.Unlock()
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	out, ok := setPinTSVRow(string(b), list, got)
	if !ok {
		return "", fmt.Errorf("no pin row for %s in %s", list, path)
	}
	if _, err := parsePinTSV(out); err != nil {
		return "", fmt.Errorf("%s: %v", path, err)
	}
	return msg, os.WriteFile(path, []byte(out), 0o644)
}

// repinSource rewrites the row in src to {got[0], got[1]}; comments and order
// are untouched. Groups 1 and 2 of row are what surrounds the numbers.
func repinSource(src string, row *regexp.Regexp, got [2]int) (string, bool) {
	if !row.MatchString(src) {
		return src, false
	}
	return row.ReplaceAllString(src, fmt.Sprintf("${1}%d, %d${2}", got[0], got[1])), true
}

// repinFile moves a pin held in Go source (the standard-meta pin) at path from
// pin to got, under repinFall's rule.
func repinFile(path string, row *regexp.Regexp, list string, pin, got [2]int, fallReason string) (string, error) {
	msg, err := repinFall(list, pin, got, fallReason)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	out, ok := repinSource(string(b), row, got)
	if !ok {
		return "", fmt.Errorf("no pin row for %s in %s", list, path)
	}
	return msg, os.WriteFile(path, []byte(out), 0o644)
}

func TestPinTablesParse(t *testing.T) {
	all := acceptancePins(t)
	if _, ok := all["ALL"]; !ok {
		t.Errorf("%s has no ALL row", acceptanceTSV)
	}
	build := readPinTSV(t, buildAcceptanceTSV)
	if _, ok := build["ALL"]; !ok || len(build) != 1 {
		t.Errorf("%s holds %v, want the one ALL row", buildAcceptanceTSV, build)
	}
	for _, bad := range []string{
		"",
		"list\taccepted\tcards",
		"list accepted cards\nA\t1\t2\n",
		"list\taccepted\tcards\nB\t1\t2\nA\t1\t2\n",
		"list\taccepted\tcards\nA\t1\t2\nA\t1\t2\n",
		"list\taccepted\tcards\nA\t1\n",
		"list\taccepted\tcards\nA\t01\t2\n",
		"list\taccepted\tcards\nA\t3\t2\n",
		"list\taccepted\tcards\n\nA\t1\t2\n",
	} {
		if _, err := parsePinTSV(bad); err == nil {
			t.Errorf("parsePinTSV(%q) accepted a malformed table", bad)
		}
	}
}

// TestPinRiseNonStrictPassesStrictFails holds the two modes apart on the live
// ALL pin: a count one above it (a pin set one below the live count) passes by
// default and fails under DIRAC_PIN_STRICT=1; a fall fails both; equal passes
// both.
func TestPinRiseNonStrictPassesStrictFails(t *testing.T) {
	pin := acceptancePins(t)["ALL"]
	for _, c := range []struct {
		name            string
		got             [2]int
		lenient, strict bool // whether each mode passes
	}{
		{"equal", pin, true, true},
		{"rise", [2]int{pin[0] + 1, pin[1]}, true, false},
		{"rise with the list", [2]int{pin[0] + 1, pin[1] + 1}, true, false},
		{"list grows, accepted holds", [2]int{pin[0], pin[1] + 1}, true, false},
		{"fall", [2]int{pin[0] - 1, pin[1]}, false, false},
	} {
		if err := judgePin("ALL", pin, c.got, false); (err == nil) != c.lenient {
			t.Errorf("%s, non-strict: err = %v, want pass %v", c.name, err, c.lenient)
		}
		if err := judgePin("ALL", pin, c.got, true); (err == nil) != c.strict {
			t.Errorf("%s, strict: err = %v, want pass %v", c.name, err, c.strict)
		}
	}
}

func TestRepinRewritesOnlyNumbers(t *testing.T) {
	src := "list\taccepted\tcards\nALL\t1\t2\nBLB\t3\t4\nBLBX\t5\t6\n"
	out, ok := setPinTSVRow(src, "BLB", [2]int{30, 40})
	if !ok {
		t.Fatal("row not found")
	}
	want := strings.Replace(src, "BLB\t3\t4\n", "BLB\t30\t40\n", 1)
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
	if _, ok := setPinTSVRow(src, "XXX", [2]int{1, 1}); ok {
		t.Error("missing list reported found")
	}
}

// fakePinFile is a pin table holding one list, FAKE, at {10, 20}.
func fakePinFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pins.tsv")
	if err := os.WriteFile(path, []byte(pinTSVHeader+"\nFAKE\t10\t20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func wantFakePin(t *testing.T, path string, want [2]int) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, w := string(b), fmt.Sprintf("%s\nFAKE\t%d\t%d\n", pinTSVHeader, want[0], want[1]); got != w {
		t.Errorf("pin file holds %q, want %q", got, w)
	}
}

func TestRepinRefusesFall(t *testing.T) {
	path := fakePinFile(t)
	_, err := repinTSV(path, "FAKE", [2]int{10, 20}, [2]int{9, 20}, "")
	if err == nil {
		t.Fatal("a fall with no reason was accepted")
	}
	if !strings.Contains(err.Error(), "FAKE") {
		t.Errorf("error does not name the list: %v", err)
	}
	wantFakePin(t, path, [2]int{10, 20})
}

func TestRepinFallWithReason(t *testing.T) {
	path := fakePinFile(t)
	msg, err := repinTSV(path, "FAKE", [2]int{10, 20}, [2]int{9, 20}, "a refusal that closes a divergence")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "a refusal that closes a divergence") {
		t.Errorf("message does not print the reason: %q", msg)
	}
	wantFakePin(t, path, [2]int{9, 20})
}

func TestRepinTakesARise(t *testing.T) {
	path := fakePinFile(t)
	if _, err := repinTSV(path, "FAKE", [2]int{10, 20}, [2]int{11, 21}, ""); err != nil {
		t.Fatal(err)
	}
	wantFakePin(t, path, [2]int{11, 21})
}
