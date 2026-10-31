package eventhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// humanSeatsReq builds a four-seat request with the given event seats
// human, the rest bots.
func humanSeatsReq(humans []int, bestOf int) protocol.CreateEventRequest {
	req := protocol.CreateEventRequest{Set: "FDN", BestOf: bestOf}
	for i := 0; i < 4; i++ {
		s := protocol.CreateEventSeat{Kind: "bot"}
		for _, h := range humans {
			if h == i {
				s.Kind = "human"
			}
		}
		req.Seats = append(req.Seats, s)
	}
	return req
}

// inviteToken returns the create response's invite token for seat.
func inviteToken(t *testing.T, resp protocol.CreateEventResponse, seat int) string {
	t.Helper()
	for _, inv := range resp.Invites {
		if inv.Seat == seat {
			return inv.Token
		}
	}
	t.Fatalf("no invite for seat %d", seat)
	return ""
}

// legalIntent answers d with the first Min options: always valid per
// Decision.Validate for any Kind, so the driver needs no rules knowledge
// and can answer whatever decision the engine actually asks the human.
func legalIntent(d *decision.Decision) decision.Intent {
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

// humanDriveUntilBetween answers every decision the two human seats of
// series (a,b) are asked with the first Min options until the series
// reaches its between window. The driver follows the series' live table
// across crash retries (each retry logs a new table) and fails the test on
// deadline. The human seats sit at table slots 0 and 1 (event seat a at
// slot 0, b at slot 1, match 1).
func humanDriveUntilBetween(t *testing.T, reg *host.Registry, r *run, a, b int) {
	t.Helper()
	deadline := time.Now().Add(50 * time.Second)
	var last [2]uint64
	for {
		r.mu.Lock()
		table := r.seriesTable[[2]int{a, b}]
		v, ok := r.ev.SeriesState(a, b)
		r.mu.Unlock()
		if ok && v.Between {
			return
		}
		if ok && v.Ended {
			t.Fatalf("series (%d,%d) ended before between: %+v", a, b, v)
		}
		if table == "" {
			if time.Now().After(deadline) {
				t.Fatalf("series (%d,%d): no game table within deadline", a, b)
			}
			time.Sleep(pollEvery)
			continue
		}
		acted := false
		for slot := 0; slot < 2; slot++ {
			d, err := reg.Pending(table, 1, state.PlayerID(slot))
			if err != nil || d.Seq == last[slot] {
				continue
			}
			if err := reg.SubmitIntent(table, 1, state.PlayerID(slot), legalIntent(d)); err != nil {
				t.Fatalf("SubmitIntent slot %d: %v", slot, err)
			}
			last[slot] = d.Seq
			acted = true
		}
		if time.Now().After(deadline) {
			t.Fatalf("series (%d,%d) never reached between within deadline", a, b)
		}
		if !acted {
			time.Sleep(pollEvery)
		}
	}
}

// waitSeriesInFlight polls until series (a,b) has the named game in flight
// and returns its attempt. tableOut, when non-nil, receives the live table.
func waitSeriesInFlight(t *testing.T, r *run, a, b, game int, d time.Duration) (event.GameAttempt, host.TableID) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		r.mu.Lock()
		v, ok := r.ev.SeriesState(a, b)
		table := r.seriesTable[[2]int{a, b}]
		r.mu.Unlock()
		if ok && v.InFlight != nil && v.InFlight.Game == game {
			return *v.InFlight, table
		}
		if time.Now().After(deadline) {
			if !ok {
				t.Fatalf("series (%d,%d) missing after deadline", a, b)
			}
			t.Fatalf("series (%d,%d) game %d never started: %+v", a, b, game, v)
		}
		time.Sleep(pollEvery)
	}
}

