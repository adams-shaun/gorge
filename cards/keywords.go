package cards

import (
	"strings"
)

// expandKeywords turns each keyword the engine implements through ordinary
// machinery into the triggered ability, replacement effect or activated
// ability Forge itself expands it to (CardFactoryUtil, in spirit), tagged
// Params["Keyword"] so nothing downstream needs to know the difference. The
// SVars it adds start with "__kw" and cannot collide with a script's own.
// Idempotent: an already-expanded keyword *line* (the full "Head:param:..."
// text, not just the head) is never added twice, so a face with two
// distinct K:Equip: lines (different costs or restrictions) still expands
// both -- ruling FL-13. Keywords whose meaning is a casting option (Kicker,
// Surge, Flashback, Delve, Flash, Miracle) or a static property (Protection,
// Indestructible, Devoid) are not expanded: rules reads them directly.
func (f *Face) expandKeywords() {
	// has reports whether the exact keyword line k (head and every param,
	// verbatim) already produced a T:/R:/A: entry of the given kind, via
	// the KeywordLine tag every case below sets alongside the head-only
	// Keyword tag. Reading f.Triggers/f.Repls/f.Abilities live means a
	// second Link() call -- or two equal lines in the same pass -- can
	// never double-add.
	has := func(kind, k string) bool {
		switch kind {
		case "T":
			for _, t := range f.Triggers {
				if t.Params["KeywordLine"] == k {
					return true
				}
			}
		case "R":
			for _, r := range f.Repls {
				if r.Params["KeywordLine"] == k {
					return true
				}
			}
		case "A":
			for _, a := range f.Abilities {
				if a.Params["KeywordLine"] == k {
					return true
				}
			}
		case "S":
			// A keyword that appends a static directly (kw:Class's level
			// bands) needs the same idempotence check the trigger/
			// replacement/ability arms give, or a second Link() of a
			// cached face would double-append the static.
			for _, s := range f.Statics {
				if s.Params["KeywordLine"] == k {
					return true
				}
			}
		}
		return false
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

// kwExpander expands one keyword line onto the face.
//
// The parameters are what the switch arms this replaced closed over: f is the
// face, i the keyword's index in f.Keywords (mint __kw SVar names from it so
// two lines of the same keyword cannot collide), k the FULL keyword line
// verbatim, head and param its split halves, and has the idempotence check --
// whether this exact LINE already produced a T:/R:/A: entry, which is what
// lets a face with two distinct K:Equip: lines expand both (ruling FL-13)
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
