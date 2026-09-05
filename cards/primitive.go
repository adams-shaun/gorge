package cards

import "sort"

// Primitives lists every engine symbol this face needs, prefixed by kind. The
// result is sorted so it is stable across runs — coverage reports and the IR
// cache both depend on that. Because Link runs expandKeywords first
// (keywords.go), an expanded keyword's own triggers/replacements/abilities
// are already on the Face by the time this walks it: a Batterskull lists
// not just kw:Living Weapon and kw:Equip but also the api:Token, api:Attach
// and trig:ChangesZone its expansion needs, exactly as if those lines had
// been printed in the script by hand.
func (f *Face) Primitives() []string {
	set := map[string]struct{}{}
	var walk func(sa *SA, depth int)
	walk = func(sa *SA, depth int) {
		if sa == nil || depth > maxSVarDepth {
			return
		}
		set["api:"+sa.API] = struct{}{}
		walk(sa.Sub, depth+1)
	}
	for _, a := range f.Abilities {
		walk(a, 0)
	}
	for _, t := range f.Triggers {
		set["trig:"+t.Mode] = struct{}{}
		walk(t.Effect, 0)
	}
	for _, s := range f.Statics {
		set["stat:"+s.Mode] = struct{}{}
	}
	for _, r := range f.Repls {
		set["repl:"+r.Event] = struct{}{}
		walk(r.With, 0)
	}
	for _, k := range f.Keywords {
		set["kw:"+KeywordHead(k)] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Primitives is the union across every face.
func (c *Card) Primitives() []string {
	set := map[string]struct{}{}
	for _, f := range c.Faces {
		for _, p := range f.Primitives() {
			set[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
