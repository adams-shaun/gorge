package host

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// sampleLoader serves testutil.SampleDecks' four decks under the names
// "a".."d", so every host test is corpus-free and deterministic.
func sampleLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	byName := map[string][]*cards.Card{}
	for i, n := range names {
		byName[n] = decks[i]
	}
	return func(name string) (Deck, error) {
		cs, ok := byName[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: cs}, nil
	}
}

func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{LoadDeck: sampleLoader(t), Sleep: func(time.Duration) {}}
}

func fourSeatTable(id TableID, perpetual bool) TableConfig {
	return TableConfig{ID: id, Name: "Table " + string(id), Seats: 4, Decks: []string{"a", "b", "c", "d"},
		Seed: 99, Pace: 0, Spectator: view.Omniscient, Perpetual: perpetual}
}

func TestATablePlaysOneMatchToCompletionAndGoesIdle(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if got := r.Tables(); len(got) != 1 || got[0].State != protocol.TableIdle || got[0].Match != 0 {
		t.Fatalf("before start: %+v", got)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ti := r.Tables()[0]
	if ti.State != protocol.TableIdle || ti.Match != 1 || ti.Seats != 4 || ti.Spectator != "omniscient" {
		t.Fatalf("after one match: %+v", ti)
	}
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches %v, %v", ms, err)
	}
	m := ms[0]
	if m.State != protocol.MatchFinished || m.Match != 1 || m.Seed != MatchSeed(99, 1) || m.Head == "" || m.Events < 100 || m.Turns < 2 {
		t.Fatalf("match info %+v", m)
	}
	if m.Result != "win" && m.Result != "draw" {
		t.Fatalf("result %q", m.Result)
	}
	if (m.Result == "win") != (m.Winner != nil) {
		t.Fatalf("winner/result disagree: %+v", m)
	}
	if len(m.Seats) != 4 || m.Seats[0].Colour != protocol.SeatColours[0] || m.Seats[1].Deck != "c" {
		// k=1: seat i plays Decks[(i+1)%4] → a,b,c,d rotate to b,c,d,a.
		t.Fatalf("seats %+v", m.Seats)
	}
}

func TestTheSameConfigurationPlaysTheSameMatch(t *testing.T) {
	run := func() protocol.MatchInfo {
		r, _ := New(testOptions(t))
		defer r.Close()
		if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
			t.Fatal(err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatal(err)
		}
		r.Wait("t1")
		ms, _ := r.Matches("t1")
		return ms[0]
	}
	a, b := run(), run()
	if a.Head != b.Head || a.Events != b.Events || a.Turns != b.Turns {
		t.Fatalf("two runs differ: %+v vs %+v", a, b)
	}
}

func TestAPerpetualTableStartsTheNextMatchWithTheDerivedSeed(t *testing.T) {
	cooled := 0
	o := testOptions(t)
	o.Cooldown = time.Second
	var r *Registry
	o.Sleep = func(d time.Duration) {
		if d == time.Second {
			cooled++
			if cooled == 2 {
				// Stop after two matches. Close is asynchronous (it waits for
				// this very goroutine), so wait for its stop signal before
				// returning, or the loop could start a third match first.
				go r.Close()
				<-r.Done()
			}
		}
	}
	r, _ = New(o)
	if err := r.AddTable(fourSeatTable("t1", true)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if len(ms) < 2 {
		t.Fatalf("perpetual table played %d matches", len(ms))
	}
	if ms[0].Seed != MatchSeed(99, 1) || ms[1].Seed != MatchSeed(99, 2) || ms[0].Head == ms[1].Head {
		t.Fatalf("seeds/heads %+v %+v", ms[0], ms[1])
	}
	if ms[1].Seats[0].Deck != "c" { // k=2: seat 0 plays Decks[(0+2)%4]
		t.Fatalf("decks did not rotate: %+v", ms[1].Seats)
	}
}

func TestCloseAbortsALiveMatch(t *testing.T) {
	o := testOptions(t)
	var r *Registry
	n := 0
	o.Sleep = func(time.Duration) {
		n++
		if n == 50 {
			go r.Close()
			<-r.Done()
		}
	}
	r, _ = New(o)
	if err := r.AddTable(fourSeatTable("t1", true)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchAborted || ms[0].Result != "" {
		t.Fatalf("after Close: %+v", ms)
	}
	if r.Tables()[0].State != protocol.TableIdle {
		t.Fatalf("table state %s after Close", r.Tables()[0].State)
	}
}

func TestConfigurationIsValidated(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	bad := []TableConfig{
		{},
		{ID: "x", Seats: 0, Decks: []string{"a"}},
		{ID: "x", Seats: 9, Decks: []string{"a"}},
		{ID: "x", Seats: 2},
		{ID: "x", Seats: 2, Decks: []string{"a"}, Spectator: view.Seat},
		{ID: "x", Seats: 2, Decks: []string{"nope"}},
	}
	for i, c := range bad {
		if err := r.AddTable(c); err == nil {
			t.Errorf("config %d accepted: %+v", i, c)
		}
	}
	good := TableConfig{ID: "x", Seats: 2, Decks: []string{"a"}, Spectator: view.Public}
	if err := r.AddTable(good); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(good); err == nil {
		t.Fatal("duplicate table id accepted")
	}
	if err := r.Start("missing"); err == nil {
		t.Fatal("Start of an unknown table succeeded")
	}
	if _, err := r.Matches("missing"); err == nil {
		t.Fatal("Matches of an unknown table succeeded")
	}
}

func TestNewRequiresLoadDeckAndSleep(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("New accepted empty Options")
	}
}
