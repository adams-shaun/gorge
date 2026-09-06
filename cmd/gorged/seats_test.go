package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestSeatGateMintsAndResolvesTokens covers the resolver's own contract,
// which the end-to-end tests reach only through one concrete token: the
// fixed-seed derivation for later slots, the random default, and the two
// acquisition paths with header-over-query precedence (R-E3-3).
func TestSeatGateMintsAndResolvesTokens(t *testing.T) {
	// Fixed seed: the first human slot takes the literal string, later
	// slots derive "<tok>-<slot>".
	g, err := newSeatGate("tok", []int{0, 2})
	if err != nil {
		t.Fatal(err)
	}
	if tok := g.token(state.PlayerID(0)); tok != "tok" {
		t.Fatalf("slot 0 token %q, want %q", tok, "tok")
	}
	if tok := g.token(state.PlayerID(2)); tok != "tok-2" {
		t.Fatalf("slot 2 token %q, want %q", tok, "tok-2")
	}
	got, ok := g.resolve(httptest.NewRequest(http.MethodGet, "/pending?seat=0&token=tok-2", nil))
	if !ok || got.Seat != state.PlayerID(2) {
		t.Fatalf("resolved %+v ok=%v, want seat 2", got, ok)
	}

	// The Authorization header is read first: a query token naming a
	// different seat must lose to the header.
	req := httptest.NewRequest(http.MethodPost, "/intent?seat=0&token=tok-2", nil)
	req.Header.Set("Authorization", "Bearer tok")
	got, ok = g.resolve(req)
	if !ok || got.Seat != state.PlayerID(0) {
		t.Fatalf("header did not win: %+v ok=%v, want seat 0", got, ok)
	}

	// An unknown or absent token declines the request (401 upstream).
	if _, ok := g.resolve(httptest.NewRequest(http.MethodGet, "/pending?seat=0&token=wrong", nil)); ok {
		t.Fatal("unknown token resolved")
	}
	if _, ok := g.resolve(httptest.NewRequest(http.MethodGet, "/pending?seat=0", nil)); ok {
		t.Fatal("absent token resolved")
	}

	// Random default: every slot gets its own 32-hex token, distinct.
	gr, err := newSeatGate("", []int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	t0, t1 := gr.token(0), gr.token(1)
	if t0 == "" || len(t0) != 32 || t0 == t1 {
		t.Fatalf("random tokens not opaque and distinct: %q %q", t0, t1)
	}
	for _, c := range t0 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("token %q is not hex", t0)
			break
		}
	}
}

// TestJoinHostIsDialable pins that the printed join URL can actually be
// opened. A listener on the default ":8080" reports "[::]:8080" — the
// unspecified address is listenable but not dialable — and a join URL is
// printed for exactly one reason, to be pasted into a browser.
func TestJoinHostIsDialable(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"[::]:8080", "127.0.0.1:8080"},
		{"0.0.0.0:8080", "127.0.0.1:8080"},
		{"127.0.0.1:8080", "127.0.0.1:8080"},
		{"192.168.1.5:8080", "192.168.1.5:8080"},
	} {
		if got := joinHost(stubAddr(tc.in)); got != tc.want {
			t.Errorf("joinHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// stubAddr is a net.Addr with a literal String, so the table above can name
// addresses this machine cannot actually bind.
type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

// TestJoinURLPointsAtTheLiveTable pins the path of the printed join URL. "/"
// is the lobby; clicking a live match there navigates to /t/t1/m/1, the
// finished-match REPLAY route, which paints a frozen snapshot with no stream
// and never advances. The first person to play through this client joined at
// "/" and spent the game looking at history while the real one waited. The
// URL must land on the live table route.
func TestJoinURLPointsAtTheLiveTable(t *testing.T) {
	g, err := newSeatGate("tok", []int{0})
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("http://%s/t/t1?seat=%d&token=%s", joinHost(stubAddr("[::]:8080")), 0, g.token(0))
	want := "http://127.0.0.1:8080/t/t1?seat=0&token=tok"
	if got != want {
		t.Fatalf("join URL = %q, want %q", got, want)
	}
	if strings.Contains(got, "/?seat=") {
		t.Fatalf("join URL %q lands on the lobby, not the live table", got)
	}
}
