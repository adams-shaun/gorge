package dirac

import (
	"fmt"
	"go/ast"
	gobuild "go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestObjCodeCastsOnlyShrink is the shrink-only ratchet on object and target
// codes that pass through a narrower integer, or through Action's packing by
// hand, in the kernel: every non-test .go file of package dirac that the
// default size class builds (this directory only; subpackages and tests
// excluded). It type-checks the package with go/types and counts, per family:
//
//   - narrow: a conversion of a non-constant Obj or TCode to any other
//     one-byte integer type (uint8(o), byte(a.Target()), Slot(c)). When the
//     multiplayer class widens Obj and TCode, each of these truncates a code
//     without a compile error. Keep the code in an Obj or TCode instead.
//   - launder: a conversion to Obj or TCode from a non-constant one-byte
//     integer that is neither (TCode(t) for t uint8, Obj(e.tgt[i])). The
//     operand is a code held in a byte: retype the field, variable or
//     parameter that holds it.
//   - actbits: Action's packing read or written outside its accessors in
//     types.go (MakeAction, Kind, Source, Target, Mode): a shift or bitwise
//     operator with an Action operand, a conversion of an Action to another
//     type, or a conversion of a non-constant to Action. The multiplayer
//     class packs Action differently; go through the accessors.
//
// The scan cannot see an int that holds an object index narrowed to uint8
// (uint8(i) for a loop index): such a value is no Obj until it is converted.
// Store codes in Obj or TCode fields so that the narrowing fails to compile.
//
// A site that is genuinely not a code (a seat decoded from a player code, a
// byte that a format fixes at eight bits) is exempted by a trailing
// "// code-ok: <reason>" comment on its line.
//
// The counts are pinned per family and file in testdata/objcodecast.tsv. The
// test fails when a count rises (naming that file's sites) and when it falls
// without the pin being lowered; DIRAC_CODE_REPIN=1 rewrites the pin to the
// measurement. Never raise a pin by hand.
func TestObjCodeCastsOnlyShrink(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the package from source")
	}
	got, sites := scanObjCodeCasts(t)
	pinPath := filepath.Join("testdata", "objcodecast.tsv")
	if os.Getenv("DIRAC_CODE_REPIN") == "1" {
		var b strings.Builder
		b.WriteString("# family\tfile\tcount: object codes narrowed, laundered or packed by hand in package dirac; shrink only (TestObjCodeCastsOnlyShrink)\n")
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
	const repin = "DIRAC_CODE_REPIN=1 go test -run TestObjCodeCastsOnlyShrink ./dirac"
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
			t.Errorf("%s: %d sites, pin %d. %s; a site that is genuinely not a code takes a trailing \"// code-ok: <reason>\" comment.%s Sites:\n  %s",
				where, g, p, codeFixFor[fam], moved, strings.Join(sites[k], "\n  "))
		case g < p:
			t.Errorf("%s: %d sites, pin %d: lower the pin, from the repo root: %s", where, g, p, repin+strings.Replace(moved, " if you only moved", " if you also moved", 1))
		}
	}
	t.Logf("narrow %d, launder %d, actbits %d", famGot["narrow"], famGot["launder"], famGot["actbits"])
	if os.Getenv("DIRAC_CODE_LIST") == "1" {
		for _, k := range sortedSeatKeys(got) {
			for _, s := range sites[k] {
				t.Logf("%s\t%s", seatFam(k), s)
			}
		}
	}
}

var codeFixFor = map[string]string{
	"narrow":  "Keep the code in an Obj or TCode (TCode(o), Obj(a.Source()))",
	"launder": "Retype the byte that holds the code as Obj or TCode",
	"actbits": "Use MakeAction and Action's Kind, Source, Target and Mode",
}

// actionAccessors are the functions of types.go that own Action's packing.
var actionAccessors = map[string]bool{"MakeAction": true, "Kind": true, "Source": true, "Target": true, "Mode": true}

// codeSourceImporter type-checks imports from source. A -trimpath test binary
// knows no GOROOT unless its environment gives it one, and go/build can't find
// a package without it.
func codeSourceImporter(t *testing.T, fset *token.FileSet) types.Importer {
	t.Helper()
	if gobuild.Default.GOROOT == "" {
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			t.Fatal(err)
		}
		gobuild.Default.GOROOT = strings.TrimSpace(string(out))
	}
	return importer.ForCompiler(fset, "source", nil)
}

func scanObjCodeCasts(t *testing.T) (map[string]int, map[string][]string) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	ctx := gobuild.Default // the default size class: no dirac_mp tag
	fset := token.NewFileSet()
	var files []*ast.File
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		if ok, err := ctx.MatchFile(".", n); err != nil {
			t.Fatal(err)
		} else if !ok {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: codeSourceImporter(t, fset)}
	pkg, err := conf.Check("github.com/adams-shaun/gorge/dirac", fset, files, info)
	if err != nil {
		t.Fatal(err)
	}
	named := func(ty types.Type, name string) bool {
		n, ok := types.Unalias(ty).(*types.Named)
		return ok && n.Obj().Pkg() == pkg && n.Obj().Name() == name
	}
	isCode := func(ty types.Type) bool { return named(ty, "Obj") || named(ty, "TCode") }
	isByte := func(ty types.Type) bool {
		b, ok := ty.Underlying().(*types.Basic)
		return ok && (b.Kind() == types.Uint8 || b.Kind() == types.Int8)
	}
	isAction := func(ty types.Type) bool { return ty != nil && named(ty, "Action") }

	got := map[string]int{}
	sites := map[string][]string{}
	for _, f := range files {
		file := fset.Position(f.Pos()).Filename
		exempt := map[int]bool{}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(c.Text, "//")), "code-ok:") {
					exempt[fset.Position(c.Pos()).Line] = true
				}
			}
		}
		add := func(fam string, n ast.Node) {
			p := fset.Position(n.Pos())
			if exempt[p.Line] {
				return
			}
			k := fam + "\t" + file
			got[k]++
			sites[k] = append(sites[k], fmt.Sprintf("%s:%d: %s", file, p.Line, types.ExprString(n.(ast.Expr))))
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && file == "types.go" && actionAccessors[fd.Name.Name] {
				continue
			}
			ast.Inspect(d, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					fun, ok := info.Types[x.Fun]
					if !ok || !fun.IsType() || len(x.Args) != 1 {
						return true
					}
					arg := info.Types[x.Args[0]]
					to := fun.Type
					if arg.Value != nil || arg.Type == nil {
						return true // a constant: a width change overflows it at compile time
					}
					switch {
					case isCode(arg.Type) && isByte(to) && !isCode(to):
						add("narrow", x)
					case isCode(to) && isByte(arg.Type) && !isCode(arg.Type):
						add("launder", x)
					case isAction(arg.Type) != isAction(to):
						add("actbits", x)
					}
				case *ast.BinaryExpr:
					switch x.Op {
					case token.SHL, token.SHR, token.AND, token.OR, token.XOR, token.AND_NOT:
						if isAction(info.Types[x.X].Type) || isAction(info.Types[x.Y].Type) {
							add("actbits", x)
						}
					}
				}
				return true
			})
		}
	}
	for k := range sites {
		sort.Strings(sites[k])
	}
	return got, sites
}
