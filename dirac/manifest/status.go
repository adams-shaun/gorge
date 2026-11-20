package manifest

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// StatusMismatches holds t to invariant 2 against the kernel in fsys (a tree
// rooted at the module): an entry is active exactly when package dirac's
// non-test, non-generated source references its code. A kop entry is
// referenced as kop.<Name>, a dirac entry (key, mech) by its bare name, and
// an ExtOp or Op also by a //kmanifest:ext or :op handler, a FilterWord by its
// :fword handler and an ask by its :askopts handler (ScanKernel). It returns one line
// per mismatch, naming the entry's file, sorted.
func StatusMismatches(fsys fs.FS, t *Tree) ([]error, error) {
	used, err := kernelRefs(fsys)
	if err != nil {
		return nil, err
	}
	hs, err := ScanKernel(fsys)
	if err != nil {
		return nil, err
	}
	for _, h := range hs.Ext {
		used["kop."+h.Op] = true
	}
	for _, h := range hs.Op {
		used["kop."+h.Op] = true
	}
	for _, h := range hs.Ask {
		used[h.Ask] = true
	}
	for _, h := range hs.Word {
		used["kop."+h.Word] = true
	}
	var errs []error
	for _, e := range t.Entries {
		ref := e.Name
		if e.Fam().Pkg == "kop" {
			ref = "kop." + e.Name
		}
		switch active := e.Status.Kind == Active; {
		case active && !used[ref]:
			errs = append(errs, fmt.Errorf("%s: status active, but package %s references no %s: implement it, or keep the entry %s", e.Path, KernelDir, ref, Reserved))
		case !active && used[ref]:
			errs = append(errs, fmt.Errorf("%s: package %s references %s, but its status is %s: set status: active", e.Path, KernelDir, ref, e.Status))
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs, nil
}

// kernelRefs is every kop.<Name> selector and bare identifier package dirac's
// non-test source outside DiracGen uses, keyed "kop.<Name>" and "<Name>".
func kernelRefs(fsys fs.FS) (map[string]bool, error) {
	des, err := fs.ReadDir(fsys, KernelDir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	fset := token.NewFileSet()
	for _, de := range des {
		n := de.Name()
		p := path.Join(KernelDir, n)
		if de.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") || p == DiracGen {
			continue
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(fset, p, b, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(nd ast.Node) bool {
			switch x := nd.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "kop" {
					used["kop."+x.Sel.Name] = true
					return false
				}
			case *ast.Ident:
				used[x.Name] = true
			}
			return true
		})
	}
	return used, nil
}
