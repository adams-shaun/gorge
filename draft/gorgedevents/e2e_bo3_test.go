package gorgedevents_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/eventhost"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The E3-7 brief: one 2-human + 2-bot Bo3 FDN draft event, driven through
// the served HTTP surface only, reaching COMPLETE with a standings table
// that matches the log. The script, fixed because the engine is
// deterministic at a fixed seed:
//
//   - seats 0 and 3 are humans ("Ann", "Bob"); seats 1 and 2 are bots on
//     two policies.
//   - Round 1 pairs (0,3) and (1,2). The (1,2) series is bot vs bot and
//     drives itself; (0,3) plays three quick games: the game-1 chooser (the
//     CR 103.1 toss winner, either seat) takes "play", seat 0 concedes;
//     between the games seat 0 swaps one main card for an unused pool card
//     and seat 3 keeps; game 2's chooser is seat 0, which takes "draw" (so
//     turn 1 of that match belongs to slot 1, seat 3) and seat 3 concedes;
//     before game 3 seat 3 swaps and seat 0 keeps; game 3's chooser is
//     seat 3, which takes "play", and seat 0 concedes — series 2-1 to
//     seat 3.
//   - Each later round gives each human one bot series: the human takes
//     "play" when asked, keeps its deck in the between window and concedes
//     every game (2 games per series, 0-2).
//
// Seat 0 therefore plays 7 games and seat 3 plays 7; seat 3 finishes with
// 3 points and seat 0 with 0.
func TestDraftEventBo3TwoHumansOverHTTP(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	store, err := eventhost.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	relay := &eventhost.Relay{}
	seriesHook := eventhost.NewLazySeries()
	r, err := host.New(host.Options{
		Dir: t.TempDir(), LoadDeck: store.DeckLoader(reg, nil), Tokens: reg.Tokens,
		NameUniverse: reg.Universe(),
		// ThinkTimeout 0 parks a human seat on a decision forever: this
		// test IS the human client, and no caretaker may act for it.
		ThinkTimeout:          0,
		BotUnboundedDecisions: true, MaxDecisionsPerTurn: host.DefaultMaxDecisionsPerTurn,
		OnMatchEnd: relay.OnMatchEnd,
		// A series table asks the lazy hook; the Host's Series lands below,
		// once the Host exists (rounds_test.go's startGamePair wiring).
		Series: seriesHook.Series,
		Sleep:  func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	gate := &mapGate{tokens: map[string]httpapiSeatClaim{}, claims: map[httpapiSeatClaim]string{}}
	evh, err := eventhost.New(eventhost.Options{
		Registry: r, Store: store, Cards: reg, Seed: func() uint64 { return 11 },
		DefaultPolicy: "bot",
		// Mulligans 0: no London round, so a game's first decision is the
		// starting_player ask (or a priority window).
		Mulligans: 0, MintSeat: gate.mint,
	})
	if err != nil {
		r.Close()
		t.Fatal(err)
	}
	relay.Set(evh.OnMatchEnd)
	seriesHook.Set(evh.Series)
	srv := httptest.NewServer(httpapi.NewHandler(r, httpapi.Options{Events: evh, Seat: gate.resolve}))
	t.Cleanup(func() { evh.Close(); r.Close(); srv.Close() })

	// The stream: subscribe seat 0 so the collector sees every frame the
	// event ever publishes to it — and nothing published to seat 3.
	ctx, cancel := context.WithCancel(context.Background())
	resp := openStream(t, ctx, srv.URL, "")
	frames := make(chan sseFrame, 4096)
	streamDone := make(chan struct{})
	go collectEventFrames(resp.Body, frames, streamDone)
	t.Cleanup(func() {
		cancel()
		<-streamDone
	})
	hello := <-frames
	if hello.event != "hello" {
		t.Fatalf("first stream frame %q, want hello", hello.event)
	}
	var h protocol.Hello
	_ = hello.frame.Decode(&h)
	if h.Session == "" {
		t.Fatal("hello carries no session id")
	}

	// Create: seats 0 and 3 human, 1 and 2 bots. best_of 3, the standard
	// pick timer, and a server-minted seed.
	req := protocol.CreateEventRequest{Set: "FDN", BestOf: 3, PickTimer: "standard",
		Seats: []protocol.CreateEventSeat{
			{Kind: "human"}, {Kind: "bot", Policy: "bot"},
			{Kind: "bot", Policy: "lethal-pressure"}, {Kind: "human"},
		}}
	var created protocol.CreateEventResponse
	if code, e := callJSON(t, "POST", srv.URL+"/api/events", "", req, &created); code != 200 {
		t.Fatalf("create event: %d %s", code, e.Code)
	}
	if created.Event == "" || len(created.Invites) != 2 {
		t.Fatalf("create reply %+v: want an event id and two human invites", created)
	}
	tok := map[int]string{}
	for _, inv := range created.Invites {
		if inv.Token == "" || inv.Seat != 0 && inv.Seat != 3 || !strings.Contains(inv.Join, "token=") {
			t.Fatalf("invite %+v: want a seat-0/3 invite with a token and join link", inv)
		}
		tok[inv.Seat] = inv.Token
	}
	if _, ok := tok[0]; !ok {
		t.Fatal("no invite for seat 0")
	}
	if _, ok := tok[3]; !ok {
		t.Fatal("no invite for seat 3")
	}
	tok0, tok3 := tok[0], tok[3]
	evURL := srv.URL + "/api/events/" + created.Event
	id := created.Event

	if code, e := callJSON(t, "POST", evURL+"/subscribe", tok0, protocol.EventSubscribe{Session: h.Session}, nil); code != 204 {
		t.Fatalf("subscribe: %d %s", code, e.Code)
	}

	// Join Ann, then Bob: the draft opens only when both humans have names.
	var joined protocol.EventState
	if code, e := callJSON(t, "POST", evURL+"/join", tok0, protocol.JoinEventRequest{Name: "Ann"}, &joined); code != 200 {
		t.Fatalf("join seat 0: %d %s", code, e.Code)
	}
	if joined.Phase != "LOBBY" {
		t.Fatalf("phase after Ann's join %q, want LOBBY", joined.Phase)
	}
	if code, e := callJSON(t, "POST", evURL+"/join", tok3, protocol.JoinEventRequest{Name: "Bob"}, &joined); code != 200 {
		t.Fatalf("join seat 3: %d %s", code, e.Code)
	}
	if joined.Phase != "DRAFT" {
		t.Fatalf("phase after Bob's join %q, want DRAFT", joined.Phase)
	}

	// The draft: both humans pick the first card of every pack their queue
	// head offers (a pack only travels when someone picks, so the two seats
	// alternate), seat 0 re-sends its first pick once for the 409, and the
	// first pack of each seat carries the standard 14-card pick limit.
	type human struct {
		seat int
		tok  string
	}
	humans := []human{{0, tok0}, {3, tok3}}
	firstPack := map[int]bool{}
	var packs0 []capturedPack
	var stalePick *protocol.PickRequest
	deadline := time.Now().Add(50 * time.Second)
	for {
		st := evStateHTTP(t, evURL, tok0)
		if st.Phase == "BUILD" {
			break
		}
		if st.Phase != "DRAFT" {
			t.Fatalf("phase %q mid-draft, want DRAFT or BUILD", st.Phase)
		}
		if time.Now().After(deadline) {
			t.Fatalf("draft not over in 50s (phase %s)", st.Phase)
		}
		for _, hs := range humans {
			var pack protocol.DraftPack
			code, e := callJSON(t, "GET", evURL+"/pack", hs.tok, nil, &pack)
			if code != 200 {
				t.Fatalf("pack seat %d: %d %s", hs.seat, code, e.Code)
			}
			if pack.Waiting || len(pack.Pack) == 0 {
				continue
			}
			if hs.seat == 0 && stalePick != nil &&
				(pack.Round != stalePick.Round || pack.PickNo != stalePick.PickNo) {
				// The seat now owes a different pick: re-sending the old
				// one is a stale answer, refused with 409 stale_pick.
				sp := *stalePick
				stalePick = nil
				if code, e := callJSON(t, "POST", evURL+"/pick", hs.tok, sp, nil); code != 409 || e.Code != "stale_pick" {
					t.Fatalf("re-sent pick r%d p%d: %d %s, want 409 stale_pick", sp.Round, sp.PickNo, code, e.Code)
				}
			}
			if pack.Event != id || pack.Seat != hs.seat {
				t.Fatalf("pack %+v: want event %s seat %d", pack, id, hs.seat)
			}
			if !firstPack[hs.seat] {
				firstPack[hs.seat] = true
				if pack.Round != 1 || pack.PickNo != 1 {
					t.Fatalf("seat %d's first pack is r%d p%d, want r1 p1", hs.seat, pack.Round, pack.PickNo)
				}
				if pack.TimeLimitMS != 70000 {
					t.Fatalf("seat %d's first pack time_limit_ms = %d, want 70000 (the standard 14-card limit)", hs.seat, pack.TimeLimitMS)
				}
				if pack.RemainingMS <= 0 || pack.RemainingMS > 70000 {
					t.Fatalf("seat %d's first pack remaining_ms = %d, want (0, 70000]", hs.seat, pack.RemainingMS)
				}
			}
			if hs.seat == 0 {
				ids := make([]string, 0, len(pack.Pack))
				for _, c := range pack.Pack {
					ids = append(ids, c.ID)
				}
				pool := make([]string, 0, len(pack.Pool))
				for _, c := range pack.Pool {
					pool = append(pool, c.ID)
				}
				packs0 = append(packs0, capturedPack{round: pack.Round, pickNo: pack.PickNo,
					body: bodyJSON(t, pack), ids: ids, pool: pool, taken: pack.Pack[0].ID})
			}
			pick := protocol.PickRequest{Round: pack.Round, PickNo: pack.PickNo, ID: pack.Pack[0].ID}
			var next protocol.DraftPack
			if code, e = callJSON(t, "POST", evURL+"/pick", hs.tok, pick, &next); code != 200 {
				t.Fatalf("pick seat %d r%d p%d: %d %s", hs.seat, pack.Round, pack.PickNo, code, e.Code)
			}
			if hs.seat == 0 && stalePick == nil {
				stalePick = &pick
			}
		}
		time.Sleep(eventsPollEvery)
	}
	if len(packs0) != 42 {
		t.Fatalf("seat 0 took %d packs, want 42 (3 rounds of 14)", len(packs0))
	}

	// Build: each human saves the builder's suggestion (rev 1) and readies
	// it; the bots save and ready themselves during the advance.
	for _, hs := range humans {
		pool := evPoolHTTP(t, evURL, hs.tok)
		if len(pool.Pool) != 42 {
			t.Fatalf("seat %d's pool holds %d cards, want 42", hs.seat, len(pool.Pool))
		}
		var saved protocol.DeckCheck
		if code, e := callJSON(t, "POST", evURL+"/deck", hs.tok, pool.Suggestion, &saved); code != 200 || saved.Rev != 1 {
			t.Fatalf("seat %d deck save: %d %s rev=%d", hs.seat, code, e.Code, saved.Rev)
		}
		var readied protocol.EventState
		if code, e := callJSON(t, "POST", evURL+"/ready", hs.tok, protocol.ReadyRequest{Rev: 1}, &readied); code != 200 {
			t.Fatalf("seat %d ready: %d %s", hs.seat, code, e.Code)
		}
	}
	waitEventPhase(t, evURL, tok0, "ROUND1", 15*time.Second)

	// The rounds. Per pass both humans read their own state, act on every
	// "between" window their script names (sideboard once per window), and
	// answer every pending decision their own slot is asked — play/draw per
	// the script, concede for the scripted game loser, pass for the other
	// side, first-Min options for anything else.
	type sbKey struct{ a, b, game, seat int }
	type dsKey struct{ a, b, game, slot int }
	sideDone := map[sbKey]bool{}
	lastSeq := map[dsKey]uint64{}
	var table03 string
	turn1Seen := false
	deadline = time.Now().Add(50 * time.Second)
	for {
		if st := evStateHTTP(t, evURL, tok0); st.Phase == "COMPLETE" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("event not COMPLETE in 50s (phase %q)", evStateHTTP(t, evURL, tok0).Phase)
		}
		progressed := false
		for _, hs := range humans {
			st := evStateHTTP(t, evURL, hs.tok)
			if st.You == nil || *st.You != hs.seat {
				t.Fatalf("seat %d state You=%v, want %d", hs.seat, st.You, hs.seat)
			}
			for i := range st.Series {
				row := &st.Series[i]
				if row.A != hs.seat && row.B != hs.seat {
					continue
				}
				if row.State != "live" && row.State != "between" {
					continue
				}
				if row.BestOf != 3 {
					t.Fatalf("series (%d,%d) best_of = %d, want 3", row.A, row.B, row.BestOf)
				}
				if row.A == 0 && row.B == 3 {
					if table03 == "" {
						table03 = row.Table
					} else if row.Table != table03 {
						t.Fatalf("series (0,3) table moved from %q to %q", table03, row.Table)
					}
					if row.Table != fmt.Sprintf("%s-r1-s03-g%d-a%d", id, 1, 1) {
						t.Fatalf("series (0,3) table %q, want %s-r1-s03-g1-a1", row.Table, id)
					}
				}
				switch row.State {
				case "between":
					key := sbKey{row.A, row.B, row.Game, hs.seat}
					// Waiting names the series seats still to lock; by the
					// time either human reads the row the other may already
					// have sideboarded, so only the membership is pinned.
					for _, w := range row.Waiting {
						if w != row.A && w != row.B {
							t.Fatalf("between window Waiting %v carries stranger seat %d", row.Waiting, w)
						}
					}
					if sideDone[key] {
						continue
					}
					sb := protocol.SideboardRequest{Game: row.Game}
					if bo3Swaps(row.A, row.B, hs.seat, row.Game) {
						pool := evPoolHTTP(t, evURL, hs.tok)
						used := map[string]bool{}
						for _, mid := range pool.Suggestion.Main {
							used[mid] = true
						}
						swap := ""
						for _, c := range pool.Pool {
							if !used[c.ID] {
								swap = c.ID
								break
							}
						}
						if swap == "" {
							t.Fatalf("seat %d: no unused pool card to swap in", hs.seat)
						}
						d := pool.Suggestion
						d.Main[0] = swap
						sb.Deck = &d
					} else {
						sb.Keep = true
					}
					var after protocol.EventState
					code, e := callJSON(t, "POST", evURL+"/sideboard", hs.tok, sb, &after)
					switch {
					case code == 200:
						sideDone[key] = true
						progressed = true
					case code == 409:
						// The window may have closed between the read and
						// the call; the next pass re-reads it.
					default:
						t.Fatalf("sideboard seat %d (%d,%d) g%d: %d %s", hs.seat, row.A, row.B, row.Game, code, e.Code)
					}
				case "live":
					if row.Join == "" {
						t.Fatalf("live series (%d,%d) has no join link for seat %d", row.A, row.B, hs.seat)
					}
					u, err := url.Parse(row.Join)
					if err != nil {
						t.Fatalf("join link %q: %v", row.Join, err)
					}
					gameTok := u.Query().Get("token")
					if gameTok == "" {
						t.Fatalf("join link %q carries no token", row.Join)
					}
					wantSlot := 0
					if hs.seat == row.B {
						wantSlot = 1
					}
					gotSlot, err := strconv.Atoi(u.Query().Get("seat"))
					if err != nil || gotSlot != wantSlot {
						t.Fatalf("join link %q: seat %q, want %d (event seat %d at slot %d)",
							row.Join, u.Query().Get("seat"), wantSlot, hs.seat, wantSlot)
					}
					table := u.Path
					table = strings.TrimPrefix(table, "/t/")
					d := evPendingHTTP(t, srv.URL, table, row.Game, wantSlot, gameTok)
					if d == nil {
						continue
					}
					dk := dsKey{row.A, row.B, row.Game, wantSlot}
					if d.Seq == lastSeq[dk] {
						continue
					}
					in := scriptedIntent(t, row.A, row.B, row.Game, wantSlot, d)
					if code, e := callJSON(t, "POST",
						fmt.Sprintf("%s/api/tables/%s/matches/%d/intent", srv.URL, table, row.Game),
						gameTok, in, nil); code != 204 {
						if code != 409 {
							t.Fatalf("intent seat %d match %d seq %d: %d %s", hs.seat, row.Game, d.Seq, code, e.Code)
						}
						continue // superseded between the read and the answer
					}
					lastSeq[dk] = d.Seq
					progressed = true
				}
				// The (0,3) game-2 check: its first turn-change names slot 1
				// (seat 0 drew), read through seat 0's own scoped view.
				if hs.seat == 0 && row.A == 0 && row.B == 3 && row.State == "live" && row.Game == 2 && !turn1Seen {
					u, err := url.Parse(row.Join)
					if err != nil {
						t.Fatalf("join link %q: %v", row.Join, err)
					}
					gameTok := u.Query().Get("token")
					var bodies []protocol.EventBody
					code, e := callJSON(t, "GET",
						fmt.Sprintf("%s/api/tables/%s/matches/2/events?seat=0", srv.URL, row.Table),
						gameTok, nil, &bodies)
					if code != 200 {
						t.Fatalf("match 2 events: %d %s", code, e.Code)
					}
					for _, eb := range bodies {
						if eb.Event.Kind != "turn" {
							continue
						}
						turn1Seen = true
						if eb.Event.Player != 1 {
							t.Fatalf("match 2's first turn-change names player %d, want 1 (seat 0 drew)", eb.Event.Player)
						}
						break
					}
				}
			}
		}
		if !progressed {
			time.Sleep(eventsPollEvery)
		}
	}
	if !turn1Seen {
		t.Fatal("precondition missing: match 2 of (0,3) never showed a turn-change event")
	}

	// COMPLETE: a sane final state and the unredacted public log.
	final := evStateHTTP(t, evURL, tok0)
	if final.Phase != "COMPLETE" || final.Round != 3 || final.Error != "" {
		t.Fatalf("final state %+v: want COMPLETE round 3 with no error", final)
	}
	if final.BestOf != 3 {
		t.Fatalf("final best_of = %d, want 3", final.BestOf)
	}
	if len(final.Standings) != 4 {
		t.Fatalf("standings rows %d, want 4", len(final.Standings))
	}
	for _, hs := range humans {
		if st := evStateHTTP(t, evURL, hs.tok); st.You == nil || *st.You != hs.seat {
			t.Fatalf("token'd state You=%v, want %d", st.You, hs.seat)
		}
	}
	var endLog protocol.EventLog
	if code, e := callJSON(t, "GET", evURL+"/log", "", nil, &endLog); code != 200 {
		t.Fatalf("final log: %d %s", code, e.Code)
	}
	if !endLog.Complete {
		t.Fatal("final log reports incomplete")
	}
	entries := make([]event.Entry, 0, len(endLog.Entries))
	for _, raw := range endLog.Entries {
		var en event.Entry
		if err := json.Unmarshal(raw, &en); err != nil {
			t.Fatalf("log entry %s: %v", raw, err)
		}
		entries = append(entries, en)
	}

	// Group the series' entries and assert each script.
	gameEnded := map[[2]int][]event.Entry{}
	gameStarted := map[[2]int][]event.Entry{}
	seriesEnded := map[[2]int]event.Entry{}
	deckSaved := map[int]map[int]bool{}
	sideboarded := map[[3]int]map[int]int{} // (a,b,game) -> seat -> rev
	savedAt := map[string]int{}             // "seat/rev" -> log position
	startedAt := map[string]int{}           // "a/b/game" -> first log position
	for i, en := range entries {
		switch en.Kind {
		case event.KindGameEnded:
			gameEnded[[2]int{en.A, en.B}] = append(gameEnded[[2]int{en.A, en.B}], en)
		case event.KindGameStarted:
			k := [2]int{en.A, en.B}
			if _, ok := startedAt[fmt.Sprintf("%d/%d/%d", en.A, en.B, en.Game)]; !ok {
				startedAt[fmt.Sprintf("%d/%d/%d", en.A, en.B, en.Game)] = i
			}
			gameStarted[k] = append(gameStarted[k], en)
		case event.KindSeriesEnded:
			seriesEnded[[2]int{en.A, en.B}] = en
		case event.KindDeckSaved:
			if deckSaved[en.Seat] == nil {
				deckSaved[en.Seat] = map[int]bool{}
			}
			deckSaved[en.Seat][en.Rev] = true
			savedAt[fmt.Sprintf("%d/%d", en.Seat, en.Rev)] = i
		case event.KindSideboarded:
			k := [3]int{en.A, en.B, en.Game}
			if sideboarded[k] == nil {
				sideboarded[k] = map[int]int{}
			}
			sideboarded[k][en.Seat] = en.Rev
		}
	}
	if len(seriesEnded) != 6 {
		t.Fatalf("SeriesEnded series %v, want 6 (3 rounds of 2)", keysOf(seriesEnded))
	}
	// Every series' score equals its games.
	for k, se := range seriesEnded {
		games := gameEnded[k]
		aWins, bWins := 0, 0
		for _, ge := range games {
			switch {
			case ge.Winner != nil && *ge.Winner == k[0]:
				aWins++
			case ge.Winner != nil && *ge.Winner == k[1]:
				bWins++
			}
		}
		if se.Score != [2]int{aWins, bWins} {
			t.Fatalf("series (%d,%d) score %v, want [%d %d] from its %d games", k[0], k[1], se.Score, aWins, bWins, len(games))
		}
	}
	// The (0,3) series: 3 quick games on one table, the scripted winners,
	// the scripted choosers (game 1 the toss winner, game 2 seat 0, game 3
	// seat 3), one attempt each.
	g03 := gameEnded[[2]int{0, 3}]
	if len(g03) != 3 {
		t.Fatalf("series (0,3) played %d games, want 3", len(g03))
	}
	wantWinners := []int{3, 0, 3}
	for i, ge := range g03 {
		if ge.Game != i+1 {
			t.Fatalf("series (0,3) game %d logged as game %d", i+1, ge.Game)
		}
		if ge.Winner == nil || *ge.Winner != wantWinners[i] {
			t.Fatalf("series (0,3) game %d winner %v, want %d", i+1, ge.Winner, wantWinners[i])
		}
		if ge.Head == "" {
			t.Fatalf("series (0,3) game %d logged no chain head", i+1)
		}
		if ge.Attempt != 1 {
			t.Fatalf("series (0,3) game %d attempt %d, want 1", i+1, ge.Attempt)
		}
	}
	if se := seriesEnded[[2]int{0, 3}]; se.Score != [2]int{1, 2} {
		t.Fatalf("series (0,3) score %v, want [1 2]", se.Score)
	}
	gs03 := gameStarted[[2]int{0, 3}]
	if len(gs03) != 3 {
		t.Fatalf("series (0,3) started %d games, want 3", len(gs03))
	}
	wantTable := fmt.Sprintf("%s-r1-s03-g1-a1", id)
	wantChooser := []int{-1, 0, 3}
	for i, gs := range gs03 {
		if gs.Game != i+1 || gs.Attempt != 1 {
			t.Fatalf("series (0,3) GameStarted %+v, want game %d attempt 1", gs, i+1)
		}
		if gs.Table != wantTable {
			t.Fatalf("series (0,3) game %d table %q, want the series' own %q", i+1, gs.Table, wantTable)
		}
		if gs.GameSeed == 0 {
			t.Fatalf("series (0,3) game %d logged no seed", i+1)
		}
		if gs.GameSeed != gs03[0].GameSeed {
			t.Fatalf("series (0,3) game %d seed %d, want the table's %d", i+1, gs.GameSeed, gs03[0].GameSeed)
		}
		if gs.Chooser == nil || *gs.Chooser != wantChooser[i] {
			if i == 0 {
				toss := host.MatchTossWinner(gs.GameSeed, 1, 2)
				if toss != 0 {
					toss = 3
				}
				if gs.Chooser == nil || *gs.Chooser != toss {
					t.Fatalf("series (0,3) game 1 chooser %v, want the toss winner %d", gs.Chooser, toss)
				}
				continue
			}
			t.Fatalf("series (0,3) game %d chooser %v, want %d", i+1, gs.Chooser, wantChooser[i])
		}
	}
	// The deck revisions: each human saved rev 1 at build and the scripted
	// swaps produced rev 2; the swaps landed before the games they served.
	for _, seat := range []int{0, 3} {
		if !deckSaved[seat][1] || !deckSaved[seat][2] {
			t.Fatalf("seat %d DeckSaved revisions %v, want 1 and 2", seat, deckSaved[seat])
		}
	}
	if at := savedAt["0/2"]; at > startedAt["0/3/2"] {
		t.Fatalf("seat 0's rev 2 was saved at log position %d, after game 2 started at %d", at, startedAt["0/3/2"])
	}
	if at := savedAt["3/2"]; at > startedAt["0/3/3"] {
		t.Fatalf("seat 3's rev 2 was saved at log position %d, after game 3 started at %d", at, startedAt["0/3/3"])
	}
	sb2 := sideboarded[[3]int{0, 3, 2}]
	if sb2[0] != 2 || sb2[3] != 1 {
		t.Fatalf("game-2 sideboarded revisions %v, want seat 0 rev 2 (swap), seat 3 rev 1 (keep)", sb2)
	}
	sb3 := sideboarded[[3]int{0, 3, 3}]
	if sb3[0] != 2 || sb3[3] != 2 {
		t.Fatalf("game-3 sideboarded revisions %v, want seat 0 rev 2 (keep), seat 3 rev 2 (swap)", sb3)
	}
	for _, game := range []int{2, 3} {
		for _, en := range entries {
			if en.Kind != event.KindSideboarded || en.A != 0 || en.B != 3 || en.Game != game {
				continue
			}
			if en.Seat != 0 && en.Seat != 3 {
				continue
			}
			if en.Auto {
				t.Fatalf("human sideboarded entry for seat %d game %d is Auto", en.Seat, game)
			}
		}
	}
	// Every human-vs-bot series: the human kept, the bot won 2-0.
	for k, se := range seriesEnded {
		human := scriptHuman(k[0], k[1])
		if human < 0 || (k[0] == 0 && k[1] == 3) {
			continue
		}
		bot := k[0] + k[1] - human
		for _, ge := range gameEnded[k] {
			if ge.Winner == nil || *ge.Winner != bot {
				t.Fatalf("series (%d,%d) game %d winner %v, want bot seat %d", k[0], k[1], ge.Game, ge.Winner, bot)
			}
		}
		if se.Score[0]+se.Score[1] != 2 {
			t.Fatalf("series (%d,%d) score %v, want two played games", k[0], k[1], se.Score)
		}
		for seat, rev := range sideboarded[[3]int{k[0], k[1], 2}] {
			if seat == human && rev != 1 {
				t.Fatalf("series (%d,%d): human seat %d sideboarded rev %d, want its kept rev 1", k[0], k[1], seat, rev)
			}
		}
	}
	// Game and point totals: 7 games per human, seat 3 on 3 points, seat 0
	// on 0, and the 12 series' 36 match points all handed out.
	gameCount := map[int]int{}
	for k, games := range gameEnded {
		gameCount[k[0]] += len(games)
		gameCount[k[1]] += len(games)
	}
	if gameCount[0] != 7 || gameCount[3] != 7 {
		t.Fatalf("games played: seat 0 %d, seat 3 %d, want 7 each", gameCount[0], gameCount[3])
	}
	points := map[int]int{}
	for _, s := range final.Standings {
		points[s.Seat] = s.Points
	}
	if points[3] != 3 {
		t.Fatalf("seat 3 finished with %d points, want 3 (its 2-1 over seat 0)", points[3])
	}
	if points[0] != 0 {
		t.Fatalf("seat 0 finished with %d points, want 0", points[0])
	}
	total := 0
	for _, p := range points {
		total += p
	}
	if total != 18 {
		t.Fatalf("standings points sum %d, want 18 (3 per series over the 6 series)", total)
	}

	// The standings table recomputes exactly from the log's results.
	// Sorted keys: ComputeStandings sums each seat's opponents'
	// percentages in slice order, so map order would make the floats
	// 1-ULP-nondeterministic and flake the exact-struct compare.
	keys := keysOf(seriesEnded)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	var results []event.SeriesResult
	for _, k := range keys {
		se := seriesEnded[k]
		games := make([]event.GameOutcome, 0, len(gameEnded[k]))
		for _, ge := range gameEnded[k] {
			switch {
			case ge.Winner != nil && *ge.Winner == k[0]:
				games = append(games, event.AWon)
			case ge.Winner != nil && *ge.Winner == k[1]:
				games = append(games, event.BWon)
			default:
				games = append(games, event.Draw)
			}
		}
		results = append(results, event.SeriesResult{Round: se.Round, A: k[0], B: k[1], Score: se.Score, Games: games})
	}
	want := event.ComputeStandings(results)
	for i := range want {
		if i >= len(final.Standings) {
			t.Fatalf("standings has %d rows, want %d", len(final.Standings), len(want))
		}
		got, w := final.Standings[i], want[i]
		if got != (protocol.EventStanding{Seat: w.Seat, Points: w.Points, MW: w.MW, OMW: w.OMW, GW: w.GW, OGW: w.OGW}) {
			t.Fatalf("standings row %d = %+v, want %+v", i, got, w)
		}
	}

	// The series table's match 2 and match 3 played the saved revisions:
	// seat 0's rev 2 for both, seat 3's rev 1 for game 2 (keep) and rev 2
	// for game 3 (swap).
	var matches []protocol.MatchInfo
	if code, e := callJSON(t, "GET", srv.URL+"/api/tables/"+table03+"/matches", "", nil, &matches); code != 200 {
		t.Fatalf("series table matches: %d %s", code, e.Code)
	}
	byMatch := map[int]*protocol.MatchInfo{}
	for i := range matches {
		byMatch[matches[i].Match] = &matches[i]
	}
	for _, wantMatch := range []struct {
		k            int
		deck0, deck1 string
	}{
		{2, "ev/" + id + "/0/2", "ev/" + id + "/3/1"},
		{3, "ev/" + id + "/0/2", "ev/" + id + "/3/2"},
	} {
		mi := byMatch[wantMatch.k]
		if mi == nil || len(mi.Seats) != 2 {
			t.Fatalf("series table match %d = %+v, want two seats", wantMatch.k, mi)
		}
		if mi.Seats[0].DeckID != wantMatch.deck0 || mi.Seats[1].DeckID != wantMatch.deck1 {
			t.Fatalf("match %d decks = %q/%q, want %q/%q", wantMatch.k,
				mi.Seats[0].DeckID, mi.Seats[1].DeckID, wantMatch.deck0, wantMatch.deck1)
		}
	}

	// The stream: every event frame kind appeared, a between series row and
	// a best-of-3 event state were published — and seat 0's stream never
	// carried a card only seat 3 could own.
	cancel()
	<-streamDone
	seen := map[string]int{}
	bestOf3State := false
	betweenSeries := false
	offered := map[string]bool{}
	for _, cp := range packs0 {
		for _, id := range cp.ids {
			offered[id] = true
		}
	}
	var bodies []string
drain:
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				break drain
			}
			seen[f.event]++
			bodies = append(bodies, string(f.frame.Body))
			switch f.event {
			case "event_state":
				var es protocol.EventState
				if f.frame.Decode(&es) == nil && es.BestOf == 3 {
					bestOf3State = true
				}
			case "series_update":
				var su protocol.SeriesUpdate
				if f.frame.Decode(&su) == nil && su.Series.A == 0 && su.Series.B == 3 && su.Series.State == "between" {
					betweenSeries = true
				}
			case "draft_pack":
				var dp protocol.DraftPack
				if f.frame.Decode(&dp) == nil && dp.Seat == 0 {
					for _, c := range dp.Pack {
						offered[c.ID] = true
					}
				}
			}
		default:
			break drain
		}
	}
	if !bestOf3State {
		t.Fatalf("stream never published a best-of-3 event state; saw %v", seen)
	}
	if !betweenSeries {
		t.Fatalf("stream never published (0,3) between; saw %v", seen)
	}
	for _, kind := range []string{"event_state", "draft_pack", "pod_progress", "series_update"} {
		if seen[kind] == 0 {
			t.Fatalf("stream never delivered a %q frame; saw %v", kind, seen)
		}
	}

	// The hidden-info check. A card in seat 3's pool that seat 0 never
	// owned was, at worst, on offer in a pack seat 0 legitimately read —
	// so only an id that was never offered to seat 0 may appear nowhere in
	// any body seat 0 read. (A raw pool-difference scan would
	// false-positive on pack travellers.)
	pool0 := evPoolHTTP(t, evURL, tok0)
	pool3 := evPoolHTTP(t, evURL, tok3)
	in0 := map[string]bool{}
	for _, c := range pool0.Pool {
		in0[c.ID] = true
	}
	var leak []string
	for _, c := range pool3.Pool {
		if !in0[c.ID] && !offered[c.ID] {
			leak = append(leak, c.ID)
		}
	}
	if len(leak) == 0 {
		t.Fatal("precondition missing: no seat-3 card was ever hidden from seat 0")
	}
	bodies = append(bodies, bodyJSON(t, pool0))
	for _, cp := range packs0 {
		bodies = append(bodies, cp.body)
	}
	if len(packs0) == 0 {
		t.Fatal("precondition missing: seat 0 read no pack body")
	}
	for _, b := range bodies {
		for _, id := range leak {
			if strings.Contains(b, `"`+id+`"`) {
				t.Fatalf("seat 0's stream leaks seat 3's card %q", id)
			}
		}
	}
}

