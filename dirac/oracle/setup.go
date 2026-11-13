package oracle

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
)

// NoCardIndex is the table index of no card (a token, a copy).
const NoCardIndex uint8 = 255

// Matchup is one game's fixed inputs apart from the seed: the two deck lists,
// resolved, and the card table both sides index.
type Matchup struct {
	Names [2]string        // deck names (pool file stems)
	Decks [2][]*cards.Card // gorge's resolved decks, in decklist order
	// Table is the matchup's distinct card names, sorted; the kernel's CardID
	// for a deck card is its index here.
	// NoCardIndex (255) is none.
	Table []string
	// Deck[s][i] is the table index of deck s's i-th card.
	Deck [2][]uint8
	// Sideboards are the cards outside the game (CR 400.11), gorge's
	// Config.Sideboards; Side[s][i] is the table index of seat s's i-th. A
	// sideboard card is a card instance after both decks' (InstanceCard).
	Sideboards [2][]*cards.Card
	Side       [2][]uint8
	// Mulligans is Config.Mulligans: London mulligans each player may take
	// (0 skips the round), as in a gorge self-play game.
	Mulligans int
	// Commander plays a Commander game (CR 903): Config's FormatCommander
	// and 40 starting life, and Commanders[s], an index into Decks[s] (-1
	// none), is seat s's commander, put into the command zone at genesis;
	// Partners[s] (-1 none) is the second of a partner pair (CR 702.124),
	// seated after it. A further seat's are its MoreSeat's (SeatCommanders).
	// Corpus.Matchup and MatchupN set them from commander deck files'
	// commanders.
	Commander  bool
	Commanders [2]int
	Partners   [2]int
	// DraftPartners is Config.DraftPartners: a seat's two commanders are a
	// Commander-draft pair (deck.IsDraftCommanderPair, the rule Mystery
	// Booster Commander drafts play by) and not a constructed one, which
	// gorge would otherwise reject whole while the kernel seats both.
	// readCommanders sets it from the deck files.
	DraftPartners bool
	// NameUniverse, when set, replaces the registry's name universe in the
	// gorge game's Config: a name ask (a card choosing a card name as it
	// enters) offers only these cards' names. NewMatchup sets it to the
	// matchup's own cards when one of them chooses a card name as it enters
	// (NameUniverseFor), which is what the kernel lists (Table.NameCards).
	NameUniverse *cards.Universe
	// More is the seats past the first two of a matchup of more than two
	// seats (NewMatchupN), in seat order; nil at two seats, so every
	// two-seat field above reads as it always has.
	More []MoreSeat

	reg *cards.Registry
	// chars is the kernel vocabulary the gorge side maps derived
	// characteristics into (RealKernel sets it; nil: not compared)
	chars *charVocab
}

// MaxSeats is the most seats a matchup holds (gorge plays any number; the
// projection holds this many).
const MaxSeats = 4

// MoreSeat is one seat past the first two: its deck name, resolved deck and
// table indices (Matchup.Names, Decks and Deck for seats 0 and 1), and in a
// Commander matchup its commander and partner (Matchup.Commanders and
// Partners; -1 none).
type MoreSeat struct {
	Name               string
	Deck               []*cards.Card
	Ix                 []uint8
	Commander, Partner int
}

// NSeat is the matchup's seat count.
func (m *Matchup) NSeat() int { return 2 + len(m.More) }

// SeatName is seat s's deck name.
func (m *Matchup) SeatName(s int) string {
	if s < 2 {
		return m.Names[s]
	}
	return m.More[s-2].Name
}

// SeatDeck is seat s's resolved deck.
func (m *Matchup) SeatDeck(s int) []*cards.Card {
	if s < 2 {
		return m.Decks[s]
	}
	return m.More[s-2].Deck
}

// SeatCommanders is seat s's commander and partner, indices into its deck
// (-1 none).
func (m *Matchup) SeatCommanders(s int) (cmd, partner int) {
	if s < 2 {
		return m.Commanders[s], m.Partners[s]
	}
	return m.More[s-2].Commander, m.More[s-2].Partner
}

