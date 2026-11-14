package dirac

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// The State layout pins live in testdata/layout.tsv, rows of the class
// "default" (layoutClass): State's size, perms' cache-line phase, the
// ObjHash/ViewHash pins of TestCapacityHashPins, and the fixed-State field
// inventory of TestNoNewFixedStateFields. The file is generated, never edited
// by hand: a deliberate layout change runs layoutRegen, which rewrites it.
const (
	layoutPath  = "testdata/layout.tsv"
	layoutRegen = "DIRAC_UPDATE_LAYOUT=1 go test -run 'TestStateLayoutPinned|TestNoNewFixedStateFields|TestCapacityHashPins' ./dirac"
)

// layoutClasses is the file's class order.
var layoutClasses = []string{layoutClass}

// layoutKeys are a class's scalar pins, in file order.
var layoutKeys = []string{"size", "perms%64", "ObjHash", "ViewHash"}

// layoutField is one field of a fixed-State struct: its name (Type.field),
// byte offset within that struct, and size.
type layoutField struct {
	name      string
	off, size uintptr
}

func (f layoutField) String() string { return fmt.Sprintf("%s\t%d\t%d", f.name, f.off, f.size) }

// layoutPin is one class's rows.
type layoutPin struct {
	vals   map[string]string
	fields []layoutField
}

func layoutUpdating() bool { return os.Getenv("DIRAC_UPDATE_LAYOUT") == "1" }

// readLayout parses testdata/layout.tsv into its classes.
func readLayout(t *testing.T) map[string]*layoutPin {
	t.Helper()
	pins := map[string]*layoutPin{}
	f, err := os.Open(layoutPath)
	if err != nil {
		if os.IsNotExist(err) && layoutUpdating() {
			return pins
		}
		t.Fatalf("%v: regenerate with %s", err, layoutRegen)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		col := strings.Split(line, "\t")
		if len(col) < 3 {
			t.Fatalf("%s:%d: %q has fewer than 3 columns", layoutPath, n, line)
		}
		p := pins[col[0]]
		if p == nil {
			p = &layoutPin{vals: map[string]string{}}
			pins[col[0]] = p
		}
		if col[1] != "field" {
			if len(col) != 3 {
				t.Fatalf("%s:%d: %q: a pin row is class, key, value", layoutPath, n, line)
			}
			p.vals[col[1]] = col[2]
			continue
		}
		if len(col) != 5 {
			t.Fatalf("%s:%d: %q: a field row is class, field, name, offset, size", layoutPath, n, line)
		}
		off, err1 := strconv.ParseUint(col[3], 10, 64)
		size, err2 := strconv.ParseUint(col[4], 10, 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s:%d: %q: bad offset or size", layoutPath, n, line)
		}
		p.fields = append(p.fields, layoutField{col[2], uintptr(off), uintptr(size)})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return pins
}

// updateLayout applies edit to the class's rows and rewrites the file in its
// canonical order. Rows of any class not in layoutClasses are dropped.
func updateLayout(t *testing.T, edit func(p *layoutPin)) {
	t.Helper()
	pins := readLayout(t)
	p := pins[layoutClass]
	if p == nil {
		p = &layoutPin{vals: map[string]string{}}
		pins[layoutClass] = p
	}
	edit(p)
	var b strings.Builder
	fmt.Fprintf(&b, "# Generated State layout pins (layout_pins_test.go); do not edit by hand. Regenerate: %s\n", layoutRegen)
	b.WriteString("# class\tkey\tvalue; field rows: class\tfield\tType.field\toffset\tsize\n")
	for _, c := range layoutClasses {
		p := pins[c]
		if p == nil {
			continue
		}
		for _, k := range layoutKeys {
			if v, ok := p.vals[k]; ok {
				fmt.Fprintf(&b, "%s\t%s\t%s\n", c, k, v)
			}
		}
		for _, f := range p.fields {
			fmt.Fprintf(&b, "%s\tfield\t%s\n", c, f)
		}
	}
	if err := os.WriteFile(layoutPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("rewrote the %s class's rows in %s", layoutClass, layoutPath)
}

// layoutWant returns the running class's pin for key.
func layoutWant(t *testing.T, key string) uint64 {
	t.Helper()
	p := readLayout(t)[layoutClass]
	if p == nil {
		t.Fatalf("%s has no %s class: regenerate with %s", layoutPath, layoutClass, layoutRegen)
	}
	s, ok := p.vals[key]
	if !ok {
		t.Fatalf("%s has no %s %s pin: regenerate with %s", layoutPath, layoutClass, key, layoutRegen)
	}
	v, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		t.Fatalf("%s: %s %s = %q: %v", layoutPath, layoutClass, key, s, err)
	}
	return v
}

// checkStateLayout pins State's size and perms' cache-line phase. New State
// bytes come out of an arena key, a cold region or a scr overlay; a change to
// either number is a deliberate layout change.
func checkStateLayout(t *testing.T) {
	t.Helper()
	sz, phase := unsafe.Sizeof(State{}), unsafe.Offsetof(State{}.perms)%64
	if layoutUpdating() {
		updateLayout(t, func(p *layoutPin) {
			p.vals["size"] = strconv.FormatUint(uint64(sz), 10)
			p.vals["perms%64"] = strconv.FormatUint(uint64(phase), 10)
		})
		return
	}
	if want := layoutWant(t, "size"); uint64(sz) != want {
		t.Fatalf("State is %d bytes, pinned at %d (%s): a deliberate layout change regenerates it with %s", sz, want, layoutPath, layoutRegen)
	}
	if want := layoutWant(t, "perms%64"); uint64(phase) != want {
		t.Fatalf("perms at %d (mod 64 = %d), pinned mod 64 = %d (%s): a deliberate layout change regenerates it with %s",
			unsafe.Offsetof(State{}.perms), phase, want, layoutPath, layoutRegen)
	}
}

// checkLayoutHashes pins a scenario's ObjHash and ViewHash for the running
// class (TestCapacityHashPins).
func checkLayoutHashes(t *testing.T, objHash, viewHash uint64) {
	t.Helper()
	if layoutUpdating() {
		updateLayout(t, func(p *layoutPin) {
			p.vals["ObjHash"] = fmt.Sprintf("%#x", objHash)
			p.vals["ViewHash"] = fmt.Sprintf("%#x", viewHash)
		})
		return
	}
	if want := layoutWant(t, "ObjHash"); objHash != want {
		t.Fatalf("ObjHash %#x, want %#x (%s)", objHash, want, layoutPath)
	}
	if want := layoutWant(t, "ViewHash"); viewHash != want {
		t.Fatalf("ViewHash %#x, want %#x (%s)", viewHash, want, layoutPath)
	}
}

// fixedStateFields is the inventory of the structs whose every field costs
// fixed bytes in every game (or every table's prep): State's core, the
// generated state, a permanent slot, the generated prep and the cold arrays'
// view headers. Field order is declaration order.
func fixedStateFields() []layoutField {
	var out []layoutField
	for _, typ := range []reflect.Type{
		reflect.TypeFor[stateCore](),
		reflect.TypeFor[genState](),
		reflect.TypeFor[perm](),
		reflect.TypeFor[genPrep](),
		reflect.TypeFor[coldArrays](),
	} {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			out = append(out, layoutField{typ.Name() + "." + f.Name, f.Offset, f.Type.Size()})
		}
	}
	return out
}

