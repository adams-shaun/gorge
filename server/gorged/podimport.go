package gorged

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/manabrew"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// deckImports holds the decks a client submitted with POST /api/games, under
// the ids their tables name them by, and checks each one before it is
// admitted. The decks are process-scoped like the on-demand tables that deal
// them: the host asks the loader for them at match start and again whenever
// it replays a match, so they stay for the life of the process.
type deckImports struct {
	reg       *cards.Registry
	supported map[string]bool

	mu    sync.Mutex
	decks map[string]host.Deck
}

func newDeckImports(reg *cards.Registry) *deckImports {
	return &deckImports{reg: reg, supported: effects.Supported(), decks: map[string]host.Deck{}}
}

// loader answers an imported deck's id and hands every other name to next.
func (d *deckImports) loader(next func(string) (host.Deck, error)) func(string) (host.Deck, error) {
	return func(name string) (host.Deck, error) {
		d.mu.Lock()
		got, ok := d.decks[name]
		d.mu.Unlock()
		if ok {
			return got, nil
		}
		return next(name)
	}
}

// maxStoredImports bounds how many submitted decks the process holds: tables
// are process-scoped, so a deck is never evicted, and POST /api/games is open
// to any client. A full store refuses a request that would add to it.
const maxStoredImports = 512

// putAll stores the submitted decks under ids (parallel to decks by seat, ""
// where a seat has none), or stores nothing when that would pass the cap.
func (d *deckImports) putAll(ids []string, decks map[int]host.Deck) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	add := 0
	for _, id := range ids {
		if id != "" {
			add++
		}
	}
	if len(d.decks)+add > maxStoredImports {
		return fmt.Errorf("this server already holds %d submitted decks, the most it keeps: use a deck from its pool", len(d.decks))
	}
	for i, id := range ids {
		if id != "" {
			d.decks[id] = decks[i]
		}
	}
	return nil
}

func (d *deckImports) drop(ids ...string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	for _, id := range ids {
		delete(d.decks, id)
	}
	d.mu.Unlock()
}

// maxImportedDeckCards bounds a submitted deck's main board: a Commander deck
// is 100 and no constructed deck the engine seats needs more than a few
// hundred.
const maxImportedDeckCards = 250

// minConstructedDeckCards is the smallest constructed deck an imported seat
// may bring: the 40 of the smallest limited deck.
const minConstructedDeckCards = 40

// check parses one submitted ManaBrew deck and refuses it unless the server
// itself finds it fit for the table's format: every card resolves, the engine
// can play every card, and the deck is legal (Commander construction for a
// commander table; no commander and a legal size for a constructed one).
// The client's own claims -- its format field, its validity -- decide nothing.
func (d *deckImports) check(format host.Format, seat int, raw json.RawMessage) (host.Deck, error) {
	var md mb.Deck
	if err := json.Unmarshal(raw, &md); err != nil {
		return host.Deck{}, fmt.Errorf("seat %d deck is not a ManaBrew deck: %w", seat, err)
	}
	label := fmt.Sprintf("seat %d deck %q", seat, md.Name)
	if err := deckFormatMatches(format, md.Format); err != nil {
		return host.Deck{}, fmt.Errorf("%s: %w", label, err)
	}
	if len(md.Cards) > maxImportedDeckCards {
		return host.Deck{}, fmt.Errorf("%s has %d cards, more than the %d a table seats", label, len(md.Cards), maxImportedDeckCards)
	}
	f, rep, err := manabrew.ImportDeck(d.reg, md, d.supported)
	var why []string
	if err != nil {
		why = append(why, fmt.Sprintf("cards not in the card pool: %s", capNames(rep.Unresolved)))
	}
	if len(rep.Unsupported) > 0 {
		names := make([]string, 0, len(rep.Unsupported))
		for n := range rep.Unsupported {
			names = append(names, n)
		}
		sort.Strings(names)
		parts := make([]string, len(names))
		for i, n := range names {
			parts[i] = fmt.Sprintf("%s (needs %s)", n, strings.Join(rep.Unsupported[n], ", "))
		}
		why = append(why, fmt.Sprintf("cards the engine cannot play yet: %s", capNames(parts)))
	}
	if len(why) > 0 {
		return host.Deck{}, fmt.Errorf("%s: %s", label, strings.Join(why, "; "))
	}
	if err := checkImportedLegality(d.reg, format, f); err != nil {
		return host.Deck{}, fmt.Errorf("%s: %w", label, err)
	}
	cs, err := f.Resolve(d.reg)
	if err != nil {
		return host.Deck{}, fmt.Errorf("%s: %w", label, err)
	}
	sb, err := f.ResolveSideboard(d.reg)
	if err != nil {
		return host.Deck{}, fmt.Errorf("%s: %w", label, err)
	}
	out := host.Deck{Name: f.Name, Cards: cs, Sideboard: sb, Archetype: f.Archetype}
	if len(f.CommanderNames()) > 0 {
		out.Commanders = f.CommanderIndices()
	}
	return out, nil
}

