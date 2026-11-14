package manifest

import (
	"fmt"
	"strings"
)

// Find is the entries a seat's query names, in tree order: a Go name, a
// <Family>/<Name> pair, an entry path, a refusal class
// (manifest:<status>:<Family>/<Name>) or a forge token, matched without case.
func (t *Tree) Find(q string) []*Entry {
	q = strings.TrimSpace(q)
	if rest, ok := strings.CutPrefix(q, "manifest:"); ok {
		if _, fn, ok := strings.Cut(rest, ":"); ok {
			q = fn
		}
	}
	var out []*Entry
	for _, e := range t.Entries {
		hit := q == e.Name || q == e.Family+"/"+e.Name || q == e.Path
		for _, tok := range e.Forge {
			hit = hit || strings.EqualFold(q, tok)
		}
		if hit {
			out = append(out, e)
		}
	}
	return out
}

// Show is e for a seat: the path, value, status and State category on the
// first line (a Next entry's value is "next (provisional N)"), then its other
// lines, indented.
func (e *Entry) Show() string {
	var b strings.Builder
	v := fmt.Sprint(e.Value)
	if e.Next {
		v = fmt.Sprintf("next (provisional %d)", e.Value)
	}
	fmt.Fprintf(&b, "%s value %s %s", e.Path, v, e.Status)
	if e.State != "" {
		fmt.Fprintf(&b, " state %s", e.State)
	}
	b.WriteByte('\n')
	for _, f := range []struct{ k, v string }{
		{"forge", strings.Join(e.Forge, " | ")},
		{"census", e.Census},
		{"approval", e.Approval},
		{"ticket", e.Ticket},
		{"doc", e.Doc},
	} {
		if f.v != "" {
			fmt.Fprintf(&b, "\t%s: %s\n", f.k, f.v)
		}
	}
	return b.String()
}
