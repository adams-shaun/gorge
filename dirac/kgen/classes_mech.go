package kgen

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Mechanism families are the kernel taxonomy a refusal class belongs to. The
// list and its order are the reference census's MECH list: the first matching
// family wins, so a class about an imprinted object is object memory before it
// is a filter program. Kind is atom for a class that names one vocabulary item;
// an `other:` class distinguishes a missing value (it quotes a `"_"` value)
// from a guard (a shape the kernel does not model). These are classifications
// only: no refusal text, class string or lowering depends on them.
type mechFamily struct {
	id   string
	name string
	re   *regexp.Regexp
}

var mechFamilies = []mechFamily{
	{"M0", "non-paper", regexp.MustCompile(`(?i)seek|conjure|spellbook|perpetual|intensity`)},
	{"M1", "token", regexp.MustCompile(`(?i)token`)},
	{"M2", "object memory", regexp.MustCompile(`(?i)remember|imprint|cleanup|exileonmoved|forgetonmoved|exiledwithsource`)},
	{"M3", "reference algebra", regexp.MustCompile(`(?i)defined|targeted|target|origin|destination|reference`)},
	{"M4", "duration/Effect", regexp.MustCompile(`(?i)duration|effect|pump|animate|continuous`)},
	{"M5", "filter program", regexp.MustCompile(`(?i)filter|valid|affected|choices|target|tgt`)},
	{"M6", "value/condition", regexp.MustCompile(`Amount|Condition|SVar|Compare|Power|Toughness|^count:|^head:|^Num|Count\$|value`)},
	{"M7", "keyword", regexp.MustCompile(`^kw:`)},
	{"M8", "mode tables", regexp.MustCompile(`^(trig|stat|repl):|(?i)mode|trigger|static|replacement`)},
	{"M9", "cost", regexp.MustCompile(`(?i)cost|kicker|additional|alternate|reduce|raise`)},
	{"M10", "mana", regexp.MustCompile(`(?i)mana|produced`)},
	{"M11", "choice", regexp.MustCompile(`(?i)optional|choose|choice|vote|chooser|random|mandatory`)},
	{"M12", "API/param atom", regexp.MustCompile(`^(api|param|prop):`)},
	{"M13", "effect shape", regexp.MustCompile(`^other:`)},
}

// reMechGuard matches the `other:` shapes that are guards rather than a value
// the kernel lacks. It only decides kind: the family comes from mechFamilies.
var reMechGuard = regexp.MustCompile(`(?i)outside|without|empty|two comparisons|in one clause|no |not |unknown|missing|unread|refused|requires|need|only|ambiguous|conflict|must|before|after|beyond|partial|conditional|unbounded|no longer|unsupported|not supported|not a |is not |cannot|can't|fails|failed|expected`)

// classMechanism returns a refusal class's mechanism family and kind.
func classMechanism(class string) (fam, kind string) {
	for _, f := range mechFamilies {
		if f.re.MatchString(class) {
			return f.id, classKind(class)
		}
	}
	return "M13", classKind(class)
}

// classKind is atom outside `other:`, where a guard wording wins over the
// value marker and anything else is a guard.
func classKind(class string) string {
	if !strings.HasPrefix(class, "other:") {
		return "atom"
	}
	if reMechGuard.MatchString(class) {
		return "guard"
	}
	if strings.Contains(class, `"_"`) {
		return "value"
	}
	return "guard"
}

// paramFamilies is the PFAM list for params a refusal left unread. It is
// separate from mechFamilies because a bare param name carries no class
// prefix: `ValidTgts` is a filter param even though no `filter` appears.
var paramFamilies = []mechFamily{
	{"M0", "non-paper", regexp.MustCompile(`(?i)seek|conjure|spellbook|perpetual|intensity`)},
	{"M1", "token", regexp.MustCompile(`(?i)token`)},
	{"M2", "object memory", regexp.MustCompile(`(?i)cleanup|imprint|remember|exileonmoved|forgetonmoved|exiledwithsource`)},
	{"M3", "reference algebra", regexp.MustCompile(`^Defined|^Origin|^Destination|^Reference`)},
	{"M4", "duration/Effect", regexp.MustCompile(`(?i)duration|effect|pump|animate|continuous`)},
	{"M5", "filter program", regexp.MustCompile(`(?i)valid|affected|choices|filter|target|tgt`)},
	{"M6", "value/condition", regexp.MustCompile(`Amount|Num|Condition|SVar|Compare|Power|Toughness|Life|Damage|^Count|^Value`)},
	{"M7", "keyword", regexp.MustCompile(`KW|Keyword`)},
	{"M8", "mode tables", regexp.MustCompile(`(?i)mode|trigger|static|replacement`)},
	{"M9", "cost", regexp.MustCompile(`(?i)cost|kicker|additional|alternate|reduce|raise`)},
	{"M10", "mana", regexp.MustCompile(`(?i)mana|produced`)},
	{"M11", "choice", regexp.MustCompile(`(?i)choose|choice|optional|vote|random|mandatory`)},
}

