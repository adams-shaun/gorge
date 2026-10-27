package cards

import "maps"

// TextSub is one CR 612 text-changing substitution: every instance of From
// used as a color word or a subtype word is read as To (api:ChangeText's
// ChangeColorWord$/ChangeTypeWord$ pair).
type TextSub struct {
	From, To string
	// UntilEOT marks a change that ends in the cleanup step (CR 514.2):
	// api:ChangeText without Duration$ Permanent. WithTextChanges ignores it.
	UntilEOT bool
}

// textLockedKeys are the parameters whose values are never text-changed:
// names and script references, not rules-text words (CR 612.2: a text
// change cannot change a name; Forge's CardTraitBase noChangeKeys plus the
// keys that name SVars).
var textLockedKeys = NewNameSet("TokenScript", "NewName", "DefinedName", "ChooseFromList",
	"AddAbility", "Execute", "SubAbility", "ReplaceWith", "Abilities", "Triggers",
	"SVars", "Choices", "AddTrigger", "AddStaticAbility", "AddReplacementEffect",
	"AddSVar", "AddSVars", "StaticAbilities", "Replacements", "RepeatSubAbility")

// lockInTextParam is Forge's LockInText$ marker. Forge's
// CardTraitBase.hasParam is presence-based, and
// AbilityUtils.applyAbilityTextChangeEffects / applyDescriptionTextChangeEffects
// return the text unchanged for such an ability: the words the script spells
// (a per-colour filter chain, for example) are not words printed in the card's
// Oracle text, so CR 612.2 has nothing to change.
const lockInTextParam = "LockInText"

func textChangeLockedSA(s *SA) bool         { _, ok := s.Params[lockInTextParam]; return ok }
func textChangeLockedStatic(s *Static) bool { _, ok := s.Params[lockInTextParam]; return ok }
func textChangeLockedBody(body string) bool { _, ok := parseParams(body)[lockInTextParam]; return ok }

// WithTextChanges returns a copy of f whose text reads with subs applied in
// order (CR 612.1/612.2): the type line, keywords, Oracle text, SVar bodies
// and every parameter of every ability, trigger, static and replacement
// (sub-ability chains included). A locked key (textLockedKeys), a value
// that names one of f's SVars, and a `LockInText$` ability, SVar body or
// static (Forge's per-ability lock: lockInTextParam) are kept verbatim. f is
// never mutated; nil f or empty subs returns f itself. The copy shares f's
// slots and keeps no compiled catalog row or ParamSet, so every reader takes
// the text path.
// No derive(): it records slice identities as uintptrs, which would make the
// face differ between a live game and its log-only replay (see
// face_runtime.go); the identity-guarded caches (printed facts, type words,
// keyword heads, the type-static probe) read the new slices' slow path.
func (f *Face) WithTextChanges(subs []TextSub) *Face {
	if f == nil || len(subs) == 0 {
		return f
	}
	t := textRewriter{subs: subs, f: f}
	g := *f
	g.ForgetCompiled()
	g.Oracle = t.word(f.Oracle)
	g.Types = t.words(f.Types)
	g.Keywords = t.words(f.Keywords)
	g.SVars = maps.Clone(f.SVars)
	for k, v := range g.SVars {
		// An SVar body carrying LockInText$ is left verbatim: it is compiled
		// into an ability after this point, and Forge's text change never
		// reaches such an ability. parseParams is the same pipe grammar the
		// body is compiled with, so no substring test can disagree with it.
		if !textChangeLockedBody(v) {
			g.SVars[k] = t.word(v)
		}
	}
	g.Abilities = make([]*SA, len(f.Abilities))
	for i, a := range f.Abilities {
		g.Abilities[i] = t.sa(a, 0)
	}
	g.Triggers = append([]Trigger(nil), f.Triggers...)
	for i := range g.Triggers {
		tr := &g.Triggers[i]
		tr.Params, tr.ps = maps.Clone(tr.Params), nil
		for k, v := range tr.Params {
			tr.Params[k] = t.param(k, v)
		}
		tr.Effect = t.sa(tr.Effect, 0)
	}
	g.Statics = append([]Static(nil), f.Statics...)
	for i := range g.Statics {
		st := &g.Statics[i]
		st.Params, st.ps = maps.Clone(st.Params), nil
		if textChangeLockedStatic(st) {
			continue
		}
		for k, v := range st.Params {
			st.Params[k] = t.param(k, v)
		}
	}
	g.Repls = append([]Repl(nil), f.Repls...)
	for i := range g.Repls {
		r := &g.Repls[i]
		r.Params, r.ps = maps.Clone(r.Params), nil
		for k, v := range r.Params {
			r.Params[k] = t.param(k, v)
		}
		r.With = t.sa(r.With, 0)
	}
	g.typeStaticsFirst, g.typeStaticsLen, g.typeStaticsBound = nil, 0, false
	return &g
}

// textRewriter applies one object's substitutions to IR text; f is the
// face being rewritten (its SVar names are never rewritten as values).
type textRewriter struct {
	subs []TextSub
	f    *Face
}