// deckFormatMatches compares a submitted deck's declared format to the
// table's. An empty declaration is accepted (the server judges legality
// itself); a declared Commander deck is refused at a constructed table and a
// declared non-Commander format at a commander one.
func deckFormatMatches(format host.Format, declared string) error {
	d := strings.ToLower(strings.TrimSpace(declared))
	isCommander := d == "commander" || d == "edh"
	switch {
	case d == "":
		return nil
	case format == host.FormatCommander && !isCommander:
		return fmt.Errorf("format %q is not commander", declared)
	case format != host.FormatCommander && isCommander:
		return fmt.Errorf("format %q is not %s", declared, format)
	}
	return nil
}

func checkImportedLegality(reg *cards.Registry, format host.Format, f deck.File) error {
	if format == host.FormatCommander {
		if len(f.CommanderNames()) == 0 {
			return fmt.Errorf("names no commander, and a commander table seats only commander decks")
		}
		return f.ValidateCommander(reg)
	}
	if len(f.CommanderNames()) > 0 {
		return fmt.Errorf("names a commander, and a constructed table seats only constructed decks")
	}
	total := 0
	for _, e := range f.Cards {
		total += e.Count
	}
	if total < minConstructedDeckCards {
		return fmt.Errorf("has %d cards, but a constructed deck needs at least %d", total, minConstructedDeckCards)
	}
	return nil
}

// diracSeats is the seat count -engine dirac plays a table at; a Commander pod
// seats diracPodSeats.
const (
	diracSeats    = 2
	diracPodSeats = 4
)

// checkPodShape refuses a request this server cannot seat before any work is
// done: a table past what -engine dirac plays, a submitted deck dirac was
// never checked against, deck import switched off, and the two-seat deck
// picks, which name no seat of a pod.
func (c config) checkPodShape(req httpapi.CreateGameOptions, seats, humans int) error {
	switch {
	case seats < 2 || seats > 4:
		return fmt.Errorf("seats %d, want 2 to 4", seats)
	case humans < 1 || humans > seats:
		return fmt.Errorf("humans %d, want 1 to %d", humans, seats)
	case seats > 2 && req.Format != host.FormatCommander:
		return fmt.Errorf("a %d-seat table needs format commander, not %s", seats, req.Format)
	case c.diracOn() && seats != diracSeats && seats != diracPodSeats:
		return fmt.Errorf("-engine %s plays %d seats or a %d-seat Commander pod, so it cannot seat a %d-seat pod: use the gorge engine", c.engine, diracSeats, diracPodSeats, seats)
	case c.diracOn() && len(req.Decks) > 0:
		return fmt.Errorf("-engine %s plays only the decks it checked at startup, not a submitted deck: use the gorge engine", c.engine)
	case len(req.Decks) > 0 && c.imports == nil:
		return fmt.Errorf("deck import is not enabled on this server")
	case seats > 2 && (req.HumanDeck != "" || req.BotDeck != ""):
		return fmt.Errorf("human_deck and bot_deck name the two seats of a two-seat table; a %d-seat pod takes decks per seat in decks", seats)
	}
	return nil
}

