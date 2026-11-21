package host

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func commanderPairedTable(id TableID, seats int, choice string) TableConfig {
	c := pairedTable([]int{0})
	c.ID, c.Seats, c.Format, c.PairedBot = id, seats, FormatCommander, choice
	c.Decks = append([]string(nil), commanderDeckNames[:seats]...)
	return c
}

// A table that leaves its paired bot at the default gets the default when it
// plays the table's format at the table's seat count, and otherwise the first
// choice that does; with no choice for its seat count, the first choice for
// its format, which says why it cannot advise there.
func TestPairedBotDefaultFollowsTheFormat(t *testing.T) {
	def, cmd, pod := pairedTestEntry(t), pairedTestEntry(t), pairedTestEntry(t)
	cmd.Name, cmd.Formats = "test-commander", []string{"commander"}
	pod.Name, pod.Formats, pod.MaxSeats = "test-pod", []string{"constructed", "commander"}, 4
	for _, tc := range []struct {
		name    string
		choices []*bots.Entry
		four    string // the 4-seat Commander table's bot, "" when it has none
	}{
		{"no pod choice", []*bots.Entry{cmd}, ""},
		{"a pod choice", []*bots.Entry{cmd, pod}, pod.Name},
	} {
		o := testOptions(t)
		o.LoadDeck = commanderDeckLoader(t)
		o.PairedBot, o.PairedBotChoices = def, tc.choices
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })

		add := func(c TableConfig) PairedStatusView {
			t.Helper()
			if err := r.AddTable(c); err != nil {
				t.Fatal(err)
			}
			st, err := r.PairedBotStatus(c.ID)
			if err != nil {
				t.Fatal(err)
			}
			return st
		}
		if st := add(commanderPairedTable("cmd", 2, "")); !st.Offered || st.Policy != cmd.Name {
			t.Errorf("%s: commander table at the default: %+v", tc.name, st)
		}
		con := commanderPairedTable("con", 2, "")
		con.Format = FormatConstructed
		if st := add(con); !st.Offered || st.Policy != def.Name {
			t.Errorf("%s: constructed table at the default: %+v", tc.name, st)
		}
		if st := add(commanderPairedTable("off", 2, PairedBotOff)); st.Offered {
			t.Errorf("%s: a table naming off: %+v", tc.name, st)
		}
		st := add(commanderPairedTable("four", 4, ""))
		switch {
		case tc.four == "" && (st.Offered || !strings.Contains(st.Reason, "at most 2 seats")):
			t.Errorf("%s: a 4-seat commander table: %+v", tc.name, st)
		case tc.four != "" && (!st.Offered || st.Policy != tc.four):
			t.Errorf("%s: a 4-seat commander table: %+v, want %s", tc.name, st, tc.four)
		}
	}
}

// PairedStatusView keeps the test's helper signature readable.
type PairedStatusView = protocol.PairedBotStatus

// The real closed search, over a belief of Commander decks, advises a human at
// a 2-seat Commander table.
func TestPairedBotAdvisesACommanderTable(t *testing.T) {
	t.Parallel()
	adviseCommanderTable(t, 2, &bots.Entry{
		Info: bots.Info{Name: "test-commander", Tier: bots.Experimental, Env: true, Search: true, ClosedLists: true,
			Formats: []string{"commander"}, MaxSeats: 2},
	}, func(b *searchprobe.Belief) bots.Factory { return azredeal.ClosedFactory(b, 12) })
}

// The closed max^n search advises a human at a 4-seat Commander pod: the
// table at the default offers it, and a priority decision is advised with a
// search.
func TestPairedBotAdvisesAFourSeatCommanderTable(t *testing.T) {
	t.Parallel()
	adviseCommanderTable(t, 4, &bots.Entry{Info: azredeal.ClosedMaxNInfo(4)},
		func(b *searchprobe.Belief) bots.Factory { return azredeal.ClosedMaxNFactory(b, 4) })
}

// adviseCommanderTable seats a human at seat 0 of a seats-seat Commander table
// with the paired bot e (its factory built over a belief of the Commander
// decks) as the default, and asks it about the human's decisions: no root is
// refused, and some priority decision is advised with a search.
func adviseCommanderTable(t *testing.T, seats int, e *bots.Entry, factory func(*searchprobe.Belief) bots.Factory) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var belief [][]*cards.Card
	for _, n := range commanderDeckNames {
		belief = append(belief, testutil.RepoDeck(t, reg, n))
	}
	b, err := searchprobe.NewBelief(belief)
	if err != nil {
		t.Fatal(err)
	}
	o := testOptions(t)
	o.LoadDeck = commanderDeckLoader(t)
	e.New = factory(b)
	o.PairedBot = e
	o.Seats = func([]string, uint64) []seat.Seat {
		out := make([]seat.Seat, seats)
		for i := range out {
			out[i] = passSeat{}
		}
		return out
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := commanderPairedTable("t1", seats, "")
	c.Decks[0], c.Decks[1] = "foundations-tramplesaurus-rex", "foundations-wretched-ranks"
	if seats > 2 {
		c.Decks[2], c.Decks[3] = "foundations-calling-all-angels", "foundations-keen-engineering"
	}
	if err := r.AddTable(c); err != nil {
		t.Fatal(err)
	}
	st, err := r.PairedBotStatus("t1")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Offered || st.Policy != e.Name {
		t.Fatalf("paired-bot status %+v, want %s offered", st, e.Name)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	searched := 0
	var after uint64
	for i := 0; i < 120; i++ {
		// The seat plays lands first, so its first searchable priority comes
		// after about fifty decisions.
		d := censusParked(r, after)
		if d == nil {
			break
		}
		a, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
		if err != nil {
			t.Fatalf("seq %d (%s): %v", d.Seq, d.Kind, err)
		}
		if d.Kind == decision.KPriority {
			if a.State == protocol.AdviceRefused && a.Reason == adviceNoRoot {
				t.Fatalf("seq %d: refused with %q", d.Seq, a.Reason)
			}
			if a.State == protocol.AdviceReady {
				var n explain.Note
				if err := json.Unmarshal(a.Explain, &n); err != nil {
					t.Fatal(err)
				}
				if n.Search != nil {
					searched++
				}
			}
		}
		after = d.Seq
		if err := r.SubmitIntent("t1", 1, 0, landFirstAnswer(d)); err != nil {
			t.Fatal(err)
		}
	}
	if searched == 0 {
		t.Fatal("no priority decision was advised with a search")
	}
}
