// Package adapter serves dirac behind the host's game interfaces with no rules
// engine beside it. A game's decisions are the kernel's, folded to whole answers;
// its board is a read-only image of the kernel's state, so the view and its
// redaction are gorge's own code.
package adapter

import (
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
)

// Name is the name dirac reports and a host selects it by.
const Name = "dirac"

// Engine plays games on one or more card sets.
type Engine struct {
	sets    []cardset.Set
	reg     *cards.Registry
	surface dirac.Surface
}

// New is an engine over the named sets, tried in order for each game. A set whose
// tables are not built fails here, naming the command that builds them.
func New(sets []string, reg *cards.Registry, surface dirac.Surface) (*Engine, error) {
	if len(sets) == 0 {
		return nil, errors.New("dirac: no card sets")
	}
	e := &Engine{reg: reg, surface: surface}
	for _, n := range sets {
		s, err := cardset.ByName(n)
		if err != nil {
			return nil, fmt.Errorf("dirac: %w", err)
		}
		e.sets = append(e.sets, s)
	}
	return e, nil
}

func (*Engine) Name() string { return Name }

func (*Engine) Caps() game.Caps { return game.Caps{Engine: Name} }

// Autopay reports whether this engine's games play on the autopay surface.
func (e *Engine) Autopay() bool { return e.surface == dirac.SurfaceAutopay }

// Accepts says why c cannot play here, naming the limit or the card, or nil.
func (e *Engine) Accepts(c game.Config) error {
	_, _, err := e.table(c)
	return err
}

// CheckDeck says why a deck alone cannot play on any set, or nil.
func (e *Engine) CheckDeck(deck []*cards.Card) error {
	var best error
	fewest := -1
	es := entries(deck)
	for _, s := range e.sets {
		n := refused(s, deck)
		if n == 0 {
			if _, _, _, err := s.Matchup(es, nil); err == nil {
				return nil
			} else if best == nil {
				best, fewest = err, 0
			}
			continue
		}
		if fewest < 0 || n < fewest {
			_, _, _, best = s.Matchup(es, nil)
			fewest = n
		}
	}
	return best
}

// table picks the first set holding both decks and builds their table.
func (e *Engine) table(c game.Config) (cardset.Set, *kop.Table, error) {
	var cmdErr error
	if c.Format == game.FormatCommander {
		cmdErr = checkCommander(c)
	}
	switch {
	case len(c.Decks) != 2:
		return nil, nil, fmt.Errorf("dirac plays two seats, table has %d", len(c.Decks))
	case c.Format != game.FormatConstructed && c.Format != game.FormatCommander:
		return nil, nil, errors.New("dirac plays constructed and two-seat commander only")
	case cmdErr != nil:
		return nil, nil, cmdErr
	case c.StartChooser != nil:
		return nil, nil, errors.New("dirac tosses for the first turn: a chosen starting player needs the gorge engine")
	case c.Sideboards != nil:
		return nil, nil, errors.New("dirac plays no sideboards: a match with sideboarding needs the gorge engine")
	}
	if n := len(c.Decks[0]) + len(c.Decks[1]); n > dirac.MaxObj {
		return nil, nil, fmt.Errorf("dirac holds %d objects, the decks have %d cards", dirac.MaxObj, n)
	}
	var best error
	fewest := -1
	for _, s := range e.sets {
		tbl, _, _, err := s.Matchup(entries(c.Decks[0]), entries(c.Decks[1]))
		if err == nil {
			return s, tbl, nil
		}
		if n := refused(s, c.Decks[0]) + refused(s, c.Decks[1]); fewest < 0 || n < fewest {
			best, fewest = err, n
		}
	}
	return nil, nil, best
}

// entries is a deck as the name counts a table is built from.
func entries(deck []*cards.Card) []cardset.Entry {
	es := make([]cardset.Entry, 0, 24)
	at := map[string]int{}
	for _, c := range deck {
		n := frontName(c)
		if i, ok := at[n]; ok {
			es[i].Count++
			continue
		}
		at[n] = len(es)
		es = append(es, cardset.Entry{Name: n, Count: 1})
	}
	return es
}

// refused counts a deck's cards s does not play.
func refused(s cardset.Set, deck []*cards.Card) int {
	plays := s.Supported()
	n := 0
	for _, c := range deck {
		if i, ok := s.Lookup(frontName(c)); !ok || !plays[i] {
			n++
		}
	}
	return n
}

func frontName(c *cards.Card) string {
	if len(c.Faces) > 0 && c.Faces[0] != nil && c.Faces[0].Name != "" {
		return c.Faces[0].Name
	}
	return c.Path
}

