package effects

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// NameChoices is the ONE card-name universe builder every NameCard path
// shares: the cast-time "as this enters" ask (rules.etbOptions), the
// mid-resolution DB$/SP$/AB$ NameCard ask (effNameCard) and the R-9 no-host
// fallback all read it, so the offered names, the recorded answer and a bot
// policy can never disagree about what a name choice ranges over.
//
// The universe is the compiled corpus the embedder supplied as
// state.Game.NameUniverse, so an unseen land (Pithing Needle) or an unseen
// opponent card is offered. Order is the sorted distinct primary-face name;
// a nil or face-less universe yields nil and each caller keeps its own R-9
// deterministic stand-in.
//
// spec is the SA's ValidCards$ filter, evaluated through the engine's own
// predicate grammar (MatchesObjectCtx) against each card's printed face, so
// `Card.nonLand` (Phyrexian Revoker, Cabal Therapy), `Card.Land+nonBasic`
// (Alpine Moon), `Card.Creature` (Wood Sage) and `Card.nonLand+nonArtifact`
// (Lost Legacy) all restrict exactly as their script says. An EMPTY spec is
// deliberately unrestricted -- Forge's NameCardEffect does not filter when
// ValidCards$ is absent (Pithing Needle names any card, lands included).
//
// description is the SA's ValidDescription$. Forge reads it as the rendered
// PROMPT text only: ChooseCardNameEffect.resolve filters the common-card set
// by ValidCards$ (defaulting to "Card") and uses ValidDescription$ just to
// word the "choose a specific card name" message, which is why
// rules/paramcensus_test.go's ignoredParamKeys cites it as UI text. When
// ValidCards$ is present it is therefore the ONLY filter. When ValidCards$
// is absent but a description this build can read is present, the description
// is mapped to its equivalent predicate as a SAFETY fallback (see
// descriptionSpec): it only ever narrows an otherwise-unrestricted offer,
// never widens one, so it cannot change real corpus behaviour (no corpus
// NameCard carries ValidDescription$ without ValidCards$). This keeps the
// brief's "ValidCards$/ValidDescription semantics" honest without
// re-implementing Forge's message localisation.
//
// A spec this build cannot evaluate for a printed face (a dynamic
// game-state comparator such as `Creature.cmcEQX`, or an object predicate
// with no resolving context) fails closed in the matcher, and a filter that
// would empty the ask falls back to the unrestricted universe. That is the
// totality rule (R-9): a name ask is never posted with zero options.
func NameChoices(g *state.Game, spec, description string) []string {
	return NameChoicesFromList(g, spec, description, "")
}

// NameChoicesFromList applies ChooseFromList$ after the ordinary card filter.
// The universe snapshot remains authoritative during replay; list order is
// normalized to the same sorted option order as other name choices.
//
// The returned list may be SHARED across games (namecard_cache.go): callers
// must treat it as read-only.
func NameChoicesFromList(g *state.Game, spec, description, chooseFromList string, strictFilter ...bool) []string {
	if g == nil {
		return nil
	}
	if spec == "" {
		spec = descriptionSpec(description)
	}
	strict := len(strictFilter) > 0 && strictFilter[0]
	if !pureNameSpec(spec) {
		return nameChoicesFiltered(g, spec, chooseFromList, strict)
	}
	key := nameFilterKey{
		universe:       cardsKey(g.NameUniverse),
		snapshot:       namesKey(g.NameUniverseNames),
		spec:           spec,
		chooseFromList: chooseFromList,
		strict:         strict,
	}
	if spec == "" {
		if chooseFromList == "" {
			// The snapshot (or the memoised universe list) is itself
			// immutable and shared; hand it out directly.
			return nameUniverseSnapshot(g.NameUniverse, g.NameUniverseNames)
		}
		return cachedNameChoices(key, func() []string {
			return filterNameList(nameUniverseSnapshot(g.NameUniverse, g.NameUniverseNames), chooseFromList)
		})
	}
	return cachedNameChoices(key, func() []string {
		return nameChoicesFiltered(g, spec, chooseFromList, strict)
	})
}

