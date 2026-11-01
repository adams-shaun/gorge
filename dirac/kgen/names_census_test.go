package kgen

import (
	"go/build"
	"go/constant"
	"go/importer"
	"go/token"
	"go/types"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sourceImporter type-checks from source. A -trimpath test binary knows no GOROOT unless its
// environment gives it one, and go/build can't find a package without it.
func sourceImporter(t *testing.T, fset *token.FileSet) types.Importer {
	t.Helper()
	if build.Default.GOROOT == "" {
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			t.Fatal(err)
		}
		build.Default.GOROOT = strings.TrimSpace(string(out))
	}
	return importer.ForCompiler(fset, "source", nil)
}

// TestEveryEnumConstHasName is the class fix for the kgen enum panic (Heron
// of Hope / Boon Reflection named ReplLifeDouble/ReplLifePlus): every
// exported constant of every enum type registered in enumNames must have a
// symbolic name. Sentinels (the one-past-the-end counters and the reserved
// *_ markers) are named by no generated literal, so they are allowed to be
// unnamed. Mask types degrade to a hex literal (printer.go), so they are out
// of scope.
func TestEveryEnumConstHasName(t *testing.T) {
	const kopPath = "github.com/adams-shaun/gorge/dirac/kop"
	fset := token.NewFileSet()
	pkg, err := sourceImporter(t, fset).Import(kopPath)
	if err != nil {
		t.Fatal(err)
	}
	sentinels := map[string]bool{
		"TrigEventCount": true, "StatCount": true, "ReplCount": true,
		"ExCount_": true, "CondCount_": true, "NumCounterKinds2": true,
		"NumCounterKinds3": true, "NumSteps": true, "OpCount": true, "OpHighEnd": true,
	}
	scope := pkg.Scope()
	for rt := range enumNames {
		if rt.PkgPath() != kopPath {
			continue
		}
		obj := scope.Lookup(rt.Name())
		if obj == nil {
			t.Errorf("enum type %s not found in %s", rt.Name(), kopPath)
			continue
		}
		named := obj.Type()
		have := map[uint64]bool{}
		for _, n := range enumNames[rt] {
			have[n.v] = true
		}
		for _, name := range scope.Names() {
			c, ok := scope.Lookup(name).(*types.Const)
			if !ok || !types.Identical(c.Type(), named) || sentinels[name] {
				continue
			}
			v, ok := constant.Uint64Val(constant.ToInt(c.Val()))
			if !ok {
				t.Errorf("%s.%s is not an unsigned integer constant", rt.Name(), name)
				continue
			}
			if !have[v] {
				t.Errorf("%s.%s = %d has no name in dirac/kgen/names.go", rt.Name(), name, v)
			}
		}
	}
}

// TestGenerateLifeReplacementPool is the reproduction: a pool holding
// Heron of Hope and Boon Reflection (kop.ReplLifePlus / kop.ReplLifeDouble)
// generates without the writeLit panic.
func TestGenerateLifeReplacementPool(t *testing.T) {
	_, cardsDir := testInputs(t)
	p, err := loadCardNames([]string{"Boon Reflection", "Cerebral Download", "Heron of Hope"}, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if _, err := generate(p, "zzheron", out, filepath.Join(out, idsFile)); err != nil {
		t.Fatal(err)
	}
}
