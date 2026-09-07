package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Command gorged runs perpetual bot tables and serves them to browsers:
// the host library behind a net/http server with the Svelte client
// embedded. It is the M2a deliverable; mtgserve embeds the same packages.
type config struct {
	addr, cards, decks, dir, spectator string
	tables, seats                      int
	pace, cooldown                     time.Duration
	seed                               uint64
	perpetual                          bool
	// mulligans is the London mulligan allowance the served tables hand the
	// engine: each player may take up to this many mulligans between the deal
	// and turn 1 (R-E5-1). Defaults to 1 — a mulligan is the first decision of
	// a real game — and 0 restores the pre-task behaviour exactly (no round).
	mulligans int
	// formatsRaw is the -format flag: a comma-separated table-format list,
	// table i taking formats[(i-1) mod n]. "constructed" (the default) and
	// "commander" are the two names — the smallest thing that lets a person
	// run both formats on one server is a two-element list, no table
	// configuration language. applyFormats parses it into formats at serve
	// time so a malformed list fails before any table is added (R-E3-1).
	formatsRaw string
	formats    []host.Format
	// humansRaw is the -humans flag: a comma-separated list of table t1's
	// slot indices that are real people. Empty stays all-bot, identical to
	// today. applyHumans parses it into humans at serve time so a malformed
	// list fails before any table is added (R-E3-1).
	humansRaw string
	humans    []int
	// seatToken is the -seat-token flag: a fixed bearer token for the first
	// human slot instead of a random per-slot one. Tests and local use only
	// (R-E3-3) — production runs mint random tokens.
	seatToken string
}

func main() {
	var c config
	flag.StringVar(&c.addr, "addr", ":8080", "listen address")
	flag.StringVar(&c.cards, "cards", ".cards", "corpus directory (ir.gob.gz / cardsfolder)")
	flag.StringVar(&c.decks, "decks", "internal/testutil/decks", "directory of deck JSON files")
	flag.IntVar(&c.tables, "tables", 4, "number of tables")
	flag.IntVar(&c.seats, "seats", 4, "seats per table")
	flag.DurationVar(&c.pace, "pace", 500*time.Millisecond, "sleep after every decision; 0 = as fast as possible")
	flag.DurationVar(&c.cooldown, "cooldown", 5*time.Second, "pause between matches on a perpetual table")
	flag.StringVar(&c.dir, "dir", "gorged-data", "persistence directory")
	flag.StringVar(&c.spectator, "spectator", "omniscient", "spectator visibility: public or omniscient")
	flag.Uint64Var(&c.seed, "seed", 1, "seed of table 1; table i uses seed+i-1")
	flag.BoolVar(&c.perpetual, "perpetual", true, "start a new match when one ends")
	flag.IntVar(&c.mulligans, "mulligans", 1, "London mulligans per player before turn 1; 0 disables the pre-game round")
	flag.StringVar(&c.formatsRaw, "format", "constructed", "comma-separated table formats (constructed, commander); table i uses formats[i-1 mod n], e.g. -format commander,constructed runs one of each")
	flag.StringVar(&c.humansRaw, "humans", "", "comma-separated slots of table t1 that are real people (e.g. 0,2); t2..tN stay bot tables")
	flag.StringVar(&c.seatToken, "seat-token", "", "fixed bearer token for the first human slot (tests and local use only; default mints a random token per slot)")
	flag.Parse()

	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gorged:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, c, ln); err != nil {
		fmt.Fprintln(os.Stderr, "gorged:", err)
		os.Exit(1)
	}
}

