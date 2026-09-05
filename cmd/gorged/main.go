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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
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
}

func main() {
	var c config
	flag.StringVar(&c.addr, "addr", ":8080", "listen address")
	flag.StringVar(&c.cards, "cards", ".cards", "corpus directory (ir.gob.gz / cardsfolder)")
	flag.StringVar(&c.decks, "decks", "internal/testutil/decks", "directory of deck JSON files")
	flag.IntVar(&c.tables, "tables", 4, "number of tables")
	flag.IntVar(&c.seats, "seats", 4, "seats per table")
	flag.DurationVar(&c.pace, "pace", 1500*time.Millisecond, "sleep after every decision; 0 = as fast as possible")
	flag.DurationVar(&c.cooldown, "cooldown", 5*time.Second, "pause between matches on a perpetual table")
	flag.StringVar(&c.dir, "dir", "gorged-data", "persistence directory")
	flag.StringVar(&c.spectator, "spectator", "omniscient", "spectator visibility: public or omniscient")
	flag.Uint64Var(&c.seed, "seed", 1, "seed of table 1; table i uses seed+i-1")
	flag.BoolVar(&c.perpetual, "perpetual", true, "start a new match when one ends")
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
	names, err := deckFiles(c.decks)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no deck files in %s", c.decks)
	}

	r, err := host.New(c.hostOptions(reg, deckLoader(reg, c.decks)))
	if err != nil {
		return err
	}
	if len(r.Tables()) == 0 {
		for i := 1; i <= c.tables; i++ {
			cfg := host.TableConfig{ID: host.TableID(fmt.Sprintf("t%d", i)), Name: fmt.Sprintf("Table %d", i), Seats: c.seats,
				Decks: names, Seed: c.seed + uint64(i-1), Pace: c.pace, Spectator: vis, Perpetual: c.perpetual}
			if err := r.AddTable(cfg); err != nil {
				return err
			}
		}
	}
	if err := r.StartAll(); err != nil {
		return err
	}
	srv := &http.Server{Handler: httpapi.NewHandler(r, httpapi.Options{Web: webFS()})}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	fmt.Fprintf(os.Stderr, "gorged: %d tables of %d on %s (dir %s)\n", len(r.Tables()), c.seats, ln.Addr(), c.dir)
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

// hostOptions builds the Registry options for a config, threading the FL-40
// token corpus (reg.Tokens) into host.Options.Tokens so every live match and
// its replay mints the same tokens. Split from serve so TestHostThreads
// CorpusTokens can assert the thread works without driving the whole server.
func (g config) hostOptions(reg *cards.Registry, load func(string) (host.Deck, error)) host.Options {
	// Sleep is left nil: host installs its default interruptible sleep, the
	// package's only sanctioned clock read, so a long pace or cooldown still
	// yields to Close.
	return host.Options{Dir: g.dir, LoadDeck: load, Tokens: reg.Tokens, Sync: true, Cooldown: g.cooldown}
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
// host asks for the same decks every match.
func deckLoader(reg *cards.Registry, dir string) func(string) (host.Deck, error) {
	var mu sync.Mutex
	cache := map[string]host.Deck{}
	return func(name string) (host.Deck, error) {
		mu.Lock()
		defer mu.Unlock()
		if d, ok := cache[name]; ok {
			return d, nil
		}
		// The seat is named after the file stem (PL-14), not the deck
		// file's own name field, so the parsed File is not needed here.
		_, cs, err := deck.Load(reg, filepath.Join(dir, name+".json"))
		if err != nil {
			return host.Deck{}, err
		}
		d := host.Deck{Name: name, Cards: cs}
		cache[name] = d
		return d, nil
	}
}
