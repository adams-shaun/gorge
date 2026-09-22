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
	if g == nil {
		return nil
	}
	if spec == "" {
		spec = descriptionSpec(description)
	}
	if spec == "" {
		return nameUniverse(g.NameUniverse)
	}
	seen := make(map[string]bool)
	filtered := make([]string, 0, len(g.NameUniverse))
	for _, c := range g.NameUniverse {
		if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
			continue
		}
		o := &state.Object{Card: c}
		if !MatchesObjectCtx(g, spec, o, SpecContext{}) {
			continue
		}
		name := c.Faces[0].Name
		if name != "" && !seen[name] {
			seen[name] = true
			filtered = append(filtered, name)
		}
	}
	if len(filtered) == 0 {
		// The filter matched nothing evaluable; keep the ask total.
		return nameUniverse(g.NameUniverse)
	}
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

// nameUniverse is the unrestricted distinct-name pass, shared by the empty
// spec and the totality fallback so the two can never diverge.
func nameUniverse(universe []*cards.Card) []string {
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