// serve runs until ctx is cancelled, then aborts live matches and shuts
// the server down. Split from main so a test can drive it on a random
// port.
func serve(ctx context.Context, c config, ln net.Listener) error {
	vis, err := view.ParseVisibility(c.spectator)
	if err != nil {
		return err
	}
	if vis == view.Seat {
		return fmt.Errorf("-spectator must be public or omniscient")
	}
	reg, err := cards.OpenCorpus(c.cards)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run make fetch-cards compile-cards)", c.cards, err)
	}
	// R-E3-1: -humans applies to table t1 alone (SeatClaim carries no
	// table, so one human table is the only configuration in which an
	// un-table-scoped claim is honest). Parse it now so a malformed list or
	// "-humans with -tables 0" fails before anything listens.
	if err := c.applyHumans(); err != nil {
		return err
	}
	if err := c.applyFormats(); err != nil {
		return err
	}
	// The deck directory is split into the two pools the tables deal: the
	// commander-declared files (each validated up front, below) and the
	// constructed rest. One directory serves both formats, so the numbered
	// tables can run a Commander game and a constructed game side by side
	// without any table-level deck configuration.
	cmdPool, conPool, err := splitDecks(reg, c.decks)
	if err != nil {
		return err
	}
	if len(cmdPool)+len(conPool) == 0 {
		return fmt.Errorf("no deck files in %s", c.decks)
	}
	// A format with no deck to deal is a configuration error, not a table
	// that fills its seats with the wrong pool: name the gap so a misbuild
	// is fixed, not guessed.
	if c.wantsFormat(host.FormatCommander) && len(cmdPool) == 0 {
		return fmt.Errorf("-format includes commander but no deck in %s names a commander", c.decks)
	}
	if c.wantsFormat(host.FormatConstructed) && len(conPool) == 0 {
		return fmt.Errorf("-format includes constructed but no deck in %s is a constructed deck", c.decks)
	}

	r, err := host.New(c.hostOptions(reg, deckLoader(reg, c.decks)))
	if err != nil {
		return err
	}
	if len(r.Tables()) == 0 {
		for _, cfg := range c.tableConfigs(cmdPool, conPool, vis) {
			if err := r.AddTable(cfg); err != nil {
				return err
			}
		}
	}
	if err := r.StartAll(); err != nil {
		return err
	}
	opts := httpapi.Options{Web: webFS()}
	var gate *seatGate
	if len(c.humans) > 0 {
		// R-E3-3: arm Options.Seat with a real token check — one opaque
		// token per human slot, minted at startup. With no humans the
		// resolver stays nil and the server is spectator-only, exactly as
		// before the flag existed.
		gate, err = newSeatGate(c.seatToken, c.humans)
		if err != nil {
			return err
		}
		opts.Seat = gate.resolve
	}
	srv := &http.Server{Handler: httpapi.NewHandler(r, opts)}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	fmt.Fprintf(os.Stderr, "gorged: %d tables of %d on %s (dir %s)\n", len(r.Tables()), c.seats, ln.Addr(), c.dir)
	if gate != nil {
		for _, s := range c.humans {
			seat := state.PlayerID(s)
			// The path is the LIVE table route, not "/". "/" is the
			// lobby, and clicking a live match there navigates to
			// /t/t1/m/1 -- the finished-match replay route, which paints
			// a frozen snapshot and never advances. A player who joined
			// through the lobby therefore ended up watching history while
			// the real game waited for them. The join URL goes straight
			// to the seat. Humans are on t1 alone (FL-97), so the table
			// is not a guess.
			fmt.Fprintf(os.Stderr, "gorged: table t1 seat %d joins at http://%s/t/t1?seat=%d&token=%s\n",
				s, joinHost(ln.Addr()), s, gate.token(seat))
		}
	}
	select {
	case err := <-errc:
		r.Close()
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return r.Close()
}

// applyHumans parses the -humans flag into c.humans and rejects the
// configurations R-E3-1 forbids up front: a malformed list, or humans with
// no table to seat them on. A slot index out of range is deliberately not
// checked again here — TableConfig.validate owns that check, and the t1
// AddTable fails with the same information.
func (c *config) applyHumans() error {
	if strings.TrimSpace(c.humansRaw) == "" {
		c.humans = nil
		return nil
	}
	parts := strings.Split(c.humansRaw, ",")
	humans := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return fmt.Errorf("-humans %q: %q is not a slot index", c.humansRaw, p)
		}
		humans = append(humans, n)
	}
	if c.tables < 1 {
		return fmt.Errorf("-humans %s: needs at least one table (-tables 1..)", c.humansRaw)
	}
	c.humans = humans
	return nil
}

// applyFormats parses the -format flag into c.formats. The list is cycled
// over the tables — a single -format commander makes every table a
// Commander game, and -format commander,constructed runs one of each — and
// a name that is neither constructed nor commander fails here, before any
// table is added (R-E3-1), so a typo'd format can never half-start a
// server of wrong-format tables.
func (c *config) applyFormats() error {
	c.formats = nil
	// The zero value (no -format flag at all — tests, and a config that
	// never sets the field) means constructed, exactly what every table
	// before this flag was; applyHumans follows the same empty-means-
	// default convention.
	if strings.TrimSpace(c.formatsRaw) == "" {
		c.formats = []host.Format{host.FormatConstructed}
		return nil
	}
	for _, p := range strings.Split(c.formatsRaw, ",") {
		f, err := host.ParseFormat(strings.TrimSpace(p))
		if err != nil {
			return err
		}
		c.formats = append(c.formats, f)
	}
	return nil
}

// wantsFormat reports whether any table in the configuration plays format f.
func (c config) wantsFormat(f host.Format) bool {
	for _, g := range c.formats {
		if g == f {
			return true
		}
	}
	return false
}

