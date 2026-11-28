package irmeter

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The kgen-filter section: dirac/kgen's filter refusal sites split by
// reader. A refusal site is a call to refuse, fmt.Errorf or errors.New. The
// typed reader is every function (and package var) declared in
// filter_ir*.go, less its fallback sentinels: a typed-file var the typed
// files test with errors.Is(err, v) marks a fallback, not a refusal; the string reader is what parseFilter, parseClause,
// historicClauses and (*lClause).prop reach. Reach follows the
// package-local call graph by name (a selector resolves to every method of
// that name, an identifier to the function or package var of that name);
// the typed reader's reach stops at the string reader's roots, its fallback.
const kgenFilterSection = "kgen-filter"

// kgenFilterStringRoots are the string reader's entry points ("Recv.Name"
// for a method).
var kgenFilterStringRoots = []string{"parseFilter", "parseClause", "historicClauses", "lClause.prop"}

// KgenFilter is the kgen-filter section's measurement.
type KgenFilter struct {
	TypedOnly, StringOnly, Shared int
}

// Rows are the section's three counts, metric kgen_filter_refuse_<part>.
func (k KgenFilter) Rows() []Row {
	return []Row{
		{"kgen_filter_refuse_shared", kgenScope, k.Shared},
		{"kgen_filter_refuse_string_only", kgenScope, k.StringOnly},
		{"kgen_filter_refuse_typed_only", kgenScope, k.TypedOnly},
	}
}

// DropPct is 100 * (1 - typed_only / string_only), the share of the string
// reader's own refusal sites the typed reader does without; 0 with no
// string-only site.
func (k KgenFilter) DropPct() float64 {
	if k.StringOnly == 0 {
		return 0
	}
	return 100 * (1 - float64(k.TypedOnly)/float64(k.StringOnly))
}

// DropRow is the drop percentage as a printed row, one decimal.
func (k KgenFilter) DropRow() string {
	return fmt.Sprintf("kgen_filter_refuse_drop_pct\t%s\t%.1f", kgenScope, k.DropPct())
}

// Sites is every filter refusal site, whichever reader reaches it.
func (k KgenFilter) Sites() int { return k.Shared + k.StringOnly + k.TypedOnly }

// SiteSharePct is 100 * Sites / refuseSites, the filter sites against kgen's
// refuse_sites count; 0 with no refuse site. refuse_sites counts only refuse
// calls and Sites also counts fmt.Errorf and errors.New, so this compares two
// sizes: it is not the share of a set the filter sites belong to.
func (k KgenFilter) SiteSharePct(refuseSites int) float64 {
	if refuseSites == 0 {
		return 0
	}
	return 100 * float64(k.Sites()) / float64(refuseSites)
}

// RemovedByTyped is the refusal sites kgen would lose with the string reader
// deleted, string_only - typed_only; never negative.
func (k KgenFilter) RemovedByTyped() int { return max(k.StringOnly-k.TypedOnly, 0) }

// siteRows are the printed rows that follow the drop row, the share one
// decimal.
func siteRows(k KgenFilter, refuseSites int) []string {
	return []string{
		fmt.Sprintf("kgen_filter_refuse_sites\t%s\t%d", kgenScope, k.Sites()),
		fmt.Sprintf("kgen_filter_refuse_site_share_pct\t%s\t%.1f", kgenScope, k.SiteSharePct(refuseSites)),
		fmt.Sprintf("kgen_refuse_sites_removed_by_typed\t%s\t%d", kgenScope, k.RemovedByTyped()),
	}
}

// fnode is one function or package var of the package: its refusal sites
// and the package-local names it references.
type fnode struct {
	sites    int
	typed    bool // declared in filter_ir*.go
	isVar    bool // a package var, not a function
	sentinel bool // a typed-file var the typed files match with errors.Is
	refs     []string
}

// MeasureKgenFilter measures the kgen-filter section over root's dirac/kgen
// (non-test files).
func MeasureKgenFilter(root string) (KgenFilter, error) {
	nodes, err := kgenNodes(filepath.Join(root, filepath.FromSlash(kgenScope)))
	if err != nil {
		return KgenFilter{}, err
	}
	stop := map[string]bool{}
	for _, r := range kgenFilterStringRoots {
		stop[r] = true
	}
	var typedRoots []string
	for k, n := range nodes {
		if n.typed {
			typedRoots = append(typedRoots, k)
		}
	}
	sort.Strings(typedRoots)
	typed := reach(nodes, typedRoots, stop)
	str := reach(nodes, kgenFilterStringRoots, nil)
	var k KgenFilter
	for key, n := range nodes {
		switch {
		case n.typed && n.sentinel:
		case n.typed:
			k.TypedOnly += n.sites
		case str[key] && typed[key]:
			k.Shared += n.sites
		case str[key]:
			k.StringOnly += n.sites
		}
	}
	return k, nil
}