// setSeatCommanders sets seat s's commander and partner.
func (m *Matchup) setSeatCommanders(s, cmd, partner int) {
	if s < 2 {
		m.Commanders[s], m.Partners[s] = cmd, partner
		return
	}
	m.More[s-2].Commander, m.More[s-2].Partner = cmd, partner
}

// seatIx is seat s's deck as table indices.
func (m *Matchup) seatIx(s int) []uint8 {
	if s < 2 {
		return m.Deck[s]
	}
	return m.More[s-2].Ix
}

// Corpus is an opened gorge corpus plus a deck directory, with resolved decks
// cached (filled once, read-only afterwards).
type Corpus struct {
	Reg      *cards.Registry
	DecksDir string

	mu    sync.Mutex
	decks map[string][]*cards.Card
	files map[string]deck.File // decks held in memory (AddDecks), by name
}

// AddDecks makes files playable by name without a file on disk: a synthetic pool
// is dealt, never written. A name here hides a file of the same name.
func (c *Corpus) AddDecks(files map[string]deck.File) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.files == nil {
		c.files = make(map[string]deck.File, len(files))
	}
	for name, f := range files {
		c.files[name] = f
		delete(c.decks, name)
	}
}

// deckFile is the parsed deck named name, from memory or DecksDir.
func (c *Corpus) deckFile(name string) (deck.File, error) {
	c.mu.Lock()
	f, ok := c.files[name]
	c.mu.Unlock()
	if ok {
		return f, nil
	}
	raw, err := os.ReadFile(filepath.Join(c.DecksDir, name+".json"))
	if err != nil {
		return deck.File{}, err
	}
	return deck.Parse(raw)
}

// OpenCorpus opens the gorge corpus at cardsDir exactly as gorge self-play does
// (cards.SharedCorpus) and reads decks from decksDir.
func OpenCorpus(cardsDir, decksDir string) (*Corpus, error) {
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		return nil, err
	}
	return &Corpus{Reg: reg, DecksDir: decksDir, decks: map[string][]*cards.Card{}}, nil
}

// OpenKernelCorpus is OpenCorpus for a process that plays the kernel alone:
// its registry is cards.OpenCorpusSubset's, decoding only the cards names
// lists plus every token script, instead of the whole corpus (a self-play runner's
// bench plays 39 distinct cards of 34k; the full registry's open is most of
// its peak RSS).
//
// Every name still resolves: the subset's Lookup answers any corpus name
// (decoding a card outside names on first use, exactly as the full registry
// holds it), and its Tokens is the full set, so every resolution kernel play
// makes -- deck loads, FeatPool, HBotFacts, SBTCards, KernelMatchup, a card
// a deck's cards create or name -- is the full registry's. Only the
// registry's card list, and so Universe, is the subset: a gorge-rules game
// (Matchup.Config: replay, parity, lockstep) needs the full name universe,
// so it needs OpenCorpus, and Config panics on a subset matchup.
func OpenKernelCorpus(cardsDir, decksDir string, names []string) (*Corpus, error) {
	reg, err := cards.OpenCorpusSubset(cardsDir, names)
	if err != nil {
		return nil, err
	}
	return &Corpus{Reg: reg, DecksDir: decksDir, decks: map[string][]*cards.Card{}}, nil
}

// Deck resolves a pool deck by name (engine/deck.Load).
func (c *Corpus) Deck(name string) ([]*cards.Card, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d, ok := c.decks[name]; ok {
		return d, nil
	}
	var d []*cards.Card
	var err error
	if f, ok := c.files[name]; ok {
		d, err = f.Resolve(c.Reg)
	} else {
		_, d, err = deck.Load(c.Reg, filepath.Join(c.DecksDir, name+".json"))
	}
	if err != nil {
		return nil, fmt.Errorf("deck %s: %w", name, err)
	}
	c.decks[name] = d
	return d, nil
}

