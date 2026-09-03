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
	Abilities []*SA
	Triggers  []Trigger
	Statics   []Static
	Repls     []Repl
	SVars     map[string]string
}

// Card is one script file.
type Card struct {
	Path  string
	Faces []*Face
}

// Diag is a non-fatal parse complaint. The whole corpus is expected to produce
// under ten of these; a jump means either a parser regression or an upstream
// data change worth looking at.
type Diag struct {
	Path string
	Msg  string
}

func newFace() *Face { return &Face{SVars: map[string]string{}} }
