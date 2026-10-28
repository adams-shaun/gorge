package sbv2http

// TestSBV2Golden pins the opt-in /api/sbv2 wire for one fixed-seed 2-seat
// game: seat 0 is a human seat driven through the HTTP routes, seat 1 is the
// production default bot. The scripted client plays the production bot's own
// picks for seat 0 (the same agent.Pick translation the humanseat tests use)
// and posts them to /send.
//
// Capture source. The fixture records GET .../state at each decision point,
// not the live SSE stream: the stream coalesces bursts by draining its wake
// channel before each re-read, so how many null frames a run of
// engine-internal steps produces depends on goroutine scheduling, not on the
// game. GET .../state serves the identical stateBody document with no
// broadcast step in between. To still pin the stream route, the test opens a
// fresh SSE connection at each stable decision point and records its first
// (initial) frame; that frame carries the same request and is deterministic
// because the request cannot move until this client answers it. Transient
// null frames between decisions are deliberately not recorded.
//
// The fixture is one NDJSON line per captured item, each line the exact
// server/client bytes (only the state route's trailing JSON newline is
// trimmed so the item stays one line):
//
//	{"kind":"state","body":{...}}        GET .../state
//	{"kind":"stream","body":{...}}       first SSE data frame
//	{"kind":"send","body":{...}}         POST .../send body
//	{"kind":"native","body":{...}}       a native fallback answer, when the
//	                                     production bot's pick has no v2
//	                                     candidate (the humanseat give-up
//	                                     path) or the server marked the
//	                                     decision native_only
//
// Regenerate with SBV2_GOLDEN_REGEN=1. A diff here is a client-contract change
// and needs operator sign-off before regeneration, exactly like
// TestNativeWireGolden.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	goldenPath  = "testdata/golden/2seat.ndjson.gz"
	goldenSeed  = 0x5bc0
	goldenTable = host.TableID("golden")
	goldenToken = "golden-s0"
)

func TestSBV2Golden(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	r, err := host.New(parityOptions(reg, hub))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	cfg := sbv2TableCfg(goldenTable, goldenSeed)
	cfg.Decks = []string{"death-n-taxes", "mono-red-prowess"}
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		if strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ") != goldenToken {
			return SeatClaim{}, false
		}
		return SeatClaim{Table: goldenTable, Seat: 0}, true
	}})))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if err := r.Start(goldenTable); err != nil {
		t.Fatal(err)
	}
	bot, err := productionBot(goldenSeed, 0)
	if err != nil {
		t.Fatalf("production bot for seat 0: %v", err)
	}

	lines := [][]byte{goldenLine("meta", fmt.Sprintf(`{"seed":%d,"table":%q,"match":1,"seat":0}`, goldenSeed, goldenTable))}
	lines = append(lines, runGoldenGame(t, r, hub, srv.URL, bot)...)

	got := bytes.Join(lines, []byte("\n"))
	got = append(got, '\n')
	if os.Getenv("SBV2_GOLDEN_REGEN") == "1" {
		writeSBV2Golden(t, got)
		return
	}
	want := readSBV2Golden(t)
	if !bytes.Equal(got, want) {
		wl, gl := bytes.Split(want, []byte("\n")), bytes.Split(got, []byte("\n"))
		for i := 0; i < len(wl) && i < len(gl); i++ {
			if bytes.Equal(wl[i], gl[i]) {
				continue
			}
			short := func(b []byte) string {
				if len(b) > 600 {
					return string(b[:600]) + "…"
				}
				return string(b)
			}
			t.Fatalf("sbv2 wire golden diverges at line %d: a diff here means the /api/sbv2 client contract changed and needs operator sign-off. Do not regenerate until that sign-off exists; then run SBV2_GOLDEN_REGEN=1 go test -run TestSBV2Golden ./server/host/sbv2http/.\n got: %s\nwant: %s",
				i, short(gl[i]), short(wl[i]))
		}
		t.Fatalf("sbv2 wire golden has %d lines, committed fixture has %d; regenerate with SBV2_GOLDEN_REGEN=1 if this is an intended change", len(gl), len(wl))
	}
}

