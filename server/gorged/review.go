package gorged

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// ReviewLoader turns one input path given to `gorged review` into the match
// it serves (match reviewer, design D9). It sees only the path; a loader that
// needs the corpus opens it itself. The table id is derived from the path
// (reviewTableID), not by the loader.
type ReviewLoader func(path string) (host.ReviewMatch, error)

// reviewDefaultAddr is review mode's listen address: loopback, so a record
// -- which holds every hidden card of its game by construction -- is never
// served off the box unless the operator says so with -review-public.
const reviewDefaultAddr = "127.0.0.1:8090"

// reviewConfig is `gorged review`'s command line.
type reviewConfig struct {
	addr    string
	public  bool
	records []string
	artDir  string
}

// reviewFlags registers review mode's flags. Inputs come from -records
// (repeatable, comma-separated) and from the positional arguments alike.
func reviewFlags() (*flag.FlagSet, *reviewConfig) {
	fs := flag.NewFlagSet("gorged review", flag.ContinueOnError)
	c := new(reviewConfig)
	fs.StringVar(&c.addr, "addr", reviewDefaultAddr, "listen address; refused unless loopback (see -review-public)")
	fs.BoolVar(&c.public, "review-public", false, "allow a non-loopback -addr: every recorded hand becomes readable by anyone who can reach it")
	fs.StringVar(&c.artDir, "art-dir", "", "serve card art from (and cache it into) this directory; empty serves no art and writes nothing")
	fs.Func("records", "a recorded game to serve (a feedback snapshot directory); repeatable and comma-separated, positional arguments too", func(arg string) error {
		for _, p := range strings.Split(arg, ",") {
			if p = strings.TrimSpace(p); p != "" {
				c.records = append(c.records, p)
			}
		}
		return nil
	})
	return fs, c
}

// parseReview parses review mode's arguments (everything after `review`).
func parseReview(args []string, out io.Writer) (*reviewConfig, error) {
	fs, c := reviewFlags()
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	c.records = append(c.records, fs.Args()...)
	if len(c.records) == 0 {
		return nil, fmt.Errorf("review: no records (pass -records PATH or positional paths)")
	}
	return c, nil
}

// reviewMain is `gorged review`: Main dispatches here when the first
// argument is "review".
func reviewMain(args []string, ext Extensions) int {
	c, err := parseReview(args, os.Stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(os.Stderr, "gorged:", err)
		return 2
	}
	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gorged:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serveReview(ctx, *c, ln, ext.Review); err != nil {
		fmt.Fprintln(os.Stderr, "gorged:", err)
		return 1
	}
	return 0
}

// ServeReview parses args as `gorged review`'s command line (without the
// leading "review") and serves on ln until ctx is done: reviewMain without
// the signal handling or the bind. It is the entry point a test drives a
// real review server through.
func ServeReview(ctx context.Context, args []string, ln net.Listener, ext Extensions) error {
	c, err := parseReview(args, io.Discard)
	if err != nil {
		return err
	}
	return serveReview(ctx, *c, ln, ext.Review)
}

// serveReview runs review mode: a memory-mode registry holding one review
// table per input (host.Registry.AddReviewTable: no slots, no run loop,
// omniscient), served by httpapi with Options.Seat nil -- spectator-only, so
// every seat route answers 403 -- and with no CreateGame and no Events, so
// neither the play-vs-bot nor the draft surface exists. Nothing is written to
// disk unless -art-dir names an art cache. ln is closed on every return.
func serveReview(ctx context.Context, c reviewConfig, ln net.Listener, load ReviewLoader) error {
	defer ln.Close()
	if !c.public && !isLoopbackListener(ln) {
		return fmt.Errorf("review mode serves every hidden card: it requires a loopback -addr (got %s) unless -review-public", ln.Addr())
	}
	if load == nil {
		return fmt.Errorf("review: this binary links no record loader (Extensions.Review)")
	}
	r, err := buildReviewRegistry(c.records, load)
	if err != nil {
		return err
	}
	defer r.Close()
	topMux := http.NewServeMux()
	if c.artDir != "" {
		ac, err := newServeArtCache(c.artDir)
		if err != nil {
			return err
		}
		topMux.HandleFunc("GET /art/named", ac.named)
		topMux.HandleFunc("GET /art/token", ac.token)
		topMux.HandleFunc("GET /art/blob/{key}", ac.blob)
		topMux.HandleFunc("GET /cards/named", ac.text)
	}
	topMux.Handle("/", reviewHandler(r))
	srv := &http.Server{Handler: topMux}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	tables := r.Tables()
	fmt.Fprintf(os.Stderr, "gorged review: %d recorded games on http://%s/\n", len(tables), ln.Addr())
	for _, t := range tables {
		fmt.Fprintf(os.Stderr, "gorged review: %s at http://%s/t/%s/m/1\n", t.ID, ln.Addr(), t.ID)
	}
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return nil
}

// reviewHandler is the review server's API and client: httpapi with no seat
// resolver (every seat route 403s), no game creation and no events.
func reviewHandler(r *host.Registry) http.Handler {
	return httpapi.NewHandler(r, httpapi.Options{Web: webFS()})
}

// buildReviewRegistry loads every input through load and registers it as a
// review table. Any failure -- an unreadable input, a log that does not
// replay, a duplicate table id -- refuses the whole start: a reviewer that
// silently dropped a game would hide exactly the one that diverged.
func buildReviewRegistry(paths []string, load ReviewLoader) (*host.Registry, error) {
	r, err := host.New(host.Options{LoadDeck: func(name string) (host.Deck, error) {
		return host.Deck{}, fmt.Errorf("review mode loads no decks (asked for %q)", name)
	}})
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		m, err := load(p)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("review: %s: %w", p, err)
		}
		if err := r.AddReviewTable(reviewTableID(p), m); err != nil {
			r.Close()
			return nil, fmt.Errorf("review: %s: %w", p, err)
		}
	}
	return r, nil
}

// reviewTableID is the table id an input is served under: its base name,
// minus a record file's extension, with any character outside a URL path
// segment's comfortable set replaced by '-'.
func reviewTableID(path string) string {
	base := filepath.Base(filepath.Clean(path))
	for _, ext := range []string{".match.json.gz", ".match.json"} {
		base = strings.TrimSuffix(base, ext)
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