// driveSeriesToEnd drives a series (a,b) all the way to its SeriesEnded:
// between games every side with a token keeps via the Sideboard API (bot
// sides are locked by the advance path themselves), and while a game is
// live the human seats submit their first-legal intents while the bots
// answer theirs. A token of "" marks a bot side.
func driveSeriesToEnd(t *testing.T, h *Host, reg *host.Registry, r *run, id, tokA, tokB string, a, b int) {
	t.Helper()
	deadline := time.Now().Add(50 * time.Second)
	var last [2]uint64
	for {
		r.mu.Lock()
		v, ok := r.ev.SeriesState(a, b)
		entries := append([]event.Entry(nil), r.ev.Log()...)
		table := r.seriesTable[[2]int{a, b}]
		r.mu.Unlock()
		if !ok {
			// The round rolled over and the pairing is no longer current:
			// ended only if its SeriesEnded is on the log.
			for _, en := range entries {
				if en.Kind == event.KindSeriesEnded && en.A == a && en.B == b {
					return
				}
			}
			t.Fatalf("series (%d,%d) missing before its SeriesEnded", a, b)
		}
		if v.Ended {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("series (%d,%d) never ended: %+v", a, b, v)
		}
		acted := false
		if v.Between && v.NextGame > 0 {
			for i, tok := range []string{tokA, tokB} {
				if tok == "" || v.Locked[i] != 0 {
					continue
				}
				if _, err := h.Sideboard(id, tok, protocol.SideboardRequest{Game: v.NextGame, Keep: true}); err != nil {
					// The state may have moved on between the read and
					// the call; the loop re-reads it either way.
					continue
				}
				acted = true
			}
		} else if table != "" && v.InFlight != nil {
			k := v.InFlight.Game // the series table plays game k as match k
			for slot := 0; slot < 2; slot++ {
				d, err := reg.Pending(table, k, state.PlayerID(slot))
				if err != nil || d.Seq == last[slot] {
					continue
				}
				if err := reg.SubmitIntent(table, k, state.PlayerID(slot), legalIntent(d)); err != nil {
					// The table may have been replaced by a fresh attempt
					// between the Pending and the submit; start over.
					last = [2]uint64{}
					break
				}
				last[slot] = d.Seq
				acted = true
			}
		}
		if !acted {
			time.Sleep(pollEvery)
		}
	}
}

// pickTwoHumans drives two human seats through the draft, alternating
// picks: a seat's next pack arrives through the other human's pick, so
// neither can be driven to completion alone.
func pickTwoHumans(t *testing.T, h *Host, id, tok0, tok3 string) {
	t.Helper()
	toks := []string{tok0, tok3}
	done := func(tok string) bool {
		return stateOf(t, h, id, tok).Phase == "BUILD"
	}
	for i := 0; i < 400 && !(done(toks[0]) && done(toks[1])); i++ {
		for seat, tok := range toks {
			if done(tok) {
				continue
			}
			p := packOf(t, h, id, tok)
			if p.Waiting {
				continue
			}
			if _, err := h.Pick(id, tok, protocol.PickRequest{
				Round: p.Round, PickNo: p.PickNo, ID: p.Pack[0].ID,
			}); err != nil {
				t.Fatalf("pick seat %d: %v", seat, err)
			}
		}
	}
	if !done(toks[0]) || !done(toks[1]) {
		t.Fatalf("draft stalled: phases %s / %s", stateOf(t, h, id, toks[0]).Phase, stateOf(t, h, id, toks[1]).Phase)
	}
}

func seriesRow(st protocol.EventState, a, b int) *protocol.EventSeries {
	for i := range st.Series {
		if st.Series[i].A == a && st.Series[i].B == b {
			return &st.Series[i]
		}
	}
	return nil
}

func TestMaxHumansDefaultsToFour(t *testing.T) {
	h, _ := newHost(t, nil)
	if got := h.maxHumans(); got != 4 {
		t.Fatalf("maxHumans() = %d, want 4 (E3-5)", got)
	}
	for n := 2; n <= 4; n++ {
		if _, err := h.CreateEvent(humanSeatsReq([]int{0, 1, 2, 3}[:n], 1)); err != nil {
			t.Fatalf("CreateEvent(%d humans): %v", n, err)
		}
	}
}

