package codeshape

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// LoweringPostMergeOnly is the lint step's refusal: a shrink-only ratchet moves down on main,
// where a follow-up change lowers it against the merged tree, never in a pull request.
const LoweringPostMergeOnly = "lowering is post-merge only"

// GuardExempt reports whether a pull request from headRef is exempt from the lowering guard:
// a merge-queue branch carries the batch's commits (whose ratchets main has already lowered
// against) on top of main. The caller passes an empty ref for a fork's branch, so a fork
// cannot claim the exemption by its name.
func GuardExempt(headRef string) bool {
	return strings.HasPrefix(headRef, "mergify/merge-queue/")
}

// LoweringReport formats the rows a diff lowered. It is empty when nothing lowered.
func LoweringReport(lowered []string) string {
	if len(lowered) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(LoweringPostMergeOnly + ":\n")
	for _, l := range lowered {
		b.WriteString("  " + l + "\n")
	}
	return b.String()
}

// LoweredConstants reports every integer constant head lowers or removes from a ratchet_test.go
// file. A raised or new constant is not a lowering; the ratchet itself refuses a raise.
func LoweredConstants(base, head []byte) ([]string, error) {
	old, err := parseIntConstants(base)
	if err != nil {
		return nil, err
	}
	now, err := parseIntConstants(head)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(old))
	for name := range old {
		names = append(names, name)
	}
	sort.Strings(names)
	var lowered []string
	for _, name := range names {
		v := old[name]
		n, ok := now[name]
		switch {
		case !ok:
			lowered = append(lowered, fmt.Sprintf("%s: constant removed (was %d)", name, v))
		case n < v:
			lowered = append(lowered, fmt.Sprintf("%s: lowered from %d to %d", name, v, n))
		}
	}
	return lowered, nil
}

func parseIntConstants(b []byte) (map[string]int64, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "ratchet_test.go", b, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					continue
				}
				v, err := strconv.ParseInt(lit.Value, 0, 64)
				if err != nil {
					continue
				}
				out[name.Name] = v
			}
		}
	}
	return out, nil
}
