package dirac

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestSeatArithmeticOnlyShrinks is the shrink-only ratchet on raw two-seat
// arithmetic in the kernel: every non-test .go file of package dirac (this
// directory only; generated *_gen.go files included, subpackages and tests
// excluded, and seatorder.go, where the helpers spell the raw forms). It parses each file with go/ast, so comments and strings never
// count, and counts these spellings per family:
//
//   - opp1minus: a subtraction whose left side is the literal 1 or ends in
//     "+ 1" (1 - p, 1-ctrl, PlayerTarget0+1-p). Use NextSeat(p) where it
//     means the next seat in turn order and IsOpponent(p, q) for a test; a
//     single "the opponent" has no helper yet (it needs a choice or a loop
//     at more than two seats), so do not add one.
//   - xor1: an XOR with the literal 1 on either side, or "^= 1" (p ^ 1).
//     As opp1minus.
//   - and1: an AND with the literal 1 on either side (p&1, [x&1]). Many are
//     parity, not seats; seat masks index by SeatBit(p).
//   - seatloop2: a for loop bounded by the literal 2 ("< 2", "<= 1",
//     "range 2"). Loop over the seats with range MaxSeats, or in turn order
//     from p with NextSeat.
//   - ptarget: a use of PlayerTarget0 or PlayerTarget1 outside types.go.
//     Encode and decode with PlayerCode(p) and isPlayerTarget / CodeSeat(c).
//
// The counts are pinned per family and file in testdata/seatarith.tsv. The
// test fails when a count rises (naming that file's sites) and when it falls without the pin being
// lowered; DIRAC_SEAT_REPIN=1 rewrites the pin to the measurement. Never
// raise a pin by hand.
func TestSeatArithmeticOnlyShrinks(t *testing.T) {
	got, sites := scanSeatArithmetic(t)
	pinPath := filepath.Join("testdata", "seatarith.tsv")
	if os.Getenv("DIRAC_SEAT_REPIN") == "1" {
		var b strings.Builder
		b.WriteString("# family\tfile\tcount: raw two-seat sites in package dirac; shrink only (TestSeatArithmeticOnlyShrinks)\n")
		for _, k := range sortedSeatKeys(got) {
			fmt.Fprintf(&b, "%s\t%d\n", k, got[k])
		}
		if err := os.WriteFile(pinPath, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("re-pinned %s", pinPath)
		return
	}
	pin := readSeatPin(t, pinPath)
	all := map[string]int{}
	for k := range got {
		all[k] = 0
	}
	for k := range pin {
		all[k] = 0
	}
	for _, k := range sortedSeatKeys(all) {
		g, p := got[k], pin[k]
		fam, _, _ := strings.Cut(k, "\t")
		switch {
		case g > p:
			t.Errorf("%s: %d raw two-seat sites, pin %d. Use %s instead. Sites:\n  %s",
				strings.Replace(k, "\t", " in ", 1), g, p, seatHelperFor[fam], strings.Join(sites[k], "\n  "))
		case g < p:
			t.Errorf("%s: %d raw two-seat sites, pin %d: lower the pin (DIRAC_SEAT_REPIN=1 go test -run TestSeatArithmeticOnlyShrinks ./dirac)",
				strings.Replace(k, "\t", " in ", 1), g, p)
		}
	}
	if os.Getenv("DIRAC_SEAT_LIST") == "1" {
		for _, k := range sortedSeatKeys(got) {
			for _, s := range sites[k] {
				fam, _, _ := strings.Cut(k, "\t")
				t.Logf("%s\t%s", fam, s)
			}
		}
	}
}

var seatFamilies = []string{"opp1minus", "xor1", "and1", "seatloop2", "ptarget"}

var seatHelperFor = map[string]string{
	"opp1minus": "NextSeat(p) for turn order or IsOpponent(p, q) for a test (a lone \"the opponent\" has no helper: it needs N-seat design, so do not add one)",
	"xor1":      "NextSeat(p) or IsOpponent(p, q), as opp1minus",
	"and1":      "SeatBit(p) for a per-seat mask",
	"seatloop2": "range MaxSeats, or NextSeat for turn order",
	"ptarget":   "PlayerCode(p) to encode, isPlayerTarget and CodeSeat(c) to decode",
}

func isLitOne(e ast.Expr) bool {
	l, ok := ast.Unparen(e).(*ast.BasicLit)
	return ok && l.Kind == token.INT && l.Value == "1"
}

func isLitTwo(e ast.Expr) bool {
	l, ok := ast.Unparen(e).(*ast.BasicLit)
	return ok && l.Kind == token.INT && l.Value == "2"
}

func scanSeatArithmetic(t *testing.T) (map[string]int, map[string][]string) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	sites := map[string][]string{}
	fset := token.NewFileSet()
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") || n == "seatorder.go" {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		add := func(fam string, pos token.Pos) {
			k := fam + "\t" + n
			got[k]++
			p := fset.Position(pos)
			sites[k] = append(sites[k], fmt.Sprintf("%s:%d", p.Filename, p.Line))
		}
		ast.Inspect(f, func(nd ast.Node) bool {
			switch x := nd.(type) {
			case *ast.BinaryExpr:
				switch x.Op {
				case token.SUB:
					l := ast.Unparen(x.X)
					if isLitOne(l) {
						add("opp1minus", x.Pos())
					} else if b, ok := l.(*ast.BinaryExpr); ok && b.Op == token.ADD && isLitOne(b.Y) {
						add("opp1minus", x.Pos())
					}
				case token.XOR:
					if isLitOne(x.X) || isLitOne(x.Y) {
						add("xor1", x.Pos())
					}
				case token.AND:
					if isLitOne(x.X) || isLitOne(x.Y) {
						add("and1", x.Pos())
					}
				}
			case *ast.AssignStmt:
				if x.Tok == token.XOR_ASSIGN && len(x.Rhs) == 1 && isLitOne(x.Rhs[0]) {
					add("xor1", x.Pos())
				}
			case *ast.ForStmt:
				if c, ok := x.Cond.(*ast.BinaryExpr); ok &&
					((c.Op == token.LSS && isLitTwo(c.Y)) || (c.Op == token.LEQ && isLitOne(c.Y))) {
					add("seatloop2", x.Pos())
				}
			case *ast.RangeStmt:
				if isLitTwo(x.X) {
					add("seatloop2", x.Pos())
				}
			case *ast.Ident:
				if (x.Name == "PlayerTarget0" || x.Name == "PlayerTarget1") && n != "types.go" {
					add("ptarget", x.Pos())
				}
			}
			return true
		})
	}
	return got, sites
}

func sortedSeatKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readSeatPin(t *testing.T, path string) map[string]int {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatalf("%v (DIRAC_SEAT_REPIN=1 writes it)", err)
	}
	defer fh.Close()
	pin := map[string]int{}
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.LastIndexByte(line, '\t')
		if i < 0 {
			t.Fatalf("%s: bad line %q", path, line)
		}
		k, v := line[:i], line[i+1:]
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("%s: bad line %q", path, line)
		}
		pin[k] = n
	}
	return pin
}
