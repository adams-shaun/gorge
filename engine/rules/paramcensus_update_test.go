package rules

// The known-key tables' regenerate path (task cli-20261014T200058Z-8bfb5843):
// rules' Test<Api>KnownKeysMatchTheCensus family holds each effects known-key
// table (the sorted `[...]string` var every <api> compiler binary-searches for
// its unread report) equal to the parameter census's measured read set. The
// census half is DERIVED from the code; the table half is a derived listing
// too, so a mismatch is bookkeeping, not a finding -- this file gives the
// family one regenerate command,
//
//	go test ./engine/rules -run 'KnownKeysMatchTheCensus' -update
//
// which rewrites each table var in place (engine/effects/*.go and the leaf package
// engine/effects/params/*.go) from the same measurement the check asserts, and every
// failure message in the family names that command. The rewrite is a MINIMAL
// token splice inside the var's braces -- surviving keys keep their bytes, so
// the table's wrapping and blank-line grouping survive and a one-key census
// drift is a one-token diff; the table's doc comment is never touched -- and
// it is a no-op on a table that already matches, so the command is idempotent
// across the whole family run (including GenericChoice, which checks the
// shared charmKnownKeys var a second time).
//
// The class-A ratchets are untouched: knownUnsupportedParams,
// knownUnsupported, the heads and every OnlyShrinks constant still need a
// human decision, and no flag here writes anywhere but the known-key tables.

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// updateKnownKeys rewrites the known-key tables from the measured census.
var updateKnownKeys = flag.Bool("update", false, "rewrite the effects known-key tables from the measured census (the Test*KnownKeysMatchTheCensus family); rerun without the flag to verify")

// knownKeysCensusWant is the census-derived want list
// checkKnownKeysMatchTheCensus holds each table to: the API's measured read
// set (minus the key the generic activation-cost tier owns) plus the ignored
// and structural keys, sorted.
func knownKeysCensusWant(api string, d *derivedReads) []string {
	want := map[string]bool{}
	for k := range d.api[api] {
		// This key is read by the generic activation-cost tier, not the
		// resolving API's compiler. Its unread suppression is conditional
		// on an AB carrying a valid typed permission.
		if k != cards.PKTapCreaturesForMana.String() {
			want[k] = true
		}
	}
	for k := range ignoredParamKeys {
		want[k] = true
	}
	// A mode body's Pawprint$ is read by its parent Charm (modePawprint), not
	// by the mode API's own compiler, so the census scopes the key to exactly
	// those APIs (ignoredAbilityParams); their tables must carry it too.
	if slices.Contains(pawprintModeAPIs, api) {
		want["Pawprint"] = true
	}
	for k := range structuralKeys["sa"] {
		want[k] = true
	}
	wantList := make([]string, 0, len(want))
	for k := range want {
		wantList = append(wantList, k)
	}
	sort.Strings(wantList)
	return wantList
}

// censusRun is the state one -update run carries across the family's checks:
// each check call sees the table's PRE-rewrite content (effects.<Api>KnownKeys
// was evaluated before any sibling rewrote the shared file), so rewrites are
// deduped by the content they were asked to fix.
var knownKeysRun censusRun

type censusRun struct {
	// done maps a table's current joined content to the want it was rewritten
	// to; a second check of the same table (GenericChoice's second look at
	// charmKnownKeys) is skipped, and two different wants for one content fail.
	done map[string]string
}

// updateKnownKeyTables rewrites the known-key table var whose current contents
// are got to want, across the effects package tree. It is called only under
// -update.
func updateKnownKeyTables(t *testing.T, got, want []string) {
	t.Helper()
	if knownKeysRun.done == nil {
		knownKeysRun.done = map[string]string{}
	}
	key := strings.Join(got, "\x00")
	wantKey := strings.Join(want, "\x00")
	switch prev := knownKeysRun.done[key]; {
	case prev == wantKey:
		return // already rewritten to this want this run (idempotent)
	case prev != "":
		t.Fatalf("the same known-key table content matched two different census wants (%q vs %q); the family is broken",
			prev, wantKey)
	}
	knownKeysRun.done[key] = wantKey
	files, err := knownKeySourceFiles()
	if err != nil {
		t.Fatalf("known-key update: %v", err)
	}
	matched := false
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("known-key update: read %s: %v", f, err)
		}
		out, found, changed, err := rewriteKnownKeyTable(src, got, want)
		if err != nil {
			t.Fatalf("known-key update: %s: %v", f, err)
		}
		if !found {
			continue
		}
		matched = true
		if !changed {
			continue
		}
		if err := os.WriteFile(f, out, 0o644); err != nil {
			t.Fatalf("known-key update: write %s: %v", f, err)
		}
		t.Logf("rewrote the known-key table in %s (%d keys)", f, len(want))
	}
	if !matched {
		t.Fatalf("known-key update: no effects known-key table var matches the current %d-key table -- the tables moved or were renamed; update rewriteKnownKeyTable", len(got))
	}
}

