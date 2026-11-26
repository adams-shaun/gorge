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
)

// The fixed-State field inventory lives in testdata/layout.tsv, rows of the
// class "default" (layoutClass): one row per field of stateCore, genState,
// perm, genPrep and coldArrays, keyed by the declared field (Type.field and
// size), not by position. The file is generated, never edited by hand: a
// deliberate layout change runs layoutRegen, which rewrites it.
const (
	layoutPath  = "testdata/layout.tsv"
	layoutRegen = "DIRAC_UPDATE_LAYOUT=1 go test -run '^(TestNoNewFixedStateFields|TestCapacityHashPins)$' ./dirac"
)

// layoutField is one field of a fixed-State struct: its name (Type.field)
// and its size. The offset is not pinned: a field another field's insertion
// pushed along did not change on purpose, and pinning it makes two PRs that
// each add a field rewrite every later row (and collide on the file).
type layoutField struct {
	name string
	size uintptr
}

func (f layoutField) String() string { return fmt.Sprintf("%s\t%d", f.name, f.size) }

func layoutUpdating() bool { return os.Getenv("DIRAC_UPDATE_LAYOUT") == "1" }

// readLayout parses testdata/layout.tsv's field rows (class, field,
// Type.field, size; blank lines and #-comments are skipped).
func readLayout(t *testing.T) []layoutField {
	t.Helper()
	f, err := os.Open(layoutPath)
	if err != nil {
		if os.IsNotExist(err) && layoutUpdating() {
			return nil
		}
		t.Fatalf("%v: regenerate with %s", err, layoutRegen)
	}
	defer f.Close()
	var out []layoutField
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		col := strings.Split(line, "\t")
		if len(col) != 4 || col[1] != "field" || col[0] != layoutClass {
			t.Fatalf("%s:%d: %q: a row is %s, field, Type.field, size", layoutPath, n, line, layoutClass)
		}
		size, err := strconv.ParseUint(col[3], 10, 64)
		if err != nil {
			t.Fatalf("%s:%d: %q: bad size", layoutPath, n, line)
		}
		out = append(out, layoutField{col[2], uintptr(size)})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// updateLayout rewrites the file in canonical order: the class's field rows
// in declaration order, one section per struct (a # struct line), so two PRs
// adding fields to different structs never touch the same lines.
func updateLayout(t *testing.T, fields []layoutField) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "# Generated fixed-State field inventory (layout_pins_test.go); do not edit by hand. Regenerate: %s\n", layoutRegen)
	b.WriteString("# class\tfield\tType.field\tsize\n")
	section := ""
	for _, f := range fields {
		if s, _, ok := strings.Cut(f.name, "."); ok && s != section {
			section = s
			fmt.Fprintf(&b, "# struct %s\n", section)
		}
		fmt.Fprintf(&b, "%s\tfield\t%s\n", layoutClass, f)
	}
	if err := os.WriteFile(layoutPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("rewrote the %s class's rows in %s", layoutClass, layoutPath)
}

// fixedStateFields is the inventory of the structs whose every field costs
// fixed bytes in every game (or every table's prep): State's core, the
// generated state, a permanent slot, the generated prep and the cold arrays'
// view headers. Field order is declaration order, struct by struct.
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
			out = append(out, layoutField{typ.Name() + "." + f.Name, f.Type.Size()})
		}
	}
	return out
}

// layoutDiff diffs the pinned inventory against the running one. A field
// keys by declared name (the k-th of a repeated blank name by its #k), so
// one insertion reads as one added field and the fields it pushed along are
// not differences; a resized field is. Nil when the inventories hold the
// same fields.
func layoutDiff(pin, got []layoutField) []string {
	key := func(fs []layoutField) ([]string, map[string]layoutField) {
		seen, keys, at := map[string]int{}, make([]string, len(fs)), map[string]layoutField{}
		for i, f := range fs {
			keys[i] = fmt.Sprintf("%s#%d", f.name, seen[f.name])
			seen[f.name]++
			at[keys[i]] = f
		}
		return keys, at
	}
	pinKeys, pinAt := key(pin)
	gotKeys, gotAt := key(got)
	var added, removed, resized []string
	for _, k := range gotKeys {
		f, was := gotAt[k], pinAt[k]
		switch _, ok := pinAt[k]; {
		case !ok:
			added = append(added, "+ "+f.String())
		case f.size != was.size:
			resized = append(resized, fmt.Sprintf("~ %s\t%d (pinned %d)", f.name, f.size, was.size))
		}
	}
	for _, k := range pinKeys {
		if _, ok := gotAt[k]; !ok {
			removed = append(removed, "- "+pinAt[k].String())
		}
	}
	return slices.Concat(added, removed, resized)
}