// TestNoNewFixedStateFields: the fields of stateCore, genState, perm, genPrep
// and coldArrays (name, offset, size) are pinned in testdata/layout.tsv. New
// State bytes come from an arena key, a newCold region without a view header,
// or a designed construct region, never from a field of one of these
// structs; a deliberate layout change regenerates the inventory, and the
// review sees a one-file diff.
func TestNoNewFixedStateFields(t *testing.T) {
	got := fixedStateFields()
	if layoutUpdating() {
		updateLayout(t, func(p *layoutPin) { p.fields = got })
		return
	}
	p := readLayout(t)[layoutClass]
	if p == nil {
		t.Fatalf("%s has no %s class: regenerate with %s", layoutPath, layoutClass, layoutRegen)
	}
	if slices.Equal(got, p.fields) {
		return
	}
	// Match fields by name (the k-th field of a name, for repeated blank
	// fields), so one insertion reads as one added field and the fields it
	// pushed along, not as every later field replaced.
	key := func(fs []layoutField) ([]string, map[string]layoutField) {
		seen, keys, at := map[string]int{}, make([]string, len(fs)), map[string]layoutField{}
		for i, f := range fs {
			keys[i] = fmt.Sprintf("%s#%d", f.name, seen[f.name])
			seen[f.name]++
			at[keys[i]] = f
		}
		return keys, at
	}
	gotKeys, gotAt := key(got)
	pinKeys, pinAt := key(p.fields)
	var added, removed, moved []string
	for _, k := range gotKeys {
		f, was := gotAt[k], pinAt[k]
		switch _, ok := pinAt[k]; {
		case !ok:
			added = append(added, "+ "+f.String())
		case f != was:
			moved = append(moved, fmt.Sprintf("~ %s\t%d\t%d (pinned %d\t%d)", f.name, f.off, f.size, was.off, was.size))
		}
	}
	for _, k := range pinKeys {
		if _, ok := gotAt[k]; !ok {
			removed = append(removed, "- "+pinAt[k].String())
		}
	}
	diff := slices.Concat(added, removed, moved)
	if len(diff) == 0 {
		diff = append(diff, "(the same fields in a different order)")
	}
	const show = 40
	more := ""
	if len(diff) > show {
		more = fmt.Sprintf("\n  ... and %d more", len(diff)-show)
		diff = diff[:show]
	}
	t.Fatalf("fixed State changed: use an arena key or a cold region, or regenerate with %s for a deliberate layout change.\n"+
		"%s class against %s (+ added, - removed, ~ moved or resized):\n  %s%s", layoutRegen, layoutClass, layoutPath, strings.Join(diff, "\n  "), more)
}
