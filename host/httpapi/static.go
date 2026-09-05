package httpapi

import (
	"net/http"
	"strings"
)

// mountStatic serves the embedded client for every non-API path, with an
// SPA fallback to index.html so deep links like /t/{table} load ("/api/"
// is already claimed by NewHandler's own routes, which the mux prefers
// over this catch-all). With no build embedded, every such path is a 503
// that says how to build it.
func (h *handler) mountStatic(mux *http.ServeMux) {
	if h.opts.Web == nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "web build missing — run make web", http.StatusServiceUnavailable)
		})
		return
	}
	files := http.FileServer(http.FS(h.opts.Web))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := h.opts.Web.Open(p); err == nil {
			f.Close()
			files.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/" // the SPA router owns unknown paths
		files.ServeHTTP(w, r)
	})
}
