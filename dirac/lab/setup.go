package lab

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/deck"
)

// Deck is one seat's deck list.
type Deck struct {
	Name       string
	Cards      []cardset.Entry
	Commanders []string // names, in Cards; used when Config.Commander is set
}

// ParseDeck reads a gorge deck file (the JSON dplay's -deck-a takes).
func ParseDeck(raw []byte) (Deck, error) {
	f, err := deck.Parse(raw)
	if err != nil {
		return Deck{}, err
	}
	d := Deck{Name: f.Name, Commanders: f.CommanderNames()}
	for _, c := range f.Cards {
		d.Cards = append(d.Cards, cardset.Entry{Name: c.Name, Count: c.Count})
	}
	return d, nil
}

// ReadDeck is ParseDeck on a file. A name left empty by the file is the path.
func ReadDeck(path string) (Deck, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Deck{}, err
	}
	d, err := ParseDeck(raw)
	if err != nil {
		return Deck{}, fmt.Errorf("%s: %w", path, err)
	}
	if d.Name == "" {
		d.Name = path
	}
	return d, nil
}

// ParseList reads a plain list, one "COUNT NAME" per line ("20 Mountain"); a
// line without a count is one copy. Blank lines and lines starting with # or //
// are skipped.
func ParseList(name, text string) (Deck, error) {
	d := Deck{Name: name}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		n, card := 1, line
		if f, rest, ok := strings.Cut(line, " "); ok {
			if c, err := strconv.Atoi(strings.TrimSuffix(f, "x")); err == nil {
				n, card = c, strings.TrimSpace(rest)
			}
		}
		if n < 1 || card == "" {
			return Deck{}, fmt.Errorf("deck %s: bad line %q", name, line)
		}
		d.Cards = append(d.Cards, cardset.Entry{Name: card, Count: n})
	}
	return d, sc.Err()
}

// Config says what game to deal.
type Config struct {
	// Set is the card set, a cardset.ByName name ("fdn"). Decks are looked up in it.
	Set   string
	Decks []Deck // one per seat, 2 to dirac.MaxSeats
	Seed  uint64 // the game NewGame deals; Setup.Game takes its own
	// Mulligans is the London mulligans each player may take; 0 skips the round.
	Mulligans int
	// Commander starts a Commander game: each Deck.Commanders names cards of its list.
	Commander bool
	// Full plays the lockstep-full surface instead of lockstep-autopay (dplay's -config).
	Full bool
	// Allow names cards of Set to play although no evidence list names them
	// yet (cardset.AllowNamed). A card that does not lower, or that is refused
	// by name, stays unplayable. It is for a caller that verifies those cards
	// itself, in lockstep on its own decks.
	Allow []string

	// Table and IDs replace Set and Decks with a prebuilt table and one deck
	// of CardIDs per seat (the vanilla table, a hand-made one).
	Table *kop.Table
	IDs   [][]kop.CardID
}

// Setup is a built matchup: the table and decks every game of it shares. Close
// it when done with its games.
type Setup struct {
	tbl   *kop.Table
	ids   [][]kop.CardID
	names []string
	cfg   dirac.Config
	owned bool // built here, so Close releases it
}

// Open builds cfg's table and decks. A card the set does not play is a
// *cardset.RefusedError naming it.
func Open(cfg Config) (*Setup, error) {
	su := &Setup{}
	var err error
	if cfg.Table != nil {
		su.tbl, su.ids = cfg.Table, cfg.IDs
		for i := range su.ids {
			su.names = append(su.names, "seat"+strconv.Itoa(i))
		}
		if cfg.Commander {
			return nil, errors.New("lab: Commander needs Set and Decks, not a prebuilt Table")
		}
	} else if err = su.build(cfg); err != nil {
		return nil, err
	}
	if n := len(su.ids); n < 2 || n > dirac.MaxSeats {
		return nil, fmt.Errorf("lab: %d seats: want 2 to %d", n, dirac.MaxSeats)
	}
	surface := dirac.SurfaceAutopay
	if cfg.Full {
		surface = dirac.SurfaceFull
	}
	su.cfg = dirac.LockstepConfig(surface)
	su.cfg.Mulligans = uint8(cfg.Mulligans)
	if cfg.Commander {
		su.cfg.Commander = true
		if err := su.commanders(cfg.Decks); err != nil {
			return nil, err
		}
	}
	dirac.Register(su.tbl)
	return su, nil
}

func (su *Setup) build(cfg Config) error {
	set, err := cardset.ByName(cfg.Set)
	if err != nil {
		return err
	}
	if len(cfg.Allow) > 0 {
		names := make(map[string]bool, len(cfg.Allow))
		for _, n := range cfg.Allow {
			names[n] = true
		}
		set = cardset.AllowNamed(set, names)
	}
	lists := make([][]cardset.Entry, len(cfg.Decks))
	for i, d := range cfg.Decks {
		lists[i] = d.Cards
		su.names = append(su.names, d.Name)
	}
	su.tbl, su.ids, err = set.MatchupN(lists)
	su.owned = err == nil
	return err
}

// commanders points the dirac config at each seat's named commanders.
func (su *Setup) commanders(decks []Deck) error {
	su.cfg.Commanders = [dirac.MaxSeats][2]int16{}
	for p := range su.cfg.Commanders {
		su.cfg.Commanders[p] = [2]int16{-1, -1}
	}
	for p, d := range decks {
		if len(d.Commanders) == 0 || len(d.Commanders) > 2 {
			return fmt.Errorf("lab: deck %s: want one or two commanders, has %d", d.Name, len(d.Commanders))
		}
		for k, name := range d.Commanders {
			at, pos := -1, 0
			for _, e := range d.Cards {
				if e.Name == name {
					at = pos
					break
				}
				pos += e.Count
			}
			if at < 0 {
				return fmt.Errorf("lab: deck %s: commander %q is not on its list", d.Name, name)
			}
			su.cfg.Commanders[p][k] = int16(at)
		}
	}
	return nil
}

// Seats is the number of seats.
func (su *Setup) Seats() int { return len(su.ids) }

// DeckNames are the seats' deck names.
func (su *Setup) DeckNames() []string { return su.names }

// Table is the matchup's card table.
func (su *Setup) Table() *kop.Table { return su.tbl }

// Close releases a table Open built (not one the Config supplied). Every Game of the Setup, forks included, is invalid after it.
func (su *Setup) Close() {
	if su.owned {
		dirac.ReleaseTable(su.tbl)
	}
}