// Matchup builds the matchup of decks a and b.
func (c *Corpus) Matchup(a, b string, mulligans int) (*Matchup, error) {
	da, err := c.Deck(a)
	if err != nil {
		return nil, err
	}
	db, err := c.Deck(b)
	if err != nil {
		return nil, err
	}
	m, err := NewMatchup(c.Reg, [2]string{a, b}, [2][]*cards.Card{da, db}, mulligans)
	if err != nil {
		return nil, err
	}
	if err := c.readCommanders(m); err != nil {
		return nil, err
	}
	return m, nil
}

// readCommanders makes m a Commander matchup when a seat's deck file is a
// commander deck, seating each such deck's commander (and partner) from the
// file's designation.
func (c *Corpus) readCommanders(m *Matchup) error {
	for s := 0; s < m.NSeat(); s++ {
		name := m.SeatName(s)
		f, err := c.deckFile(name)
		if err != nil {
			return fmt.Errorf("deck %s: %w", name, err)
		}
		if !strings.EqualFold(f.Format, "commander") || len(f.CommanderNames()) == 0 {
			continue
		}
		m.Commander = true
		if ix := f.CommanderIndices(); len(ix) > 0 {
			p := -1
			if len(ix) > 1 {
				p = ix[1]
				if d := m.SeatDeck(s); ix[0] < len(d) && p < len(d) && !deck.IsCommanderPair(d[ix[0]], d[p]) && deck.IsDraftCommanderPair(d[ix[0]], d[p]) {
					m.DraftPartners = true
				}
			}
			m.setSeatCommanders(s, ix[0], p)
		}
	}
	return nil
}

// MatchupN builds the matchup of names, one deck a seat: at two seats it is
// Matchup. A matchup of more than two seats has no sideboards; it is a
// Commander matchup when its deck files are commander decks, as Matchup's.
func (c *Corpus) MatchupN(names []string, mulligans int) (*Matchup, error) {
	if len(names) == 2 {
		return c.Matchup(names[0], names[1], mulligans)
	}
	decks := make([][]*cards.Card, len(names))
	for s, n := range names {
		d, err := c.Deck(n)
		if err != nil {
			return nil, err
		}
		decks[s] = d
	}
	m, err := NewMatchupN(c.Reg, names, decks, mulligans)
	if err != nil {
		return nil, err
	}
	if err := c.readCommanders(m); err != nil {
		return nil, err
	}
	return m, nil
}

// NewMatchupN builds a matchup of len(names) seats from resolved decks; at two
// seats it is NewMatchup.
func NewMatchupN(reg *cards.Registry, names []string, decks [][]*cards.Card, mulligans int) (*Matchup, error) {
	if len(names) != len(decks) || len(names) < 2 || len(names) > MaxSeats {
		return nil, fmt.Errorf("matchup %v: %d names, %d decks: want 2..%d seats", names, len(names), len(decks), MaxSeats)
	}
	if len(names) == 2 {
		return NewMatchup(reg, [2]string{names[0], names[1]}, [2][]*cards.Card{decks[0], decks[1]}, mulligans)
	}
	m := &Matchup{Names: [2]string{names[0], names[1]}, Decks: [2][]*cards.Card{decks[0], decks[1]}, Mulligans: mulligans, Commanders: [2]int{-1, -1}, Partners: [2]int{-1, -1}, reg: reg}
	for s := 2; s < len(names); s++ {
		m.More = append(m.More, MoreSeat{Name: names[s], Deck: decks[s], Commander: -1, Partner: -1})
	}
	if holdsNameChooserN(decks) {
		m.NameUniverse = nameUniverseOf(decks)
	}
	seen := map[string]bool{}
	n := 0
	for _, d := range decks {
		n += len(d)
		for _, c := range d {
			if nm := cardName(c); !seen[nm] {
				seen[nm] = true
				m.Table = append(m.Table, nm)
			}
		}
	}
	sort.Strings(m.Table)
	if len(m.Table) >= int(NoCardIndex) {
		return nil, fmt.Errorf("matchup %v: %d distinct cards, the table holds %d", names, len(m.Table), NoCardIndex)
	}
	if n >= MaxInst {
		return nil, fmt.Errorf("matchup %v: %d deck cards, a projection holds %d instances", names, n, MaxInst)
	}
	for s, d := range decks {
		ix := make([]uint8, 0, len(d))
		for _, c := range d {
			ix = append(ix, uint8(sort.SearchStrings(m.Table, cardName(c))))
		}
		if s < 2 {
			m.Deck[s] = ix
		} else {
			m.More[s-2].Ix = ix
		}
	}
	return m, nil
}

