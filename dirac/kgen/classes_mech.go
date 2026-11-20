package kgen

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Mechanism families are the kernel taxonomy a refusal class belongs to. The
// list and its order are the reference census's MECH list (repro/mech.py): the
// first matching family wins, so a class about an imprinted object is object
// memory before it is a filter program. Kind is atom for a class that names one
// vocabulary item; an `other:` class distinguishes a missing value (it quotes a
// `"_"` value) from a guard (a shape the kernel does not model). These are
// classifications only: no refusal text, class string or lowering depends on
// them.
type mechFamily struct {
	id   string
	name string
	re   *regexp.Regexp
}

var mechFamilies = []mechFamily{
	{"M0", "non-paper (park)", regexp.MustCompile(`Perpetual|ChaosEnsues|PlaneswalkedTo|PlaneswalkedFrom|SetInMotion|Sticker|Contraption|Attraction|api:Draft|MakeCard|Specializ|Double team|\bSeek\b|Conjure|api:Abandon|PlanarDice|api:Planeswalk|kw:Visit|Hidden agenda|Vanguard|DeckLimit|\bante\b|api:Heist|NewGame|Subgame|DBPlaneswalk`)},
	{"M1", "token (transitive)", regexp.MustCompile(`^other:token refused$`)},
	{"M2", "object memory store", regexp.MustCompile(`Remember|Imprint|ExiledWith|Forget|Note[dC]|NoteCards|StoreSVar|ChosenCard|TempRemember|Pair|Soulbond|Haunt|Champion|ExileOnMoved|RememberedLKI|LKI\$|Clear[A-Z]|Exiled\$|Tribut|Mentor|Conspire|Replicate|Squad|SetChosen`)},
	{"M3", "reference algebra (SetExpr)", regexp.MustCompile(`Defined|Targeted|Triggered|ParentTarget|target slot|no target|Spawner|unknown filter head|DefinedPlayer|DefinedCards|RepeatPlayers|Chooser|Choser|EffectOwner|Activator|ActivatingPlayer|AttachedTo\$|DamageSource|TargetedController|value head [A-Z][A-Za-z]*\$|Controller "|Caster "|referent`)},
	{"M4", "duration / Effect object", regexp.MustCompile(`Duration|AtEOT|Until|^other:Effect |^param:Effect\.|delayed trigger|emblem|Effect static|Effect replacement|Effect trigger`)},
	{"M5", "filter program", regexp.MustCompile(`^prop:|filter|ValidSA|Valid[A-Z][A-Za-z]*\$? "|^count:Valid|shares|TgtZone|TargetType|ChangeType|compar|quality|^param:[A-Za-z]+\.Valid|ValidCard|ValidBlocker|ValidAttacker|ValidTgts|CardChoices`)},
	{"M6", "value / condition expr", regexp.MustCompile(`^count:|SVar|amount|Amount|Condition|Compare|\bX=|IsPresent|CheckSVar|Repeat`)},
	{"M7", "keyword desugar", regexp.MustCompile(`^kw:`)},
	{"M8", "ability-shape tables (trig/static/repl)", regexp.MustCompile(`^trig:|^stat:|^repl:|^param:trigger\.|^param:static\.|replacement|trigger|TriggerZones|Mode=|AffectedZone|MayPlay|MayLook|CantBe|CantAttack|CantBlock|MustBlock|MustAttack|Taps |Attacks |Blocks |ChangesZone|AttackersDeclared|AttackerBlocked|CounterRemoved|SpellCast|AbilityCast|static|Drawn |grant`)},
	{"M9", "cost program", regexp.MustCompile(`[Cc]ost|^cost:|kicker|morph|cumulative|alternat`)},
	{"M10", "mana", regexp.MustCompile(`[Mm]ana`)},
	{"M11", "choice vocabulary", regexp.MustCompile(`Choose|Choice|Vote|NameCard|TwoPiles|Piles|GenericChoice|ChooseEach`)},
	{"M12", "API / effect atom", regexp.MustCompile(`^api:|^param:`)},
	{"M13", "effect shape (other)", regexp.MustCompile(`.`)},
}

// reMechGuard is the reference's GUARD regex (repro/mech.py), verbatim. It only
// decides kind: the family comes from mechFamilies.
var reMechGuard = regexp.MustCompile(`outside|shape|beside|without|with no|with an? |\bon an? |\bonly\b|other than|not an? |not the|names no|over an? |beyond|the kernel|in an? |of an? |gorge |\(|->|to an? `)

// classMechanism returns a refusal class's mechanism family and kind. M13 is
// the reference's catch-all, so every class has a family.
func classMechanism(class string) (fam, kind string) {
	for _, f := range mechFamilies {
		if f.re.MatchString(class) {
			return f.id, classKind(class)
		}
	}
	return "M13", classKind(class)
}

// classKind is atom outside `other:`, where a guard wording wins over the value
// marker and anything else is a guard.
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

// paramFamilies is the reference's PFAM list (repro/partial.py) for params a
// refusal left unread. It is separate from mechFamilies because a bare param
// name carries no class prefix: `ValidTgts` is a filter param even though no
// `filter` appears. M13 is its catch-all.
var paramFamilies = []mechFamily{
	{"M2", "object memory store", regexp.MustCompile(`Remember|Imprint|Forget|ExileOnMoved|Note|Chosen|Pair|Haunt|LKI`)},
	{"M3", "reference algebra (SetExpr)", regexp.MustCompile(`^Defined|Targeted|Triggered|Chooser|Choser|Activator|DefinedPlayer|DefinedCards|RepeatPlayers|TgtPrompt|TargetMin|TargetMax|TargetUnique|MayChooseTarget|TargetsWith`)},
	{"M4", "duration / Effect object", regexp.MustCompile(`Duration|AtEOT|Until|Static|Triggers|ReplacementEffects|Effect`)},
	{"M5", "filter program", regexp.MustCompile(`Valid|Type|ChangeType|TgtZone|Shares|Restriction`)},
	{"M6", "value / condition expr", regexp.MustCompile(`Condition|Amount|Num|SVar|Count|Check|IsPresent|PresentCompare|X|Repeat`)},
	{"M8", "ability-shape tables (trig/static/repl)", regexp.MustCompile(`Mode|Trigger|Replace|Affected|CharacteristicDefining|Mana`)},
	{"M9", "cost program", regexp.MustCompile(`Cost|Kicker|Optional`)},
	{"M11", "choice vocabulary", regexp.MustCompile(`Choice|Choose|Vote|Random`)},
	{"M13", "effect shape (other)", regexp.MustCompile(`.`)},
}

// paramMechanism returns the family of one unread param name. M13 is the
// reference's catch-all (there is no keyword, mana or API family here).
func paramMechanism(param string) string {
	for _, f := range paramFamilies {
		if f.re.MatchString(param) {
			return f.id
		}
	}
	return "M13"
}

// reUnreadAfterRefusal is the reason lower.go's noteUnreadable writes.
var reUnreadAfterRefusal = regexp.MustCompile(`^unread after refusal \[(.*)\]$`)

// partialMechanismFamilies returns the sorted set of param families in an
// unread-after-refusal reason. The bracketed list is split on whitespace, like
// the reference's `m.group(1).split()`.
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
