package manifest

import (
	"fmt"
	"sort"
	"strings"
)

// CheckOptions loosen or tighten Check.
type CheckOptions struct {
	// AllowOrdinal names ordinals.tsv rows the change may alter or remove: a
	// deliberate legacy change the commit names (the cutover's
	// NumCounterKinds3 and NumCounterKindsX).
	AllowOrdinal []string
	// Final refuses a value: next entry: landing runs it after Assign.
	Final bool
}

// Check holds t, a change, to the manifest's invariants against base, the
// tree it branched from (invariant 1): t validates; no entry of
// base is deleted or renamed; an entry with a value at base keeps it, and of
// its lines only status and ticket change; a new entry does not take the name
// of a constant base already has (an existing constant gets no manifest
// file); every ordinals.tsv row of base is still there, unchanged; and so is every
// asks.tsv row, with no new one. It
// returns every violation, sorted, one line each.
func Check(base, t *Tree, o CheckOptions) []error {
	errs := t.Validate()
	for _, b := range base.Entries {
		e := t.Entry(b.Path)
		switch {
		case e == nil:
			errs = append(errs, fmt.Errorf("%s: deleted; a name is never deleted", b.Path))
		case b.Next:
			// Not yet assigned: anything may change but the name.
		case e.Next:
			errs = append(errs, fmt.Errorf("%s: value %d at the base is now next; a value never changes", b.Path, b.Value))
		default:
			if e.Value != b.Value {
				errs = append(errs, fmt.Errorf("%s: value %d at the base is now %d; a value never changes", b.Path, b.Value, e.Value))
			}
			for _, d := range []struct{ field, was, now string }{
				{"approval", b.Approval, e.Approval},
				{"forge", strings.Join(b.Forge, " | "), strings.Join(e.Forge, " | ")},
				{"census", b.Census, e.Census},
				{"state", b.State, e.State},
				{"doc", b.Doc, e.Doc},
			} {
				if d.was != d.now {
					errs = append(errs, fmt.Errorf("%s: %s changed from %q to %q; once a value is assigned only status and ticket change", b.Path, d.field, d.was, d.now))
				}
			}
		}
	}
	for _, e := range t.Entries {
		if base.Entry(e.Path) != nil {
			continue
		}
		if r, f := base.byName[e.Name], FamilyNamed(e.Family); r != nil && f != nil && f.Pkg == "kop" {
			errs = append(errs, fmt.Errorf("%s: %s is a hand %s constant at the base; an existing constant gets no manifest file", e.Path, e.Name, r.Type))
		}
	}
	allowed := map[string]bool{}
	for _, n := range o.AllowOrdinal {
		allowed[n] = true
	}
	for i := range base.Ordinals {
		r := &base.Ordinals[i]
		if allowed[r.Name] || base.provisional(r) {
			continue
		}
		now := t.row(r.Name)
		switch {
		case now == nil:
			errs = append(errs, fmt.Errorf("%s: row %s vanished; a constant is never deleted", OrdinalsPath, r.Name))
		case *now != *r:
			errs = append(errs, fmt.Errorf("%s: row %s %s %s at the base is now %s %s; a constant keeps its value", OrdinalsPath, r.Name, r.Type, r.Value, now.Type, now.Value))
		}
	}
	for _, b := range base.Asks {
		now := t.ask(b.Name)
		switch {
		case now == nil:
			errs = append(errs, fmt.Errorf("%s: row %s vanished; an ask kind is never deleted", AsksPath, b.Name))
		case *now != b:
			errs = append(errs, fmt.Errorf("%s: row %s value %d at the base is now %d; an ask kind keeps its value", AsksPath, b.Name, b.Value, now.Value))
		}
	}
	for _, a := range t.Asks {
		// A base with no asks.tsv is the one that predates it: the file's
		// first appearance freezes the hand constants.
		if base.Asks != nil && base.ask(a.Name) == nil {
			errs = append(errs, fmt.Errorf("%s: row %s is new; a new ask kind is a dirac/manifest/ask entry, not an asks.tsv row", AsksPath, a.Name))
		}
	}
	if o.Final {
		for _, e := range t.Entries {
			if e.Next {
				errs = append(errs, fmt.Errorf("%s: still value: next; run kmanifest assign", e.Path))
			}
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

// provisional reports whether ordinals row r is the constant of one of t's
// value: next entries: a provisional value, free to move until Assign.
func (t *Tree) provisional(r *Ordinal) bool {
	for _, e := range t.Entries {
		if e.Next && e.Name == r.Name && e.Family == r.Type {
			return true
		}
	}
	return false
}