// paramMechanism returns the family of one unread param name. An unmatched
// name is an API/param atom, M12.
func paramMechanism(param string) string {
	for _, f := range paramFamilies {
		if f.re.MatchString(param) {
			return f.id
		}
	}
	return "M12"
}

// reUnreadAfterRefusal is the reason lower.go's noteUnreadable writes.
var reUnreadAfterRefusal = regexp.MustCompile(`^unread after refusal \[(.*)\]$`)

// partialMechanismFamilies returns the sorted set of param families in an
// unread-after-refusal reason.
func partialMechanismFamilies(reason string) []string {
	m := reUnreadAfterRefusal.FindStringSubmatch(reason)
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, p := range strings.Fields(m[1]) {
		seen[paramMechanism(p)] = true
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// rowMechanism is the family and kind of one TSV row: the class's mechanism,
// or the hidden-param families of a partial row.
func rowMechanism(r stdTSVRow) (family, kind string) {
	if r.class == partialClass {
		return strings.Join(partialMechanismFamilies(r.reason), ","), "hidden"
	}
	return classMechanism(r.class)
}

// mechFamilyRow is one row of the mechanism-family census.
type mechFamilyRow struct {
	fam      string
	name     string
	classes  int // distinct classes in the family
	carriers int // ALL rows carrying the family
	sole     int // cards whose every family is this one
	guards   int
	values   int
	atoms    int
	hidden   int // cards with hidden params in the family
}

// mechanismCensus counts the ALL rows by family. A partial row counts once for
// every family its params span; its class is the opaque partial class, so it
// contributes to hidden cards but not to the class count or the kind split.
func mechanismCensus(rows []stdTSVRow) []mechFamilyRow {
	out := make([]mechFamilyRow, len(mechFamilies))
	index := make(map[string]int, len(mechFamilies))
	classes := make([]map[string]bool, len(mechFamilies))
	hidden := make([]map[string]bool, len(mechFamilies))
	for i, f := range mechFamilies {
		out[i] = mechFamilyRow{fam: f.id, name: f.name}
		index[f.id] = i
		classes[i] = map[string]bool{}
		hidden[i] = map[string]bool{}
	}
	cardFams := map[string]map[string]bool{}
	addCard := func(card, fam string) {
		if cardFams[card] == nil {
			cardFams[card] = map[string]bool{}
		}
		cardFams[card][fam] = true
	}
	for _, r := range rows {
		if r.set != "ALL" || r.class == "" {
			continue
		}
		if r.class == partialClass {
			for _, f := range partialMechanismFamilies(r.reason) {
				i, ok := index[f]
				if !ok {
					continue
				}
				out[i].carriers++
				hidden[i][r.card] = true
				addCard(r.card, f)
			}
			continue
		}
		fam, kind := classMechanism(r.class)
		i, ok := index[fam]
		if !ok {
			continue
		}
		if !classes[i][r.class] {
			classes[i][r.class] = true
			switch kind {
			case "guard":
				out[i].guards++
			case "value":
				out[i].values++
			case "atom":
				out[i].atoms++
			}
		}
		out[i].carriers++
		addCard(r.card, fam)
	}
	for i := range out {
		out[i].classes = len(classes[i])
		out[i].hidden = len(hidden[i])
	}
	for _, fams := range cardFams {
		if len(fams) != 1 {
			continue
		}
		for f := range fams {
			if i, ok := index[f]; ok {
				out[i].sole++
			}
		}
	}
	return out
}

// writeMechanismSection prints the per-family table after the class ranking.
func writeMechanismSection(w io.Writer, rows []stdTSVRow) {
	census := mechanismCensus(rows)
	fmt.Fprintln(w, "## Mechanism families (ALL, every card)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Classes, the guard/value/atom split and hidden cards are distinct; carrier rows count every ALL row (a partial row once per family its params span).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| family | mechanism | classes | carrier rows | sole-family cards | guard | value | atom | hidden cards |")
	fmt.Fprintln(w, "|---|---|---:|---:|---:|---:|---:|---:|---:|")
	for _, r := range census {
		fmt.Fprintf(w, "| %s | %s | %d | %d | %d | %d | %d | %d | %d |\n",
			r.fam, r.name, r.classes, r.carriers, r.sole, r.guards, r.values, r.atoms, r.hidden)
	}
	fmt.Fprintln(w)
}
