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
// directory only; checked-in *_gen.go files included; subpackages, tests and
// seatorder.go, where the helpers spell the raw forms, excluded). It parses
// each file with go/ast, so comments and strings never count, and counts these
// spellings per family:
//
//   - opp1minus: a subtraction whose left side is the literal 1 or ends in
//     "+ 1" (1 - p, 1-ctrl, PlayerTarget0+1-p). Use NextSeat(p) where it
//     means the next seat in turn order and IsOpponent(p, q) for a test; a
//     lone "the opponent" has no helper yet (at more than two seats it is a
//     choice or a loop), so leave it raw rather than invent one.
//   - xor1: an XOR with the literal 1 on either side, or "^= 1". As opp1minus.
//   - and1: an index expression whose index is an AND with the literal 1
//     (a[p&1], a[1&p]). Use SeatIx(p). Bare parity (in.C&1) is not counted.
//   - seatloop2: a for loop bounded by the literal 2 ("< 2", "<= 1") or a
//     "range 2" loop whose variable indexes a [MaxSeats] field of a struct
//     in this package (s.pl[p], s.g.poison[p]). Bound it by MaxSeats.
//   - ptarget: a use of PlayerTarget0 or PlayerTarget1 outside types.go.
//     Use PlayerCode(p), CodeSeat(c) and isPlayerTarget(c).
//
// A site that is genuinely not seat arithmetic is exempted by a trailing
// "// seat-ok: <reason>" comment on its line.
//
// The counts are pinned per family and file in testdata/seatarith.tsv,
// sorted by family then file. The test fails when a count rises (naming that
// file's sites) and when it falls without the pin being lowered;
// DIRAC_SEAT_REPIN=1 rewrites the pin to the measurement. Never raise a pin by
// hand.
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
	famGot, famPin := map[string]int{}, map[string]int{}
	for k, v := range got {
		all[k] = 0
		famGot[seatFam(k)] += v
	}
	for k, v := range pin {
		all[k] = 0
		famPin[seatFam(k)] += v
	}
	const repin = "DIRAC_SEAT_REPIN=1 go test -run TestSeatArithmeticOnlyShrinks ./dirac"
	for _, k := range sortedSeatKeys(all) {
		g, p := got[k], pin[k]
		fam := seatFam(k)
		where := strings.Replace(k, "\t", " in ", 1)
		moved := ""
		if famGot[fam] == famPin[fam] {
			moved = fmt.Sprintf(" The %s total is unchanged (%d): if you only moved code between files, re-pin with: %s", fam, famPin[fam], repin)
		}
		switch {
		case g > p:
			t.Errorf("%s: %d raw two-seat sites, pin %d. Use %s instead; a site that is genuinely not seat arithmetic takes a trailing \"// seat-ok: <reason>\" comment.%s Sites:\n  %s",
				where, g, p, seatHelperFor[fam], moved, strings.Join(sites[k], "\n  "))
		case g < p:
			t.Errorf("%s: %d raw two-seat sites, pin %d: lower the pin, from the repo root: %s", where, g, p, repin+strings.Replace(moved, " if you only moved", " if you also moved", 1))
		}
	}
	if os.Getenv("DIRAC_SEAT_LIST") == "1" {
		for _, k := range sortedSeatKeys(got) {
			for _, s := range sites[k] {
				t.Logf("%s\t%s", seatFam(k), s)
			}
		}
	}
}

func seatFam(k string) string { f, _, _ := strings.Cut(k, "\t"); return f }

var seatHelperFor = map[string]string{
	"opp1minus": "NextSeat(p) for turn order or IsOpponent(p, q) for a test (a lone \"the opponent\" has no helper yet: it needs N-seat design, so leave it raw rather than invent one)",
	"xor1":      "NextSeat(p) or IsOpponent(p, q), as opp1minus",
	"and1":      "SeatIx(p)",
	"seatloop2": "a MaxSeats bound (p < MaxSeats, range MaxSeats)",
	"ptarget":   "PlayerCode(p) to encode, CodeSeat(c) to decode, isPlayerTarget(c) to test",
}