// evStateHTTP fetches the event state and fails the test on anything but 200.
func evStateHTTP(t *testing.T, evURL, tok string) protocol.EventState {
	t.Helper()
	var st protocol.EventState
	if code, e := callJSON(t, "GET", evURL, tok, nil, &st); code != 200 {
		t.Fatalf("event state: %d %s", code, e.Code)
	}
	return st
}

// evPoolHTTP fetches a seat's pool and fails the test on anything but 200.
func evPoolHTTP(t *testing.T, evURL, tok string) protocol.EventPool {
	t.Helper()
	var pool protocol.EventPool
	if code, e := callJSON(t, "GET", evURL+"/pool", tok, nil, &pool); code != 200 {
		t.Fatalf("pool: %d %s", code, e.Code)
	}
	return pool
}

// waitEventPhase polls until the event reaches phase.
func waitEventPhase(t *testing.T, evURL, tok, phase string, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		st := evStateHTTP(t, evURL, tok)
		if st.Phase == phase {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("event never reached %s (at %s)", phase, st.Phase)
		}
		time.Sleep(eventsPollEvery)
	}
}

// evPendingHTTP reads one seat's pending decision through the served
// endpoint: nil while the registry answers 409 (match not live, nothing
// pending), the decision on 200.
func evPendingHTTP(t *testing.T, srvURL, table string, k, seat int, gameTok string) *decision.Decision {
	t.Helper()
	u := fmt.Sprintf("%s/api/tables/%s/matches/%d/pending?seat=%d", srvURL, table, k, seat)
	var d decision.Decision
	code, e := callJSON(t, "GET", u, gameTok, nil, &d)
	switch code {
	case 200:
		return &d
	case 409, 404:
		// 409 while nothing is asked yet (or the seat is not live); 404
		// while the match is still spinning up.
		return nil
	default:
		t.Fatalf("pending table %s match %d seat %d: %d %s", table, k, seat, code, e.Code)
		return nil
	}
}

