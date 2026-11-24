package gorgebe

import "github.com/adams-shaun/gorge/engine/cards"

// ParseScript parses text, links it and applies intrinsics. Diags from the
// parse and the link are returned together.
func ParseScript(path string, text []byte) (*cards.Card, []cards.Diag) {
	c, diags := cards.ParseBytes(path, text)
	if c == nil {
		return nil, diags
	}
	diags = append(diags, c.Link()...)
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c, diags
}

// Bind gives cs catalog bindings in a private registry. Never pass Forge
// cards or tokens: CompileMetadata rebinds every face it is given.
func Bind(cs []*cards.Card) error {
	r := cards.NewRegistry()
	for _, c := range cs {
		r.Add(c)
	}
	return r.CompileMetadata()
}
