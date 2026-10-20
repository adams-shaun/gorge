package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/gorged"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// benchRecord plays a 2-seat heuristic bench game recorded through
// Hooks.Record (T1) into t.TempDir and returns the record's path and the
// live engine that played it.
func benchRecord(t *testing.T) (string, *rules.Engine) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	names := []string{"mono-black-aggro", "mono-red-prowess"}
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		d, err := testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = d
	}
	cfg := rules.Config{Seed: 424242, Names: names, Decks: decks, Tokens: reg.Tokens}
	path := filepath.Join(t.TempDir(), "g0"+matchrecord.Ext)
	sink := &matchrecord.Sink{
		Producer: matchrecord.Producer{Tool: "review-test", Policies: []string{"heuristic", "heuristic"}},
		Put:      func(r *matchrecord.Record) error { return matchrecord.Write(path, r) },
	}
	seats := []seat.Seat{heuristic.NewBot(cfg.Seed ^ 1), heuristic.NewBot(cfg.Seed ^ 2)}
	_, e, err := bench.PlayGame(cfg, seats, 60, 6000, bench.Hooks{Record: sink})
	if err != nil {
		t.Fatal(err)
	}
	return path, e
}

// TestReviewLoadsABenchRecord is phase 1 end to end: a bench game recorded
// through Hooks.Record (T1) loads through reviewLoad into a review table
// (T2), whose omniscient views carry every hand and no library.
func TestReviewLoadsABenchRecord(t *testing.T) {
	path, e := benchRecord(t)
	m, err := reviewLoad(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Policies) != 2 || len(m.Names) != 2 {
		t.Fatalf("policies %v names %v", m.Policies, m.Names)
	}
	r, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddReviewTable("g0", m); err != nil {
		t.Fatal(err)
	}
	last := uint64(len(e.L.Events) - 1)
	for _, seq := range []uint64{10, last / 2, last} {
		v, err := r.ViewAt("g0", 1, seq)
		if err != nil {
			t.Fatalf("ViewAt(%d): %v", seq, err)
		}
		for _, p := range v.Players {
			if p.Hand == nil || len(p.Hand) != p.HandSize || p.Library != nil || p.LibraryTop != nil {
				t.Fatalf("seq %d seat %d: hand %v (size %d), library %v -- want every hand, no library", seq, p.ID, p.Hand, p.HandSize, p.Library)
			}
		}
	}
}

// withExplain rewrites the record at path with explain as its sidecar.
// Explain is never hashed, so the record still verifies.
func withExplain(t *testing.T, path string, explain []json.RawMessage) {
	t.Helper()
	rec, err := matchrecord.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	rec.Explain = explain
	if err := matchrecord.Write(path, rec); err != nil {
		t.Fatal(err)
	}
}

// serveReviewOf serves the records at paths through the real review server
// (gorged.ServeReview with this binary's loader) on a loopback port and
// returns its base URL.
func serveReviewOf(t *testing.T, paths ...string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- gorged.ServeReview(ctx, paths, ln, gorged.Extensions{Review: reviewLoad}) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("ServeReview: %v", err)
		}
	})
	base := "http://" + ln.Addr().String()
	// The listener is bound before ServeReview runs, so the first request
	// simply waits for the load to finish and the server to accept.
	return base
}

func getDecisions(t *testing.T, url string) []protocol.ReviewDecision {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d %s", url, resp.StatusCode, body)
	}
	var ds []protocol.ReviewDecision
	if err := json.Unmarshal(body, &ds); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return ds
}

