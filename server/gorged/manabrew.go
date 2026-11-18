package gorged

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/manabrewhttp"
	"github.com/adams-shaun/gorge/server/manabrew"
)

// The gorged-side toggle for server/host/manabrewhttp. Everything here is
// opt-in -- with -manabrew off (the default), applyManaBrew and
// mountManaBrew are no-ops.

// manabrewRoutePrefix is where the ManaBrew routes are mounted on the main
// listener's topMux when no separate -manabrew-addr is given. It sits ahead
// of httpapi.NewHandler in the pattern space the same way /art/ and
// /api/feedback do (main.go), but net/http's ServeMux picks the most
// specific registered pattern regardless of registration order anyway: with
// -manabrew off, nothing is registered under this prefix at all, so a
// request here falls through to httpapi's own "/api/" catch-all 404
// (handler.go) exactly as it does today -- "manabrew" never collides with
// the literal "tables" component httpapi's own routes require at the same
// path position. TestManaBrewOffMountsNothing pins this.
const manabrewRoutePrefix = "/api/manabrew/v0/tables"

// applyManaBrew validates -manabrew-addr and lets a non-empty value imply
// -manabrew, so "-manabrew-addr :8091" alone is enough to turn the adapter
// on, the same convention -humans uses to imply a seat gate without a
// separate -humans-enabled flag. It runs before any table is added or any
// listener opens, so a bad address fails startup instead of mounting routes that would answer with a bind error.
func (c *config) applyManaBrew() error {
	if c.manabrewAddr == "" {
		return nil
	}
	if _, _, err := net.SplitHostPort(c.manabrewAddr); err != nil {
		return fmt.Errorf("-manabrew-addr %q: %w", c.manabrewAddr, err)
	}
	c.manabrew = true
	return nil
}

// manabrewThinkTimeout is the host.Options.ThinkTimeout override
// -manabrew-think requests, scoped to when the
// adapter is actually on. With -manabrew off this returns 0 -- the zero
// value host.Options.ThinkTimeout already carries today, since gorged sets
// no other think timeout -- so an operator who sets -manabrew-think without
// -manabrew changes nothing, and "off" stays byte-identical.
func (c config) manabrewThinkTimeout() time.Duration {
	if !c.manabrew {
		return 0
	}
	return c.manabrewThink
}

// manabrewMetaTag is how the browser client learns, with no request of its
// own, that this origin serves the ManaBrew routes (web/src/lib/wire/pref.ts).
const manabrewMetaTag = `<meta name="gorge-manabrew">`

// offerManaBrew adds manabrewMetaTag to the web build when the routes share
// its origin. A separate -manabrew-addr is another origin, which the client
// doesn't reach, so it is not offered.
func (c config) offerManaBrew(web fs.FS) fs.FS {
	if web == nil || !c.manabrew || c.manabrewAddr != "" {
		return web
	}
	return withMeta(web, manabrewMetaTag)
}

// mountManaBrew wires server/host/manabrewhttp onto either topMux (the default:
// sharing the main listener, ahead of httpapi) or, when -manabrew-addr
// names one, a second listener of its own. It is a no-op when -manabrew is
// off. gate must be non-nil whenever c.manabrew is true; serve's own
// startup check (main.go, right after applyManaBrew) already refuses to
// reach here otherwise, so a nil Options.Seat -- which would 403 every
// request -- never happens by construction. It returns the second server
// and its Serve error channel only when a separate address was requested;
// both are nil when the routes share topMux or the adapter is off.
func (c config) mountManaBrew(topMux *http.ServeMux, r *host.Registry, gate *seatGate, text manabrew.CardText) (mbSrv *http.Server, mbErrc chan error, err error) {
	if !c.manabrew {
		return nil, nil, nil
	}
	handler := manabrewhttp.NewHandler(r, manabrewhttp.Options{Seat: gate.resolve, Text: text})
	if c.manabrewAddr == "" {
		topMux.Handle(manabrewRoutePrefix+"/", http.StripPrefix(manabrewRoutePrefix, handler))
		fmt.Fprintf(os.Stderr, "gorged: manabrew routes mounted at %s/\n", manabrewRoutePrefix)
		return nil, nil, nil
	}
	ln, err := net.Listen("tcp", c.manabrewAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("manabrew: listening on %s: %w", c.manabrewAddr, err)
	}
	srv := &http.Server{Handler: c.corsHandler(handler, func(string) bool { return true })}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	fmt.Fprintf(os.Stderr, "gorged: manabrew routes served on %s\n", ln.Addr())
	return srv, errc, nil
}

// applyManaBrewOrigins parses -manabrew-origins into c.manabrewOrigins. Each
// entry must be a bare origin (scheme://host[:port], no path, query or
// userinfo); "*" and "null" are refused so no configuration can answer an
// arbitrary caller. An empty flag leaves the set nil, which adds no CORS
// handling anywhere.
func (c *config) applyManaBrewOrigins() error {
	c.manabrewOrigins = nil
	if strings.TrimSpace(c.manabrewOriginsRaw) == "" {
		return nil
	}
	set := map[string]struct{}{}
	for _, raw := range strings.Split(c.manabrewOriginsRaw, ",") {
		o := strings.TrimSpace(raw)
		u, err := url.Parse(o)
		if o == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(o, "/") {
			return fmt.Errorf("-manabrew-origins %q: want scheme://host[:port] (http or https, no wildcard, no path)", raw)
		}
		set[o] = struct{}{}
	}
	c.manabrewOrigins = set
	return nil
}

// corsMainHandler wraps the main listener's mux so the ManaBrew routes and
// the lobby's POST /api/games answer the origins in -manabrew-origins. Every
// other route is untouched.
func (c config) corsMainHandler(next http.Handler) http.Handler {
	return c.corsHandler(next, func(path string) bool {
		return path == "/api/games" || strings.HasPrefix(path, manabrewRoutePrefix+"/")
	})
}

// corsHandler answers CORS for requests whose path covers() accepts. A request
// from a listed Origin gets Access-Control-Allow-Origin echoing that origin
// (never "*"), and a preflight OPTIONS from it is answered 204 here, allowing
// the Authorization and Content-Type headers. A request from any other
// origin, or with none, goes to next with no Access-Control header added. With
// no origins configured it returns next itself, so off changes nothing.
func (c config) corsHandler(next http.Handler, covers func(path string) bool) http.Handler {
	if len(c.manabrewOrigins) == 0 {
		return next
	}
	allowed := c.manabrewOrigins
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !covers(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; !ok || origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h := w.Header()
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