// nameChoicesFiltered is the uncached ValidCards$ filter walk.
func nameChoicesFiltered(g *state.Game, spec, chooseFromList string, strict bool) []string {
	if spec == "" {
		return filterNameList(nameUniverseSnapshot(g.NameUniverse, g.NameUniverseNames), chooseFromList)
	}
	allowed := nameSet(g.NameUniverseNames)
	seen := make(map[string]bool)
	filtered := make([]string, 0, len(g.NameUniverse))
	// One scratch object serves every card: the matcher reads it and keeps
	// no reference, and a fresh ~1 KB Object per universe card was the
	// dominant allocation of a NameCard ask.
	o := new(state.Object)
	for _, c := range g.NameUniverse {
		if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
			continue
		}
		*o = state.Object{Card: c}
		if !MatchesObjectCtx(g, spec, o, SpecContext{}) {
			continue
		}
		name := c.Faces[0].Name
		if allowed != nil && !allowed[name] {
			continue
		}
		if name != "" && !seen[name] {
			seen[name] = true
			filtered = append(filtered, name)
		}
	}
	if len(filtered) == 0 {
		// Ordinary asks retain the totality fallback. Random selection must
		// never turn an unevaluable/empty filter into an unrestricted lottery.
		if chooseFromList != "" || strict {
			return nil
		}
		return nameUniverseSnapshot(g.NameUniverse, g.NameUniverseNames)
	}
	filtered = filterNameList(filtered, chooseFromList)
	sort.Strings(filtered)
	return filtered
}

// descriptionSpec maps a NameCard ValidDescription$ to the predicate
// equivalent of the description forms the corpus uses, so a description-only
// script is offered a safely-narrowed list rather than the whole universe.
// It is a fallback only: a present ValidCards$ is the filter and this is
// never consulted (Forge semantics; see NameChoices). An unrecognised
// description returns "" (unrestricted), which matches Forge's default and
// keeps the offer total.
func filterNameList(names []string, chooseFromList string) []string {
	if chooseFromList == "" {
		return names
	}
	listed := make(map[string]bool)
	for _, name := range strings.Split(chooseFromList, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			listed[name] = true
		}
	}
	selected := make([]string, 0, len(names))
	for _, name := range names {
		if listed[name] {
			selected = append(selected, name)
		}
	}
	return selected
}

func descriptionSpec(description string) string {
	switch strings.ToLower(strings.TrimSpace(description)) {
	case "nonland":
		return "Card.nonLand"
	case "creature", "creature card":
		return "Card.Creature"
	case "artifact", "artifact card":
		return "Card.Artifact"
	case "land", "land card":
		return "Card.Land"
	case "nonbasic land", "card other than a basic land":
		return "Card.Land+nonBasic"
	case "nonartifact, nonland":
		return "Card.nonLand+nonArtifact"
	case "noncreature, nonland":
		return "Card.nonLand+nonCreature"
	}
	return ""
}

// buildNameUniverseNames computes the sorted, distinct primary-face-name
// list NameUniverseNames memoises.
func buildNameUniverseNames(universe []*cards.Card) []string {
	seen := make(map[string]bool, len(universe))
	out := make([]string, 0, len(universe))
	for _, c := range universe {
		if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
			continue
		}
		name := c.Faces[0].Name
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// nameUniverseSnapshot prefers a persisted match's immutable list over the
// current corpus so a later corpus update cannot renumber an answer. Both
// lists are immutable, so the result is returned without a copy and is
// read-only for the caller.
func nameUniverseSnapshot(universe []*cards.Card, snapshot []string) []string {
	if len(snapshot) > 0 {
		return snapshot[:len(snapshot):len(snapshot)]
	}
	return NameUniverseNames(universe)
}

func nameSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}