// NewMatchup builds a matchup from resolved decks.
func NewMatchup(reg *cards.Registry, names [2]string, decks [2][]*cards.Card, mulligans int) (*Matchup, error) {
	return NewMatchupSide(reg, names, decks, [2][]*cards.Card{}, mulligans)
}

// NewMatchupSide is NewMatchup with each seat's sideboard.
func NewMatchupSide(reg *cards.Registry, names [2]string, decks, sideboards [2][]*cards.Card, mulligans int) (*Matchup, error) {
	m := &Matchup{Names: names, Decks: decks, Sideboards: sideboards, Mulligans: mulligans, Commanders: [2]int{-1, -1}, Partners: [2]int{-1, -1}, reg: reg}
	if holdsNameChooser(decks) {
		m.NameUniverse = NameUniverseFor(reg, decks)
	}
	seen := map[string]bool{}
	for _, ds := range [...][2][]*cards.Card{decks, sideboards} {
		for _, d := range ds {
			for _, c := range d {
				n := cardName(c)
				if !seen[n] {
					seen[n] = true
					m.Table = append(m.Table, n)
				}
			}
		}
	}
	sort.Strings(m.Table)
	if len(m.Table) >= int(NoCardIndex) {
		return nil, fmt.Errorf("matchup %v: %d distinct cards, the table holds %d", names, len(m.Table), NoCardIndex)
	}
	if n := len(decks[0]) + len(decks[1]) + len(sideboards[0]) + len(sideboards[1]); n >= MaxInst {
		return nil, fmt.Errorf("matchup %v: %d deck cards, a projection holds %d instances", names, n, MaxInst)
	}
	for s, d := range decks {
		for _, c := range d {
			m.Deck[s] = append(m.Deck[s], uint8(sort.SearchStrings(m.Table, cardName(c))))
		}
	}
	for s, d := range sideboards {
		for _, c := range d {
			m.Side[s] = append(m.Side[s], uint8(sort.SearchStrings(m.Table, cardName(c))))
		}
	}
	return m, nil
}

// holdsNameChooser reports whether a deck card chooses a card name as it
// enters (an ETB replacement whose effect is NameCard: Kai Winn, Faithless
// Leader).
func holdsNameChooser(decks [2][]*cards.Card) bool {
	return holdsNameChooserN(decks[:])
}

