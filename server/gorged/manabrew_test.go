package gorged

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestManaBrewAddrValidates pins the -manabrew-addr validation: an address
// implies -manabrew. This needs no corpus: applyManaBrew is pure config
// validation.
func TestManaBrewAddrValidates(t *testing.T) {
	t.Parallel()
	c := &config{manabrewAddr: ":8091"}
	if err := c.applyManaBrew(); err != nil {
		t.Fatalf("-manabrew-addr :8091: unexpected error: %v", err)
	}
	if !c.manabrew {
		t.Fatal("a non-empty -manabrew-addr should imply -manabrew")
	}
	// A malformed address (no port at all) is refused.
	bad := &config{manabrewAddr: "not-an-address"}
	if err := bad.applyManaBrew(); err == nil {
		t.Fatal("a malformed -manabrew-addr should fail validation")
	}
	// Off by default: an empty -manabrew-addr never touches c.manabrew.
	empty := &config{}
	if err := empty.applyManaBrew(); err != nil {
		t.Fatalf("empty -manabrew-addr: unexpected error: %v", err)
	}
	if empty.manabrew {
		t.Fatal("an empty -manabrew-addr must not imply -manabrew")
	}
}

// TestManaBrewOnRequiresSeatGate pins the §5.4 startup validation: -manabrew
// with neither -humans nor -vsbot must fail before any table is added,
// rather than mounting routes that could only ever answer 403. The check
// runs on the flags themselves (before splitDecks/host.New), so this
// returns fast with no bot table ever started.
func TestManaBrewOnRequiresSeatGate(t *testing.T) {
	t.Parallel()
	testutil.CorpusRegistry(t) // skips when .cards/ is absent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	cfg := config{cards: testutil.CorpusDirOrSkip(t), decks: "../../testkit/testutil/decks", tables: 1, seats: 2,
		dir: t.TempDir(), spectator: "omniscient", seed: 1, perpetual: false, manabrew: true}
	err = serve(context.Background(), cfg, ln)
	if err == nil {
		t.Fatal("expected -manabrew without -humans/-vsbot to fail startup")
	}
	if !strings.Contains(err.Error(), "manabrew") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestManaBrewOfferedOnlyOnSameOrigin pins the tag the browser client reads:
// served when the routes share the page's origin, and nowhere else.
func TestManaBrewOfferedOnlyOnSameOrigin(t *testing.T) {
	t.Parallel()
	web := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html><head></head></html>")}}
	for _, tc := range []struct {
		name string
		c    config
		want bool
	}{
		{"off", config{}, false},
		{"mounted", config{manabrew: true}, true},
		{"separate listener", config{manabrew: true, manabrewAddr: "127.0.0.1:0"}, false},
	} {
		page, err := fs.ReadFile(tc.c.offerManaBrew(web), "index.html")
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(page), manabrewMetaTag); got != tc.want {
			t.Errorf("%s: tag served = %v, want %v:\n%s", tc.name, got, tc.want, page)
		}
	}
	if (config{manabrew: true}).offerManaBrew(nil) != nil {
		t.Error("a server with no web build must stay without one")
	}
}

// TestManaBrewOffMountsNothing pins §5.4's "off is byte-identical" claim
// (MB-9 item 1): with -manabrew off, a request under the ManaBrew route
// prefix gets exactly the same 404 JSON httpapi's own catch-all already
// serves for any unknown /api/ route -- because nothing registers a more
// specific pattern to intercept it.
func TestManaBrewOffMountsNothing(t *testing.T) {
	t.Parallel()
	// startServe, not a hand-rolled serve goroutine: every serving test in
	// the package must cancel and wait for serve to return before the test
	// ends, because t.TempDir's RemoveAll cleanup races a still-running
	// persistence writer (this test once failed the module gate with
	// "TempDir RemoveAll cleanup: directory not empty" when the server was
	// still shutting down as the cleanup scan ran).
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, pace: 0, perpetual: false})
	waitForServer(t, url)

	want := mustGet(t, url+"/api/definitely-not-a-route")
	got := mustGet(t, url+"/api/manabrew/v0/tables/t1/matches/1/stream")

	if got.status != want.status {
		t.Fatalf("status: got %d, want %d", got.status, want.status)
	}
	if got.contentType != want.contentType {
		t.Fatalf("Content-Type: got %q, want %q", got.contentType, want.contentType)
	}
	if !bytes.Equal(got.body, want.body) {
		t.Fatalf("body: got %q, want %q", got.body, want.body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

type getResult struct {
	status      int
	contentType string
	body        []byte
}

func mustGet(t *testing.T, url string) getResult {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s: reading body: %v", url, err)
	}
	return getResult{status: resp.StatusCode, contentType: resp.Header.Get("Content-Type"), body: body}
}

// waitForServer polls /api/tables until the server started by serve() in a
// background goroutine is actually listening, the same readiness pattern
// TestServesTablesOverHTTP uses.
func waitForServer(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url + "/api/tables")
		if err == nil {
			resp.Body.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