// scriptHuman names the human event seat of series (a,b), or -1 for the
// bot-vs-bot round-1 pairing.
func scriptHuman(a, b int) int {
	switch {
	case a == 0 || b == 0:
		return 0
	case a == 3 || b == 3:
		return 3
	}
	return -1
}

// bo3Loser names the event seat the script makes concede game of series
// (a,b): the (0,3) series alternates loser per game (seat 0 the odd games,
// seat 3 the even ones), every human-vs-bot series loses to the bot.
func bo3Loser(a, b, game int) int {
	if a == 0 && b == 3 {
		if game%2 == 1 {
			return a
		}
		return b
	}
	return scriptHuman(a, b)
}

// bo3WantsDraw reports whether the script takes "draw" for game of series
// (a,b): game 2 of (0,3), so seat 3 takes turn 1.
func bo3WantsDraw(a, b, game int) bool {
	return a == 0 && b == 3 && game == 2
}

// bo3Swaps reports whether the script swaps one main card for an unused
// pool card in the between window before game of series (a,b): seat 0
// before (0,3)'s game 2, seat 3 before its game 3. Every other window is a
// keep.
func bo3Swaps(a, b, seat, game int) bool {
	return a == 0 && b == 3 && ((game == 2 && seat == 0) || (game == 3 && seat == 3))
}

