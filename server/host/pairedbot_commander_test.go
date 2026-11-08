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
// plays the table's format, and otherwise the first choice that does.
func TestPairedBotDefaultFollowsTheFormat(t *testing.T) {
	def, cmd := pairedTestEntry(t), pairedTestEntry(t)
	cmd.Name, cmd.Formats = "test-commander", []string{"commander"}
	o := testOptions(t)
	o.LoadDeck = commanderDeckLoader(t)
	o.PairedBot, o.PairedBotChoices = def, []*bots.Entry{cmd}
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
		t.Errorf("commander table at the default: %+v", st)
	}
	con := commanderPairedTable("con", 2, "")
	con.Format = FormatConstructed
	if st := add(con); !st.Offered || st.Policy != def.Name {
		t.Errorf("constructed table at the default: %+v", st)
	}
	if st := add(commanderPairedTable("off", 2, PairedBotOff)); st.Offered {
		t.Errorf("a table naming off: %+v", st)
	}
	st := add(commanderPairedTable("four", 4, ""))
	if st.Offered || !strings.Contains(st.Reason, "at most 2 seats") {
		t.Errorf("a 4-seat commander table: %+v", st)
	}
}

// PairedStatusView keeps the test's helper signature readable.
type PairedStatusView = protocol.PairedBotStatus

// The real closed search, over a belief of Commander decks, advises a human at
// a 2-seat Commander table.
func TestPairedBotAdvisesACommanderTable(t *testing.T) {
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
	o.PairedBot = &bots.Entry{
		Info: bots.Info{Name: "test-commander", Tier: bots.Experimental, Env: true, Search: true, ClosedLists: true,
			Formats: []string{"commander"}, MaxSeats: 2},
		New: azredeal.ClosedFactory(b, 12),
	}
	o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{passSeat{}, passSeat{}} }
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := commanderPairedTable("t1", 2, "")
	c.Decks = []string{"foundations-tramplesaurus-rex", "foundations-wretched-ranks"}
	if err := r.AddTable(c); err != nil {
		t.Fatal(err)
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
