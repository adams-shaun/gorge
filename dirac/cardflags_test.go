package dirac

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestCardFlagsShareOneIotaBlock holds the per-card generated flags (the gf
// constants genPrep.flags carries) to the one `1 << iota` block in
// cardflags.go. A gf flag declared elsewhere with a literal bit took one
// already numbered there, so a card carrying one flag read as carrying the
// other: a spent-mana reader and a gated mana source shared bit 17, a
// counters-added reader and a fee-free filter bit 23.
func TestCardFlagsShareOneIotaBlock(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("no Go files: %v", err)
	}
	fset := token.NewFileSet()
	var block *ast.GenDecl
	inBlock := 0
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for _, id := range vs.Names {
					if !isCardFlagName(id.Name) {
						continue
					}
					if n != "cardflags.go" {
						t.Errorf("%s: card flag %s is declared outside cardflags.go's iota block", fset.Position(id.Pos()), id.Name)
						continue
					}
					if block == nil {
						block = gd
					}
					if gd != block {
						t.Errorf("%s: card flag %s is declared outside the first gf const block", fset.Position(id.Pos()), id.Name)
						continue
					}
					inBlock++
				}
			}
		}
	}
	if block == nil {
		t.Fatal("cardflags.go declares no gf flag")
	}
	for i, sp := range block.Specs {
		vs := sp.(*ast.ValueSpec)
		if len(vs.Names) != 1 || !isCardFlagName(vs.Names[0].Name) {
			t.Errorf("%s: the gf block holds only gf flags, one per line", fset.Position(vs.Pos()))
			continue
		}
		if i == 0 {
			if !isOneShlIota(vs) {
				t.Errorf("%s: %s must be `uint32 = 1 << iota`", fset.Position(vs.Pos()), vs.Names[0].Name)
			}
			continue
		}
		if vs.Type != nil || len(vs.Values) != 0 {
			t.Errorf("%s: %s must repeat the block's `1 << iota` (no type, no value)", fset.Position(vs.Pos()), vs.Names[0].Name)
		}
	}
	if inBlock > 32 {
		t.Errorf("%d gf flags do not fit genPrep.flags' uint32", inBlock)
	}
}

// isCardFlagName reports whether name is a per-card generated flag: gf and
// an upper-case letter.
func isCardFlagName(name string) bool {
	return len(name) > 2 && strings.HasPrefix(name, "gf") && name[2] >= 'A' && name[2] <= 'Z'
}

// isOneShlIota reports whether vs is `uint32 = 1 << iota`.
func isOneShlIota(vs *ast.ValueSpec) bool {
	typ, ok := vs.Type.(*ast.Ident)
	if !ok || typ.Name != "uint32" || len(vs.Values) != 1 {
		return false
	}
	b, ok := vs.Values[0].(*ast.BinaryExpr)
	if !ok || b.Op != token.SHL {
		return false
	}
	one, ok := b.X.(*ast.BasicLit)
	iota, ok2 := b.Y.(*ast.Ident)
	return ok && ok2 && one.Value == "1" && iota.Name == "iota"
}