func isLitOne(e ast.Expr) bool {
	l, ok := ast.Unparen(e).(*ast.BasicLit)
	return ok && l.Kind == token.INT && l.Value == "1"
}

func isLitTwo(e ast.Expr) bool {
	l, ok := ast.Unparen(e).(*ast.BasicLit)
	return ok && l.Kind == token.INT && l.Value == "2"
}

func isAndOne(e ast.Expr) bool {
	b, ok := ast.Unparen(e).(*ast.BinaryExpr)
	return ok && b.Op == token.AND && (isLitOne(b.X) || isLitOne(b.Y))
}

// seatFields is the set of struct field names in the package whose type is
// [MaxSeats]T.
func seatFields(files []*ast.File) map[string]bool {
	m := map[string]bool{}
	for _, f := range files {
		ast.Inspect(f, func(nd ast.Node) bool {
			st, ok := nd.(*ast.StructType)
			if !ok {
				return true
			}
			for _, fl := range st.Fields.List {
				if at, ok := fl.Type.(*ast.ArrayType); ok {
					if id, ok := at.Len.(*ast.Ident); ok && id.Name == "MaxSeats" {
						for _, n := range fl.Names {
							m[n.Name] = true
						}
					}
				}
			}
			return true
		})
	}
	return m
}

// indexesSeatField reports whether ident v is the index of x.f[v] for a
// seat field f anywhere in body.
func indexesSeatField(body ast.Node, v string, fields map[string]bool) bool {
	found := false
	ast.Inspect(body, func(nd ast.Node) bool {
		ix, ok := nd.(*ast.IndexExpr)
		if !ok || found {
			return !found
		}
		id, ok := ast.Unparen(ix.Index).(*ast.Ident)
		sel, ok2 := ix.X.(*ast.SelectorExpr)
		if ok && ok2 && id.Name == v && fields[sel.Sel.Name] {
			found = true
		}
		return !found
	})
	return found
}

func loopVar(x *ast.ForStmt) string {
	if a, ok := x.Init.(*ast.AssignStmt); ok && len(a.Lhs) == 1 {
		if id, ok := a.Lhs[0].(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func scanSeatArithmetic(t *testing.T) (map[string]int, map[string][]string) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	var fnames []string
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") || n == "seatorder.go" {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		fnames = append(fnames, n)
	}
	fields := seatFields(files)
	got := map[string]int{}
	sites := map[string][]string{}
	for i, f := range files {
		n := fnames[i]
		exempt := map[int]bool{}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(c.Text, "//")), "seat-ok:") {
					exempt[fset.Position(c.Pos()).Line] = true
				}
			}
		}
		add := func(fam string, pos token.Pos) {
			p := fset.Position(pos)
			if exempt[p.Line] {
				return
			}
			k := fam + "\t" + n
			got[k]++
			sites[k] = append(sites[k], fmt.Sprintf("dirac/%s:%d", n, p.Line))
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
				}
			case *ast.IndexExpr:
				if isAndOne(x.Index) {
					add("and1", x.Index.Pos())
				}
			case *ast.AssignStmt:
				if x.Tok == token.XOR_ASSIGN && len(x.Rhs) == 1 && isLitOne(x.Rhs[0]) {
					add("xor1", x.Pos())
				}
			case *ast.ForStmt:
				if c, ok := x.Cond.(*ast.BinaryExpr); ok &&
					((c.Op == token.LSS && isLitTwo(c.Y)) || (c.Op == token.LEQ && isLitOne(c.Y))) {
					if v := loopVar(x); v != "" && indexesSeatField(x.Body, v, fields) {
						add("seatloop2", x.Pos())
					}
				}
			case *ast.RangeStmt:
				if isLitTwo(x.X) {
					if id, ok := x.Key.(*ast.Ident); ok && indexesSeatField(x.Body, id.Name, fields) {
						add("seatloop2", x.Pos())
					}
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
