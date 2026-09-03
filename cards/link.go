package cards

// maxSVarDepth caps sub-ability nesting. The deepest real chain in the corpus
// is well under this; the cap exists so a cyclic SVar reference in upstream
// data cannot hang the compiler.
const maxSVarDepth = 32

// Link resolves SVar-named sub-abilities into a tree. Call it once, after
// Parse, before the card is used.
func (c *Card) Link() []Diag {
	var diags []Diag
	for _, f := range c.Faces {
		diags = append(diags, f.link(c.Path)...)
	}
	return diags
}

func (f *Face) link(path string) []Diag {
	var diags []Diag

	var resolve func(name string, depth int) *SA
	resolve = func(name string, depth int) *SA {
		if name == "" || depth > maxSVarDepth {
			return nil
		}
		body, ok := f.SVars[name]
		if !ok {
			diags = append(diags, Diag{path, "unresolved SVar ref: " + name})
			return nil
		}
		sa, d := parseSA(path, body)
		diags = append(diags, d...)
		if sa != nil {
			sa.Sub = resolve(sa.Params["SubAbility"], depth+1)
		}
		return sa
	}
	walk := func(sa *SA) {
		for d := 0; sa != nil && d <= maxSVarDepth; d++ {
			if sa.Sub == nil {
				sa.Sub = resolve(sa.Params["SubAbility"], d+1)
			}
			sa = sa.Sub
		}
	}

	for _, a := range f.Abilities {
		walk(a)
	}
	for i := range f.Triggers {
		f.Triggers[i].Effect = resolve(f.Triggers[i].Params["Execute"], 0)
		walk(f.Triggers[i].Effect)
	}
	for i := range f.Repls {
		f.Repls[i].With = resolve(f.Repls[i].Params["ReplaceWith"], 0)
		walk(f.Repls[i].With)
	}
	return diags
}
