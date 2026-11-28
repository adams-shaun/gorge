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
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/manabrew"
	"github.com/adams-shaun/gorge/server/protocol"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// deckImports holds the decks a client submitted with POST /api/games, under
// the ids their tables name them by, and checks each one before it is
// admitted. A table's decks live as long as the table: the host asks the
// loader for them at match start and again whenever it replays a match, and
// dropTable releases them when the registry reaps the finished table.
type deckImports struct {
	reg       *cards.Registry
	supported map[string]bool
	// engine names the engine a report is for; demand, when non-nil, logs
	// each checked deck's unsupported names (-deck-demand-log).
	engine string
	demand *demandLog

	mu    sync.Mutex
	decks map[string]host.Deck
}

func newDeckImports(reg *cards.Registry) *deckImports {
	return &deckImports{reg: reg, supported: effects.Supported(), engine: gorgegame.Name, decks: map[string]host.Deck{}}
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

// maxStoredImports bounds how many submitted decks the process holds: a deck
// leaves only with its reaped table (dropTable), and POST /api/games is open
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

// dropTable releases every deck submitted for table, seats 0 through 3.
func (d *deckImports) dropTable(table host.TableID) {
	d.drop(importDeckID(table, 0), importDeckID(table, 1), importDeckID(table, 2), importDeckID(table, 3))
}

// importDeckID is the id a deck submitted for table's seat is stored under.
func importDeckID(table host.TableID, seat int) string {
	return fmt.Sprintf("import-%s-s%d", table, seat)
}

// maxImportedDeckCards bounds a submitted deck's main board: a Commander deck
// is 100 and no constructed deck the engine seats needs more than a few
// hundred.
const maxImportedDeckCards = 250

// minConstructedDeckCards is the smallest constructed deck an imported seat
// may bring: the 40 of the smallest limited deck.
const minConstructedDeckCards = 40

// check parses one submitted deck -- ManaBrew JSON in sd.Deck, or a
// plain-text decklist in sd.Text -- and refuses it unless the server itself
// finds it fit for the table's format: every card resolves, the engine can
// play every card, and the deck is legal (Commander construction for a
// commander table; no commander and a legal size for a constructed one).
// With sd.Substitute it first replaces up to maxSubstituted unsupported cards
// with basics and reports each swap. The client's own claims -- its format
// field, its validity -- decide nothing.
func (d *deckImports) check(format host.Format, sd httpapi.SeatDeck) (host.Deck, protocol.DeckReport, error) {
	if sd.Text != "" {
		return d.checkText(format, sd.Seat, sd.Text, sd.Substitute)
	}
	var md mb.Deck
	if err := json.Unmarshal(sd.Deck, &md); err != nil {
		return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("seat %d deck is not a ManaBrew deck: %w", sd.Seat, err)
	}
	label := fmt.Sprintf("seat %d deck %q", sd.Seat, md.Name)
	if err := deckFormatMatches(format, md.Format); err != nil {
		return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("%s: %w", label, err)
	}
	if len(md.Cards) > maxImportedDeckCards {
		return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("%s has %d cards, more than the %d a table seats", label, len(md.Cards), maxImportedDeckCards)
	}
	f, rep, _ := manabrew.ImportDeck(d.reg, md, d.supported)
	return d.checkFile(label, format, f, rep.Unresolved, rep.Unsupported, sd.Substitute)
}

// checkText parses one seat's plain-text decklist and checks the deck it
// names exactly as a ManaBrew one. A line the parser cannot read is refused
// naming the line; a list past maxImportedDeckCards copies is refused before
// any card is resolved, so a huge count never reaches Resolve.
func (d *deckImports) checkText(format host.Format, seat int, text string, consent bool) (host.Deck, protocol.DeckReport, error) {
	label := fmt.Sprintf("seat %d deck", seat)
	tl, err := deck.ParseText([]byte(text))
	if err != nil {
		return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("%s: %w", label, err)
	}
	total := 0
	for _, e := range tl.Cards {
		if e.Count > maxImportedDeckCards || total+e.Count > maxImportedDeckCards {
			return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("%s has more than the %d cards a table seats", label, maxImportedDeckCards)
		}
		total += e.Count
	}
	f := tl.File(label)
	unresolved, unsupported := d.census(f)
	return d.checkFile(label, format, f, unresolved, unsupported, consent)
}

// census looks up every card f names -- main deck, sideboard and
// commanders -- and returns the names the card pool does not hold (first-seen order, no
// repeats) and, under each card's front-face name, the primitives the engine
// does not yet implement for the cards it does hold.
func (d *deckImports) census(f deck.File) (unresolved []string, unsupported map[string][]string) {
	unsupported = map[string][]string{}
	missed := map[string]bool{}
	names := make([]string, 0, len(f.Cards)+len(f.Sideboard)+len(f.Commanders))
	for _, e := range f.Cards {
		names = append(names, e.Name)
	}
	for _, e := range f.Sideboard {
		names = append(names, e.Name)
	}
	names = append(names, f.Commanders...)
	for _, n := range names {
		c, ok := d.reg.Lookup(n)
		if !ok {
			if !missed[n] {
				missed[n] = true
				unresolved = append(unresolved, n)
			}
			continue
		}
		if missing := d.reg.Unsupported(c, d.supported); len(missing) > 0 {
			unsupported[frontFaceName(c, n)] = missing
		}
	}
	return unresolved, unsupported
}

// frontFaceName is the card's front-face name, or name when the card carries
// none.
func frontFaceName(c *cards.Card, name string) string {
	if len(c.Faces) == 0 || c.Faces[0].Name == "" {
		return name
	}
	return c.Faces[0].Name
}

// checkFile reports on f (and logs its demand), substitutes the unsupported
// cards when the seat consented, then refuses it with a
// httpapi.DeckRefused carrying that report, naming the cards, unless every card resolved and the
// engine plays every card, then checks it for the table's format and
// resolves it into the deck the table seats. Both the ManaBrew and the text
// path end here.
func (d *deckImports) checkFile(label string, format host.Format, f deck.File, unresolved []string, unsupported map[string][]string, consent bool) (host.Deck, protocol.DeckReport, error) {
	orig := f
	var (
		substituted []protocol.DeckSubstitution
		problem     string
	)
	if consent && len(unresolved) == 0 {
		f, substituted, problem = substitute(d.reg, f, unsupported)
	}
	rep := d.report(format, f.Name, f, orig, unresolved, unsupported, d.engine, substituted)
	if problem != "" {
		rep.Problems = append(rep.Problems, problem)
		rep.Legal, rep.Playable = false, false
	}
	d.demand.record(d.engine, rep)
	refuse := func(msg string) (host.Deck, protocol.DeckReport, error) {
		return host.Deck{}, rep, httpapi.DeckRefused{Report: rep, Msg: msg}
	}
	var why []string
	if len(unresolved) > 0 {
		why = append(why, fmt.Sprintf("cards not in the card pool: %s", capNames(unresolved)))
	}
	if len(unsupported) > 0 && len(substituted) == 0 {
		names := make([]string, 0, len(unsupported))
		for n := range unsupported {
			names = append(names, n)
		}
		sort.Strings(names)
		parts := make([]string, len(names))
		for i, n := range names {
			parts[i] = fmt.Sprintf("%s (needs %s)", n, strings.Join(unsupported[n], ", "))
		}
		why = append(why, fmt.Sprintf("cards the engine cannot play yet: %s", capNames(parts)))
	}
	if problem != "" {
		why = append(why, problem)
	}
	if len(why) > 0 {
		return refuse(fmt.Sprintf("%s: %s", label, strings.Join(why, "; ")))
	}
	if err := checkImportedLegality(d.reg, format, f); err != nil {
		return refuse(fmt.Sprintf("%s: %v", label, err))
	}
	cs, err := f.Resolve(d.reg)
	if err != nil {
		return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("%s: %w", label, err)
	}
	sb, err := f.ResolveSideboard(d.reg)
	if err != nil {
		return host.Deck{}, protocol.DeckReport{}, fmt.Errorf("%s: %w", label, err)
	}
	out := host.Deck{Name: f.Name, Cards: cs, Sideboard: sb, Archetype: f.Archetype}
	if len(f.CommanderNames()) > 0 {
		out.Commanders = f.CommanderIndices()
	}
	return out, rep, nil
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

// importSeatDecks checks every submitted deck and returns them by seat,
// beside one report per submitted seat in seat order.
func (c config) importSeatDecks(req httpapi.CreateGameOptions) (map[int]host.Deck, []protocol.DeckReport, error) {
	if len(req.Decks) == 0 {
		return nil, nil, nil
	}
	seats := append([]httpapi.SeatDeck{}, req.Decks...)
	sort.Slice(seats, func(i, j int) bool { return seats[i].Seat < seats[j].Seat })
	out := make(map[int]host.Deck, len(req.Decks))
	reports := make([]protocol.DeckReport, 0, len(req.Decks))
	for _, sd := range seats {
		d, rep, err := c.imports.check(req.Format, sd)
		if err != nil {
			return nil, nil, err
		}
		out[sd.Seat] = d
		reports = append(reports, rep)
	}
	return out, reports, nil
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
		ids[seat] = importDeckID(table, seat)
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

// tableRemover is the registry's OnTableRemoved hook: a reaped table's
// submitted decks and seat tokens go with it. A nil gate revokes nothing.
func tableRemover(imports *deckImports, gate *seatGate) func(host.TableID) {
	return func(id host.TableID) {
		imports.dropTable(id)
		gate.revoke(id)
	}
}
