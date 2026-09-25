package cards

import (
	"sort"
	"strings"
)

// ValueHeadPrefix is the primitive kind a count-expression head reports
// under: "count:Domain" for a face that reads SVar:X:Count$Domain,
// "count:PlayerCountPropertyYou$SacrificedThisTurn" for the bare
// PlayerCount family. It is a coverage primitive exactly like api:/trig:/
// kw:, answered by effects.Supported, but it is NOT part of Primitives():
// Primitives stays the symbol set the engine dispatches on (botbench's
// deck-coverage plans and the IR tests read it), and a value head is a
// PARAMETER-level read. Registry.Unsupported unions the two, so a card
// whose gate or amount reads a count head the evaluator does not model is
// reported unsupported instead of joining the playable pool with a check
// that can never pass.
const ValueHeadPrefix = "count:"

// ValueHead extracts the gated head of one SVar value body, or ok=false
// when the body is not a head this gate classifies. Two families are
// classified -- the ones whose verdict does not depend on a resolution
// context (a target, a trigger event, a remembered set):
//
//   - Count$<Head>...: the head is the text after "Count$" up to the first
//     space, '/', '$', '.', ':' or '_' (Count$Void.1.0 -> Void,
//     Count$ThisTurnCast_Creature -> ThisTurnCast, Count$ValidGraveyard,Exile
//     X -> ValidGraveyard,Exile, Count$CardCounters.P1P1 -> CardCounters);
//   - PlayerCount<Group>$<Property>...: the head is the whole group
//     reference plus the property up to the first space, '/', '.' or '_'
//     (PlayerCountPropertyYou$SacrificedThisTurn Permanent ->
//     PlayerCountPropertyYou$SacrificedThisTurn).
//
// Every other body (Number$, SVar$, Remembered$, Targeted$..., TriggerCount$,
// AI hint values) is unclassified.
func ValueHead(body string) (string, bool) {
	body = strings.TrimSpace(body)
	if rest, ok := strings.CutPrefix(body, "Count$"); ok {
		if i := strings.IndexAny(rest, " /$.:_"); i >= 0 {
			rest = rest[:i]
		}
		if rest == "" {
			return "", false
		}
		return rest, true
	}
	if strings.HasPrefix(body, "PlayerCount") {
		group, prop, ok := strings.Cut(body, "$")
		if !ok || strings.ContainsAny(group, " /") {
			return "", false
		}
		if i := strings.IndexAny(prop, " /._"); i >= 0 {
			prop = prop[:i]
		}
		if prop == "" {
			return "", false
		}
		return group + "$" + prop, true
	}
	return "", false
}

// ValueHeads lists the "count:<head>" primitives this face's REFERENCED
// value SVars read. An SVar counts as referenced when its name appears as a
// token in any ability line, trigger/static/replacement parameter, keyword,
// or another SVar body of the face (a compare operand's GE<name> spelling
// included); an unreferenced body is an AI hint or dead text and never
// reaches the evaluator. Sorted and de-duplicated.
func (f *Face) ValueHeads() []string {
	if len(f.SVars) == 0 {
		return nil
	}
	refs := f.referencedNames()
	set := map[string]struct{}{}
	for name, body := range f.SVars {
		if !refs[name] {
			continue
		}
		if head, ok := ValueHead(body); ok {
			set[ValueHeadPrefix+head] = struct{}{}
		}
		// Arithmetic operands may themselves be Count$ expressions (for
		// example Count$CardPower/Minus.Count$CardBasePower). They are
		// evaluated by the same count registry and must participate in the
		// honesty gate even though ValueHead(body) reports only the outer head.
		for rest := body; ; {
			i := strings.Index(rest, "Count$")
			if i < 0 {
				break
			}
			rest = rest[i:]
			if head, ok := ValueHead(rest); ok {
				set[ValueHeadPrefix+head] = struct{}{}
			}
			rest = rest[len("Count$"):]
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ValueHeads is the union across every face.
func (c *Card) ValueHeads() []string {
	set := map[string]struct{}{}
	for _, f := range c.Faces {
		for _, h := range f.ValueHeads() {
			set[h] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// referencedNames tokenises every place a face can name an SVar.
func (f *Face) referencedNames() map[string]bool {
	refs := map[string]bool{}
	add := func(s string) {
		for _, tok := range strings.FieldsFunc(s, func(r rune) bool {
			return !(r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
		}) {
			refs[tok] = true
			if len(tok) > 2 {
				switch tok[:2] {
				case "GE", "GT", "LE", "LT", "EQ", "NE":
					refs[tok[2:]] = true
				}
			}
		}
	}
	addParams := func(p map[string]string) {
		for _, v := range p {
			add(v)
		}
	}
	var walk func(sa *SA, depth int)
	walk = func(sa *SA, depth int) {
		if sa == nil || depth > maxSVarDepth {
			return
		}
		addParams(sa.Params)
		walk(sa.Sub, depth+1)
	}
	for _, a := range f.Abilities {
		walk(a, 0)
	}
	for _, t := range f.Triggers {
		addParams(t.Params)
		walk(t.Effect, 0)
	}
	for _, s := range f.Statics {
		addParams(s.Params)
	}
	for _, r := range f.Repls {
		addParams(r.Params)
		walk(r.With, 0)
	}
	for _, k := range f.Keywords {
		add(k)
	}
	for _, body := range f.SVars {
		add(body)
	}
	return refs
}
