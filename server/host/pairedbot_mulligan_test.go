package host

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// pairedMulliganTable starts a paired-bot table whose human seat 0 plays own
// against the heuristic bot on opp, with seed and up to seven mulligans.
func pairedMulliganTable(t *testing.T, own, opp []*cards.Card, seed uint64) *Registry {
	t.Helper()
	o := testOptions(t)
	o.LoadDeck = func(name string) (Deck, error) {
		if name == "human" {
			return Deck{Name: "own", Cards: own}, nil
		}
		return Deck{Name: "opposing", Cards: opp}, nil
	}
	o.PairedBot = pairedTestEntry(t)
	o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{passSeat{}, heuristic.NewBot(seed + 1)} }
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	c := pairedTable([]int{0})
	c.Seed, c.Mulligans = seed, 7
	if err := r.AddTable(c); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	return r
}

// nextMulligan is seat 0's next mulligan ask after decision after, answering
// a choice of who plays first on the way; nil once the round is over.
func nextMulligan(t *testing.T, r *Registry, after uint64) *decision.Decision {
	t.Helper()
	for {
		d := censusParked(r, after)
		if d == nil {
			return nil
		}
		switch d.Kind {
		case decision.KMulligan:
			return d
		case decision.KStartingPlayer:
			if err := r.SubmitIntent("t1", 1, 0, minimalAnswer(d)); err != nil {
				t.Fatal(err)
			}
			after = d.Seq
		default:
			return nil
		}
	}
}