// checkReservedPool refuses a submitted deck at a table whose bot policy has a
// reserved pool: that policy plays only the decks its network was trained on,
// at every seat, exactly as for human_deck and bot_deck. It is the one home of
// that rule for the decks field and runs before any deck is read.
func checkReservedPool(req httpapi.CreateGameOptions, policy string, hasPool bool) error {
	if hasPool && len(req.Decks) > 0 {
		return fmt.Errorf("bot policy %q plays only the decks it was trained on, not a submitted deck: leave decks out", policy)
	}
	return nil
}

// importSeatDecks checks every submitted deck and returns them by seat.
func (c config) importSeatDecks(req httpapi.CreateGameOptions) (map[int]host.Deck, error) {
	if len(req.Decks) == 0 {
		return nil, nil
	}
	out := make(map[int]host.Deck, len(req.Decks))
	for _, sd := range req.Decks {
		d, err := c.imports.check(req.Format, sd.Seat, sd.Deck)
		if err != nil {
			return nil, err
		}
		out[sd.Seat] = d
	}
	return out, nil
}

// podDecks assigns a deck id to every seat: a submitted deck gets a fresh
// import id, the two-seat human_deck/bot_deck picks name seats 0 and 1, and
// every seat left over is dealt from the shuffled pool, skipping ids already
// seated and cycling (a mirror) only when the pool runs out. want is by seat;
// ids is the import id of each seat that has one ("" elsewhere), parallel to
// want.
func podDecks(req httpapi.CreateGameOptions, shuffled []string, submitted map[int]host.Deck, table host.TableID, seats int) (want, ids []string, err error) {
	want, ids = make([]string, seats), make([]string, seats)
	for seat := range submitted {
		ids[seat] = fmt.Sprintf("import-%s-s%d", table, seat)
		want[seat] = ids[seat]
	}
	for seat, pick := range []string{req.HumanDeck, req.BotDeck} {
		if pick == "" || seat >= seats {
			continue
		}
		if want[seat] != "" {
			return nil, nil, fmt.Errorf("seat %d names both a pool deck (%q) and a submitted deck", seat, pick)
		}
		want[seat] = pick
	}
	used := map[string]bool{}
	for _, id := range want {
		used[id] = true
	}
	next := 0
	for seat := range want {
		if want[seat] != "" {
			continue
		}
		pick := ""
		for next < len(shuffled) && pick == "" {
			if id := shuffled[next]; !used[id] {
				pick = id
			}
			next++
		}
		if pick == "" {
			pick = shuffled[seat%len(shuffled)]
		}
		want[seat] = pick
		used[pick] = true
	}
	return want, ids, nil
}

// rotateForMatchOne orders want, by seat, as TableConfig.Decks: match 1 of a
// table plays Decks[(seat+1)%len], so seat i's deck sits one place after.
func rotateForMatchOne(want []string) []string {
	out := make([]string, len(want))
	for i, id := range want {
		out[(i+1)%len(want)] = id
	}
	return out
}

func humanSeats(humans int) []int {
	out := make([]int, humans)
	for i := range out {
		out[i] = i
	}
	return out
}

func podTableName(format host.Format, seats int) string {
	if seats == 2 {
		return fmt.Sprintf("Play vs bot (%s)", format)
	}
	return fmt.Sprintf("%s pod (%d seats)", format, seats)
}

// podPlayerNames keeps the two-seat names ("You", "Bot") and numbers the seats
// of a pod or a table with several people at it.
func podPlayerNames(seats, humans int) []string {
	names := make([]string, seats)
	for i := range names {
		switch {
		case i < humans && humans == 1:
			names[i] = "You"
		case i < humans:
			names[i] = fmt.Sprintf("Player %d", i+1)
		case seats == 2:
			names[i] = "Bot"
		default:
			names[i] = fmt.Sprintf("Bot %d", i+1)
		}
	}
	return names
}

// capNames joins names for an error, the first few and a count of the rest, so
// a deck of mostly unknown names does not answer with a page.
func capNames(names []string) string {
	const shown = 12
	if len(names) <= shown {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, and %d more", strings.Join(names[:shown], ", "), len(names)-shown)
}