// New builds a game through to its first decision.
func (e *Engine) New(c game.Config) (game.Game, error) {
	s, tbl, err := e.table(c)
	if err != nil {
		return nil, err
	}
	var decks [2][]dirac.CardID
	for i := range decks {
		decks[i], err = deckIDs(tbl, c.Decks[i])
		if err != nil {
			return nil, err
		}
	}
	dirac.Register(tbl)
	g := &Game{eng: e, set: s, tbl: tbl, cfg: c, log: newLog(c.Seed)}
	for i, d := range c.Decks {
		var name, arch string
		if i < len(c.Names) {
			name = c.Names[i]
		}
		if i < len(c.Archetypes) {
			arch = c.Archetypes[i]
		}
		g.manifests = append(g.manifests, deck.NewManifest(name, arch, d, nil, nil))
	}
	g.tr.BindSet(s, tbl, e.reg)
	g.bind()
	g.s.Init(tbl, decks[0], decks[1], c.Seed, e.config(c))
	g.s.Project(&g.kp)
	g.ids.sync(&g.kp)
	g.kp.Turn = 0 // so the log opens with the first turn, which the view's round anchors on
	g.settle()
	return g, nil
}

// config is every kernel setting a served game plays under. Lockstep holds
// LockstepConfig to gorge; nothing else reaches a game. The rest order gives
// Intent.Rest its kernel meaning.
func (e *Engine) config(c game.Config) dirac.Config {
	cfg := dirac.LockstepConfig(e.surface)
	cfg.NoElide = true
	cfg.Mulligans = uint8(c.Mulligans)
	cfg.ArrangeRestOrder = true
	if c.Format == game.FormatCommander {
		cfg.Commander = true
		for i := range cfg.Commanders {
			if i < len(c.Commanders) && len(c.Commanders[i]) <= 2 {
				for j, ix := range c.Commanders[i] {
					cfg.Commanders[i][j] = int16(ix)
				}
			}
		}
	}
	return cfg
}

// checkCommander names what a Commander table asks of dirac beyond Constructed: one
// commander per seat or a partner pair (CR 702.124), from inside its deck, at the 40
// life the kernel starts on (CR 903.7).
func checkCommander(c game.Config) error {
	if c.StartingLife != 0 && c.StartingLife != 40 {
		return fmt.Errorf("dirac starts a commander game at 40 life, the table asks for %d", c.StartingLife)
	}
	if c.DraftPartners {
		return fmt.Errorf("dirac does not seat the Commander-draft partner rule")
	}
	// table names a non-two-seat table before this check, so a wrong seat
	// count keeps that error even when the commander list is short.
	if len(c.Decks) != 2 {
		return nil
	}
	for i := range c.Decks {
		if i >= len(c.Commanders) || len(c.Commanders[i]) == 0 {
			return fmt.Errorf("dirac plays one or two commanders per seat: seat %d names none", i)
		}
		cs := c.Commanders[i]
		if len(cs) > 2 {
			return fmt.Errorf("dirac plays one or two commanders per seat: seat %d has %d", i, len(cs))
		}
		for _, ix := range cs {
			if ix < 0 || ix >= len(c.Decks[i]) {
				return fmt.Errorf("seat %d: commander index %d is outside its %d-card deck", i, ix, len(c.Decks[i]))
			}
		}
		// the pair gate gorge's deck layer applies (CR 903.3, 702.124): two
		// distinct cards that may be commanders together
		if len(cs) == 2 && (cs[0] == cs[1] || !deck.IsPartnerPair(c.Decks[i][cs[0]], c.Decks[i][cs[1]])) {
			return fmt.Errorf("seat %d: commanders %d and %d are not a legal partner pair", i, cs[0], cs[1])
		}
	}
	if len(c.Commanders) > len(c.Decks) {
		return fmt.Errorf("dirac plays two seats, the table names commanders for seat %d", len(c.Decks))
	}
	return nil
}

// deckIDs is a deck as table cards, one per copy in the deck's own order: that
// order numbers the game's objects, as it does gorge's.
func deckIDs(tbl *kop.Table, deck []*cards.Card) ([]dirac.CardID, error) {
	at := make(map[string]dirac.CardID, len(tbl.Cards))
	for i := len(tbl.Cards) - 1; i >= 0; i-- {
		at[tbl.Cards[i].Name] = dirac.CardID(i)
	}
	out := make([]dirac.CardID, len(deck))
	for i, c := range deck {
		id, ok := at[frontName(c)]
		if !ok {
			return nil, fmt.Errorf("dirac: %q is not in table %s", frontName(c), tbl.Name)
		}
		out[i] = id
	}
	return out, nil
}
