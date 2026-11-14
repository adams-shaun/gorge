package manifest

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// AliasesPath is the seed's alias and exclusion list, a slash path from the
// module root (Read skips it: it is a file, not a family directory).
const AliasesPath = Dir + "/aliases.tsv"

// censusFamily is the family each census kind seeds. A kind not here (link)
// names no code.
var censusFamily = map[string]string{
	"api":   "ExtOp",
	"count": "ExprKind",
	"ctr":   "CounterKind",
	"kw":    "KwID",
	"repl":  "ReplKind",
	"stat":  "StaticKind",
	"trig":  "TrigEvent",
	"zone":  "Zone",
}

// keyKind is the pseudo census kind of a bounded State key, seeded from the
// classes table's state_key column into the key family.
const keyKind = "key"

// CensusRow is one vocabulary item of the research census
// (state-prealloc census.tsv): a ForgeScript item, its card counts, how far
// kgen lowers it and the State it needs.
type CensusRow struct {
	Kind, Item         string
	All, Std, MBC      int
	Lowered, Refusals  int
	Sole               int
	Status             string // lowered, refused, co-refused or partial
	StateCat, StateKey string
	BytesDef, BytesMP  int
}

// ClassRow is one kgen refusal class of the research classes table, with the
// State it needs.
type ClassRow struct {
	Class              string
	All, Std, MBC      int
	StateCat, StateKey string
	BytesDef, BytesMP  int
}

var (
	censusHeader = []string{"kind", "item", "cards_all", "cards_std", "cards_mbc", "lowered_cards", "refused_naming_it", "sole_blocker", "status", "state_category", "state_key", "bytes_def", "bytes_mp"}
	classHeader  = []string{"class", "cards_all", "cards_std", "cards_mbc", "state_category", "state_key", "bytes_def", "bytes_mp"}
)

// readTSV reads a headed TSV (gzip when it starts with the gzip magic) and
// returns its rows after checking the header.
func readTSV(name string, r io.Reader, header []string) ([][]string, error) {
	br := bufio.NewReader(r)
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		defer zr.Close()
		br = bufio.NewReader(zr)
	}
	raw, err := io.ReadAll(br)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if got := strings.Join(strings.Split(lines[0], "\t"), " "); got != strings.Join(header, " ") {
		return nil, fmt.Errorf("%s: header %q, want %q", name, got, strings.Join(header, " "))
	}
	var rows [][]string
	for i, l := range lines[1:] {
		f := strings.Split(l, "\t")
		if len(f) != len(header) {
			return nil, fmt.Errorf("%s:%d: %d fields, want %d", name, i+2, len(f), len(header))
		}
		rows = append(rows, f)
	}
	return rows, nil
}

// atois parses fs as decimal integers into the pointers.
func atois(name string, line int, fs []string, out ...*int) error {
	for i, s := range fs {
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("%s:%d: %q is not an integer", name, line, s)
		}
		*out[i] = v
	}
	return nil
}

