package gorgebe

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbCreateToken, printCreateToken) }

func printCreateToken(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Token == nil {
		return nil, 0, unprintable("verb:create-token", st.Src)
	}
	if len(steps) > 1 && steps[1].Verb == ir.VerbCreateToken {
		// Forge chains one Token node with a TokenScript list for a run of tokens.
		return nil, 0, unprintable("shape:consecutive-tokens", st.Src)
	}
	if e := st.Enter; e != nil && (e.UnderOwner || e.Controller != nil || e.Counter != "" || e.N != nil || len(e.Effects) > 0) {
		return nil, 0, unprintable("token:enter", st.Src)
	}
	amt, err := p.amount(st.N)
	if err != nil {
		return nil, 0, err
	}
	key, err := p.tokenKey(st.Token, st.Src)
	if err != nil {
		return nil, 0, err
	}
	owner := "You"
	if st.Who != nil {
		if owner, err = p.defined(st.Who); err != nil {
			return nil, 0, err
		}
	}
	n := &node{API: "Token", Params: []kv{{"TokenAmount", amt}, {"TokenScript", key}, {"TokenOwner", owner}}}
	if st.Enter != nil && st.Enter.Tapped {
		n.set("TokenTapped", "True")
	}
	if st.Optional {
		n.set("OptionalDecider", "You")
	}
	return n, 1, nil
}

// tokenKey returns the first key, in sorted order, of a Forge token script
// whose front face matches f.
func (p *pr) tokenKey(f *ir.Face, src string) (string, error) {
	kws := map[string]bool{} // membership only, never ranged
	nkw := 0
	for _, a := range f.Abilities {
		if a.Kind != ir.AbKeyword || a.Keyword == nil {
			return "", unprintable("token:abilities", src)
		}
		k := a.Keyword
		if k.N != nil || k.Cost != nil || k.Filter != nil {
			return "", unprintable("token:keyword-args", src)
		}
		if !kws[k.Name] {
			nkw++
		}
		kws[k.Name] = true
	}
	if p.tokens == nil {
		return "", unprintable("token:no-registry", src)
	}
	var want []string
	for _, l := range [][]string{f.Supertypes, f.Types, f.Subtypes} {
		for _, w := range l {
			want = append(want, Respell(w))
		}
	}
	sort.Strings(want)
	var colours []string
	for i, n := range []string{"white", "blue", "black", "red", "green"} {
		if f.ColorIndicator&(1<<i) != 0 {
			colours = append(colours, n)
		}
	}
	name := f.Name
	if name == "" {
		var subs []string
		for _, w := range f.Subtypes {
			subs = append(subs, Respell(w))
		}
		name = strings.Join(subs, " ")
	}
	keys := make([]string, 0, len(p.tokens))
	for k := range p.tokens {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := p.tokens[k]
		if c == nil || len(c.Faces) == 0 {
			continue
		}
		g := c.Faces[0]
		if f.Name != "" && g.Name != name && g.Name != name+" Token" {
			continue
		}
		if f.Name == "" && (g.Name != name+" Token" || len(g.Abilities) > 0 || len(g.Triggers) > 0 || len(g.Statics) > 0) {
			continue
		}
		if !sameStrings(tokenColours(g.Colors), colours) || !sameStrings(g.Types, want) {
			continue
		}
		if f.Power != nil && f.Toughness != nil && g.PT != pt(f.Power)+"/"+pt(f.Toughness) {
			continue
		}
		if !keywordsMatch(g.Keywords, kws, nkw) {
			continue
		}
		return k, nil
	}
	return "", unprintable("token:unmatched", src)
}

func tokenColours(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' })
}

// sameStrings compares a and b as multisets.
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// keywordsMatch reports whether got and want are the same set.
func keywordsMatch(got []string, want map[string]bool, nwant int) bool {
	seen := map[string]bool{}
	for _, k := range got {
		if !want[k] {
			return false
		}
		seen[k] = true
	}
	return len(seen) == nwant
}