// TestTwoHumanBo3SeriesSideboardAndKeep is the E3-5 end-to-end: two humans
// (seats 0 and 3) meet in round 1 of a Bo3, play game 1 driven through the
// registry's Pending/SubmitIntent, both sideboard (one Keep, one a new
// deck revision) and the next game starts on the series' own table with
// per-slot join links. The pools stay hidden behind each seat's token.
func TestTwoHumanBo3SeriesSideboardAndKeep(t *testing.T) {
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	})
	resp := createOK(t, p.h, humanSeatsReq([]int{0, 3}, 3))
	id := resp.Event
	tok0 := inviteToken(t, resp, 0)
	tok3 := inviteToken(t, resp, 3)
	joinOK(t, p.h, id, tok0, "Ann")
	joinOK(t, p.h, id, tok3, "Bob")
	pickTwoHumans(t, p.h, id, tok0, tok3)
	finishHumanBuild(t, p.h, id, tok0)
	finishHumanBuild(t, p.h, id, tok3)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	r := runOf(t, p.h, id)
	humanDriveUntilBetween(t, p.reg, r, 0, 3)
	st0 := stateOf(t, p.h, id, tok0)
	row := seriesRow(st0, 0, 3)
	if row == nil || row.State != "between" || row.Game != 2 {
		t.Fatalf("seat 0 series row = %+v, want between/game 2", row)
	}
	if row.Waiting == nil || len(row.Waiting) != 2 || row.Waiting[0] != 0 || row.Waiting[1] != 3 {
		t.Fatalf("between row Waiting = %v, want [0 3] before either side locks", row.Waiting)
	}
	table1 := row.Table // game 1's table; the series plays every game on it

	// A stale game number, a both-set body and a neither-set body are all
	// refused, in that order.
	_, err := p.h.Sideboard(id, tok0, protocol.SideboardRequest{Game: 3, Keep: true})
	wantErr(t, err, codeConflict, 409)
	_, err = p.h.Sideboard(id, tok0, protocol.SideboardRequest{Game: 2, Keep: true,
		Deck: &protocol.DeckRequest{Main: []string{"x"}}})
	wantErr(t, err, codeBadRequest, 400)
	_, err = p.h.Sideboard(id, tok0, protocol.SideboardRequest{Game: 2})
	wantErr(t, err, codeBadRequest, 400)

	// While between, the pool already names the deck game 1 was played
	// with: revision 1.
	pool0, err := p.h.EventPool(id, tok0)
	if err != nil {
		t.Fatalf("EventPool(seat 0): %v", err)
	}
	if pool0.Playing == nil || pool0.PlayingRev != 1 {
		t.Fatalf("seat 0 pool Playing = %+v/%d, want rev 1", pool0.Playing, pool0.PlayingRev)
	}

	// Seat 0 sideboards a new deck revision: its first main card is
	// swapped for an unused pool card, so the submission is real. The
	// revision is saved, becomes the playing deck and locks ONLY seat 0's
	// side — seat 3's token never touches it, and until seat 3 acts its
	// side stays unlocked.
	d := pool0.Suggestion
	if len(d.Main) < 1 {
		t.Fatalf("seat 0 suggestion too small: %+v", d)
	}
	used := map[string]bool{}
	for _, mid := range d.Main {
		used[mid] = true
	}
	swap := ""
	for _, c := range pool0.Pool {
		if !used[c.ID] {
			swap = c.ID
			break
		}
	}
	if swap == "" {
		t.Fatal("precondition: seat 0's pool has no unused card to swap in")
	}
	d.Main[0] = swap
	if _, err := p.h.Sideboard(id, tok0, protocol.SideboardRequest{Game: 2, Deck: &d}); err != nil {
		t.Fatalf("Sideboard(seat 0 deck): %v", err)
	}
	r.mu.Lock()
	v, ok := r.ev.SeriesState(0, 3)
	r.mu.Unlock()
	if !ok || v.Locked != [2]int{2, 0} {
		t.Fatalf("Locked after seat 0's deck = %+v, want [2 0]", v)
	}
	if _, err := p.h.Sideboard(id, tok0, protocol.SideboardRequest{Game: 2, Keep: true}); err == nil {
		t.Fatal("second Sideboard by seat 0 succeeded")
	}

	// Seat 3 keeps the deck it played: that locks ONLY seat 3's side.
	if _, err := p.h.Sideboard(id, tok3, protocol.SideboardRequest{Game: 2, Keep: true}); err != nil {
		t.Fatalf("Sideboard(seat 3 keep): %v", err)
	}
	r.mu.Lock()
	v, ok = r.ev.SeriesState(0, 3)
	r.mu.Unlock()
	if !ok || v.Locked != [2]int{2, 1} {
		t.Fatalf("Locked after seat 3 keep = %+v, want [2 1]", v)
	}

	// Neither pool body carries the other human's cards.
	pool3, err := p.h.EventPool(id, tok3)
	if err != nil {
		t.Fatalf("EventPool(seat 3): %v", err)
	}
	if pool3.Playing == nil || pool3.PlayingRev != 1 {
		t.Fatalf("seat 3 pool Playing = %+v/%d, want rev 1 (its keep)", pool3.Playing, pool3.PlayingRev)
	}
	in0, in3 := map[string]bool{}, map[string]bool{}
	for _, c := range pool0.Pool {
		in0[c.ID] = true
	}
	for _, c := range pool3.Pool {
		in3[c.ID] = true
	}
	b0, _ := json.Marshal(pool0)
	b3, _ := json.Marshal(pool3)
	for _, c := range pool3.Pool {
		if !in0[c.ID] && strings.Contains(string(b0), `"`+c.ID+`"`) {
			t.Errorf("seat 0's pool body leaks seat 3 card %q", c.ID)
		}
	}
	for _, c := range pool0.Pool {
		if !in3[c.ID] && strings.Contains(string(b3), `"`+c.ID+`"`) {
			t.Errorf("seat 3's pool body leaks seat 0 card %q", c.ID)
		}
	}

	// The serialized event log each seat reads is redacted exactly as
	// event.LogFor promises while the event is incomplete: another seat's
	// PackOpened card list, Picked card and DeckSaved deck are hidden, and
	// no seat sees a GameStarted seed. (An id-contains scan would
	// false-positive: a card that later lands in seat 0's pool was
	// legitimately listed in packs opened to seat 3 on its way around the
	// pod.)
	log0, err := p.h.EventLog(id, tok0)
	if err != nil {
		t.Fatalf("EventLog(seat 0): %v", err)
	}
	log3, err := p.h.EventLog(id, tok3)
	if err != nil {
		t.Fatalf("EventLog(seat 3): %v", err)
	}
	for _, chk := range []struct {
		viewer int
		log    protocol.EventLog
	}{{0, log0}, {3, log3}} {
		for _, raw := range chk.log.Entries {
			var en event.Entry
			if err := json.Unmarshal(raw, &en); err != nil {
				t.Fatalf("log entry for seat %d: %v", chk.viewer, err)
			}
			switch en.Kind {
			case event.KindPackOpened:
				if en.Seat != chk.viewer && len(en.Cards) != 0 {
					t.Errorf("seat %d's log shows seat %d's pack cards", chk.viewer, en.Seat)
				}
			case event.KindPicked:
				if en.Seat != chk.viewer && en.ID != "" {
					t.Errorf("seat %d's log shows seat %d's pick %q", chk.viewer, en.Seat, en.ID)
				}
			case event.KindDeckSaved:
				if en.Seat != chk.viewer && en.Deck != nil {
					t.Errorf("seat %d's log shows seat %d's saved deck", chk.viewer, en.Seat)
				}
			case event.KindGameStarted:
				if en.GameSeed != 0 {
					t.Errorf("seat %d's log shows a GameStarted seed", chk.viewer)
				}
			}
		}
	}

	// The next game starts on the series' own table — the same table game
	// 1 was played on; each seat's join link names its own slot and token.
	att, table := waitSeriesInFlight(t, r, 0, 3, 2, 50*time.Second)
	if att.Game != 2 {
		t.Fatalf("game 2 attempt = %+v", att)
	}
	if string(table) != table1 {
		t.Fatalf("game 2 table %q, want the series' own table %q", table, table1)
	}
	// Game 2 is live: both saved revisions are the playing decks now.
	pool0, err = p.h.EventPool(id, tok0)
	if err != nil {
		t.Fatalf("EventPool(seat 0) after game 2 started: %v", err)
	}
	if pool0.Rev != 2 || pool0.PlayingRev != 2 || pool0.Playing == nil {
		t.Fatalf("seat 0 pool after game 2 started: rev %d playing %d/%+v, want rev 2", pool0.Rev, pool0.PlayingRev, pool0.Playing)
	}
	if got := *pool0.Playing; len(got.Main) != len(d.Main) || got.Main[0] != swap {
		t.Fatalf("seat 0 playing deck = %+v, want main[0] %q", got, swap)
	}
	var m2 *protocol.MatchInfo
	deadline := time.Now().Add(50 * time.Second)
	for {
		mi, merr := p.reg.Matches(table)
		if merr == nil && len(mi) >= 2 {
			m2 = &mi[1]
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("match 2 never appeared on %s: %d matches, %v", table, len(mi), merr)
		}
		time.Sleep(pollEvery)
	}
	if m2.Seats[0].DeckID != DeckID(id, 0, 2) || m2.Seats[1].DeckID != DeckID(id, 3, 1) {
		t.Fatalf("match 2 decks = %q/%q, want %q/%q", m2.Seats[0].DeckID, m2.Seats[1].DeckID, DeckID(id, 0, 2), DeckID(id, 3, 1))
	}
	st0 = stateOf(t, p.h, id, tok0)
	st3 := stateOf(t, p.h, id, tok3)
	row0, row3 := seriesRow(st0, 0, 3), seriesRow(st3, 0, 3)
	want0 := fmt.Sprintf("/t/%s?seat=0&token=tok-%s-0&event=%s", table, table, id)
	want3 := fmt.Sprintf("/t/%s?seat=1&token=tok-%s-1&event=%s", table, table, id)
	if row0 == nil || row0.Join != want0 {
		t.Fatalf("seat 0 join = %+v, want %q", row0, want0)
	}
	if row3 == nil || row3.Join != want3 {
		t.Fatalf("seat 3 join = %+v, want %q", row3, want3)
	}

	// The public state — what a spectator without a token reads — shows
	// the series but never a join link.
	pub := stateOf(t, p.h, id, "")
	for i := range pub.Series {
		if pub.Series[i].Join != "" {
			t.Errorf("public series row %+v carries a join link", pub.Series[i])
		}
	}

	// The log carries both Sideboarded entries with the revisions played,
	// and seat 0's DeckSaved for the new revision.
	r.mu.Lock()
	entries := append([]event.Entry(nil), r.ev.Log()...)
	r.mu.Unlock()
	saw := map[int]int{} // seat -> Rev
	sawDeckSave := false
	for _, en := range entries {
		if en.Kind == event.KindSideboarded && en.A == 0 && en.B == 3 && en.Game == 2 {
			if en.Auto {
				t.Errorf("human Sideboarded entry for seat %d is Auto", en.Seat)
			}
			saw[en.Seat] = en.Rev
		}
		if en.Kind == event.KindDeckSaved && en.Seat == 0 && en.Rev == 2 {
			sawDeckSave = true
		}
	}
	if saw[0] != 2 || saw[3] != 1 {
		t.Errorf("sideboarded revisions = %v, want seat 0 rev 2, seat 3 rev 1", saw)
	}
	if !sawDeckSave {
		t.Error("no DeckSaved entry for seat 0's revision 2")
	}

	// Both sides keep their way through the rest of the series, and the
	// series ends with a log entry. (After the last game the round rolls
	// over and the pairing leaves SeriesState, so the log is the record.)
	driveSeriesToEnd(t, p.h, p.reg, r, id, tok0, tok3, 0, 3)
	r.mu.Lock()
	entries = append([]event.Entry(nil), r.ev.Log()...)
	r.mu.Unlock()
	sawEnd := false
	for _, en := range entries {
		if en.Kind == event.KindSeriesEnded && en.A == 0 && en.B == 3 {
			sawEnd = true
			if en.Score[0]+en.Score[1] < 2 || en.Score[0] > 2 || en.Score[1] > 2 {
				t.Errorf("SeriesEnded score = %v, impossible for a Bo3", en.Score)
			}
		}
	}
	if !sawEnd {
		t.Fatal("no SeriesEnded entry for (0,3)")
	}
	// After the series ended the sideboard window is closed for good.
	_, err = p.h.Sideboard(id, tok0, protocol.SideboardRequest{Game: 2, Keep: true})
	wantErr(t, err, codeConflict, 409)
}