// TestNoNewFixedStateFields: the fields of stateCore, genState, perm, genPrep
// and coldArrays (declared name and size) are pinned in testdata/layout.tsv.
// New State bytes come from an arena key, a newCold region without a view
// header, or a designed construct region, never from a field of one of these
// structs; a deliberate layout change regenerates the inventory, and the
// review sees a one-file diff.
func TestNoNewFixedStateFields(t *testing.T) {
	got := fixedStateFields()
	if layoutUpdating() {
		updateLayout(t, got)
		return
	}
	pin := readLayout(t)
	if len(pin) == 0 {
		t.Fatalf("%s has no %s rows: regenerate with %s", layoutPath, layoutClass, layoutRegen)
	}
	diff := layoutDiff(pin, got)
	if len(diff) == 0 {
		return
	}
	const show = 40
	more := ""
	if len(diff) > show {
		more = fmt.Sprintf("\n  ... and %d more", len(diff)-show)
		diff = diff[:show]
	}
	t.Fatalf("fixed State changed: use an arena key or a cold region, or regenerate with %s for a deliberate layout change.\n"+
		"%s class against %s (+ added, - removed, ~ resized):\n  %s%s", layoutRegen, layoutClass, layoutPath, strings.Join(diff, "\n  "), more)
}

// TestLayoutInsertIsOneRow: the diff logic keys a row by declared field, not
// position, so a field inserted mid-struct is exactly one added row (the
// fields it pushed along are not differences), a resized field exactly one
// resized row, and a removed field exactly one removed row. This is what
// keeps two State-touching PRs off the same lines of testdata/layout.tsv.
func TestLayoutInsertIsOneRow(t *testing.T) {
	pin := fixedStateFields()
	// A named stateCore field (not a blank) away from the inventory's two
	// ends: the midpoint an insertion sits beside.
	var named []int
	for i, f := range pin {
		if f.name != "stateCore._" && strings.HasPrefix(f.name, "stateCore.") {
			named = append(named, i)
		}
	}
	if len(named) < 3 {
		t.Fatalf("stateCore has %d named fields, want several", len(named))
	}
	midAt := named[len(named)/2]
	mid := pin[midAt].name
	if midAt == 0 || midAt == len(pin)-1 {
		t.Fatalf("midpoint field %s sits at %d of %d rows, not mid-inventory", mid, midAt, len(pin))
	}
	// one field inserted mid-struct: exactly one + row
	ins := slices.Insert(slices.Clone(pin), midAt+1, layoutField{"stateCore.insertedField", 1})
	if d := layoutDiff(pin, ins); len(d) != 1 || !strings.HasPrefix(d[0], "+ stateCore.insertedField\t") {
		t.Fatalf("one inserted field: diff %q, want exactly one + row for it", d)
	}
	// a resized field: exactly one ~ row
	rs := slices.Clone(pin)
	rs[midAt].size += 8
	if d := layoutDiff(pin, rs); len(d) != 1 || !strings.HasPrefix(d[0], "~ "+mid+"\t") {
		t.Fatalf("one resized field: diff %q, want exactly one ~ row for %s", d, mid)
	}
	// a removed field: exactly one - row
	rm := slices.Delete(slices.Clone(pin), midAt, midAt+1)
	if d := layoutDiff(pin, rm); len(d) != 1 || !strings.HasPrefix(d[0], "- "+mid+"\t") {
		t.Fatalf("one removed field: diff %q, want exactly one - row for %s", d, mid)
	}
}