// splitDecks lists the deck stems in dir (deckFiles) and splits them into
// the commander and constructed pools a table of each format deals, then
// validates every commander deck up front (deck.ValidateCommander, the m35
// CR 903.4/903.5 gate) so a deployed commander table can never half-start
// on an illegal deck — the deck is named in the error. A file that names a
// commander belongs to the commander pool and every other file to the
// constructed pool: the two pools are disjoint and the five Foundations
// decks can never be dealt as 100-card constructed piles. Both pools keep
// deckFiles' sorted order, so seat assignment stays deterministic.
func splitDecks(reg *cards.Registry, dir string) (commander, constructed []string, err error) {
	names, err := deckFiles(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range names {
		f, _, lerr := deck.Load(reg, filepath.Join(dir, n+".json"))
		if lerr != nil {
			return nil, nil, lerr
		}
		if f.Commander == "" {
			constructed = append(constructed, n)
			continue
		}
		if verr := f.ValidateCommander(reg); verr != nil {
			return nil, nil, fmt.Errorf("commander deck %q: %w", n, verr)
		}
		commander = append(commander, n)
	}
	return commander, constructed, nil
}

// tableConfigs builds one TableConfig per table, in ID order, dealing each
// table the deck pool of its format — commander tables the commander pool,
// constructed tables the constructed pool — so one server runs both formats
// side by side and a commander deck is never dealt as a 100-card
// constructed pile. A table the -format list does not reach (fewer entries
// than tables) is constructed, the zero value. R-E3-1: the human slots
// apply to table t1 alone — SeatClaim carries no table, so a claim minted
// for t1 seat s would satisfy the same seat on every table. R-E3-2: a
// human-seated table is single-shot by definition, and the -perpetual flag
// defaults to true, so a naive copy of the bot config would make AddTable
// reject it (perpetual+humans); Perpetual is forced false for t1,
// regardless of the flag, and the bot tables keep the flag. AddTable still
// validates the result (slot range, duplicates, commander decks), so serve
// fails before listening on a bad -humans list or a commander table dealt
// a deck with no commander.
func (c config) tableConfigs(cmdPool, conPool []string, vis view.Visibility) []host.TableConfig {
	cfgs := make([]host.TableConfig, 0, c.tables)
	for i := 1; i <= c.tables; i++ {
		format := host.FormatConstructed
		if len(c.formats) > 0 {
			format = c.formats[(i-1)%len(c.formats)]
		}
		pool := conPool
		if format == host.FormatCommander {
			pool = cmdPool
		}
		cfg := host.TableConfig{ID: host.TableID(fmt.Sprintf("t%d", i)), Name: fmt.Sprintf("Table %d", i), Seats: c.seats,
			Decks: pool, Seed: c.seed + uint64(i-1), Pace: c.pace, Spectator: vis, Perpetual: c.perpetual,
			Mulligans: c.mulligans, Format: format}
		if i == 1 && len(c.humans) > 0 {
			cfg.Humans = c.humans
			cfg.Perpetual = false
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs
}

// hostOptions builds the Registry options for a config, threading the FL-40
// token corpus (reg.Tokens) into host.Options.Tokens so every live match and
// its replay mints the same tokens. Split from serve so TestHostThreads
// CorpusTokens can assert the thread works without driving the whole server.
func (g config) hostOptions(reg *cards.Registry, load func(string) (host.Deck, error)) host.Options {
	// Sleep is left nil: host installs its default interruptible sleep, the
	// package's only sanctioned clock read, so a long pace or cooldown still
	// yields to Close.
	//
	// MaxDecisionsPerTurn arms the host's per-turn progress guard (Task HW1)
	// for every served table at the recommended value: a bot policy stall
	// that re-offers one no-op action forever — the 36-of-200 frozen
	// four-seat Commander games measured on main — previously left the table
	// TableLive with no match ever following, because ThinkTimeout only
	// covers a parked human seat. The value cannot fire on a legitimate
	// turn (measured legit ceiling 244 decisions; the stall ran ~10000 per
	// turn; 25000 is 100x the first and swept by the second within three
	// stall-turns); the count-based design keeps a match's outcome a pure
	// function of its seed on every machine and every replay, exactly like
	// the engine and the bots themselves.
	return host.Options{Dir: g.dir, LoadDeck: load, Tokens: reg.Tokens, Sync: true, Cooldown: g.cooldown,
		MaxDecisionsPerTurn: host.DefaultMaxDecisionsPerTurn}
}

// deckFiles lists the deck names (file stems) in dir, sorted, so seat
// assignment is the same on every machine.
func deckFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, deck.Stem(e.Name()))
		}
	}
	sort.Strings(names)
	return names, nil
}

// deckLoader resolves a name to dir/<name>.json once and caches it: the
// host asks for the same decks every match. The seat is named after the
// file stem (PL-14), not the deck file's own name field; the File is read
// anyway because a deck that names a commander carries its command-zone
// index on the host Deck (deck.File.CommanderIndex — the same resolution
// rules' own commander games use, and the pool split validated up front),
// so a commander table's seats get their command zone and a constructed
// table's seats get none.
func deckLoader(reg *cards.Registry, dir string) func(string) (host.Deck, error) {
	var mu sync.Mutex
	cache := map[string]host.Deck{}
	return func(name string) (host.Deck, error) {
		mu.Lock()
		defer mu.Unlock()
		if d, ok := cache[name]; ok {
			return d, nil
		}
		f, cs, err := deck.Load(reg, filepath.Join(dir, name+".json"))
		if err != nil {
			return host.Deck{}, err
		}
		d := host.Deck{Name: name, Cards: cs}
		if f.Commander != "" {
			d.Commanders = []int{f.CommanderIndex()}
		}
		cache[name] = d
		return d, nil
	}
}