// knownKeySourceFiles lists the non-test Go sources of effects and its leaf
// package engine/effects/params, where every known-key table var lives.
func knownKeySourceFiles() ([]string, error) {
	var out []string
	for _, dir := range []string{"../effects", "../effects/params"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			// effects' tables live in <api>_params.go files; the whole leaf
			// package engine/effects/params is param sources.
			if dir == "../effects" && !strings.Contains(name, "params") {
				continue
			}
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}

// rewriteKnownKeyTable edits one source file's `[...]string` var whose
// elements are exactly cur to hold exactly want, leaving everything else in
// the file untouched. The edit is a MINIMAL token splice: the keys that are
// already right keep their position, wrapping and line grouping; only the
// extra keys are cut (with one separator) and the missing keys are inserted at
// their sorted place -- so a one-key census drift is a one-token diff, not a
// reformat of the whole table. found reports whether the file holds the table
// (so the caller can tell "already up to date" from "no match"); changed
// reports whether the returned bytes differ from src. A table already equal to
// want is found with changed=false, a file with no matching table is
// found=false, and two tables with cur's exact content are an error.
func rewriteKnownKeyTable(src []byte, cur, want []string) (out []byte, found, changed bool, err error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "table.go", src, 0)
	if err != nil {
		return nil, false, false, err
	}
	type span struct{ lbrace, rbrace token.Pos }
	var match span
	for _, decl := range af.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			list, lit, ok := stringArrayLit(vs.Values[0])
			if !ok {
				continue
			}
			if slices.Equal(list, want) {
				// Already up to date: the table was found and needs nothing.
				return src, true, false, nil
			}
			if !slices.Equal(list, cur) {
				continue
			}
			if match != (span{}) {
				return nil, false, false, fmt.Errorf("two [...]string vars hold the same %d-key content; the table identity is ambiguous", len(cur))
			}
			match = span{lit.Lbrace, lit.Rbrace}
		}
	}
	if match == (span{}) {
		return src, false, false, nil
	}
	start := fset.Position(match.lbrace).Offset + 1
	end := fset.Position(match.rbrace).Offset
	edited, err := spliceKnownKeys(src[start:end], cur, want)
	if err != nil {
		return nil, false, false, err
	}
	var b bytes.Buffer
	b.Write(src[:start])
	b.Write(edited)
	b.Write(src[end:])
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return nil, false, false, fmt.Errorf("regenerated table does not gofmt: %v", err)
	}
	return formatted, true, !bytes.Equal(formatted, src), nil
}

