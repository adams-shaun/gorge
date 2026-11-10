// Package cardset is the engine-neutral interface over a card pool and the way
// to one: ByName serves the binary tables dirac/kgen writes (load.go). pools/ holds
// each pool's committed inputs, its card list and CardID index.
package cardset

import "github.com/adams-shaun/gorge/dirac/kop"

// Entry is one deck line.
type Entry struct {
	Name  string
	Count int
}

// Set is one card pool, read from a binary table (NewDataSet).
type Set interface {
	// Name is the pool's name, the key ByName takes.
	Name() string

	// Lookup returns a card's pool index by name (deck cards only).
	Lookup(name string) (int, bool)

	// Supported reports, per pool index, whether this set plays the entry.
	// The slice spans the whole index space: deck cards first (NumCards of
	// them), then the tokens they make.
	Supported() []bool

	// Status is pool index i's status, whichever entries this set plays.
	Status(i int) Status

	// Reason says why pool index i is not StatusSupported; "" when it is.
	Reason(i int) string

	// AbilityOrigin maps pool card i's kop ability ab to gorge's ability:
	// the Face.Abilities index, 0x80|the Face.Triggers index, or 0xFF.
	AbilityOrigin(i, ab int) uint8

	// Matchup builds the kop.Table for two decks and returns it with each
	// deck as CardIDs (one per copy, in deck-list order). A deck holding a
	// card this set does not play is a *RefusedError.
	Matchup(a, b []Entry) (*kop.Table, []kop.CardID, []kop.CardID, error)

	// MatchupN is Matchup for any number of decks: one table over every
	// deck's cards, and each deck as CardIDs in the order given. Two decks
	// build the table Matchup builds.
	MatchupN(decks [][]Entry) (*kop.Table, [][]kop.CardID, error)

	// Pool builds one kop.Table over the whole pool: every pool card and
	// every token they make, pool index i at CardID PoolID(i).
	Pool() (*kop.Table, error)

	// PoolID is pool index i's CardID in Pool's table.
	PoolID(i int) kop.CardID

	// InertID reports whether Pool's CardID id is an inert card: the fallback
	// of a card this set does not play, or a card that left the pool.
	InertID(id kop.CardID) bool

	// NumIDs is the pool table's CardID count (cards, tokens and absent).
	NumIDs() int

	// Subtypes is the pool's full subtype vocabulary (pool indexes).
	Subtypes() []string

	// Names is the pool's names by pool index: deck cards first (NumCards
	// of them, sorted; only these are Lookup-able), then the tokens they
	// make.
	Names() []string

	// NumCards is the number of pool (deck) cards; the tokens follow them.
	NumCards() int

	// NumTokens is the number of token definitions.
	NumTokens() int

	// Def is pool index i's card definition, its Name blank (see Names).
	Def(i int) *kop.CardDef

	// Inert reports whether pool index i is a deck card this set does not
	// play, which Pool's table holds as an inert fallback.
	Inert(i int) bool
}