// seatHand is seat 0's hand, the engine's live one.
func seatHand(r *Registry) (ids []state.ObjID, names []string, lands int) {
	_, m, _ := r.lookup("t1", 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.e.G.Zone(state.ZHand, 0) {
		f := m.e.G.Obj(id).Face()
		ids, names = append(ids, id), append(names, f.Name)
		if slices.Contains(f.Types, "Land") {
			lands++
		}
	}
	return ids, names, lands
}

// seatBoard is seat 0's board, for the bot's own answers.
func seatBoard(r *Registry) botpolicy.Board {
	_, m, _ := r.lookup("t1", 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return botpolicy.BoardFromGameInto(m.e.G, m.e, 0, new(botpolicy.Board)).Detached()
}

// adviceNote is a ready advice's note.
func adviceNote(t *testing.T, a protocol.Advice) explain.Note {
	t.Helper()
	var n explain.Note
	if err := json.Unmarshal(a.Explain, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

// adviceChoice is what the advice recommends: the chosen search row, or the
// rules answer.
func adviceChoice(n explain.Note) []int {
	if n.Search != nil {
		for _, r := range n.Search.Rows {
			if r.Chosen {
				return r.Choices
			}
		}
	}
	return n.Bot
}

// riggedSeven reorders own so that seat 0's opening seven at seed is the
// cards pick chooses (deck positions), every other card keeping its order.
// The engine adds a deck's objects in list order and shuffles their ids with
// the game's seed alone, so the positions the opening seven is drawn from
// are a function of the seed and the deck's size; the list the seat holds is
// the same multiset.
func riggedSeven(t *testing.T, own, opp []*cards.Card, seed uint64, pick []int) []*cards.Card {
	t.Helper()
	if len(pick) != 7 {
		t.Fatalf("pick %d cards, want 7", len(pick))
	}
	r := pairedMulliganTable(t, own, opp, seed)
	defer r.Close()
	if nextMulligan(t, r, 0) == nil {
		t.Fatal("fixture: no mulligan was asked")
	}
	ids, _, _ := seatHand(r)
	_, m, _ := r.lookup("t1", 1)
	m.mu.RLock()
	first := state.ObjID(0)
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range m.e.G.Zone(z, 0) {
			if first == 0 || id < first {
				first = id
			}
		}
	}
	m.mu.RUnlock()
	out := make([]*cards.Card, len(own))
	taken := make([]bool, len(own))
	for j, id := range ids {
		at := int(id - first)
		out[at], taken[pick[j]] = own[pick[j]], true
	}
	k := 0
	for i := range out {
		if out[i] != nil {
			continue
		}
		for taken[k] {
			k++
		}
		out[i], taken[k] = own[k], true
	}
	return out
}

// sevenOf picks seven deck positions: lands lands first (basics first, so
// the lands make the deck's colours), then the cheapest spells.
func sevenOf(deck []*cards.Card, lands int) []int {
	var ls, ss []int
	for i, c := range deck {
		if c.Faces[0].IsLand() {
			ls = append(ls, i)
		} else {
			ss = append(ss, i)
		}
	}
	slices.SortStableFunc(ls, func(a, b int) int {
		ba, bb := deck[a].Faces[0].IsBasic(), deck[b].Faces[0].IsBasic()
		switch {
		case ba && !bb:
			return -1
		case bb && !ba:
			return 1
		}
		return 0
	})
	slices.SortStableFunc(ss, func(a, b int) int {
		return int(deck[a].Faces[0].ManaValue() - deck[b].Faces[0].ManaValue())
	})
	return append(append([]int{}, ls[:lands]...), ss[:7-lands]...)
}

// Every mulligan ask a human meets -- keep or mulligan, and which cards to
// put back -- is answered by search: a ready advice whose note carries the
// search's lines.
func TestPairedBotSearchesTheMulligan(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	own, opp := testutil.RepoDeck(t, reg, "death-n-taxes"), testutil.RepoDeck(t, reg, "dimir-tempo")
	r := pairedMulliganTable(t, own, opp, 20261105)
	defer r.Close()
	var after uint64
	shapes := map[string]int{}
	for d := nextMulligan(t, r, 0); d != nil; d = nextMulligan(t, r, after) {
		a, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
		if err != nil {
			t.Fatal(err)
		}
		shape := "keep or mulligan"
		if d.Options[0].Kind == "bottom" {
			shape = "bottom"
		}
		shapes[shape]++
		if a.State != protocol.AdviceReady {
			t.Fatalf("%s ask %d (%q): %s (%s)", shape, d.Seq, d.Prompt, a.State, a.Reason)
		}
		n := adviceNote(t, a)
		if n.Search == nil || n.Search.Sims <= 0 {
			t.Fatalf("%s ask %d (%q): the advice carries no search (reason %q)", shape, d.Seq, d.Prompt, a.Reason)
		}
		// Mulligan once, so the round asks for a card to put back.
		in := minimalAnswer(d)
		if shapes["keep or mulligan"] == 1 && shape != "bottom" {
			in.Choices = []int{1}
		}
		if err := r.SubmitIntent("t1", 1, 0, in); err != nil {
			t.Fatal(err)
		}
		after = d.Seq
	}
	if shapes["keep or mulligan"] < 2 || shapes["bottom"] < 1 {
		t.Fatalf("fixture: asked %v, want both shapes", shapes)
	}
}

// The searched mulligan reads the hand: an opening seven of no lands, or of
// nothing but lands, is sent back, and three lands with cheap spells is
// kept. The default bot's coin cannot pass it.
func TestPairedBotMulliganAdviceReadsTheHand(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	own, opp := testutil.RepoDeck(t, reg, "death-n-taxes"), testutil.RepoDeck(t, reg, "dimir-tempo")
	const seed = 20261105
	for _, c := range []struct {
		lands int
		want  string
	}{{0, "mulligan"}, {7, "mulligan"}, {3, "keep"}} {
		deck := riggedSeven(t, own, opp, seed, sevenOf(own, c.lands))
		r := pairedMulliganTable(t, deck, opp, seed)
		d := nextMulligan(t, r, 0)
		if d == nil {
			r.Close()
			t.Fatal("fixture: no mulligan was asked")
		}
		_, names, lands := seatHand(r)
		if lands != c.lands {
			r.Close()
			t.Fatalf("fixture: rigged %d lands, dealt %d: %v", c.lands, lands, names)
		}
		a, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if a.State != protocol.AdviceReady {
			t.Fatalf("%d lands: %s (%s)", c.lands, a.State, a.Reason)
		}
		n := adviceNote(t, a)
		got := adviceChoice(n)
		if n.Search == nil || len(got) != 1 || d.Options[got[0]].Kind != c.want {
			t.Errorf("%d lands %v: advice %v, want %s (search %+v)", c.lands, names, got, c.want, n.Search)
		}
	}
}