// spliceKnownKeys turns the body of a `[...]string{...}` literal holding cur
// into one holding want, by cutting the extra keys as the walk passes them and
// emitting the missing ones at their sorted place. Every surviving element
// keeps its bytes, so the file's wrapping and blank-line grouping survive the
// edit. A table whose KEPT keys are not already in file-sorted order (a
// misordered table, not the drift case) falls back to the plain one-key-per-
// line rewrite, which is still deterministic and gofmt-clean.
func spliceKnownKeys(body []byte, cur, want []string) ([]byte, error) {
	type tok struct{ start, end int } // end is past the closing quote
	var toks []tok
	var keys []string
	for i := 0; i < len(body); {
		q := bytes.IndexByte(body[i:], '"')
		if q < 0 {
			break
		}
		start := i + q
		endQ := bytes.IndexByte(body[start+1:], '"')
		if endQ < 0 {
			return nil, fmt.Errorf("unterminated string literal in the table body")
		}
		end := start + 1 + endQ + 1
		k, err := strconv.Unquote(string(body[start:end]))
		if err != nil {
			return nil, fmt.Errorf("unquote %s: %v", body[start:end], err)
		}
		toks = append(toks, tok{start, end})
		keys = append(keys, k)
		i = end
	}
	if !slices.Equal(keys, cur) {
		return nil, fmt.Errorf("the literal's keys do not match the table's current contents")
	}
	wantSet := make(map[string]bool, len(want))
	for _, k := range want {
		wantSet[k] = true
	}
	curSet := make(map[string]bool, len(cur))
	for _, k := range cur {
		curSet[k] = true
	}
	var kept []string
	for _, k := range keys {
		if wantSet[k] {
			kept = append(kept, k)
		}
	}
	if !slices.IsSorted(kept) {
		return plainKnownKeyBody(want)
	}
	var missing []string // want keys the table lacks, in want (sorted) order
	for _, k := range want {
		if !curSet[k] {
			missing = append(missing, k)
		}
	}
	mi := 0
	var b bytes.Buffer
	pos := 0 // body cursor: body[:pos] is already accounted for
	writeGapTo := func(to int) {
		b.Write(body[pos:to])
		pos = to
	}
	for j := range toks {
		if !wantSet[keys[j]] {
			// An extra: cut it with the gap before it and its trailing comma.
			end := toks[j].end
			if end < len(body) && body[end] == ',' {
				end++
			}
			pos = end
			continue
		}
		writeGapTo(toks[j].start)
		for mi < len(missing) && missing[mi] < keys[j] {
			b.WriteString(strconv.Quote(missing[mi]) + ", ")
			mi++
		}
		b.Write(body[toks[j].start:toks[j].end])
		end := toks[j].end
		if end < len(body) && body[end] == ',' {
			b.WriteByte(',')
			end++
		}
		pos = end
	}
	// The missing keys that sort after every kept key go on the last line.
	for mi < len(missing) {
		b.WriteString(strconv.Quote(missing[mi]) + ", ")
		mi++
	}
	writeGapTo(len(body))
	return b.Bytes(), nil
}

// plainKnownKeyBody renders want as a one-key-per-line table body (the
// fallback for a misordered table).
func plainKnownKeyBody(want []string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('\n')
	for _, k := range want {
		b.WriteString("\t" + strconv.Quote(k) + ",\n")
	}
	return b.Bytes(), nil
}

// stringArrayLit reports whether e is a `[...]string{...}` composite literal
// of string literals, and returns its elements.
func stringArrayLit(e ast.Expr) ([]string, *ast.CompositeLit, bool) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, nil, false
	}
	arr, ok := lit.Type.(*ast.ArrayType)
	if !ok || arr.Len == nil {
		return nil, nil, false
	}
	if _, ell := arr.Len.(*ast.Ellipsis); !ell {
		return nil, nil, false
	}
	id, ok := arr.Elt.(*ast.Ident)
	if !ok || id.Name != "string" {
		return nil, nil, false
	}
	var list []string
	for _, elt := range lit.Elts {
		bl, ok := elt.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return nil, nil, false // a keyed or computed element: not a table this rewriter owns
		}
		k, err := strconv.Unquote(bl.Value)
		if err != nil {
			return nil, nil, false
		}
		list = append(list, k)
	}
	return list, lit, true
}