// goldenLine builds one NDJSON line without re-encoding body, so the fixture
// preserves the exact bytes the server or client wrote.
func goldenLine(kind, body string) []byte {
	line := make([]byte, 0, len(kind)+len(body)+24)
	line = append(line, `{"kind":"`...)
	line = append(line, kind...)
	line = append(line, `","body":`...)
	line = append(line, body...)
	line = append(line, '}')
	return line
}

// runGoldenGame drives seat 0 to game over and returns the captured lines.
func runGoldenGame(t *testing.T, r *host.Registry, hub *Hub, base string, bot seat.Seat) [][]byte {
	t.Helper()
	p := &picker{bot: bot}
	ss := hub.seat(seatKey{table: goldenTable, match: 1, seat: 0})
	statePath := fmt.Sprintf("/api/sbv2/v1/tables/%s/matches/1/state", goldenTable)
	sendPath := fmt.Sprintf("/api/sbv2/v1/tables/%s/matches/1/send", goldenTable)
	handled := map[string]bool{}
	seenNative := map[string]bool{}
	var lines [][]byte
	deadline := time.Now().Add(2 * time.Minute)

	for {
		if done, _ := matchDone(t, r, goldenTable); done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("golden game did not finish within 2m")
		}
		raw := goldenGetState(t, base, statePath)
		var doc stateBody
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("state body %q: %v", raw, err)
		}
		if doc.Request == nil {
			time.Sleep(time.Millisecond)
			continue
		}
		id := doc.Request.RequestID
		if handled[id] {
			time.Sleep(time.Millisecond)
			continue
		}
		// Confirm the state route and the hub agree; a body can be in flight
		// while the decision is superseded or answered.
		req, _ := ss.snapshot()
		if req == nil || fmt.Sprint(req.id) != id {
			time.Sleep(time.Millisecond)
			continue
		}
		lines = append(lines, goldenLine("state", strings.TrimRight(string(raw), "\r\n")))
		frame := goldenStreamFrame(t, base, goldenTable, goldenToken, id)
		lines = append(lines, goldenLine("stream", string(frame)))

		if isNativeOnly(doc.Request.Decision) {
			if !seenNative[id] {
				seenNative[id] = true
			}
			in := goldenNativeIntent(t, r, bot)
			lines = append(lines, goldenLine("native", string(in)))
			handled[id] = true
			continue
		}

		cand, miss := p.pick(req.ask)
		if miss != "" {
			// The production bot's pick is not one of the v2 candidates.
			// Mirror humanseat's give-up path: answer the parked native
			// decision directly, then resume v2.
			raw := goldenIntentJSON(t, p.intent)
			if err := r.SubmitIntent(goldenTable, 1, 0, p.intent); err != nil {
				t.Fatalf("native fallback for seq %d: %v", p.planSeq, err)
			}
			lines = append(lines, goldenLine("native", string(raw)))
			handled[id] = true
			continue
		}
		body, err := json.Marshal(map[string]any{"request_id": id, "candidate_id": cand})
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, goldenLine("send", string(body)))
		if status, resp := postSBV2Send(t, base, sendPath, goldenToken, id, cand); status != http.StatusNoContent {
			t.Fatalf("send request %s candidate %d: %d %q", id, cand, status, resp)
		}
		handled[id] = true
	}

	// Terminal state: the v2 routes serve {"request":null} once the match is
	// over. Both routes are pinned here; this is the only null frame recorded.
	raw := goldenGetState(t, base, statePath)
	var doc stateBody
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("terminal state body %q: %v", raw, err)
	}
	if doc.Request != nil {
		t.Fatalf("terminal state still carries request %+v", doc.Request)
	}
	lines = append(lines, goldenLine("state", strings.TrimRight(string(raw), "\r\n")))
	lines = append(lines, goldenLine("stream", string(goldenStreamFrame(t, base, goldenTable, goldenToken, ""))))
	return lines
}

