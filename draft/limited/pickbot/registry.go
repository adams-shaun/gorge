// Package pickbot answers a draft's pick and build decisions the way a bot
// seat does: the rating-colour Drafter and the best23 Builder, plus the
// committed/Balanced pair named `balanced`. Ratings come from the embedded 17Lands FDN file where it covers the
// set, and from a card-facts heuristic elsewhere. The same functions back
// the human UI's Auto lands and Reset-to-bot-suggestion buttons.
//
// The package is pure: it imports only the limited vocabulary, the deck
// format, the card IR and draft's PickView, and takes its card facts
// through a caller-supplied Lookup so it never loads the corpus itself.
package pickbot

import (
	"math/rand/v2"
	"sort"
	"sync"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/engine/cards"
)

// PickView is one seat's view at a pick decision; it is draft.PickView,
// so the pod and the bot share one type.
type PickView = draft.PickView

// Lookup resolves a card name to its card facts. It is supplied by the
// caller (the corpus registry in tests and the host) so this package never
// loads the corpus itself. A nil Lookup, or a name it does not know, means
// "no facts": the card has no colours and no type for the heuristic, which
// is why the FDN rated path — name-keyed — works without it.
type CardLookup func(name string) *cards.Card

// Tuning is every constant the drafter and builder read, so the draftsim
// tuning loop turns one knob set. It is also the knob file a `knobs:<path>`
// agent reads (ParseTuning): every field is a JSON key, and a zero field
// means today's default (Filled), so the zero Tuning is the committed
// drafter and the best23 builder. A field that should really be zero is
// set to a negligible value (1e-9), as the research variants do.
type Tuning struct {
	// RatingCentre is the win rate a centred rating of zero stands for
	// ((gih_wr - RatingCentre) * RatingScale).
	RatingCentre float64 `json:"rating_centre"`
	// RatingScale converts a win-rate gap to rating points.
	RatingScale float64 `json:"rating_scale"`
	// HeuristicCentre and HeuristicScale put the heuristic's raw points on
	// the same rough centred scale as a rated card.
	HeuristicCentre float64 `json:"heuristic_centre"`
	HeuristicScale  float64 `json:"heuristic_scale"`
	// ColourFitMax is the full weight k of the colour term; ColourFitRamp
	// is how many picks of pack 1 ramp k from 0 to it (full by pick
	// ColourFitRamp+1).
	ColourFitMax  float64 `json:"colour_fit_max"`
	ColourFitRamp int     `json:"colour_fit_ramp"`
	// ColourlessFit is the colour term a colourless card earns (0.5 of the
	// maximum fit of 1).
	ColourlessFit float64 `json:"colourless_fit"`
	// MainSize is how many non-land cards the builder plays, DeckSize the
	// total it fills to with basics.
	MainSize int `json:"main_size"`
	DeckSize int `json:"deck_size"`
	// MinCreatures is the creature count the builder prefers; HighValueMV
	// and MaxHighValue cap how many cards of that mana value or more it
	// plays.
	MinCreatures int `json:"min_creatures"`
	HighValueMV  int `json:"high_value_mv"`
	MaxHighValue int `json:"max_high_value"`
	// ShortPairPenalty is what the builder's pair scan charges a colour pair
	// per maindeck slot its own cards cannot fill, so a thin pair padded
	// with the pool's best off-colour cards cannot outscores a deep real
	// pair (a pair is scored on its in-pair cards; the
	// off-colour fill is build-side only). The default −15 puts one
	// unfilled slot at about what a 0.40-GIH card would earn on the centred
	// rating scale, and is insensitive between −5 and −30.
	ShortPairPenalty float64 `json:"short_pair_penalty"`

	// Drafter knobs read by Knobbed (zero = the committed drafter).

	// Need is the committed drafter's needs weight; zero is
	// DefaultNeedWeight.
	Need float64 `json:"need"`
	// SignalWeight is the table-signal term's weight (Signals.Weight);
	// zero leaves it off. SignalStart is the pack 1 pick it starts from;
	// zero is pick 4.
	SignalWeight float64 `json:"signal_weight"`
	SignalStart  int     `json:"signal_start"`
	// Bias is added to a card's score per colour, WUBRG, as the mean over
	// the card's colours. PairBias is added to a card whose colour identity
	// is exactly one two-colour pair, in PairNames order.
	Bias     [5]float64  `json:"bias"`
	PairBias [10]float64 `json:"pair_bias"`
	// Deny is the hate-draft weight: from the 10th pick a card off the
	// seat's pair earns Deny*max(0, centred)/10.
	Deny float64 `json:"deny"`

	// Builder knobs read by Best23 beyond the ones above (zero = no bound).

	// Lands is the basic-land count; zero is DeckSize-MainSize, and a
	// non-zero Lands with a zero MainSize sets MainSize to DeckSize-Lands.
	Lands int `json:"lands"`
	// CreatureMax caps the main's creatures; zero is no cap.
	CreatureMax int `json:"creature_max"`
	// CurveMin and CurveMax bound the main's count per curve bucket (mana
	// value <=1, 2, 3, 4, 5, 6+); zero is no bound.
	CurveMin [6]int `json:"curve_min"`
	CurveMax [6]int `json:"curve_max"`
}

