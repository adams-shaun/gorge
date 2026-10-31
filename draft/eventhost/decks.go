package eventhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/server/host"
)

// deckIDShape is the event-deck id shape: "ev/<event>/<seat>/<rev>". The
// anchored groups also rule out path traversal and out-of-range seats.
var deckIDShape = regexp.MustCompile(`^ev/(e[1-9][0-9]*)/([0-3])/([1-9][0-9]*)$`)

// DeckID names an event deck revision for host.Options.LoadDeck:
// "ev/<event>/<seat>/<rev>".
func DeckID(event string, seat, rev int) string {
	return fmt.Sprintf("ev/%s/%d/%d", event, seat, rev)
}

// DeckLoader returns a host.Options.LoadDeck that serves event deck ids from
// this store and passes every other id to next (nil next: not found). An id
// with the "ev/" prefix that does not parse is an error, never passed on.
// Resolved decks are cached by id — a saved revision never changes — so the
// resolved *cards.Card slice is shared between every match that loads it.
func (s *Store) DeckLoader(reg *cards.Registry, next func(string) (host.Deck, error)) func(string) (host.Deck, error) {
	var mu sync.Mutex
	cache := map[string]host.Deck{}
	return func(id string) (host.Deck, error) {
		m := deckIDShape.FindStringSubmatch(id)
		if m == nil {
			if strings.HasPrefix(id, "ev/") {
				return host.Deck{}, fmt.Errorf("eventhost: %q is not an event deck id", id)
			}
			if next == nil {
				return host.Deck{}, host.ErrNotFound
			}
			return next(id)
		}
		mu.Lock()
		defer mu.Unlock()
		if d, ok := cache[id]; ok {
			return d, nil
		}
		seat, _ := strconv.Atoi(m[2])
		rev, _ := strconv.Atoi(m[3])
		f, err := s.readDeck(m[1], seat, rev)
		if err != nil {
			return host.Deck{}, err
		}
		cs, err := f.Resolve(reg)
		if err != nil {
			return host.Deck{}, err
		}
		d := host.Deck{Name: f.Name, Cards: cs}
		cache[id] = d
		return d, nil
	}
}

// deckPath is one stored deck revision's path: <dir>/<event>/decks/<seat>-<rev>.json.
func (s *Store) deckPath(eventID string, seat, rev int) string {
	return filepath.Join(s.dir, eventID, "decks", fmt.Sprintf("%d-%d.json", seat, rev))
}

// writeDeck persists one event deck revision: memory mode a map, disk mode
// the deck file written atomically (writeFileAtomic: temp, fsync, rename).
func (s *Store) writeDeck(eventID string, seat, rev int, f deck.File) error {
	b, err := json.MarshalIndent(&f, "", "  ")
	if err != nil {
		return fmt.Errorf("eventhost: marshal deck %s rev %d: %w", eventID, rev, err)
	}
	b = append(b, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		e, ok := s.mem[eventID]
		if !ok {
			e = &memEvent{}
			s.mem[eventID] = e
		}
		if e.decks == nil {
			e.decks = map[[2]int]deck.File{}
		}
		e.decks[[2]int{seat, rev}] = f
		return nil
	}
	if err := os.MkdirAll(filepath.Join(s.dir, eventID, "decks"), 0o755); err != nil {
		return fmt.Errorf("eventhost: store %s: %w", s.dir, err)
	}
	if err := writeFileAtomic(s.deckPath(eventID, seat, rev), b); err != nil {
		return fmt.Errorf("eventhost: store %s: %w", s.dir, err)
	}
	return nil
}

// readDeck reads one event deck revision back. A missing one is
// host.ErrNotFound: the id was well-formed but names nothing.
func (s *Store) readDeck(eventID string, seat, rev int) (deck.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		e, ok := s.mem[eventID]
		if !ok {
			return deck.File{}, fmt.Errorf("eventhost: %w (event %s)", host.ErrNotFound, eventID)
		}
		f, ok := e.decks[[2]int{seat, rev}]
		if !ok {
			return deck.File{}, fmt.Errorf("eventhost: %w (event %s seat %d rev %d)", host.ErrNotFound, eventID, seat, rev)
		}
		return f, nil
	}
	b, err := os.ReadFile(s.deckPath(eventID, seat, rev))
	if errors.Is(err, os.ErrNotExist) {
		return deck.File{}, fmt.Errorf("eventhost: %w (event %s seat %d rev %d)", host.ErrNotFound, eventID, seat, rev)
	}
	if err != nil {
		return deck.File{}, fmt.Errorf("eventhost: read deck %s: %w", eventID, err)
	}
	var f deck.File
	if err := json.Unmarshal(b, &f); err != nil {
		return deck.File{}, fmt.Errorf("eventhost: parse deck %s: %w", eventID, err)
	}
	return f, nil
}
