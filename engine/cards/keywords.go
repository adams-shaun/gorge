package cards

import (
	"strconv"
	"strings"
)

// expandKeywords turns each keyword the engine implements through ordinary
// machinery into the triggered ability, replacement effect or activated
// ability Forge itself expands it to (CardFactoryUtil, in spirit), tagged
// Params["Keyword"] so nothing downstream needs to know the difference. The
// SVars it adds start with "__kw" and cannot collide with a script's own.
// Idempotent per keyword LINE INSTANCE, not per line text: a second Link()
// adds nothing, but a face that prints the same K: line twice expands it
// twice (CR 702.108b for prowess, and the Oracle text of Scurry of Squirrels,
// "Myriad, myriad", or Evercoat Ursine, "Hideaway 3, hideaway 3"). Forge
// expands every K: line independently, so matching it means each instance is
// its own expansion. Two distinct K:Equip: lines (different costs or
// restrictions) still expand both. Keywords whose meaning is
// a casting option (Kicker, Surge, Flashback, Delve, Flash, Miracle) or a
// static property (Protection, Indestructible, Devoid) are not expanded:
// rules reads them directly.
func (f *Face) expandKeywords() {
	// The idempotence key is (kind, keyword line text). Lines expanded by a
	// PRIOR Link() pass are recorded in pre before this pass starts; a line
	// expanded earlier in THIS pass is deliberately NOT recorded, so two
	// equal lines at different indices each expand. Within one pass every
	// has() therefore answers false for a line this pass has not yet added,
	// which is what lets an expander that checks has() and then delegates
	// to addKeywordTrigger (which checks has() again) append exactly once.
	pre := f.expandedKeywordKeys()
	// has reports whether a PREVIOUS pass already produced a T:/R:/A:/S:
	// entry for tag. It does not consult this pass' own additions, so a
	// second identical line in the same pass is a second instance (CR
	// 702.108b) while a repeat Link() adds nothing.
	has := func(kind, tag string) bool {
		return pre[kind+"\x00"+tag]
	}
	for i, k := range f.Keywords {
		head := KeywordHead(k)
		param := ""
		if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		// Per-keyword dispatch: see kwExpanders. A head with no registered
		// expander is not expanded, which is what the switch did by having no
		// default arm -- rules reads those keywords directly.
		if fn := kwExpanders[head]; fn != nil {
			fn(f, i, k, head, param, has)
		}
	}
}

// expandedKeywordKeys collects the (kind, KeywordLine) tags of every T:/R:/
// A:/S: entry already on the face, so expandKeywords can treat a prior Link()
// pass' expansions as already present while a current pass' own additions do
// not suppress a second identical line. KeywordLine is the tag every expander
// sets alongside Keyword; it is never set on printed (hand-written) abilities,
// so a printed T: line cannot be mistaken for a prior expansion.
func (f *Face) expandedKeywordKeys() map[string]bool {
	keys := map[string]bool{}
	add := func(kind, line string) {
		if line != "" {
			keys[kind+"\x00"+line] = true
		}
	}
	for _, t := range f.Triggers {
		add("T", t.Params["KeywordLine"])
	}
	for _, r := range f.Repls {
		add("R", r.Params["KeywordLine"])
	}
	for _, a := range f.Abilities {
		add("A", a.Params["KeywordLine"])
	}
	for _, s := range f.Statics {
		add("S", s.Params["KeywordLine"])
	}
	return keys
}

// kwExpander expands one keyword line onto the face.
//
// The parameters are what the switch arms this replaced closed over: f is the
// face, i the keyword's index in f.Keywords (mint __kw SVar names from it so
// two lines of the same keyword cannot collide), k the FULL keyword line
// verbatim, head and param its split halves, and has the idempotence check --
// whether this exact LINE already produced a T:/R:/A: entry, which is what
// lets a face with two distinct K:Equip: lines expand both
// while a second Link() call adds nothing.
type kwExpander func(f *Face, i int, k, head, param string, has func(kind, line string) bool)