// DefaultTuning is the default knob set.
var DefaultTuning = Tuning{
	RatingCentre:     0.55,
	RatingScale:      100,
	HeuristicCentre:  2,
	HeuristicScale:   2,
	ColourFitMax:     8,
	ColourFitRamp:    9,
	ColourlessFit:    0.5,
	MainSize:         23,
	DeckSize:         40,
	MinCreatures:     17,
	HighValueMV:      6,
	MaxHighValue:     3,
	ShortPairPenalty: -15,
}

// Drafter picks one card ID out of the pack a PickView offers. r is the
// seat's seeded rng, used only to break score ties.
type Drafter interface {
	Pick(v PickView, r *rand.Rand) string
}

// Builder turns a finished pool into a deck.
type Builder interface {
	Build(set string, pool []limited.CardRef) build.Deck
}

type entry struct {
	newDrafter func(CardLookup) Drafter
	newBuilder func(CardLookup) Builder
}

var (
	entriesMu sync.RWMutex
	entries   = map[string]entry{}
)

func init() {
	Register("rating-colour",
		func(lookup CardLookup) Drafter { return &RatingColour{Lookup: lookup} },
		func(lookup CardLookup) Builder { return &Best23{Lookup: lookup} })
	Register("best23",
		func(lookup CardLookup) Drafter { return &RatingColour{Lookup: lookup} },
		func(lookup CardLookup) Builder { return &Best23{Lookup: lookup} })
	Register("balanced",
		func(lookup CardLookup) Drafter { return &Committed{Lookup: lookup} },
		func(lookup CardLookup) Builder { return &Balanced{Lookup: lookup} })
}

// Register names one drafter/builder pair in the registry, mirroring
// bots.Register. The pair is registered as constructors so New can build a
// fresh instance with a caller's lookup injected; re-registering a name
// replaces its entry.
func Register(name string, nd func(CardLookup) Drafter, nb func(CardLookup) Builder) {
	entriesMu.Lock()
	defer entriesMu.Unlock()
	entries[name] = entry{newDrafter: nd, newBuilder: nb}
}

// New constructs a fresh drafter and builder pair by registry name with
// lookup injected, so a caller (cmd/draftsim's -drafter/-builder flags,
// eventhost's persona) owns its instances instead of sharing the
// registry's. An unknown name is (nil, nil, false).
func New(name string, lookup CardLookup) (Drafter, Builder, bool) {
	entriesMu.RLock()
	defer entriesMu.RUnlock()
	e, ok := entries[name]
	if !ok {
		return nil, nil, false
	}
	return e.newDrafter(lookup), e.newBuilder(lookup), true
}

// Lookup returns the registered drafter and builder pair by name, built
// with a nil lookup: the FDN rated path is name-keyed and needs none, and
// a caller that has card facts uses New.
func Lookup(name string) (Drafter, Builder, bool) {
	return New(name, nil)
}

// Names lists the registered names, sorted.
func Names() []string {
	entriesMu.RLock()
	defer entriesMu.RUnlock()
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
