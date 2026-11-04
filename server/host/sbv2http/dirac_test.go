//go:build spellbench

package sbv2http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSBV2ServesADiracGame plays whole games over the v2 routes on a dirac table: seat
// 0 a v2 client picking as the production bot does, seat 1 the bot. No rules engine
// is built, so whatever a v2 observation reads comes through the seam, and no decision
// may fall back to the native wire.
func TestSBV2ServesADiracGame(t *testing.T) {
	const decks = "../../../dirac/testdata/bench/fdn"
	reg := testutil.CorpusRegistry(t)
	e, err := adapter.New([]string{"fdn"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	names := testutil.DeckDirNamesPlaying(t, reg, decks, e.CheckDeck)
	for g := range 12 {
		t.Run(fmt.Sprintf("game%d", g), func(t *testing.T) {
			seed := uint64(3000 + g)
			hub := NewHub()
			o := sbv2RepoOptions(t, reg, hub, &recorder{})
			o.LoadDeck = func(name string) (host.Deck, error) {
				_, cs, err := deck.Load(reg, filepath.Join(decks, name+".json"))
				return host.Deck{Name: name, Cards: cs}, err
			}
			o.Engines = map[string]game.Engine{e.Name(): e}
			r, err := host.New(o)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			cfg := host.TableConfig{ID: "v2", Name: "v2", Seats: 2, Decks: []string{names[g], names[g+1]}, Seed: seed,
				Spectator: view.Omniscient, AutoMana: true, BotAutoPayMana: true, Humans: []int{0}, Engine: e.Name()}
			if err := r.AddTable(cfg); err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
				return SeatClaim{Table: "v2", Seat: 0}, strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ")
			}})))
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			if err := r.Start("v2"); err != nil {
				t.Fatal(err)
			}
			const base = "/api/sbv2/v1/tables/v2/matches/1/"
			stream := openSBV2Stream(t, srv.URL, base+"stream", "tok")
			p := &picker{bot: referenceBot(t, seed)}
			seat := hub.seat(seatKey{table: "v2", match: 1, seat: 0})
			answered := 0
			for deadline := time.Now().Add(time.Minute); ; {
				if done, _ := matchDone(t, r, "v2"); done {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("the game did not finish: %d v2 answers", answered)
				}
				if n := seat.nativeOnlyCount(); n > 0 {
					t.Fatalf("%d decisions left to the native wire", n)
				}
				select {
				case st, open := <-stream:
					if !open {
						t.Fatal("the stream closed before the game finished")
					}
					if st.Request == nil || isNativeOnly(st.Request.Decision) {
						continue
					}
					req, _ := seat.snapshot()
					if req == nil || fmt.Sprint(req.id) != st.Request.RequestID {
						continue
					}
					if status, body := getSBV2State(t, srv.URL, base+"state", "tok"); status != http.StatusOK || !json.Valid(body) {
						t.Fatalf("state: %d %q", status, body)
					}
					cand, miss := p.pick(req.ask)
					if miss != "" {
						cand = 0
					}
					if status, body := postSBV2Send(t, srv.URL, base+"send", "tok", st.Request.RequestID, cand); status != http.StatusNoContent {
						t.Fatalf("send %s candidate %d: %d %q", st.Request.RequestID, cand, status, body)
					}
					answered++
				case <-time.After(time.Millisecond):
				}
			}
			t.Logf("%s vs %s: %d v2 answers, %d bot misses", cfg.Decks[0], cfg.Decks[1], answered, p.misses)
			if answered == 0 {
				t.Fatal("no decision came over the v2 wire")
			}
		})
	}
}