// TestReviewDecisionsIndexEveryIntent is design T4's endpoint (minus shadow
// verdicts) on an in-test bench record: one entry per intent; ask_seq
// strictly increasing and equal, entry for entry, to the log's decision_ask
// seqs (the client's Decision step stops), made_seq the matching
// decision_made; the asking seat and kind those events carry; every option
// labelled; the chosen indices among the listed options; and an injected
// explain note served verbatim on its own intent, null elsewhere.
func TestReviewDecisionsIndexEveryIntent(t *testing.T) {
	path, e := benchRecord(t)
	l := e.L
	note := json.RawMessage(`{"seat":1,"policy":"az","source":"policy","bot":[0],"search":{"engine":"azmcts","sims":64,"root_value":-0.25,"rows":[{"choices":[0],"visits":40,"prior":0.5,"q":-0.2,"bot":true},{"choices":[1],"visits":24,"prior":0.5,"q":-0.4,"chosen":true}]}}`)
	const noted = 7
	explain := make([]json.RawMessage, len(l.Intents))
	for i := range explain {
		explain[i] = json.RawMessage("null")
	}
	explain[noted] = note
	withExplain(t, path, explain)

	base := serveReviewOf(t, path)
	ds := getDecisions(t, base+"/api/tables/g0/matches/1/decisions")
	if len(ds) != len(l.Intents) || len(ds) < 50 {
		t.Fatalf("%d decision entries for %d intents", len(ds), len(l.Intents))
	}
	var asks, mades []events.Event
	for _, ev := range l.Events {
		switch ev.Kind {
		case events.DecisionAsk:
			asks = append(asks, ev)
		case events.DecisionMade:
			mades = append(mades, ev)
		}
	}
	kinds := map[string]bool{}
	for i, d := range ds {
		in := l.Intents[i]
		if d.Intent != i || d.AskSeq != asks[i].Seq || d.MadeSeq != mades[i].Seq || d.Made != mades[i].Text {
			t.Fatalf("entry %d: intent %d ask %d made %d (%q), log ask %d made %d (%q)", i, d.Intent, d.AskSeq, d.MadeSeq, d.Made, asks[i].Seq, mades[i].Seq, mades[i].Text)
		}
		if i > 0 && d.AskSeq <= ds[i-1].AskSeq {
			t.Fatalf("entry %d: ask_seq %d not after %d", i, d.AskSeq, ds[i-1].AskSeq)
		}
		if d.MadeSeq <= d.AskSeq || int(d.Seat) != int(asks[i].Player) || d.Kind != asks[i].Text || int(d.Seat) != int(in.Player) {
			t.Fatalf("entry %d: seat %d kind %q made %d, ask %+v intent player %d", i, d.Seat, d.Kind, d.MadeSeq, asks[i], in.Player)
		}
		if len(d.Options) == 0 || len(d.Options) > d.OptionsTotal {
			t.Fatalf("entry %d: %d options of %d", i, len(d.Options), d.OptionsTotal)
		}
		listed := map[int]bool{}
		for _, o := range d.Options {
			if o.Label == "" {
				t.Fatalf("entry %d option %d has no label", i, o.Index)
			}
			listed[o.Index] = true
		}
		if fmt.Sprint(d.Chosen) != fmt.Sprint(in.Choices) && !(len(d.Chosen) == 0 && len(in.Choices) == 0) {
			t.Fatalf("entry %d: chosen %v, intent %v", i, d.Chosen, in.Choices)
		}
		for _, c := range d.Chosen {
			if !listed[c] {
				t.Fatalf("entry %d: chosen %d is not among the listed options %+v", i, c, d.Options)
			}
		}
		if d.Answer == "" || d.Turn < 0 || d.Round < 1 && d.Turn > 0 {
			t.Fatalf("entry %d: answer %q turn %d round %d", i, d.Answer, d.Turn, d.Round)
		}
		want := "null"
		if i == noted {
			want = string(note)
		}
		if got := string(d.Explain); got != want && !(want == "null" && got == "") {
			t.Fatalf("entry %d: explain %s, want %s", i, got, want)
		}
		kinds[d.Kind] = true
	}
	if !kinds["priority"] || len(kinds) < 3 {
		t.Fatalf("decision kinds %v: want priority and at least two more", kinds)
	}
}

// TestReviewDropsAMisalignedExplain: an explain sidecar whose length is not
// the intent count is dropped by the loader (with a stderr warning) and the
// record still serves, every entry's explain null.
func TestReviewDropsAMisalignedExplain(t *testing.T) {
	path, e := benchRecord(t)
	short := make([]json.RawMessage, len(e.L.Intents)-1)
	for i := range short {
		short[i] = json.RawMessage(`{"seat":0,"source":"heuristic"}`)
	}
	withExplain(t, path, short)
	m, err := reviewLoad(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Explain != nil {
		t.Fatalf("a %d-entry explain for %d intents was kept", len(short), len(e.L.Intents))
	}
	ds := getDecisions(t, serveReviewOf(t, path)+"/api/tables/g0/matches/1/decisions")
	if len(ds) != len(e.L.Intents) {
		t.Fatalf("%d entries for %d intents", len(ds), len(e.L.Intents))
	}
	for i, d := range ds {
		if string(d.Explain) != "null" && len(d.Explain) != 0 {
			t.Fatalf("entry %d carries explain %s from a dropped sidecar", i, d.Explain)
		}
	}
}
