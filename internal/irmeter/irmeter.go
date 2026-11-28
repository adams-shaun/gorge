// Package irmeter measures how much of the code still reads Forge-spelled
// strings: the kernel generator's refusal sites, the engine's
// string-returning parameter reads by operand family, strings.* calls and
// Forge-spelling literals per package. It is a report (cmd/irmeter prints
// it), the baseline the typed-interface work is judged against; it holds no
// ceilings (cmd/countgrowth owns growth rules).
//
// Only source is read, with go/parser and no type checking: no corpus, no
// build. The rows are sorted, so two runs on one tree print identical bytes.
package irmeter

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Row is one measurement: a metric over a scope.
type Row struct {
	Metric, Scope string
	Value         int
}

// Scopes are the repo-relative trees measured, each walked recursively.
var Scopes = []string{"engine/effects", "engine/rules", "engine/cards", "engine/events", "dirac/kgen"}

const kgenScope = "dirac/kgen"

// families are the operand families param_string_reads is split by, in
// output-independent declaration order.
var families = []string{"filter", "reference", "cost", "condition", "value", "other"}

// filterKeys are the filter-family keys that do not start with "Valid".
var filterKeys = map[string]bool{
	"Affected": true, "ChangeType": true, "ConditionPresent": true, "ConditionPresent2": true,
	"IsPresent": true, "IsPresent2": true, "Valid": true, "ValidCard": true, "ValidCards": true,
	"ValidCause": true, "ValidSource": true, "ValidTarget": true, "ValidTgts": true,
}

var valueKeys = map[string]bool{
	"Amount": true, "LifeAmount": true, "CounterNum": true, "ChangeNum": true, "DigNum": true,
}

// family names the operand family of a parameter key, first match wins.
func family(key string) string {
	switch {
	case filterKeys[key] || strings.HasPrefix(key, "Valid"):
		return "filter"
	case strings.HasPrefix(key, "Defined"):
		return "reference"
	case key == "Cost" || key == "UnlessCost":
		return "cost"
	case hasAnyPrefix(key, "Condition", "CheckSVar", "SVarCompare", "PresentCompare"):
		return "condition"
	case strings.HasPrefix(key, "Num") || valueKeys[key]:
		return "value"
	}
	return "other"
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

var forgeLiteral = regexp.MustCompile(`^[A-Z][A-Za-z0-9_.+$ ]+$`)

// counts is one scope's measurement.
type counts struct {
	files, lines, stringsCalls       int
	refuse, refuseAll, refuseFiles   int
	paramString, paramCode, paramRaw int
	byFamily                         map[string]int
	literals                         map[string]bool
}

// Measure walks root (the module root) and returns the rows, sorted by
// metric then scope. A scope directory that does not exist measures zero.
func Measure(root string) ([]Row, error) {
	var rows []Row
	for _, scope := range Scopes {
		c, err := measureScope(root, scope)
		if err != nil {
			return nil, err
		}
		rows = append(rows, c.rows(scope)...)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Metric != rows[j].Metric {
			return rows[i].Metric < rows[j].Metric
		}
		return rows[i].Scope < rows[j].Scope
	})
	return rows, nil
}

// Print writes the report, one metric<TAB>scope<TAB>value line each: rows,
// then the kgen-filter section (its counts, its drop row and its site rows,
// the share read against rows' refuse_sites count).
func Print(w io.Writer, rows []Row, kf KgenFilter) error {
	b := bufio.NewWriter(w)
	line := func(r Row) { fmt.Fprintf(b, "%s\t%s\t%d\n", r.Metric, r.Scope, r.Value) }
	refuseSites := 0
	for _, r := range rows {
		if r.Metric == "refuse_sites" && r.Scope == kgenScope {
			refuseSites = r.Value
		}
		line(r)
	}
	for _, r := range kf.Rows() {
		line(r)
	}
	fmt.Fprintln(b, kf.DropRow())
	for _, s := range siteRows(kf, refuseSites) {
		fmt.Fprintln(b, s)
	}
	return b.Flush()
}

func (c *counts) rows(scope string) []Row {
	rows := []Row{
		{"go_files", scope, c.files},
		{"go_lines", scope, c.lines},
		{"strings_calls", scope, c.stringsCalls},
		{"forge_literals", scope, len(c.literals)},
	}
	if scope == kgenScope {
		return append(rows,
			Row{"refuse_sites", scope, c.refuse},
			Row{"refuse_sites_all", scope, c.refuseAll},
			Row{"refuse_files", scope, c.refuseFiles})
	}
	rows = append(rows,
		Row{"param_string_reads", scope, c.paramString},
		Row{"param_code_reads", scope, c.paramCode},
		Row{"param_raw_reads", scope, c.paramRaw})
	for _, f := range families {
		rows = append(rows, Row{"param_string_reads_" + f, scope, c.byFamily[f]})
	}
	return rows
}

func measureScope(root, scope string) (*counts, error) {
	c := &counts{byFamily: map[string]int{}, literals: map[string]bool{}}
	dir := filepath.Join(root, filepath.FromSlash(scope))
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != dir && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		return c.file(p, scope == "engine/cards")
	})
	return c, err
}

// file counts one non-test Go file.
func (c *counts) file(path string, inCards bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	c.files++
	c.lines += bytes.Count(src, []byte("\n"))
	if len(src) > 0 && src[len(src)-1] != '\n' {
		c.lines++
	}
	stringsName := ""
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == "strings" {
			stringsName = "strings"
			if im.Name != nil {
				stringsName = im.Name.Name
			}
		}
	}
	refused := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if v, err := strconv.Unquote(x.Value); err == nil && forgeLiteral.MatchString(v) {
					c.literals[v] = true
				}
			}
		case *ast.IndexExpr:
			if se, ok := x.X.(*ast.SelectorExpr); ok && se.Sel.Name == "Params" {
				if lit, ok := x.Index.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					c.paramRaw++
				}
			}
		case *ast.CallExpr:
			if c.call(x, stringsName, inCards) {
				refused = true
			}
		}
		return true
	})
	if refused {
		c.refuseFiles++
	}
	return nil
}

// call counts one call expression and reports whether it is a refuse call.
func (c *counts) call(x *ast.CallExpr, stringsName string, inCards bool) bool {
	switch fn := x.Fun.(type) {
	case *ast.Ident:
		if strings.HasPrefix(fn.Name, "refus") {
			c.refuseAll++
		}
		if fn.Name == "refuse" {
			c.refuse++
			return true
		}
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok && stringsName != "" && id.Name == stringsName {
			c.stringsCalls++
			return false
		}
		if len(x.Args) == 0 {
			return false
		}
		key, ok := paramKey(x.Args[0], inCards)
		if !ok {
			return false
		}
		switch fn.Sel.Name {
		case "Param", "ParamStr":
			c.paramString++
			c.byFamily[family(key)]++
		case "ParamCode":
			c.paramCode++
		}
	}
	return false
}

// paramKey returns the key name of a cards.PK<Name> argument (inside
// engine/cards, the bare PK<Name>).
func paramKey(arg ast.Expr, inCards bool) (string, bool) {
	var id string
	switch a := arg.(type) {
	case *ast.SelectorExpr:
		pkg, ok := a.X.(*ast.Ident)
		if !ok || pkg.Name != "cards" {
			return "", false
		}
		id = a.Sel.Name
	case *ast.Ident:
		if !inCards {
			return "", false
		}
		id = a.Name
	default:
		return "", false
	}
	if len(id) <= 2 || !strings.HasPrefix(id, "PK") {
		return "", false
	}
	return id[2:], true
}