// kwExpanders maps a keyword head to its expansion. A head with no entry is
// not expanded -- the switch had no default arm either, because keywords whose
// meaning is a casting option or a static property are read directly by rules.
var kwExpanders = map[string]kwExpander{}

// registerKeyword installs fn for each named head. Called from init() in the
// per-keyword kw_*.go files.
//
// A duplicate registration panics rather than silently replacing: the point of
// the split is that many tickets edit different files at once, so two files
// claiming one head must be loud at startup, not an expansion that quietly
// stopped being reached.
func registerKeyword(fn kwExpander, heads ...string) {
	for _, head := range heads {
		if _, dup := kwExpanders[head]; dup {
			panic("cards: duplicate keyword expander registered for " + head)
		}
		kwExpanders[head] = fn
	}
}

// KeywordExpandable reports whether head has a registered expander, without
// touching a face. It is the compile-time gate the Clone conditional-keyword
// rider's whitelist runs (engine/effects/clone_params.go): only a head whose
// expansion this package can mint on demand may be granted.
func KeywordExpandable(head string) bool {
	return kwExpanders[head] != nil
}

// ApplyKeywordExpansion appends line to f.Keywords and runs the registered
// expander for its head against the face, exactly as printed expansion
// (expandKeywords) would for a card carrying the line: same index, same
// __kw SVar minting, and the same idempotence answer -- a T:/R: entry a
// PRIOR expansion of the same line produced suppresses this one, while this
// call's own additions do not. It is the fold-time twin of the compile-time
// pass: a Clone's conditional keyword rider (Flesh Duplicate's "vanishing 3
// if that creature doesn't have vanishing", CR 707.9b) is folded onto the
// copy basis face inside events.Apply, so a replay rebuilds the identical
// face from the identical event and every downstream reader -- the entry
// counter fold, the trigger walk, the keyword census -- sees a printed-
// shaped face. False (and nothing appended) when line's head has no
// registered expander; the compile whitelist admits only expandable heads,
// so the fold-time false is defensive.
func (f *Face) ApplyKeywordExpansion(line string) bool {
	head := KeywordHead(line)
	fn := kwExpanders[head]
	if fn == nil {
		return false
	}
	// The idempotence pre-set is built BEFORE the append, exactly as
	// expandKeywords builds it before its pass: has() must answer false for
	// what this call is about to add.
	pre := f.expandedKeywordKeys()
	i := len(f.Keywords)
	t0, r0 := len(f.Triggers), len(f.Repls)
	f.Keywords = append(f.Keywords, line)
	param := ""
	if j := strings.IndexByte(line, ':'); j >= 0 {
		param = strings.TrimSpace(line[j+1:])
	}
	fn(f, i, line, head, param, func(kind, tag string) bool {
		return pre[kind+"\x00"+tag]
	})
	// expandKeywords' callers link the whole face afterwards (Face.link
	// binds Trigger.Effect and Repl.With from the SVar names), but a
	// RUNTIME expansion has no link pass to ride: bind the minted lines
	// here, or their bodies stay unresolvable (an unlinked trigger can
	// never be pushed, an unlinked repl never matched by a With-reader).
	f.linkMinted(t0, r0)
	return true
}

// linkMinted binds Trigger.Effect and Repl.With from Execute$/ReplaceWith$
// over the face's SVar table for the slice ranges [t0:len) and [r0:len),
// resolving SubAbility$ chains as Face.link does. Diags are dropped: a
// runtime expansion minted the SVar itself one line earlier, so a miss is
// impossible in practice and the caller fails closed on a nil body.
func (f *Face) linkMinted(t0, r0 int) {
	var resolve func(name string, depth int) *SA
	resolve = func(name string, depth int) *SA {
		if name == "" || depth > maxSVarDepth {
			return nil
		}
		body, ok := f.SVars[name]
		if !ok {
			return nil
		}
		sa, _ := parseSA("", body)
		if sa != nil {
			sa.Sub = resolve(sa.ParamStr(PKSubAbility), depth+1)
		}
		return sa
	}
	for i := t0; i < len(f.Triggers); i++ {
		f.Triggers[i].Effect = resolve(f.Triggers[i].ParamStr(PKExecute), 0)
	}
	for i := r0; i < len(f.Repls); i++ {
		f.Repls[i].With = resolve(f.Repls[i].Params["ReplaceWith"], 0)
	}
}

