package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Assignment is a final value Assign wrote.
type Assignment struct {
	Path  string
	Value int64
}

// Assign gives every value: next entry of the module at root its final value
// and writes it into the entry's file (only the value line changes), serially
// against base, the landing branch's tip. A value is the lowest free one of
// the family's Alloc band that neither the tree nor base holds, so a change
// not yet rebased past an entry the branch took cannot take that entry's
// value. It refuses an entry that has a value at base: a value never
// changes. Run WriteGen after it; nothing has seen a provisional value, so
// moving one is safe.
func Assign(root string, base *Tree) ([]Assignment, error) {
	t, err := Read(os.DirFS(root))
	if err != nil {
		return nil, err
	}
	held, errs := t.validate()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	extra, err := base.legacyTaken()
	if err != nil {
		return nil, err
	}
	for _, b := range base.Entries {
		if f := b.Fam(); f != nil && !b.Next {
			extra[f.Name].add(f, b.Value, "base "+b.Path)
		}
	}
	for _, e := range t.Entries {
		if !e.Next {
			continue
		}
		if b := base.Entry(e.Path); b != nil && !b.Next {
			errs = append(errs, fmt.Errorf("%s: has value %d at the base; a value never changes: write it back", e.Path, b.Value))
		} else if r := base.byName[e.Name]; r != nil && b == nil {
			errs = append(errs, fmt.Errorf("%s: %s is a %s constant at the base (%s)", e.Path, e.Name, r.Type, OrdinalsPath))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	if errs := t.resolve(held, extra); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	var out []Assignment
	for _, e := range t.Entries {
		if !e.Next {
			continue
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(e.Path)), e.withValue(e.Value), 0o644); err != nil {
			return out, err
		}
		e.Next = false
		out = append(out, Assignment{e.Path, e.Value})
	}
	return out, nil
}

// withValue is e's file with its value line set to v; every other byte is
// kept.
func (e *Entry) withValue(v int64) []byte {
	var out bytes.Buffer
	for _, line := range strings.SplitAfter(string(e.raw), "\n") {
		if key, _, ok := strings.Cut(line, ":"); ok && key == "value" {
			nl := ""
			if strings.HasSuffix(line, "\n") {
				nl = "\n"
			}
			line = "value: " + strconv.FormatInt(v, 10) + nl
		}
		out.WriteString(line)
	}
	return out.Bytes()
}
