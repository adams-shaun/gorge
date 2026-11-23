package codeshape

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// LoweringPostMergeOnly is the lint step's refusal: a shrink-only ratchet moves down on main,
// where a generator lowers it against the merged tree, never in a pull request.
const LoweringPostMergeOnly = "lowering is post-merge only"

// LoweringBranch is the bot branch of the daily post-merge lowering (scripts/lower-ceilings.sh).
const LoweringBranch = "regen/ceilings"

// GuardExempt reports whether a pull request from headRef is exempt from the lowering guard: the
// daily bot's branch exists to lower ceilings, and a merge-queue branch carries that commit (and
// the other pull requests') on top of main. The caller passes an empty ref for a fork's branch,
// so a fork cannot claim the exemption by its name.
func GuardExempt(headRef string) bool {
	return headRef == LoweringBranch || strings.HasPrefix(headRef, "mergify/merge-queue/")
}

// Ceiling is one func_ceilings.tsv row: the line and complexity limits of one function.
type Ceiling struct {
	Lines      int
	Complexity int
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

// LoweredCeilings reports every func_ceilings.tsv row head lowers from base, or removes. A row
// renamed with no smaller value is the move invariant 8 allows, not a lowering.
func LoweredCeilings(base, head []byte) ([]string, error) {
	old, err := parseCeilings(base)
	if err != nil {
		return nil, err
	}
	now, err := parseCeilings(head)
	if err != nil {
		return nil, err
	}
	oldKeys := sortedCeilingKeys(old)
	// The keys head adds. A missing old row is a move when one of them keeps or raises both
	// values. One new ceiling may cover several removed keys: deleting a function's row and
	// renaming another at its own ceiling is the delete+rename invariant 8 allows, and the shape
	// ratchet still refuses an unlisted function that is over a threshold. Matching one-to-one
	// in key order instead reported the rename as a removal.
	var added []string
	for _, k := range sortedCeilingKeys(now) {
		if _, was := old[k]; !was {
			added = append(added, k)
		}
	}
	var lowered []string
	for _, k := range oldKeys {
		o := old[k]
		if n, ok := now[k]; ok {
			if n.Lines < o.Lines || n.Complexity < o.Complexity {
				lowered = append(lowered, fmt.Sprintf("%s: lowered from lines %d, complexity %d to lines %d, complexity %d",
					k, o.Lines, o.Complexity, n.Lines, n.Complexity))
			}
			continue
		}
		moved := false
		for _, nk := range added {
			n := now[nk]
			if n.Lines >= o.Lines && n.Complexity >= o.Complexity {
				moved = true
				break
			}
		}
		if !moved {
			lowered = append(lowered, fmt.Sprintf("%s: row removed (was lines %d, complexity %d)", k, o.Lines, o.Complexity))
		}
	}
	return lowered, nil
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

func parseCeilings(b []byte) (map[string]Ceiling, error) {
	rows := map[string]Ceiling{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("line %d: %d tab-separated fields, want 3 (key, lines, complexity)", n, len(f))
		}
		lines, err := strconv.Atoi(f[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: lines %q is not an integer", n, f[1])
		}
		complexity, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: complexity %q is not an integer", n, f[2])
		}
		if _, dup := rows[f[0]]; dup {
			return nil, fmt.Errorf("line %d: %q listed twice", n, f[0])
		}
		rows[f[0]] = Ceiling{Lines: lines, Complexity: complexity}
	}
	return rows, sc.Err()
}

func sortedCeilingKeys(rows map[string]Ceiling) []string {
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