// TestKnownKeysUpdaterRewritesTheTable is the regenerate path's own test: the
// rewrite fixes a desynced table byte-exactly while keeping every surviving
// byte (doc comment, wrapping, grouping), is a no-op on an up-to-date table,
// and refuses an ambiguous identity.
func TestKnownKeysUpdaterRewritesTheTable(t *testing.T) {
	src := []byte(`package effects

// demoKnownKeys is the table's doc comment, which the rewrite must keep.
var demoKnownKeys = [...]string{
	"Bravo", "Alpha",
}

func use() []string { return demoKnownKeys[:] }
`)
	// A table whose kept keys are not in file-sorted order (a misordered
	// table, not census drift) falls back to the plain one-key-per-line
	// rewrite; the doc comment survives.
	out, found, changed, err := rewriteKnownKeyTable(src, []string{"Bravo", "Alpha"}, []string{"Alpha", "Bravo", "Charlie"})
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !found || !changed {
		t.Fatalf("precondition: the desynced table was not rewritten (found=%v changed=%v)", found, changed)
	}
	want := `package effects

// demoKnownKeys is the table's doc comment, which the rewrite must keep.
var demoKnownKeys = [...]string{
	"Alpha",
	"Bravo",
	"Charlie",
}

func use() []string { return demoKnownKeys[:] }
`
	if string(out) != want {
		t.Fatalf("rewritten bytes =\n%s\nwant\n%s", out, want)
	}
	if _, err := format.Source(out); err != nil {
		t.Fatalf("rewritten output does not gofmt: %v", err)
	}

	// The drift case: a sorted table with one extra key (cut, with its line's
	// other keys kept) and one missing key that sorts last (appended). The
	// grouping and the doc comment survive.
	drift, driftFound, driftChanged, driftErr := rewriteKnownKeyTable([]byte(`package effects

var demoKnownKeys = [...]string{
	"Alpha", "Bravo",
	"Zulu",
}
`), []string{"Alpha", "Bravo", "Zulu"}, []string{"Alpha", "Bravo", "Yankee"})
	if driftErr != nil || !driftFound || !driftChanged {
		t.Fatalf("drift: found=%v changed=%v err=%v", driftFound, driftChanged, driftErr)
	}
	wantDrift := `package effects

var demoKnownKeys = [...]string{
	"Alpha", "Bravo", "Yankee",
}
`
	if string(drift) != wantDrift {
		t.Fatalf("drift bytes =\n%s\nwant\n%s", drift, wantDrift)
	}

	// A key that sorts into the middle is inserted at its sorted place, on the
	// line of the key that follows it.
	mid, midFound, midChanged, midErr := rewriteKnownKeyTable([]byte(`package effects

var demoKnownKeys = [...]string{
	"Alpha", "Bravo",
}
`), []string{"Alpha", "Bravo"}, []string{"Alpha", "Beta", "Bravo"})
	if midErr != nil || !midFound || !midChanged {
		t.Fatalf("middle insert: found=%v changed=%v err=%v", midFound, midChanged, midErr)
	}
	wantMid := `package effects

var demoKnownKeys = [...]string{
	"Alpha", "Beta", "Bravo",
}
`
	if string(mid) != wantMid {
		t.Fatalf("middle insert bytes =\n%s\nwant\n%s", mid, wantMid)
	}

	// A key that sorts onto its own group line is cut with its whole line,
	// leaving the neighbouring groups untouched.
	cut, cutFound, cutChanged, cutErr := rewriteKnownKeyTable([]byte(`package effects

var demoKnownKeys = [...]string{
	"Alpha",

	"Zulu",
}
`), []string{"Alpha", "Zulu"}, []string{"Alpha"})
	if cutErr != nil || !cutFound || !cutChanged {
		t.Fatalf("group cut: found=%v changed=%v err=%v", cutFound, cutChanged, cutErr)
	}
	wantCut := `package effects

var demoKnownKeys = [...]string{
	"Alpha",
}
`
	if string(cut) != wantCut {
		t.Fatalf("group cut bytes =\n%s\nwant\n%s", cut, wantCut)
	}

	// A table already equal to the want is found but unchanged, whatever cur
	// says (the caller's pre-rewrite view is stale).
	if out2, found, changed, err := rewriteKnownKeyTable(out, nil, []string{"Alpha", "Bravo", "Charlie"}); err != nil || !found || changed || !bytes.Equal(out2, out) {
		t.Fatalf("up-to-date table: found=%v changed=%v err=%v", found, changed, err)
	}

	// A file with no matching table is found=false with no error.
	if out3, found, changed, err := rewriteKnownKeyTable(src, []string{"Zulu"}, []string{"Alpha"}); err != nil || found || changed || !bytes.Equal(out3, src) {
		t.Fatalf("unmatched table: found=%v changed=%v err=%v", found, changed, err)
	}

	// Two tables with the same current content are ambiguous.
	ambiguous := []byte(`package effects

var demoKnownKeys = [...]string{"Bravo", "Alpha"}
var otherKnownKeys = [...]string{"Bravo", "Alpha"}
`)
	if _, _, _, err := rewriteKnownKeyTable(ambiguous, []string{"Bravo", "Alpha"}, []string{"Alpha"}); err == nil {
		t.Fatal("ambiguous identity was accepted")
	}
}
