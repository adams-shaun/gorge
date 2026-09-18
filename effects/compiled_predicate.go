package effects

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// PredicateResult describes what a compiled predicate program can prove about
// one candidate. Maybe deliberately leaves the textual matcher authoritative.
type PredicateResult uint8

const (
	PredicateMaybe PredicateResult = iota
	PredicateNo
	PredicateYes
)

// PredicatePrograms is an immutable collection of programs keyed by their
// original Forge text. It is built before a game begins and never changes
// during matching, cloning, or replay.
type PredicatePrograms struct {
	byText map[string]predicateProgram
}

type predicateProgram struct {
	alternatives []predicateAlternative
}

type predicateAlternative struct {
	base      string
	baseMaybe bool
	terms     []predicateTerm
}

type predicateTerm struct {
	text  string
	maybe bool
}

// CompilePredicatePrograms compiles the subset of filter grammar that can be
// evaluated using only the candidate object and SpecContext. The compiler
// preserves unknown text as maybe rather than treating it as a non-match.
func CompilePredicatePrograms(specs []string) *PredicatePrograms {
	texts := append([]string(nil), specs...)
	sort.Strings(texts)
	programs := &PredicatePrograms{byText: make(map[string]predicateProgram, len(texts))}
	for _, spec := range texts {
		if spec == "" {
			continue
		}
		if _, exists := programs.byText[spec]; exists {
			continue
		}
		programs.byText[spec] = compilePredicateProgram(spec)
	}
	return programs
}

func compilePredicateProgram(spec string) predicateProgram {
	var p predicateProgram
	for alt := range filterAlternatives(spec) {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		a := predicateAlternative{base: base, baseMaybe: !compiledBase(base)}
		if !compiledBase(base) {
			a.terms = append(a.terms, predicateTerm{maybe: true})
		}
		for term := range strings.SplitSeq(rest, "+") {
			if term == "" {
				continue
			}
			a.terms = append(a.terms, predicateTerm{text: term, maybe: !compiledTerm(term)})
		}
		p.alternatives = append(p.alternatives, a)
	}
	return p
}

func compiledBase(base string) bool {
	if base == "CARDNAME" {
		return false
	}
	base = strings.TrimPrefix(base, "non")
	switch base {
	case "Any", "Card", "Permanent", "PermanentCard", "Spell", "SpellAbility":
		return true
	}
	return predicateTypeWords[base]
}

var compiledTerms = map[string]struct{}{
	"YouCtrl": {}, "YouDontCtrl": {}, "OppCtrl": {},
	"YouOwn": {}, "OppOwn": {}, "Self": {}, "Other": {}, "StrictlyOther": {},
	"tapped": {}, "untapped": {}, "attacking": {}, "token": {},
	"kicked": {}, "surged": {}, "escaped": {},
}

func compiledTerm(term string) bool {
	if term == "" {
		return true
	}
	if rest, ok := strings.CutPrefix(term, "!"); ok {
		return rest != "" && compiledTerm(rest)
	}
	if _, ok := compiledTerms[term]; ok {
		return true
	}
	if _, _, ok := nonPredicate(term); ok {
		return true
	}
	if kind, _ := wordPredicate(term); kind == wordColor || kind == wordType || kind == wordColorless {
		return true
	}
	return false
}

// Evaluate returns Maybe when spec was not compiled or when an alternative
// that is not already false contains unsupported grammar.
func (ps *PredicatePrograms) Evaluate(spec string, g *state.Game, o *state.Object, sc SpecContext) PredicateResult {
	if ps == nil || o == nil {
		return PredicateMaybe
	}
	p, ok := ps.byText[spec]
	if !ok {
		return PredicateMaybe
	}
	maybe := false
	for _, alt := range p.alternatives {
		if alt.baseMaybe {
			maybe = true
			continue
		}
		baseOK := matchesBase(g, alt.base, o)
		if !baseOK {
			continue
		}
		all := true
		altMaybe := false
		for _, term := range alt.terms {
			if term.maybe {
				altMaybe = true
				continue
			}
			matched, known := matchPredicate(g, term.text, o, sc)
			if !known {
				altMaybe = true
				continue
			}
			if !matched {
				all = false
				break
			}
		}
		if !all {
			continue
		}
		if !altMaybe {
			return PredicateYes
		}
		maybe = true
	}
	if maybe {
		return PredicateMaybe
	}
	return PredicateNo
}

// Len reports the number of unique non-empty source strings compiled.
func (ps *PredicatePrograms) Len() int {
	if ps == nil {
		return 0
	}
	return len(ps.byText)
}