// scriptedIntent answers d per the script: the starting_player ask takes
// the chooser's own seat (or the other one for the scripted draw), the
// priority ask concedes for the scripted game loser and passes otherwise,
// and every other kind takes the first Min options.
func scriptedIntent(t *testing.T, a, b, game, slot int, d *decision.Decision) decision.Intent {
	t.Helper()
	switch d.Kind {
	case decision.KStartingPlayer:
		wantSelf := !bo3WantsDraw(a, b, game)
		for _, o := range d.Options {
			if o.Kind != "player" {
				t.Fatalf("starting_player option %+v is not a player pick", o)
			}
			if (o.Player == state.PlayerID(slot)) == wantSelf {
				return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
			}
		}
		t.Fatalf("starting_player %v has no %s option for slot %d", d.Options,
			map[bool]string{true: "own", false: "other"}[wantSelf], slot)
	case decision.KPriority:
		loser := bo3Loser(a, b, game)
		self := a
		if slot != 0 {
			self = b
		}
		wantKind := "pass"
		if self == loser {
			wantKind = "concede"
		}
		for _, o := range d.Options {
			if o.Kind == wantKind {
				return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
			}
		}
		t.Fatalf("priority %v has no %q option (answering seat %d, scripted loser %d)", d.Options, wantKind, self, loser)
	default:
		return answerFor(d)
	}
	return decision.Intent{}
}

// keysOf lists a map's keys in sorted order, for failure messages and any
// later iteration whose order must not vary run to run.
func keysOf[V any](m map[[2]int]V) [][2]int {
	out := make([][2]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// answerFor builds a valid intent for d: the first Min options, legal for
// any Decision.Kind (gorge server/host/httpapi seat_test.go's echo client).
func answerFor(d *decision.Decision) decision.Intent {
	n := d.Min
	if n > len(d.Options) {
		n = len(d.Options)
	}
	ch := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ch = append(ch, d.Options[i].Index)
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
}