// holdsNameChooserN is holdsNameChooser over any number of decks.
func holdsNameChooserN(decks [][]*cards.Card) bool {
	for _, d := range decks {
		for _, c := range d {
			if c == nil {
				continue
			}
			for _, f := range c.Faces {
				if f == nil {
					continue
				}
				for _, r := range f.Repls {
					for a := r.With; a != nil; a = a.Sub {
						if a.API == "NameCard" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// NameUniverseFor is the name universe a gorge game over decks is dealt: the
// registry's whole universe, except that a matchup holding a card that
// chooses a card name as it enters names only its own cards, the set the
// kernel lists (Table.NameCards), so both sides offer the same choices.
func NameUniverseFor(reg *cards.Registry, decks [2][]*cards.Card) *cards.Universe {
	if !holdsNameChooser(decks) {
		return reg.Universe()
	}
	return nameUniverseOf(decks[:])
}

// nameUniverseOf is the universe of decks' distinct cards.
func nameUniverseOf(decks [][]*cards.Card) *cards.Universe {
	seen := map[string]bool{}
	var cs []*cards.Card
	for _, d := range decks {
		for _, c := range d {
			if n := cardName(c); !seen[n] {
				seen[n] = true
				cs = append(cs, c)
			}
		}
	}
	return cards.UniverseOf(cs)
}

// Config is the rules.Config a gorge self-play game builds for this
// matchup and seed: the same Names, Decks, Tokens, NameUniverse and
// Mulligans, and bench.PlayGame's default object cap. (-mull-heuristic is a
// seat policy there, not setup; here the Policy answers the mulligan asks.)
// A Commander matchup adds FormatCommander, 40 life and its commanders.
func (m *Matchup) Config(seed uint64) rules.Config {
	cfg := rules.Config{
		Seed: seed, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{m.Decks[0], m.Decks[1]},
		Mulligans: m.Mulligans,
	}
	for s := 2; s < m.NSeat(); s++ {
		cfg.Names = append(cfg.Names, fmt.Sprintf("p%d", s))
		cfg.Decks = append(cfg.Decks, m.SeatDeck(s))
	}
	if m.Commander {
		cfg.Format, cfg.StartingLife = rules.FormatCommander, 40
		cfg.DraftPartners = m.DraftPartners
		cfg.Commanders = make([][]int, m.NSeat())
		for s := range cfg.Commanders {
			if ix, p := m.SeatCommanders(s); ix >= 0 {
				cfg.Commanders[s] = []int{ix}
				if p >= 0 {
					cfg.Commanders[s] = append(cfg.Commanders[s], p)
				}
			}
		}
	}
	if len(m.Sideboards[0])+len(m.Sideboards[1]) > 0 {
		cfg.Sideboards = [][]*cards.Card{m.Sideboards[0], m.Sideboards[1]}
	}
	if m.reg != nil {
		if m.reg.IsSubset() {
			// a subset's Universe is not the corpus': the game could differ
			panic("oracle: Matchup.Config on an OpenKernelCorpus registry (a gorge game needs OpenCorpus's full name universe)")
		}
		cfg.Tokens, cfg.NameUniverse = m.reg.Tokens, m.reg.Universe()
	}
	if m.NameUniverse != nil {
		cfg.NameUniverse = m.NameUniverse
	}
	cfg.LoopGuard = rules.WithDefaultObjectCap(cfg.LoopGuard)
	return cfg
}

// kernelCommanders is m's commanders as dirac.Config carries them: each
// seat's commander and its partner (CR 702.124), and -1 in every other slot.
// A partner without a commander seats nothing, as Config drops it. The
// kernel side refuses a matchup of more seats than its build holds
// (RealKernel), so every seat here has a slot.
func (m *Matchup) kernelCommanders() (bool, [dirac.MaxSeats][2]int16) {
	var c [dirac.MaxSeats][2]int16
	for i := range c {
		c[i] = [2]int16{-1, -1}
	}
	for s := 0; s < m.NSeat() && s < dirac.MaxSeats; s++ {
		ix, p := m.SeatCommanders(s)
		c[s][0] = int16(ix)
		if ix >= 0 {
			c[s][1] = int16(p)
		}
	}
	return m.Commander, c
}

// cardName is a compiled card's front face name.
func cardName(c *cards.Card) string {
	if c == nil {
		return ""
	}
	if len(c.Faces) > 0 && c.Faces[0] != nil && c.Faces[0].Name != "" {
		return c.Faces[0].Name
	}
	return c.Path
}

// PoolDeck is one row of a deck pool's pool.tsv.
type PoolDeck struct{ Name, Split, Colors string }

// ReadPool reads pool.tsv rows of split ("all" for every row), as gorge self-play's
// readPool does.
func ReadPool(path, split string) ([]PoolDeck, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []PoolDeck
	sc := bufio.NewScanner(f)
	for line := 0; sc.Scan(); line++ {
		if line == 0 {
			continue
		}
		v := strings.Split(sc.Text(), "\t")
		if len(v) < 3 {
			continue
		}
		if split == "all" || v[1] == split {
			out = append(out, PoolDeck{Name: v[0], Split: v[1], Colors: v[2]})
		}
	}
	return out, sc.Err()
}