// TestSideboardErrors drives the error surface: before game 1 ends the
// series is not between games; identity comes only from the token; a
// stale game number, a both-set or neither-set body, a card not in the
// seat's pool and a second submission are each refused.
func TestSideboardErrors(t *testing.T) {
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	})
	req := oneHumanReq()
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	pickTop(t, p.h, id, tok, 45)
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	// Before game 1 ends the series is not between games: neither keep
	// nor a deck may be submitted.
	_, err := p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 1, Keep: true})
	wantErr(t, err, codeConflict, 409)
	// Identity comes only from the event seat token: an empty token and a
	// token no seat owns (the bot seats have no invites at all) are both
	// 401, and nothing is locked by either.
	_, err = p.h.Sideboard(id, "", protocol.SideboardRequest{Game: 1, Keep: true})
	wantErr(t, err, codeUnauthorized, 401)
	_, err = p.h.Sideboard(id, "deadbeef", protocol.SideboardRequest{Game: 1, Keep: true})
	wantErr(t, err, codeUnauthorized, 401)

	r := runOf(t, p.h, id)
	humanDriveUntilBetween(t, p.reg, r, 0, 3)

	_, err = p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 3, Keep: true})
	wantErr(t, err, codeConflict, 409)
	_, err = p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 2, Keep: true,
		Deck: &protocol.DeckRequest{Main: []string{"x"}}})
	wantErr(t, err, codeBadRequest, 400)
	_, err = p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 2})
	wantErr(t, err, codeBadRequest, 400)
	_, err = p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 2,
		Deck: &protocol.DeckRequest{Main: []string{"no-such-card"}}})
	wantErr(t, err, codeInvalidDeck, 422)
	r.mu.Lock()
	saved := r.ev.SavedRev(0)
	r.mu.Unlock()
	if saved != 1 {
		t.Fatalf("SavedRev after the refused deck = %d, want 1 (a 422 writes nothing)", saved)
	}

	// A token no seat owns cannot act while seat 0's side is unlocked —
	// and nothing is locked by it.
	_, err = p.h.Sideboard(id, "deadbeef", protocol.SideboardRequest{Game: 2, Keep: true})
	wantErr(t, err, codeUnauthorized, 401)
	r.mu.Lock()
	v, ok := r.ev.SeriesState(0, 3)
	r.mu.Unlock()
	if !ok || v.Locked != [2]int{0, 0} {
		t.Fatalf("Locked after the garbage token = %+v, want [0 0]", v)
	}

	// The seat's own token locks its side; a second submission is a 409.
	if _, err := p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 2, Keep: true}); err != nil {
		t.Fatalf("Sideboard(keep): %v", err)
	}
	_, err = p.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 2, Keep: true})
	wantErr(t, err, codeConflict, 409)
	// A token no seat owns still cannot act, and the garbage token never
	// locked seat 3's side — that side belongs to the bot path.
	_, err = p.h.Sideboard(id, "deadbeef", protocol.SideboardRequest{Game: 2, Keep: true})
	wantErr(t, err, codeUnauthorized, 401)
	r.mu.Lock()
	v, ok = r.ev.SeriesState(0, 3)
	r.mu.Unlock()
	if !ok || v.Locked[0] != 1 {
		t.Fatalf("Locked after the error tour = %+v, want seat 0's own side locked", v)
	}
}

