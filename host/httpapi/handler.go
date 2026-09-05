package httpapi

import (
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/host"
)

// Options configures the handler. Every duration defaults when zero.
type Options struct {
	Authorize      func(*http.Request) error // nil allows everyone; an error is a 401
	WidgetInterval time.Duration             // SSE widget flush cadence; 0 = 250ms (Task 15)
	KeepAlive      time.Duration             // SSE comment cadence; 0 = 15s (Task 15)
	ResumeGrace    time.Duration             // how long a disconnected session survives; 0 = 30s (Task 15)
	Web            fs.FS                     // the built client (has index.html); nil = 503 for non-API paths (Task 15)
}

func (o Options) withDefaults() Options {
	if o.WidgetInterval == 0 {
		o.WidgetInterval = 250 * time.Millisecond
	}
	if o.KeepAlive == 0 {
		o.KeepAlive = 15 * time.Second
	}
	if o.ResumeGrace == 0 {
		o.ResumeGrace = 30 * time.Second
	}
	return o
}

type handler struct {
	reg  *host.Registry
	opts Options

	mu       sync.Mutex
	grace    map[string]*graceTimer // session id -> pending close after disconnect (Task 15)
	graceGen uint64                 // monotonic token so stale grace callbacks recognise themselves
}

// newHandler builds the handler behind NewHandler. Tests in this package use
// it to observe the grace map directly, which is the only way to assert the
// reconnect cancellation as a state transition rather than as a deadline.
func newHandler(r *host.Registry, o Options) (*handler, http.Handler) {
	h := &handler{reg: r, opts: o.withDefaults(), grace: map[string]*graceTimer{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tables", h.tables)
	mux.HandleFunc("GET /api/tables/{t}/matches", h.matches)
	mux.HandleFunc("GET /api/tables/{t}/matches/{k}/view", h.view)
	mux.HandleFunc("GET /api/tables/{t}/matches/{k}/events", h.events)
	mux.HandleFunc("POST /api/subscribe", h.subscribe)
	mux.HandleFunc("POST /api/unsubscribe", h.unsubscribe)
	// Method-less twins of every API pattern: the mux prefers the
	// method-specific pattern, so these only ever see the wrong method and
	// answer 405 in JSON rather than the mux's default text body.
	for _, p := range []string{"/api/tables", "/api/tables/{t}/matches", "/api/tables/{t}/matches/{k}/view",
		"/api/tables/{t}/matches/{k}/events", "/api/subscribe", "/api/unsubscribe", "/api/stream"} {
		mux.HandleFunc(p, methodNotAllowed)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
	h.mountStream(mux) // Task 15
	h.mountStatic(mux) // Task 15
	return h, h.authorized(mux)
}

// NewHandler routes the API and, when Options.Web is set, the client. It is
// the package's only exported constructor; callers that need to observe the
// handler's internals (tests) should use newHandler.
func NewHandler(r *host.Registry, o Options) http.Handler {
	_, mux := newHandler(r, o)
	return mux
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not allowed here")
}

// authorized runs the hook before every request.
func (h *handler) authorized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.opts.Authorize != nil {
			if err := h.opts.Authorize(r); err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
