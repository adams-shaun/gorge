package matchrecord

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Match is the replay-input subset of a rules.Config, keyed with
// host.FeedbackMatch's JSON field names so a record and a feedback
// snapshot's match.json read the same way. It is what testkit/testutil/
// feedback's loader builds its Config through as well, so the two loaders
// cannot drift (design D3).
//
// It records no token scripts and no name-universe label list (D2): Config
// takes the token table from the caller and re-derives the name universe
// from the registry.
type Match struct {
	Seed        uint64   `json:"seed"`
	Names       []string `json:"names"`
	PlayerNames []string `json:"player_names,omitempty"`
	// DeckCards is each seat's deck as printed card names, in library
	// order before the shuffle (the order genesis mints the objects in).
	DeckCards  [][]string `json:"deck_cards"`
	Sideboards [][]string `json:"sideboards,omitempty"`
	// PlanarDecks is each seat's Planechase deck: genesis configuration
	// like Sideboards. host.FeedbackMatch has no such field (no hosted
	// table deals one), so a feedback snapshot always reads back nil.
	PlanarDecks [][]string `json:"planar_decks,omitempty"`
	// Format is "commander" for a Commander game; empty (or "constructed")
	// is constructed.
	Format       string  `json:"format,omitempty"`
	StartingLife int32   `json:"starting_life,omitempty"`
	Commanders   [][]int `json:"commanders,omitempty"`
	Mulligans    int     `json:"mulligans,omitempty"`
	// StartChooser is the seat posed CR 103.1's starting-player choice in
	// place of the toss winner; nil is the toss.
	StartChooser *int `json:"start_chooser,omitempty"`
	// NameUniverse is the name-card universe MODE bit: the game posed real
	// NameCard asks over the corpus's name universe.
	NameUniverse bool `json:"name_universe,omitempty"`
	// PaidManaCensus is the paid-cost mana census MODE bit
	// (rules.Config.DisablePaidManaCensus false).
	PaidManaCensus bool `json:"paid_mana_census,omitempty"`
}

const formatCommander = "commander"

// MatchOf records cfg's replay-input subset. Cards are recorded by their
// front face's printed name, the spelling the registry's Lookup resolves
// back to the same card (host's feedback capture records them the same
// way).
func MatchOf(cfg rules.Config) (Match, error) {
	decks, err := cardNames("deck", cfg.Decks)
	if err != nil {
		return Match{}, err
	}
	sideboards, err := cardNames("sideboard", cfg.Sideboards)
	if err != nil {
		return Match{}, err
	}
	planar, err := cardNames("planar deck", cfg.PlanarDecks)
	if err != nil {
		return Match{}, err
	}
	m := Match{
		Seed:           cfg.Seed,
		Names:          append([]string(nil), cfg.Names...),
		PlayerNames:    append([]string(nil), cfg.PlayerNames...),
		DeckCards:      decks,
		Sideboards:     sideboards,
		PlanarDecks:    planar,
		StartingLife:   cfg.StartingLife,
		Mulligans:      cfg.Mulligans,
		NameUniverse:   cfg.NameUniverse != nil,
		PaidManaCensus: !cfg.DisablePaidManaCensus,
	}
	if cfg.Format == rules.FormatCommander {
		m.Format = formatCommander
	}
	if len(cfg.Commanders) > 0 {
		m.Commanders = make([][]int, len(cfg.Commanders))
		for i, c := range cfg.Commanders {
			m.Commanders[i] = append([]int(nil), c...)
		}
	}
	if cfg.StartChooser != nil {
		c := *cfg.StartChooser
		m.StartChooser = &c
	}
	return m, nil
}

func cardNames(what string, decks [][]*cards.Card) ([][]string, error) {
	if len(decks) == 0 {
		return nil, nil
	}
	out := make([][]string, len(decks))
	for i, d := range decks {
		names := make([]string, len(d))
		for j, c := range d {
			if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
				return nil, fmt.Errorf("matchrecord: %s %d card %d has no face to name it by", what, i, j)
			}
			names[j] = c.Faces[0].Name
		}
		out[i] = names
	}
	return out, nil
}

// Config rebuilds the rules.Config m recorded: decks, sideboards and
// planar decks resolved through reg exactly the way deck.File.Resolve
// resolves a deck file (Lookup normalises the name, so the recorded
// printed name finds its card); the token table the caller supplies
// (cfg.Tokens is replay input, and the caller decides where the compiled
// tokens come from); and the format, life, commander, mulligan, chooser
// and mode settings. The name universe, when the mode bit is set, is
// reg's own, and the label list is left nil so the engine derives it from
// that corpus at genesis. The seed is carried, though a replay overwrites
// it with the log's own seed (replay's Ruling P5).
func (m Match) Config(reg *cards.Registry, tokens map[string]*cards.Card) (rules.Config, error) {
	if len(m.DeckCards) == 0 {
		return rules.Config{}, fmt.Errorf("matchrecord: the match records no deck_cards")
	}
	if reg == nil {
		return rules.Config{}, fmt.Errorf("matchrecord: no card registry to resolve the decks through")
	}
	decks, err := resolve(reg, "seat", m.DeckCards)
	if err != nil {
		return rules.Config{}, err
	}
	sideboards, err := resolve(reg, "sideboard", m.Sideboards)
	if err != nil {
		return rules.Config{}, err
	}
	planar, err := resolve(reg, "planar deck", m.PlanarDecks)
	if err != nil {
		return rules.Config{}, err
	}
	cfg := rules.Config{
		Seed:         m.Seed,
		Names:        m.Names,
		PlayerNames:  m.PlayerNames,
		Decks:        decks,
		Sideboards:   sideboards,
		PlanarDecks:  planar,
		Mulligans:    m.Mulligans,
		Tokens:       tokens,
		StartingLife: m.StartingLife,
		Commanders:   m.Commanders,
	}
	switch m.Format {
	case "", "constructed":
	case formatCommander:
		cfg.Format = rules.FormatCommander
	default:
		return rules.Config{}, fmt.Errorf("matchrecord: unknown format %q", m.Format)
	}
	// The starting-player chooser is genesis replay input like Mulligans —
	// the genesis Note names it — so a copy rides the rebuilt Config.
	if m.StartChooser != nil {
		chooser := *m.StartChooser
		cfg.StartChooser = &chooser
	}
	// The name-card universe is a match MODE, restored the way host's
	// matchForLog restores it (server/host/viewat.go): only a match that
	// recorded the mode replays with one, so a game without it takes the
	// legacy no-ask path and reproduces unchanged.
	if m.NameUniverse {
		cfg.NameUniverse = reg.Universe()
	}
	// The paid-cost census is the other match MODE; absent reads back as the
	// narrow census, the value a pre-flip feedback capture played with.
	cfg.DisablePaidManaCensus = !m.PaidManaCensus
	return cfg, nil
}

func resolve(reg *cards.Registry, what string, names [][]string) ([][]*cards.Card, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out := make([][]*cards.Card, len(names))
	for i, list := range names {
		out[i] = make([]*cards.Card, len(list))
		for j, n := range list {
			c, ok := reg.Lookup(n)
			if !ok {
				return nil, fmt.Errorf("matchrecord: %s %d card %d (%q) is not in the corpus", what, i, j, n)
			}
			out[i][j] = c
		}
	}
	return out, nil
}
