package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Dir is the manifest's directory, a slash path from the module root.
const Dir = "dirac/manifest"

// OrdinalsPath is kop's record of every constant's name, type and value
// (TestKopOrdinalsUnchanged): the values a new entry may not take.
const OrdinalsPath = "dirac/kop/testdata/ordinals.tsv"

// AsksPath is the frozen list of the hand-declared resolution ask kinds
// (name<TAB>value, package dirac's ra* constants). It is optional, and
// kmanifest check refuses any row it would add: a new ask kind is an entry of
// the ask family.
const AsksPath = "dirac/testdata/asks.tsv"

// Ask is one asks.tsv row.
type Ask struct {
	Name  string
	Value int64
}

// Ordinal is one ordinals.tsv row (or a row derived from a kop entry).
type Ordinal struct {
	Name, Type, Value string
}

// Tree is the manifest and the ordinals of one source tree.
type Tree struct {
	// Entries sorted by family, then name.
	Entries  []*Entry
	Ordinals []Ordinal
	// Asks is asks.tsv's rows, in file order (none when the tree has no file).
	Asks []Ask
	// Handlers is the kernel's //kmanifest handlers (LoadGen; Load leaves it
	// empty).
	Handlers Handlers

	byName map[string]*Ordinal
	// commit is the commit ReadGit read the tree at ("" for a working tree).
	commit string
}

// Read reads the manifest and ordinals.tsv of fsys, a tree rooted at the
// module. Directories under Dir name families (testdata and dot names
// excepted); its own files (the package's Go source) are not entries. A tree
// with no Dir reads as an empty manifest.
func Read(fsys fs.FS) (*Tree, error) {
	t := &Tree{}
	raw, err := fs.ReadFile(fsys, OrdinalsPath)
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("%s:%d: want name<TAB>type<TAB>value, got %q", OrdinalsPath, i+1, line)
		}
		t.Ordinals = append(t.Ordinals, Ordinal{f[0], f[1], f[2]})
	}
	t.byName = make(map[string]*Ordinal, len(t.Ordinals))
	for i := range t.Ordinals {
		t.byName[t.Ordinals[i].Name] = &t.Ordinals[i]
	}
	if t.Asks, err = readAsks(fsys); err != nil {
		return nil, err
	}
	if t.Entries, err = ReadEntries(fsys); err != nil {
		return nil, err
	}
	return t, nil
}

// readAsks reads asks.tsv of fsys; a tree without one has no rows.
func readAsks(fsys fs.FS) ([]Ask, error) {
	raw, err := fs.ReadFile(fsys, AsksPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Ask
	for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		name, val, ok := strings.Cut(line, "\t")
		v, err := strconv.ParseInt(val, 10, 64)
		if !ok || name == "" || err != nil {
			return nil, fmt.Errorf("%s:%d: want name<TAB>value, got %q", AsksPath, i+1, line)
		}
		out = append(out, Ask{name, v})
	}
	return out, nil
}

// ask is the asks.tsv row called name, or nil.
func (t *Tree) ask(name string) *Ask {
	for i := range t.Asks {
		if t.Asks[i].Name == name {
			return &t.Asks[i]
		}
	}
	return nil
}

