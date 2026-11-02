package kgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// dumpPool writes one readable text file per pool card holding its full
// compiled IR (a study aid; the output contains Forge script text and must
// never be committed).
func dumpPool(p *pool, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, n := range p.names {
		f, err := os.Create(filepath.Join(dir, fileStem(n)+".txt"))
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "CARD %s  (decks %d)\n", n, p.nDecks[n])
		dumpCard(f, p.byName[n])
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func dumpCard(w io.Writer, c *cards.Card) {
	fmt.Fprintf(w, "path=%s alt=%q\n", filepath.Base(c.Path), c.AlternateMode)
	for i, f := range c.Faces {
		fmt.Fprintf(w, "== face %d %q\n", i, f.Name)
		fmt.Fprintf(w, "ManaCost=%q Types=%q PT=%q Loyalty=%q Colors=%q\n", f.ManaCost, f.Types, f.PT, f.Loyalty, f.Colors)
		fmt.Fprintf(w, "Oracle=%q\n", f.Oracle)
		for _, k := range f.Keywords {
			fmt.Fprintf(w, "K: %s\n", k)
		}
		for j, a := range f.Abilities {
			fmt.Fprintf(w, "A[%d]:\n", j)
			dumpSA(w, a, 1)
		}
		for j, t := range f.Triggers {
			fmt.Fprintf(w, "T[%d]: Mode=%s %s\n", j, t.Mode, paramString(t.Params))
			dumpSA(w, t.Effect, 1)
		}
		for j, s := range f.Statics {
			fmt.Fprintf(w, "S[%d]: Mode=%s %s\n", j, s.Mode, paramString(s.Params))
		}
		for j, r := range f.Repls {
			fmt.Fprintf(w, "R[%d]: Event=%s %s\n", j, r.Event, paramString(r.Params))
			dumpSA(w, r.With, 1)
		}
		var sv []string
		for k := range f.SVars {
			sv = append(sv, k)
		}
		sort.Strings(sv)
		for _, k := range sv {
			fmt.Fprintf(w, "SVar %s = %s\n", k, f.SVars[k])
		}
	}
}

func dumpSA(w io.Writer, sa *cards.SA, depth int) {
	for ; sa != nil; sa = sa.Sub {
		fmt.Fprintf(w, "%s%s$ %s %s\n", strings.Repeat("  ", depth), sa.Kind, sa.API, paramString(sa.Params))
		depth++
	}
}

func paramString(m map[string]string) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	for _, k := range ks {
		fmt.Fprintf(&b, "| %s$ %s ", k, m[k])
	}
	return b.String()
}

// fileStem is a card name as a lower-case identifier-ish stem.
func fileStem(n string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(n) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('_')
		}
	}
	return b.String()
}