// TestIdleSeatAutoKeepsBetweenGames is the E1-8 half: a human that played
// game 1 and then goes idle keeps the deck it played for game 2 (one
// auto-keep, Auto on the entry) and — still idle once game 2 is live —
// loses the series by concession in the same sweep.
func TestIdleSeatAutoKeepsBetweenGames(t *testing.T) {
	clock := newTestClock()
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
		eo.IdleTimeout = 30 * time.Minute
		eo.Now = clock.Now
	})
	req := oneHumanReq()
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	pickTop(t, p.h, id, tok, 45)
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	r := runOf(t, p.h, id)
	humanDriveUntilBetween(t, p.reg, r, 0, 3)
	r.mu.Lock()
	v, ok := r.ev.SeriesState(0, 3)
	r.mu.Unlock()
	if !ok || !v.Between || len(v.Games) != 1 || v.InFlight != nil {
		t.Fatalf("series before the idle sweep = %+v, want between after game 1", v)
	}
	w1 := v.Games[0]
	rev := v.PlayRevs[0]

	// The seat goes idle; one sweep auto-keeps, starts game 2 and, the
	// seat still owing its game decisions, concedes the series.
	clock.Add(30*time.Minute + time.Minute)
	p.h.checkTimeouts(clock.Now())

	r.mu.Lock()
	entries := append([]event.Entry(nil), r.ev.Log()...)
	phase := r.ev.Phase()
	r.mu.Unlock()
	// The idle path may have finished round 1 entirely (the bot series
	// plays fast too), so the (0,3) pairing may already be re-paired away:
	// the assertions read the log.
	var gotKeep, sawConcedeGame2, sawSeriesEnd bool
	wantScore := [2]int{0, 2}
	if w1 == 0 {
		wantScore = [2]int{1, 2}
	}
	for _, en := range entries {
		switch {
		case en.Kind == event.KindSideboarded && en.A == 0 && en.B == 3 && en.Game == 2:
			gotKeep = true
			if !en.Auto {
				t.Errorf("auto-keep Sideboarded entry is not Auto: %+v", en)
			}
			if en.Seat != 0 || en.Rev != rev {
				t.Errorf("auto-keep Sideboarded = seat %d rev %d, want seat 0 rev %d", en.Seat, en.Rev, rev)
			}
		case en.Kind == event.KindGameEnded && en.A == 0 && en.B == 3 && en.Game == 2 && en.Head == "conceded":
			sawConcedeGame2 = true
		case en.Kind == event.KindSeriesEnded && en.A == 0 && en.B == 3:
			sawSeriesEnd = true
			if en.Score != wantScore {
				t.Errorf("series score = %v, want %v (game 1 winner %d)", en.Score, wantScore, w1)
			}
		}
	}
	if !gotKeep || !sawConcedeGame2 || !sawSeriesEnd {
		t.Fatalf("sweep log (phase %s): keep %v, conceded game 2 %v, series ended %v", phase, gotKeep, sawConcedeGame2, sawSeriesEnd)
	}
}