func (t textRewriter) word(s string) string {
	for _, x := range t.subs {
		s = SubstituteTextWord(s, x.From, x.To)
	}
	return s
}

func (t textRewriter) words(in []string) []string {
	out := make([]string, len(in))
	for i, w := range in {
		out[i] = t.word(w)
	}
	return out
}

// param is parameter k's rewritten value: a locked key and a value naming
// one of the face's SVars are kept.
func (t textRewriter) param(k, v string) string {
	if _, svar := t.f.SVars[v]; textLockedKeys.Has(k) || svar {
		return v
	}
	return t.word(v)
}

func (t textRewriter) sa(s *SA, depth int) *SA {
	if s == nil || depth > maxSVarDepth+1 {
		return s
	}
	c := *s
	c.Params, c.ps = maps.Clone(s.Params), nil
	if textChangeLockedSA(s) {
		// Forge leaves a LockInText$ ability's own text verbatim; the chain
		// is still walked because each link carries its own lock.
		c.compiledCatalog, c.compiledID = nil, 0
		c.Sub = t.sa(s.Sub, depth+1)
		return &c
	}
	for k, v := range c.Params {
		c.Params[k] = t.param(k, v)
	}
	// The compiled catalog row describes the printed text; the slots stay
	// shared (Slot is keyed by text, ExtSlot facts name their node).
	c.compiledCatalog, c.compiledID = nil, 0
	c.Line = t.word(s.Line)
	c.Sub = t.sa(s.Sub, depth+1)
	return &c
}

// SubstituteTextWord is CR 612.2's word replacement over one string: each
// occurrence of from that starts a word (string start or a non-letter
// before it, optionally behind a "non" prefix -- "nonBlack", "non-Black"
// keep the negation) and ends one (string end, a non-letter, or the
// landwalk/plural suffixes "walk" and "s": Swampwalk, Swamps) is replaced by
// to, matched case-insensitively and written in the matched word's case
// (all-lower stays lower, a capital first letter stays capital). An
// occurrence inside a "named<Card Name>" word -- "named" in the 100 bytes
// before it with no '+', '|' or '&' after that "named" -- is a card name and
// is kept (CR 612.2). An empty from returns text unchanged. ASCII only (the
// corpus's color and type words are ASCII).
func SubstituteTextWord(text, from, to string) string {
	if from == "" || len(text) < len(from) {
		return text
	}
	var out []byte
	last := 0
	for i := 0; i+len(from) <= len(text); i++ {
		if !textFoldAt(text, i, from) {
			continue
		}
		end := i + len(from)
		if !textWordStart(text, i) || !textWordEnd(text, end) || inCardName(text, i) {
			continue
		}
		out = append(out, text[last:i]...)
		out = textCased(out, to, text[i])
		last = end
		i = end - 1
	}
	if out == nil {
		return text
	}
	out = append(out, text[last:]...)
	return string(out)
}

func textLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func textLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// textFoldAt reports whether text[i:] starts with w, ASCII case-insensitively.
func textFoldAt(text string, i int, w string) bool {
	if i+len(w) > len(text) {
		return false
	}
	for j := 0; j < len(w); j++ {
		if textLower(text[i+j]) != textLower(w[j]) {
			return false
		}
	}
	return true
}

// textCased appends to in the case of the matched word's first byte.
func textCased(out []byte, to string, first byte) []byte {
	for j := 0; j < len(to); j++ {
		c := to[j]
		switch {
		case first >= 'a' && first <= 'z':
			c = textLower(c)
		case j == 0 && c >= 'a' && c <= 'z':
			c -= 'a' - 'A'
		}
		out = append(out, c)
	}
	return out
}

// textWordStart: start begins a word, directly (a "non-" hyphen is a
// non-letter) or behind a "non" prefix that itself begins one.
func textWordStart(text string, start int) bool {
	if start == 0 || !textLetter(text[start-1]) {
		return true
	}
	if start >= 3 && textFoldAt(text, start-3, "non") {
		return start == 3 || !textLetter(text[start-4])
	}
	return false
}

func textWordEnd(text string, end int) bool {
	if end >= len(text) || !textLetter(text[end]) {
		return true
	}
	for _, suf := range [...]string{"walk", "s"} {
		if textFoldAt(text, end, suf) && (end+len(suf) == len(text) || !textLetter(text[end+len(suf)])) {
			return true
		}
	}
	return false
}

// inCardName reports whether the match at i sits inside a "named<Card
// Name>" filter word: "named" occurs in the 100 bytes before i and no filter
// separator ('+', '|', '&') follows the last such occurrence.
func inCardName(text string, i int) bool {
	lo := i - 100
	if lo < 0 {
		lo = 0
	}
	for k := i - len("named"); k >= lo; k-- {
		if textFoldAt(text, k, "named") {
			for _, c := range []byte(text[k:i]) {
				if c == '+' || c == '|' || c == '&' {
					return false
				}
			}
			return true
		}
	}
	return false
}
