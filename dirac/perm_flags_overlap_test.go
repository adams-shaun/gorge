package dirac

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
	"unicode"
)

// permFlagMasks is every bit mask of perm.flags (uint16). A multi-bit mask
// belongs here too: its bits are as taken as a single flag's.
var permFlagMasks = map[string]uint16{
	"pfTapped": pfTapped, "pfSick": pfSick, "pfAttacking": pfAttacking, "pfBlocking": pfBlocking,
	"pfBlocked": pfBlocked, "pfDeathtouched": pfDeathtouched, "pfRegen": pfRegen,
	"pfDealtFirst": pfDealtFirst, "pfCtrlEOT": pfCtrlEOT, "pfAnimEOT": pfAnimEOT,
	"pfLoyaltyUsed": pfLoyaltyUsed, "pfOnceGame": pfOnceGame, "pfDtMore": pfDtMore,
	"pfBatchGone": pfBatchGone, "pfLastBear": pfLastBear, "pfSkipUntap": pfSkipUntap,
}

// permSpareMasks is every bit mask of perm.spare (uint8).
var permSpareMasks = map[string]uint8{
	"spareSwitchPT": spareSwitchPT, "spareCopyFaceDown": spareCopyFaceDown, "spareRegenX": spareRegenX,
}

// TestPermSpareBitsDoNotOverlap: no two perm.spare constants share a bit.
func TestPermSpareBitsDoNotOverlap(t *testing.T) {
	var seen uint8
	for _, n := range []string{"spareCopyFaceDown", "spareRegenX", "spareSwitchPT"} {
		if both := seen & permSpareMasks[n]; both != 0 {
			t.Errorf("%s shares bits %#02x with an earlier perm.spare constant", n, both)
		}
		seen |= permSpareMasks[n]
	}
}

// TestPermFlagsDoNotOverlap: no two perm.flags constants share a bit (pfRegenX
// once held bits 13-14, which pfBatchGone and pfLastBear own), and every
// uint16 pf* constant declared in the package is in the table above.
func TestPermFlagsDoNotOverlap(t *testing.T) {
	names := make([]string, 0, len(permFlagMasks))
	for n := range permFlagMasks {
		names = append(names, n)
	}
	for i := range names { // sorted, so a failure reads the same every run
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for i, a := range names {
		if permFlagMasks[a] == 0 {
			t.Errorf("%s is the empty mask", a)
		}
		for _, b := range names[i+1:] {
			if both := permFlagMasks[a] & permFlagMasks[b]; both != 0 {
				t.Errorf("%s and %s share bits %#04x", a, b, both)
			}
		}
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				typed := false
				for i, sp := range gd.Specs {
					vs := sp.(*ast.ValueSpec)
					if i == 0 || vs.Type != nil {
						id, ok := vs.Type.(*ast.Ident)
						typed = ok && id.Name == "uint16"
					}
					if !typed {
						continue
					}
					for _, id := range vs.Names {
						n := id.Name
						if len(n) > 2 && n[:2] == "pf" && unicode.IsUpper(rune(n[2])) {
							found++
							if _, ok := permFlagMasks[n]; !ok {
								t.Errorf("uint16 constant %s is not in permFlagMasks: add it so its bits are checked", n)
							}
						}
					}
				}
			}
		}
	}
	if found < len(permFlagMasks) {
		t.Errorf("the scan found %d pf constants, the table lists %d", found, len(permFlagMasks))
	}
}