// ReadCensus reads the census table (plain or gzip).
func ReadCensus(name string, r io.Reader) ([]CensusRow, error) {
	rows, err := readTSV(name, r, censusHeader)
	if err != nil {
		return nil, err
	}
	out := make([]CensusRow, len(rows))
	for i, f := range rows {
		c := &out[i]
		c.Kind, c.Item, c.Status, c.StateCat, c.StateKey = f[0], f[1], f[8], f[9], f[10]
		if err := atois(name, i+2, append(f[2:8:8], f[11], f[12]), &c.All, &c.Std, &c.MBC, &c.Lowered, &c.Refusals, &c.Sole, &c.BytesDef, &c.BytesMP); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ReadClasses reads the refusal-class table (plain or gzip).
func ReadClasses(name string, r io.Reader) ([]ClassRow, error) {
	rows, err := readTSV(name, r, classHeader)
	if err != nil {
		return nil, err
	}
	out := make([]ClassRow, len(rows))
	for i, f := range rows {
		c := &out[i]
		c.Class, c.StateCat, c.StateKey = f[0], f[4], f[5]
		if err := atois(name, i+2, []string{f[1], f[2], f[3], f[6], f[7]}, &c.All, &c.Std, &c.MBC, &c.BytesDef, &c.BytesMP); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// AliasAction is what the seed does with a census item an alias row names.
type AliasAction string

const (
	// AliasExisting: the item is an existing constant (Target), not a new code.
	AliasExisting AliasAction = "alias"
	// AliasExclude: the item is no code of its family (Target is "-").
	AliasExclude AliasAction = "exclude"
	// AliasName: the item is seeded under the Go name Target instead of the
	// derived one.
	AliasName AliasAction = "name"
)

// Alias is one row of aliases.tsv:
//
//	kind<TAB>item<TAB>action<TAB>target<TAB>why
//
// Lines starting with # and blank lines are comments.
type Alias struct {
	Kind, Item string
	Action     AliasAction
	Target     string
	Why        string
	Line       int
}

// ReadAliases reads aliases.tsv.
func ReadAliases(name string, r io.Reader) ([]Alias, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var out []Alias
	seen := map[string]int{}
	for i, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Split(l, "\t")
		fail := func(format string, a ...any) error {
			return fmt.Errorf("%s:%d: %s", name, i+1, fmt.Sprintf(format, a...))
		}
		if len(f) != 5 {
			return nil, fail("%d fields, want kind, item, action, target, why", len(f))
		}
		a := Alias{Kind: f[0], Item: f[1], Action: AliasAction(f[2]), Target: f[3], Why: f[4], Line: i + 1}
		if _, ok := censusFamily[a.Kind]; !ok && a.Kind != keyKind {
			return nil, fail("kind %q is not a seeded census kind", a.Kind)
		}
		if a.Item == "" || a.Why == "" {
			return nil, fail("an alias needs an item and a why")
		}
		switch a.Action {
		case AliasExisting, AliasName:
			if !token.IsIdentifier(a.Target) {
				return nil, fail("target %q is not a Go identifier", a.Target)
			}
		case AliasExclude:
			if a.Target != "-" {
				return nil, fail("an exclude's target is -, not %q", a.Target)
			}
		default:
			return nil, fail("action %q is not alias, exclude or name", a.Action)
		}
		k := a.Kind + "\t" + a.Item
		if j, ok := seen[k]; ok {
			return nil, fail("%s %q is also line %d", a.Kind, a.Item, j)
		}
		seen[k] = a.Line
		out = append(out, a)
	}
	return out, nil
}

// SeedSkip is a census item the seed did not make an entry for, and why.
type SeedSkip struct {
	Kind, Item string
	Why        string
}

// Seed is what seeding adds to a tree: the new entries, in family order and
// within a family in value order, and the not-lowered items it skipped.
type Seed struct {
	Entries []*Entry
	Skipped []SeedSkip
}

// Counts is the number of new entries per family.
func (s *Seed) Counts() map[string]int {
	n := map[string]int{}
	for _, e := range s.Entries {
		n[e.Family]++
	}
	return n
}

// candidate is one vocabulary item the seed may give an entry.
type candidate struct {
	kind, item string
	family     string
	all        int
	census     string
	state      string
	status     Status
}

// PlanSeed works out the entries seeding adds to t (a validated tree) from
// the census and classes tables and the alias list, without writing any.
//
// The candidates are the census items kgen does not lower (refused,
// co-refused, partial) of the kinds that name a family, and the bounded
// State keys of the classes table (key family). An alias row aliases an item
// to an existing constant, excludes it, or names it; every other candidate
// becomes an entry unless its family already has an entry whose forge token
// is the item (a re-run adds only new vocabulary and never edits an entry).
// Values are assigned per family in descending cards_all, then forge token
// order: the lowest free value of the family's Alloc band that no ordinals
// row, no entry and no reserved band holds.
//
// declared is the identifiers already declared in each Go package ("kop",
// "dirac"); a new name that is one of them, an ordinals row or another
// entry's name is an error naming the item, so the alias list gains a name
// or alias row for it.
func PlanSeed(t *Tree, census []CensusRow, classes []ClassRow, aliases []Alias, declared map[string]map[string]bool) (*Seed, error) {
	var errs []error
	inCensus := map[string]bool{}
	var cands []*candidate
	for _, r := range census {
		inCensus[r.Kind+"\t"+r.Item] = true
		fam, ok := censusFamily[r.Kind]
		if !ok || r.Status == "lowered" {
			continue
		}
		cands = append(cands, &candidate{
			kind: r.Kind, item: r.Item, family: fam, all: r.All,
			census: fmt.Sprintf("%s %s all=%d std=%d mbc=%d sole=%d", r.Kind, r.Item, r.All, r.Std, r.MBC, r.Sole),
			state:  stateLine(r.StateCat, r.StateKey, r.BytesDef, r.BytesMP),
			status: statusFor(r.StateCat, r.StateKey),
		})
	}
	cands = append(cands, keyCandidates(classes, inCensus)...)

	byItem := map[string]*Alias{}
	for i := range aliases {
		a := &aliases[i]
		if !inCensus[a.Kind+"\t"+a.Item] {
			errs = append(errs, fmt.Errorf("%s:%d: %s %q is not in the census", AliasesPath, a.Line, a.Kind, a.Item))
		}
		if a.Action == AliasExisting && t.byName[a.Target] == nil && t.entryNamed(a.Target) == nil {
			errs = append(errs, fmt.Errorf("%s:%d: %s %q aliases %s, which is no ordinals row or entry", AliasesPath, a.Line, a.Kind, a.Item, a.Target))
		}
		byItem[a.Kind+"\t"+a.Item] = a
	}

	s := &Seed{}
	fresh := map[string]*candidate{} // pkg.name -> the candidate taking it
	var add []*candidate
	var names []string
	for _, c := range cands {
		if strings.Contains(c.item, "|") {
			errs = append(errs, fmt.Errorf("%s %q: a forge token cannot hold |", c.kind, c.item))
			continue
		}
		f := FamilyNamed(c.family)
		name := f.Prefix + goWords(c.item)
		if a := byItem[c.kind+"\t"+c.item]; a != nil {
			switch a.Action {
			case AliasExisting:
				s.Skipped = append(s.Skipped, SeedSkip{c.kind, c.item, "alias " + a.Target + ": " + a.Why})
				continue
			case AliasExclude:
				s.Skipped = append(s.Skipped, SeedSkip{c.kind, c.item, "exclude: " + a.Why})
				continue
			case AliasName:
				name = a.Target
			}
		}
		if e := t.entryForging(c.family, c.item); e != nil {
			s.Skipped = append(s.Skipped, SeedSkip{c.kind, c.item, "has " + e.Path})
			continue
		}
		pkgName := f.Pkg + "." + name
		switch {
		case !token.IsIdentifier(name) || !strings.HasPrefix(name, f.Prefix) || len(name) == len(f.Prefix):
			errs = append(errs, fmt.Errorf("%s %q: name %q is not a %s identifier starting %q: add a name row", c.kind, c.item, name, c.family, f.Prefix))
		case t.byName[name] != nil && f.Pkg == "kop":
			errs = append(errs, fmt.Errorf("%s %q: %s is already a %s constant (%s): add an alias or name row", c.kind, c.item, name, t.byName[name].Type, OrdinalsPath))
		case declared[f.Pkg][name]:
			errs = append(errs, fmt.Errorf("%s %q: %s is already declared in package %s: add an alias or name row", c.kind, c.item, name, f.Pkg))
		case t.entryNamedIn(f.Pkg, name) != nil:
			errs = append(errs, fmt.Errorf("%s %q: %s is already %s: add an alias or name row", c.kind, c.item, name, t.entryNamedIn(f.Pkg, name).Path))
		case fresh[pkgName] != nil:
			o := fresh[pkgName]
			errs = append(errs, fmt.Errorf("%s %q: %s is also the name of %s %q: add a name row", c.kind, c.item, name, o.kind, o.item))
		default:
			fresh[pkgName] = c
			add = append(add, c)
			names = append(names, name)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Values: per family, by cards_all descending, then the forge token.
	order := make([]int, len(add))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := add[order[i]], add[order[j]]
		if a.family != b.family {
			return a.family < b.family
		}
		if a.all != b.all {
			return a.all > b.all
		}
		return a.item < b.item
	})
	held, verrs := t.validate()
	if len(verrs) > 0 {
		return nil, errors.Join(verrs...)
	}
	for _, i := range order {
		c := add[i]
		f := FamilyNamed(c.family)
		got := int64(-1)
		for pos := f.Alloc.Lo; pos <= f.Alloc.Hi; pos++ {
			v := pos
			if f.Bits {
				v = 1 << pos
			}
			if held[f.Name].holder(f, v) == "" && f.reserved(pos) == nil {
				got = v
				break
			}
		}
		if got < 0 {
			errs = append(errs, fmt.Errorf("%s %q: no free %s value in [%d,%d]", c.kind, c.item, f.Name, f.Alloc.Lo, f.Alloc.Hi))
			continue
		}
		e := &Entry{
			Path: Dir + "/" + f.Name + "/" + names[i], Family: f.Name, Name: names[i], Value: got,
			Forge: []string{c.item}, Census: c.census, State: c.state, Status: c.status,
		}
		e.raw = e.Format()
		held[f.Name].add(f, got, e.Path)
		s.Entries = append(s.Entries, e)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	// The seeded tree must validate as a whole.
	all := &Tree{Entries: append(append([]*Entry(nil), t.Entries...), s.Entries...), Ordinals: t.Ordinals, byName: t.byName}
	if _, verrs := all.validate(); len(verrs) > 0 {
		return nil, errors.Join(verrs...)
	}
	return s, nil
}

// keyCandidates are the bounded State keys of the classes table, one per
// state_key, summing the classes' cards.
func keyCandidates(classes []ClassRow, inCensus map[string]bool) []*candidate {
	byKey := map[string]*candidate{}
	var keys []string
	nclass := map[string]int{}
	for _, c := range classes {
		if !boundedState(c.StateCat) {
			continue
		}
		k := byKey[c.StateKey]
		if k == nil {
			k = &candidate{
				kind: keyKind, item: c.StateKey, family: "key",
				state:  stateLine(c.StateCat, "", c.BytesDef, c.BytesMP),
				status: Status{Kind: Reserved},
			}
			byKey[c.StateKey] = k
			keys = append(keys, c.StateKey)
			inCensus[keyKind+"\t"+c.StateKey] = true
		}
		k.all += c.All
		nclass[c.StateKey]++
	}
	sort.Strings(keys)
	out := make([]*candidate, len(keys))
	for i, k := range keys {
		c := byKey[k]
		c.census = fmt.Sprintf("key %s classes=%d all=%d", k, nclass[k], c.all)
		out[i] = c
	}
	return out
}

// boundedState reports whether a State category is a bounded key's: not
// code-only, done, unbounded or outside the object space.
func boundedState(cat string) bool {
	switch cat {
	case "code", "done", "unbounded", "objspace", "":
		return false
	}
	return true
}

// stateLine is an entry's state line: the category, its bytes in the
// default and four-seat classes, and the research's State key.
func stateLine(cat, key string, def, mp int) string {
	var s string
	switch cat {
	case "code":
		return "code 0"
	case "unbounded", "objspace":
		s = cat
	default:
		s = fmt.Sprintf("%s %d/%d", cat, def, mp)
	}
	if key != "" && !strings.Contains(key, ":") {
		s += " (" + key + ")"
	}
	return s
}

// statusFor is a seeded entry's status: an item that needs objects from
// outside the game waits on a wider object code (TCode), one that needs an
// unbounded construct waits on its design; the rest are reserved.
func statusFor(cat, key string) Status {
	switch cat {
	case "objspace":
		return Status{Kind: Blocked, Why: "objspace:tcode"}
	case "unbounded":
		return Status{Kind: Blocked, Why: "unbounded:" + strings.ReplaceAll(key, " ", "-")}
	}
	return Status{Kind: Reserved}
}

// goWords is item as the tail of a Go identifier: its words (split at every
// rune that is no letter or digit) each capitalised; an all-capitals word
// (a counter type: TICKET) is title-cased, and a word with digits (P1P0) is
// kept as it is.
func goWords(item string) string {
	var b strings.Builder
	for _, w := range strings.FieldsFunc(item, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		rs := []rune(w)
		upper, digits := true, false
		for _, r := range rs {
			upper = upper && (unicode.IsUpper(r) || unicode.IsDigit(r))
			digits = digits || unicode.IsDigit(r)
		}
		if upper && !digits && len(rs) > 1 {
			rs = []rune(strings.ToLower(w))
		}
		rs[0] = unicode.ToUpper(rs[0])
		b.WriteString(string(rs))
	}
	return b.String()
}

// entryNamed is t's entry called name in any family, or nil.
func (t *Tree) entryNamed(name string) *Entry {
	for _, e := range t.Entries {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// entryNamedIn is t's entry called name whose constant is in package pkg.
func (t *Tree) entryNamedIn(pkg, name string) *Entry {
	for _, e := range t.Entries {
		if e.Name == name && e.Fam() != nil && e.Fam().Pkg == pkg {
			return e
		}
	}
	return nil
}

// entryForging is t's entry of family whose forge tokens include tok
// (without case), or nil.
func (t *Tree) entryForging(family, tok string) *Entry {
	for _, e := range t.Entries {
		if e.Family != family {
			continue
		}
		for _, f := range e.Forge {
			if strings.EqualFold(f, tok) {
				return e
			}
		}
	}
	return nil
}

// Declared is the top-level identifiers of each manifest package's
// non-test Go files in the module at root (kop: dirac/kop, dirac: dirac),
// leaving out the generated files, whose names are the manifest's own.
func Declared(root string) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	for pkg, dir := range map[string]string{"kop": "dirac/kop", "dirac": "dirac"} {
		names := map[string]bool{}
		out[pkg] = names
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*.go"))
		if err != nil {
			return nil, err
		}
		fset := token.NewFileSet()
		for _, p := range files {
			slash := path.Join(dir, filepath.Base(p))
			if strings.HasSuffix(p, "_test.go") || slash == KopGen || slash == DiracGen {
				continue
			}
			f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil {
						names[d.Name.Name] = true
					}
				case *ast.GenDecl:
					for _, sp := range d.Specs {
						switch sp := sp.(type) {
						case *ast.TypeSpec:
							names[sp.Name.Name] = true
						case *ast.ValueSpec:
							for _, n := range sp.Names {
								names[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return out, nil
}

// WriteSeed writes s's entries into the module at root. It refuses to
// overwrite a file: seeding never edits an entry.
func WriteSeed(root string, s *Seed) error {
	for _, e := range s.Entries {
		p := filepath.Join(root, filepath.FromSlash(e.Path))
		if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: exists; seeding never edits an entry", e.Path)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, e.Format(), 0o644); err != nil {
			return err
		}
	}
	return nil
}