// KeywordCounter is one entry-counter placement a keyword's own expansion
// derives: the counter kind and the literal count its body names.
type KeywordCounter struct {
	Kind string
	N    int32
}

// KeywordEntryCounters derives the entry-counter placements a granted
// keyword line's expansion would give an object entering the battlefield
// with it: a throwaway face takes the line, and every Moved PutCounter|ETB$
// True body the expansion mints on it names one placement. The count must be
// a plain literal (vanishing's CounterNum$ 3); any non-literal count, a
// foreign referent or an unexpandable line is incomplete -- the caller fails
// closed rather than placing a guessed grant (the Clone conditional-keyword
// rider's whitelist rejects the head, effects/cloneETBShapeOK). It is a pure
// function: the throwaway face never escapes, so repeated calls answer
// identically and replay sees a deterministic derivation.
func KeywordEntryCounters(line string) (out []KeywordCounter, complete bool) {
	var f Face
	if !f.ApplyKeywordExpansion(line) {
		return nil, false
	}
	complete = true
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event != "Moved" {
			continue
		}
		// ApplyKeywordExpansion binds the minted Repl.With; ResolveSVar is
		// the unlinked-face fallback so the derivation stays total.
		body := r.With
		if body == nil {
			body = ResolveSVar(f.SVars, r.Params["ReplaceWith"])
		}
		if body == nil || body.API != "PutCounter" {
			continue
		}
		if body.Params["ETB"] != "True" {
			continue
		}
		if d := body.Params["Defined"]; d != "" && d != "Self" {
			return nil, false
		}
		n, err := strconv.ParseInt(strings.TrimSpace(body.Params["CounterNum"]), 10, 32)
		if err != nil {
			return nil, false
		}
		kind := strings.TrimSpace(body.Params["CounterType"])
		if kind == "" {
			return nil, false
		}
		out = append(out, KeywordCounter{Kind: kind, N: int32(n)})
	}
	return out, complete
}

// addKeywordTrigger appends one tagged T: line whose Execute$ is an SVar
// this function creates, unless the exact keyword line was already
// expanded (kw is the head, used only for the Keyword$ tag; line is the full
// keyword text, used both for idempotency and -- since it, unlike kw, is
// unique per call -- for the __kw SVar name. Soulbond calls this twice with
// the same kw ("Soulbond") but two different lines ("Soulbond#self" and
// "Soulbond#other"): keying the SVar name on kw alone would collide the two
// calls onto one shared SVar, silently letting the second call's effect body
// overwrite the first's).
func (f *Face) addKeywordTrigger(kw, line, trigger, effect string, has func(kind, line string) bool) {
	if has("T", line) {
		return
	}
	sv := "__kw" + strings.ReplaceAll(line, " ", "")
	f.setSVar(sv, effect)
	p := parseParams(trigger + " | Execute$ " + sv + " | Keyword$ " + kw)
	p["KeywordLine"] = line
	f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
}

// setSVar lazily initializes f.SVars before writing name/body. Most compiled
// faces never call this -- only ones with an SVar-based keyword expansion
// do -- so allocating unconditionally in expandKeywords would put a throwaway
// empty map into every face the IR cache stores for nothing.
func (f *Face) setSVar(name, body string) {
	if f.SVars == nil {
		f.SVars = map[string]string{}
	}
	f.SVars[name] = body
}