// goldenNativeIntent answers a native_only decision with the production bot
// and returns the exact intent JSON for the fixture.
func goldenNativeIntent(t *testing.T, r *host.Registry, bot seat.Seat) []byte {
	t.Helper()
	d, err := r.Pending(goldenTable, 1, 0)
	if err != nil {
		t.Fatalf("native_only pending: %v", err)
	}
	seq, err := sbv2HeadSeq(r, goldenTable, 1)
	if err != nil {
		t.Fatalf("native_only head: %v", err)
	}
	v, err := r.ViewAtSeat(goldenTable, 1, seq, 0)
	if err != nil {
		t.Fatalf("native_only view: %v", err)
	}
	v.Decision = d
	in, err := bot.Decide(t.Context(), v, *d)
	if err != nil {
		t.Fatalf("native_only answer: %v", err)
	}
	raw := goldenIntentJSON(t, in)
	if err := r.SubmitIntent(goldenTable, 1, 0, in); err != nil {
		t.Fatalf("native_only submit: %v", err)
	}
	return raw
}

func goldenIntentJSON(t *testing.T, in decision.Intent) []byte {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func goldenGetState(t *testing.T, base, path string) []byte {
	t.Helper()
	status, body, err := parityGet(base+path, goldenToken)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK {
		t.Fatalf("GET state: %d %q", status, body)
	}
	return body
}

// goldenStreamFrame opens a fresh SSE connection and returns the first data
// frame's payload. wantID names the request that must be open; "" requires the
// terminal null frame.
func goldenStreamFrame(t *testing.T, base string, table host.TableID, token, wantID string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/sbv2/v1/tables/%s/matches/1/stream", base, table), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream: status %d", resp.StatusCode)
	}
	br := bufio.NewReader(resp.Body)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("stream closed before a data frame: %v", err)
		}
		data, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "data: ")
		if !ok {
			continue
		}
		var doc stateBody
		if err := json.Unmarshal([]byte(data), &doc); err != nil {
			t.Fatalf("stream frame %q: %v", data, err)
		}
		if wantID == "" {
			if doc.Request != nil {
				t.Fatalf("terminal stream frame carries request %+v", doc.Request)
			}
		} else if doc.Request == nil || doc.Request.RequestID != wantID {
			t.Fatalf("stream frame request %+v, want %s", doc.Request, wantID)
		}
		return []byte(data)
	}
}

func writeSBV2Golden(t *testing.T, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatal(err)
	}
	gz, err := gzipDeterministic(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(gz) > 1<<20 {
		t.Fatalf("%s is %d bytes, over the 1 MB golden budget", goldenPath, len(gz))
	}
	if err := os.WriteFile(goldenPath, gz, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("regenerated %s (%d lines, %d bytes gzipped)", goldenPath, bytes.Count(raw, []byte("\n")), len(gz))
}

func readSBV2Golden(t *testing.T) []byte {
	t.Helper()
	if fi, err := os.Stat(goldenPath); err == nil && fi.Size() > 1<<20 {
		t.Fatalf("%s is %d bytes, over the 1 MB golden budget", goldenPath, fi.Size())
	}
	f, err := os.Open(goldenPath)
	if err != nil {
		t.Fatalf("%v (generate with SBV2_GOLDEN_REGEN=1 go test -run TestSBV2Golden ./server/host/sbv2http/)", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// gzipDeterministic compresses data with the header fields that would
// otherwise vary (ModTime, Name, Comment) zeroed, so two regenerations of
// byte-identical content produce byte-identical .gz files.
func gzipDeterministic(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	w.Header.ModTime = time.Time{}
	w.Header.Name = ""
	w.Header.Comment = ""
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
