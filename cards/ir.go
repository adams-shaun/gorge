package cards

// SA is a spell ability, activated ability, static ability or drawback: the
// "SP$ / AB$ / DB$ / ST$ <API> | Param$ value | ..." construct that carries
// almost all card behaviour in the Forge corpus.
type SA struct {
	Kind   string // SP, AB, DB or ST
	API    string
	Params map[string]string
	Sub    *SA // resolved SubAbility$, filled in by link.go
	Line   string

	compiledCatalog *CompiledCatalog
	compiledID      AbilityID
}

// Trigger is a T: line. Execute$ names an SVar holding the effect.
type Trigger struct {
	Mode   string
	Params map[string]string
	Effect *SA
}

// Static is an S: line: a continuous effect or a play restriction.
type Static struct {
	Mode   string
	Params map[string]string
}

// Repl is an R: line: a replacement effect. ReplaceWith$ names an SVar.
type Repl struct {
	Event  string
	Params map[string]string
	With   *SA
}

// Face is one printed face. Most cards have exactly one; ALTERNATE starts
// another.
type Face struct {
	Name      string
	ManaCost  string
	Types     []string
	PT        string
	Loyalty   string
	Defense   string
	Colors    string
	Oracle    string
	Keywords  []string
	Aliases   []string // Universes-Within flavour names a decklist may use
	Abilities []*SA
	Triggers  []Trigger
	Statics   []Static
	Repls     []Repl
	SVars     map[string]string

	// Derived values, computed once at load (Face.derive), never written
	// into the gob cache: a stale cache decodes these as zero and derive
	// repairs them immediately after decode, so Power(), Toughness() and
	// Cmc() read parsed values instead of re-parsing the printed text on
	// every call.
	power                  int32
	toughness              int32
	characteristicDefining bool
	allCreatureTypesCDA    bool
	cmc                    int32
	manaProduction         ManaProduction

	// colourIdentity is the face's colour identity, a bitmask over the five
	// colours (see ColourIdentity in face.go), derived once at load from the
	// printed text and never written into the gob cache: like the other
	// derived values below it stays unexported so a stale cache decodes it as
	// zero and derive() repairs it immediately after decode.
	colourIdentity uint8
	// compiledTriggerInterests is bound from the immutable catalog at load
	// time. Keeping this hot prefilter beside the legacy face avoids a catalog
	// slice lookup during every trigger scan; it is not serialized.
	compiledTriggerInterests TriggerInterest

	compiledCatalog *CompiledCatalog
	compiledID      FaceID
}

func (f *Face) CompiledID() FaceID {
	if f == nil || f.compiledCatalog == nil {
		return 0
	}
	return f.compiledID
}

func (f *Face) CompiledTriggerInterests() (TriggerInterest, bool) {
	if f == nil || f.compiledCatalog == nil || f.compiledID == 0 {
		return 0, false
	}
	return f.compiledTriggerInterests, true
}

func (s *SA) CompiledKind() SAKind {
	if s == nil || s.compiledCatalog == nil {
		return SAKindUnknown
	}
	return s.compiledCatalog.Abilities[s.compiledID-1].Kind
}

func (s *SA) CompiledAPI() APICode {
	if s == nil || s.compiledCatalog == nil {
		return APIUnknown
	}
	return s.compiledCatalog.Abilities[s.compiledID-1].API
}

// ColourIdentity returns the card's colour identity, the bitwise union of
// every face's (so a double-faced card carries the colours of both sides). A
// commander's identity must be a superset of it for the card to be legal.
func (c *Card) ColourIdentity() uint8 {
	var m uint8
	for _, f := range c.Faces {
		m |= f.ColourIdentity()
	}
	return m
}

// SetsName reports whether any face prints a `S:Mode$ Continuous | SetName$`
// static (CR 613.1d, layer 3). It is a card-data question the rules engine
// asks ONCE, at genesis, over the match's card pool: a match whose pool has no
// such carrier can never have a layer-3 rename, so the engine skips
// maintaining its rename table entirely (rules/setname.go). It lives here,
// with the IR it reads, rather than in rules -- it is a capability probe over
// printed script text, not a parameter read on a resolving primitive's path.
func (c *Card) SetsName() bool {
	if c == nil {
		return false
	}
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		for _, st := range f.Statics {
			if _, ok := st.Params["SetName"]; ok {
				return true
			}
		}
	}
	return false
}

// Card is one script file. AlternateMode describes how its faces relate;
// name-characteristic rules distinguish split cards from transforming DFCs.
type Card struct {
	Path          string
	AlternateMode string
	Faces         []*Face
}

// Diag is a non-fatal parse complaint. The whole corpus is expected to produce
// under ten of these; a jump means either a parser regression or an upstream
// data change worth looking at.
type Diag struct {
	Path string
	Msg  string
}

func newFace() *Face { return &Face{SVars: map[string]string{}} }