// reach is every node the roots reach, not entering stop.
func reach(nodes map[string]*fnode, roots []string, stop map[string]bool) map[string]bool {
	seen := map[string]bool{}
	todo := append([]string(nil), roots...)
	for len(todo) > 0 {
		k := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[k] || nodes[k] == nil {
			continue
		}
		seen[k] = true
		for _, r := range nodes[k].refs {
			for _, t := range resolve(nodes, r) {
				if !seen[t] && !stop[t] {
					todo = append(todo, t)
				}
			}
		}
	}
	return seen
}

// resolve maps a reference to its nodes: "." + name is every method of that
// name, a bare name the function or var of that name.
func resolve(nodes map[string]*fnode, ref string) []string {
	name, isSel := strings.CutPrefix(ref, ".")
	var out []string
	if _, ok := nodes[name]; ok {
		out = append(out, name)
	}
	if isSel {
		for k := range nodes {
			if _, m, ok := strings.Cut(k, "."); ok && m == name {
				out = append(out, k)
			}
		}
		sort.Strings(out)
	}
	return out
}

// kgenNodes parses dir's non-test files into nodes keyed "Name",
// "Recv.Name", or "init@file" (each init is its own node).
func kgenNodes(dir string) (map[string]*fnode, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	nodes := map[string]*fnode{}
	sentinels := map[string]bool{}
	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		typed := base == "filter_ir.go" || strings.HasPrefix(base, "filter_ir_")
		imports := importNames(f)
		for _, d := range f.Decls {
			addDecl(nodes, d, base, typed, imports)
		}
		if typed {
			isTargets(f, sentinels)
		}
	}
	for name := range sentinels {
		if n := nodes[name]; n != nil && n.typed && n.isVar {
			n.sentinel = true
		}
	}
	return nodes, nil
}

// isTargets adds to out every identifier f passes as errors.Is's target.
func isTargets(f *ast.File, out map[string]bool) {
	ast.Inspect(f, func(x ast.Node) bool {
		c, ok := x.(*ast.CallExpr)
		if !ok || len(c.Args) != 2 {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Is" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "errors" {
			return true
		}
		if id, ok := c.Args[1].(*ast.Ident); ok {
			out[id.Name] = true
		}
		return true
	})
}

func addDecl(nodes map[string]*fnode, d ast.Decl, file string, typed bool, imports map[string]bool) {
	add := func(key string, body ast.Node) {
		n := nodes[key]
		if n == nil {
			n = &fnode{typed: typed}
			nodes[key] = n
		}
		n.typed = n.typed || typed
		if body != nil {
			scan(n, body, imports)
		}
	}
	switch x := d.(type) {
	case *ast.FuncDecl:
		key := x.Name.Name
		if x.Recv != nil && len(x.Recv.List) == 1 {
			key = recvName(x.Recv.List[0].Type) + "." + key
		} else if key == "init" {
			key = "init@" + file
		}
		if x.Body != nil {
			add(key, x.Body)
		} else {
			add(key, nil)
		}
	case *ast.GenDecl:
		if x.Tok != token.VAR {
			return
		}
		for _, s := range x.Specs {
			vs := s.(*ast.ValueSpec)
			for i, id := range vs.Names {
				if i < len(vs.Values) {
					add(id.Name, vs.Values[i])
				} else {
					add(id.Name, nil)
				}
				nodes[id.Name].isVar = true
			}
		}
	}
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

// scan counts body's refusal sites and records the names it references; a
// selector on an imported package references nothing local.
func scan(n *fnode, body ast.Node, imports map[string]bool) {
	var visit func(x ast.Node) bool
	visit = func(x ast.Node) bool {
		switch e := x.(type) {
		case *ast.CallExpr:
			if refusalCall(e.Fun) {
				n.sites++
			}
		case *ast.SelectorExpr:
			if id, ok := e.X.(*ast.Ident); ok && imports[id.Name] {
				return false
			}
			n.refs = append(n.refs, "."+e.Sel.Name)
			ast.Inspect(e.X, visit)
			return false
		case *ast.Ident:
			n.refs = append(n.refs, e.Name)
		}
		return true
	}
	ast.Inspect(body, visit)
}

// importNames are the names f's imports bind.
func importNames(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, im := range f.Imports {
		p := strings.Trim(im.Path.Value, `"`)
		name := p[strings.LastIndexByte(p, '/')+1:]
		if im.Name != nil {
			name = im.Name.Name
		}
		out[name] = true
	}
	return out
}

// refusalCall reports a call to refuse, fmt.Errorf or errors.New.
func refusalCall(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name == "refuse"
	case *ast.SelectorExpr:
		id, ok := f.X.(*ast.Ident)
		return ok && (id.Name == "fmt" && f.Sel.Name == "Errorf" || id.Name == "errors" && f.Sel.Name == "New")
	}
	return false
}
