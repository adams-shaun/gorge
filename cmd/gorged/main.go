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
	// vsbot is the -vsbot flag: arm the on-demand play-vs-bot flow so the
	// landing page can seat a human against a bot (POST /api/games). It is
	// OPT-IN — default off — so a server that never turned it on keeps the
	// spectator-only default exactly as it was before the feature existed
	// (Options.Seat stays nil and a seat-scoped request is refused 403,
	// pinned by TestNoHumansIsSpectatorOnly). When on, Options.Seat is armed
	// so the per-game seat token resolves, and a no-token seat request
	// declines to 401 rather than 403.
	vsbot bool
	// seatToken is the -seat-token flag: a fixed bearer token for the first
	// human slot instead of a random per-slot one. Tests and local use only
	// (R-E3-3) — production runs mint random tokens.
	seatToken string
	// feedback is the -feedback flag: where a player's own bug reports are
	// written (feedback.go). It is deliberately NOT under -dir. The
	// persistence directory is disposable by design — scripts/deploy-demo.sh
	// `rm -rf`s it on every deploy so a config written by an older binary
	// cannot come back with zero-valued fields — which would mean a report
	// filed against a bug was destroyed by the deploy that shipped its fix.
	// A report is the one artifact here that cannot be regenerated.
	feedback string
}

func main() {
	var c config
	flag.StringVar(&c.addr, "addr", ":8080", "listen address")
	flag.StringVar(&c.cards, "cards", ".cards", "corpus directory (ir.gob.gz / cardsfolder)")
	flag.StringVar(&c.decks, "decks", "internal/testutil/decks", "directory of deck JSON files")
	flag.IntVar(&c.tables, "tables", 4, "number of tables")
	flag.IntVar(&c.seats, "seats", 4, "seats per table")
	flag.DurationVar(&c.pace, "pace", 250*time.Millisecond, "sleep after every decision; 0 = as fast as possible")
	flag.DurationVar(&c.cooldown, "cooldown", 5*time.Second, "pause between matches on a perpetual table")
	flag.StringVar(&c.dir, "dir", "gorged-data", "persistence directory")
	flag.StringVar(&c.feedback, "feedback", "feedback", "directory player bug reports are written to (kept OUT of -dir, which deploys wipe)")
	flag.StringVar(&c.spectator, "spectator", "omniscient", "spectator visibility: public or omniscient")
	flag.Uint64Var(&c.seed, "seed", 1, "seed of table 1; table i uses seed+i-1")
	flag.BoolVar(&c.perpetual, "perpetual", true, "start a new match when one ends")
	flag.IntVar(&c.mulligans, "mulligans", 1, "London mulligans per player before turn 1; 0 disables the pre-game round")
	flag.StringVar(&c.formatsRaw, "format", "constructed", "comma-separated table formats (constructed, commander); table i uses formats[i-1 mod n], e.g. -format commander,constructed runs one of each")
	flag.StringVar(&c.humansRaw, "humans", "", "comma-separated slots of table t1 that are real people (e.g. 0,2); t2..tN stay bot tables")
	flag.StringVar(&c.seatToken, "seat-token", "", "fixed bearer token for the first human slot (tests and local use only; default mints a random token per slot)")
	flag.BoolVar(&c.vsbot, "vsbot", false, "arm the on-demand play-vs-bot flow (landing page seats a human against a bot via POST /api/games)")
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
	deckCatalogue, err := loadDeckCatalogue(c.decks, cmdPool, conPool)
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

	load := deckLoader(reg, c.decks)
	r, err := host.New(c.hostOptions(reg, load))
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
	opts := httpapi.Options{Web: webFS(), Decks: deckCatalogue}
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
	// Task ui11: with -vsbot on and a deck pool to deal, arm the play-vs-bot
	// flow so the landing page can seat a human against a bot on demand. This
	// is additive — the startup tables above are untouched — and needs a gate
	// even when there were no -humans slots, so create one (an empty store)
	// and route the seat-scoped endpoints through it. Off (the default), the
	// server is spectator-only exactly as before, and a request naming a seat
	// is refused 403 (TestNoHumansIsSpectatorOnly).
	if c.vsbot && len(cmdPool)+len(conPool) > 0 {
		if gate == nil {
			gate = &seatGate{tokenToSeat: map[string]state.PlayerID{}, seatTokens: map[state.PlayerID]string{}}
		}
		opts.CreateGame = c.createGame(r, gate, cmdPool, conPool, vis)
		opts.Seat = gate.resolve
	}
	// Card art is cached and served from this server's own origin (art.go)
	// rather than sending every viewer's browser to Scryfall directly — see
	// art.go's doc comment. /art/ is routed ahead of httpapi's own mux (which
	// owns "/" for the SPA and every /api/ path), so this is additive: a
	// server with the art directory unavailable would only ever affect these
	// two new patterns, never anything httpapi already serves.
	ac, err := newArtCache(filepath.Join(c.dir, "art"))
	if err != nil {
		return err
	}
	// The card catalog (the six printed facts, /cards/named) is served from
	// the same art-cache records art.go keeps — one Scryfall named response
	// feeds both the image and the text — and is routed here, ahead of
	// httpapi's mux, for the same reason /art/ is: additive, never shadowing
	// anything httpapi already serves.
	// A player's own bug report (feedback.go), written to its own durable
	// directory rather than the disposable persistence dir — see config's
	// `feedback` field. Routed here, ahead of httpapi's mux, for the same
	// reason /art/ is: it is additive, and a server whose feedback directory
	// is unavailable fails at startup rather than at the first submission.
	topMux := http.NewServeMux()
	topMux.HandleFunc("GET /art/named", ac.named)
	topMux.HandleFunc("GET /art/blob/{key}", ac.blob)
	topMux.HandleFunc("GET /cards/named", ac.text)
	// An empty -feedback disarms the endpoint entirely, the same opt-in shape
	// Options.Seat and CreateGame use. That is what a config built in a test
	// gets (the flag default only applies to the real binary), so a test
	// server never creates a directory it did not ask for -- and a submission
	// to a server with feedback off is a 404 rather than a silent success.
	if c.feedback != "" {
		fb, err := newFeedbackStore(c.feedback)
		if err != nil {
			return err
		}
		topMux.HandleFunc("POST /api/feedback", fb.submit)
	}
	topMux.Handle("/", httpapi.NewHandler(r, opts))
	srv := &http.Server{Handler: topMux}
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

// loadDeckCatalogue reads display metadata for the exact pools splitDecks
// validated. The format exposed to the client is the server's pool format,
// not the deck file's free-form authoring field (constructed files currently
// call that field "custom"). Pool and catalogue therefore cannot disagree.
func loadDeckCatalogue(dir string, commander, constructed []string) ([]httpapi.DeckInfo, error) {
	formats := make(map[string]host.Format, len(commander)+len(constructed))
	for _, id := range commander {
		formats[id] = host.FormatCommander
	}
	for _, id := range constructed {
		formats[id] = host.FormatConstructed
	}
	ids, err := deckFiles(dir)
	if err != nil {
		return nil, err
	}
	out := make([]httpapi.DeckInfo, 0, len(ids))
	for _, id := range ids {
		format, ok := formats[id]
		if !ok {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, id+".json"))
		if err != nil {
			return nil, err
		}
		f, err := deck.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, id+".json"), err)
		}
		name := f.Name
		if name == "" {
			name = id
		}
		out = append(out, httpapi.DeckInfo{ID: id, Name: name, Format: format.String(), Archetype: f.Archetype, Commander: f.Commander})
	}
	return out, nil
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