// ReadEntries parses fsys's manifest entries alone, unvalidated and in
// directory order: what a reader without ordinals.tsv (kgen's embedded table
// sources) needs to tell an active code from an inactive one. A tree with no
// Dir has none.
func ReadEntries(fsys fs.FS) ([]*Entry, error) {
	fams, err := fs.ReadDir(fsys, Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Entry
	for _, fd := range fams {
		if !fd.IsDir() || fd.Name() == "testdata" || strings.HasPrefix(fd.Name(), ".") {
			continue
		}
		dir := path.Join(Dir, fd.Name())
		if FamilyNamed(fd.Name()) == nil {
			return nil, fmt.Errorf("%s: %q is not a manifest family", dir, fd.Name())
		}
		files, err := fs.ReadDir(fsys, dir)
		if err != nil {
			return nil, err
		}
		for _, ed := range files {
			if strings.HasPrefix(ed.Name(), ".") {
				continue
			}
			p := path.Join(dir, ed.Name())
			if ed.IsDir() {
				return nil, fmt.Errorf("%s: an entry is a file, not a directory", p)
			}
			b, err := fs.ReadFile(fsys, p)
			if err != nil {
				return nil, err
			}
			e, err := ParseEntry(p, b)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// Entry is the entry at path (Dir/<Family>/<Name>), or nil.
func (t *Tree) Entry(p string) *Entry {
	for _, e := range t.Entries {
		if e.Path == p {
			return e
		}
	}
	return nil
}

// Lookup is the entry of family holding value, or nil: the guard that keeps
// an inactive code out of a table (invariant 3) refuses a card whose code's
// entry has a RefusalClass. A Next entry answers to its provisional value.
func (t *Tree) Lookup(family string, value int64) *Entry {
	for _, e := range t.Entries {
		if e.Family == family && e.Value == value {
			return e
		}
	}
	return nil
}

// EntryOrdinals is the ordinals row of every kop entry, sorted by name: the
// constant the generator writes from the entry, which ordinals.tsv does not
// record (TestKopOrdinalsUnchanged adds these rows to the file's). Call it
// after Validate, so a value: next entry reports its provisional value.
func (t *Tree) EntryOrdinals() []Ordinal {
	var out []Ordinal
	for _, e := range t.Entries {
		if f := e.Fam(); f != nil && f.Pkg == "kop" {
			out = append(out, Ordinal{Name: e.Name, Type: e.Family, Value: strconv.FormatInt(e.Value, 10)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// row is the ordinals row of the constant called name: the file's row, else
// the one derived from a kop entry of that name, else nil.
func (t *Tree) row(name string) *Ordinal {
	if r := t.byName[name]; r != nil {
		return r
	}
	for _, e := range t.Entries {
		if f := e.Fam(); f != nil && e.Name == name && f.Pkg == "kop" {
			return &Ordinal{Name: e.Name, Type: e.Family, Value: strconv.FormatInt(e.Value, 10)}
		}
	}
	return nil
}

// owned reports whether ordinals row r is an entry's own constant (the
// generator's output, which ordinals.tsv records like any other): such a row
// takes no value from the entry that owns it.
func (t *Tree) owned(r *Ordinal) bool {
	for _, e := range t.Entries {
		if e.Name == r.Name && e.Family == r.Type {
			return true
		}
	}
	return false
}

// taken is a family's values that a new entry may not take.
type taken struct {
	vals map[int64]string // value -> holder (enum families)
	bits int64            // OR of every holder (flag families)
	by   map[int64]string // bit position -> holder (flag families)
}

func (k *taken) add(f *Family, v int64, who string) {
	if f.Bits {
		k.bits |= v
		for b := int64(0); b <= f.Max; b++ {
			if v&(1<<b) != 0 {
				if _, ok := k.by[b]; !ok {
					k.by[b] = who
				}
			}
		}
		return
	}
	if _, ok := k.vals[v]; !ok {
		k.vals[v] = who
	}
}

// holder is who holds v, or "".
func (k *taken) holder(f *Family, v int64) string {
	if f.Bits {
		if b := bitOf(v); b >= 0 && k.bits&v != 0 {
			return k.by[b]
		}
		return ""
	}
	return k.vals[v]
}

// legacyTaken is every family's values held by ordinals rows that are not an
// entry's own constant.
func (t *Tree) legacyTaken() (map[string]*taken, error) {
	out := map[string]*taken{}
	for _, f := range Families {
		out[f.Name] = &taken{vals: map[int64]string{}, by: map[int64]string{}}
	}
	for i := range t.Ordinals {
		r := &t.Ordinals[i]
		f := FamilyNamed(r.Type)
		if f == nil || t.owned(r) {
			continue
		}
		v, err := parseOrdinal(r.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %v", OrdinalsPath, r.Name, err)
		}
		out[f.Name].add(f, v, OrdinalsPath+" "+r.Name)
	}
	if f := FamilyNamed("ask"); f != nil {
		for _, a := range t.Asks {
			out[f.Name].add(f, a.Value, AsksPath+" "+a.Name)
		}
	}
	return out, nil
}

func parseOrdinal(s string) (int64, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, nil
	}
	u, err := strconv.ParseUint(s, 10, 64)
	return int64(u), err
}

// Validate holds t to invariant 1 on one tree and resolves every Next entry's
// provisional value (resolve). It returns every violation, one line each,
// naming the file.
func (t *Tree) Validate() []error {
	held, errs := t.validate()
	if len(errs) > 0 {
		return errs
	}
	if errs := t.resolve(held, nil); len(errs) > 0 {
		return errs
	}
	return t.coverErrs()
}

// coverErrs: an entry at or above its family's Cover is deferred or blocked,
// since the kernel serves no code there; a provisional value counts.
func (t *Tree) coverErrs() []error {
	var errs []error
	for _, e := range t.Entries {
		f := e.Fam()
		if f.Cover == 0 || e.Value < f.Cover {
			continue
		}
		if k := e.Status.Kind; k == Reserved || k == Active {
			errs = append(errs, fmt.Errorf("%s: value %d is at or above %s's cover %d, which the kernel does not serve: status %q, not %s", e.Path, e.Value, f.Name, f.Cover, Deferred.String()+" "+CoverWhy, k))
		}
	}
	return errs
}

// validate is Validate without resolving: the values the tree's constants
// and valued entries hold, and the violations.
func (t *Tree) validate() (map[string]*taken, []error) {
	var errs []error
	sort.SliceStable(t.Entries, func(i, j int) bool {
		a, b := t.Entries[i], t.Entries[j]
		if a.Family != b.Family {
			return a.Family < b.Family
		}
		return a.Name < b.Name
	})
	ok := true
	for _, e := range t.Entries {
		if err := e.checkSelf(); err != nil {
			errs = append(errs, err)
			ok = false
		}
	}
	if !ok {
		return nil, errs // the rest assumes known families and in-range values
	}
	// One declaration per name in each Go package: an entry's name is no
	// other entry's and no other type's constant.
	names := map[string]*Entry{}
	for _, e := range t.Entries {
		key := e.Fam().Pkg + "." + e.Name
		if o := names[key]; o != nil {
			errs = append(errs, fmt.Errorf("%s: name %s is also %s", e.Path, e.Name, o.Path))
		}
		names[key] = e
		if r := t.byName[e.Name]; r != nil && e.Fam().Pkg == "kop" && r.Type != e.Family {
			errs = append(errs, fmt.Errorf("%s: name %s is already a %s constant (%s)", e.Path, e.Name, r.Type, OrdinalsPath))
		}
	}
	for _, a := range t.Asks {
		if o := names["dirac."+a.Name]; o != nil {
			errs = append(errs, fmt.Errorf("%s: name %s is also a hand constant (%s)", o.Path, a.Name, AsksPath))
		}
	}
	held, err := t.legacyTaken()
	if err != nil {
		return nil, append(errs, err)
	}
	for _, e := range t.Entries {
		if e.Next {
			continue
		}
		f := e.Fam()
		if who := held[f.Name].holder(f, e.Value); who != "" {
			errs = append(errs, fmt.Errorf("%s: value %d is already %s", e.Path, e.Value, who))
			continue
		}
		held[f.Name].add(f, e.Value, e.Path)
	}
	return held, errs
}

// resolve gives each Next entry the lowest free value of its family's Alloc
// band, in (family, name) order, on top of held (and extra, a base's values,
// when Assign runs). A reserved band is free only to an entry with an
// approval line.
func (t *Tree) resolve(held, extra map[string]*taken) []error {
	var errs []error
	for _, e := range t.Entries {
		if !e.Next {
			continue
		}
		f := e.Fam()
		free := func(pos int64) bool {
			v := pos
			if f.Bits {
				v = 1 << pos
			}
			if held[f.Name].holder(f, v) != "" || extra != nil && extra[f.Name].holder(f, v) != "" {
				return false
			}
			return f.reserved(pos) == nil || e.Approval != ""
		}
		got := int64(-1)
		for pos := f.Alloc.Lo; pos <= f.Alloc.Hi; pos++ {
			if free(pos) {
				got = pos
				break
			}
		}
		if got < 0 {
			msg := fmt.Sprintf("%s: no free %s value in [%d,%d]", e.Path, f.Name, f.Alloc.Lo, f.Alloc.Hi)
			if len(f.Reserved) > 0 && e.Approval == "" {
				b := f.Reserved[0]
				msg += fmt.Sprintf(" outside the reserved band [%d,%d] (%s): add an approval line", b.Lo, b.Hi, b.Why)
			}
			errs = append(errs, errors.New(msg))
			continue
		}
		if f.Bits {
			e.Value = 1 << got
		} else {
			e.Value = got
		}
		held[f.Name].add(f, e.Value, e.Path)
	}
	return errs
}

// Load reads fsys's manifest (Read) and validates it (Validate), joining
// every violation into one error.
func Load(fsys fs.FS) (*Tree, error) {
	t, err := Read(fsys)
	if err != nil {
		return nil, err
	}
	if errs := t.Validate(); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return t, nil
}
