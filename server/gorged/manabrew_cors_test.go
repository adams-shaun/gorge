package gorged

import (
	"net/http"
	"strings"
	"testing"
)

// TestManaBrewCORS pins -manabrew-origins: a listed origin gets an exact
// Access-Control-Allow-Origin plus an Authorization/Content-Type preflight on
// the three ManaBrew routes and POST /api/games; an unlisted one gets nothing;
// "*" is never emitted or accepted; and with the flag off no route changes.
func TestManaBrewCORS(t *testing.T) {
	t.Parallel()
	const listed, unlisted = "https://play.example.com", "https://evil.example.com"
	base := config{tables: 1, seats: 2, vsbot: true, manabrew: true, formatsRaw: "constructed"}
	on := base
	on.manabrewOriginsRaw = listed + ", http://localhost:5173"
	onURL, onCancel, onDone := startServe(t, on)
	waitForServer(t, onURL)
	offURL, offCancel, offDone := startServe(t, base)
	waitForServer(t, offURL)

	paths := []string{
		"/api/manabrew/v0/tables/t1/matches/1/stream",
		"/api/manabrew/v0/tables/t1/matches/1/send",
		"/api/manabrew/v0/tables/t1/matches/1/state",
		"/api/games",
	}
	do := func(method, url, origin string, preflight bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if preflight {
			req.Header.Set("Access-Control-Request-Method", "POST")
			req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	for _, p := range paths {
		// Listed origin: preflight answered, with the headers it asked for.
		resp := do(http.MethodOptions, onURL+p, listed, true)
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("%s preflight status = %d, want 204", p, resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != listed {
			t.Errorf("%s preflight ACAO = %q, want %q", p, got, listed)
		}
		ah := resp.Header.Get("Access-Control-Allow-Headers")
		if !strings.Contains(ah, "Authorization") || !strings.Contains(ah, "Content-Type") {
			t.Errorf("%s Allow-Headers = %q, want Authorization and Content-Type", p, ah)
		}
		if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "POST") {
			t.Errorf("%s Allow-Methods = %q, want POST", p, resp.Header.Get("Access-Control-Allow-Methods"))
		}
		// Listed origin, real request: ACAO on whatever the route answers.
		method := http.MethodGet
		if strings.HasSuffix(p, "/send") || p == "/api/games" {
			method = http.MethodPost
		}
		resp = do(method, onURL+p, listed, false)
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != listed {
			t.Errorf("%s %s ACAO = %q, want %q", method, p, got, listed)
		}
		// Unlisted origin: no Access-Control header at all, preflight or not.
		for _, pre := range []bool{true, false} {
			m := method
			if pre {
				m = http.MethodOptions
			}
			resp = do(m, onURL+p, unlisted, pre)
			for k := range resp.Header {
				if strings.HasPrefix(k, "Access-Control-") {
					t.Errorf("%s %s from unlisted origin carries %s", m, p, k)
				}
			}
			if pre && resp.StatusCode == http.StatusNoContent {
				t.Errorf("%s unlisted preflight was answered 204", p)
			}
		}
		// Flag off: no Access-Control header even for the would-be-listed origin.
		resp = do(http.MethodOptions, offURL+p, listed, true)
		for k := range resp.Header {
			if strings.HasPrefix(k, "Access-Control-") {
				t.Errorf("flag off: %s carries %s", p, k)
			}
		}
	}
	// A route outside the allow-listed surface is not touched.
	if resp := do(http.MethodGet, onURL+"/api/tables", listed, false); resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("/api/tables must not carry CORS headers")
	}

	// Flag parsing: wildcard and malformed entries are refused; off is nil.
	for _, bad := range []string{"*", "null", "play.example.com", "https://a.example/path", "https://a.example/", "ftp://a.example", "https://a.example,*"} {
		c := config{manabrewOriginsRaw: bad}
		if err := c.applyManaBrewOrigins(); err == nil {
			t.Errorf("-manabrew-origins %q accepted", bad)
		}
	}
	var off config
	if err := off.applyManaBrewOrigins(); err != nil || off.manabrewOrigins != nil {
		t.Errorf("empty flag: err=%v set=%v", err, off.manabrewOrigins)
	}

	onCancel()
	offCancel()
	if err := <-onDone; err != nil {
		t.Fatalf("serve: %v", err)
	}
	if err := <-offDone; err != nil {
		t.Fatalf("serve: %v", err)
	}
}
