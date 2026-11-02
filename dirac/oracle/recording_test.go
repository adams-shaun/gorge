package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// playedRecording plays a game on adapter e under a random policy, through lockstep,
// and returns its log.
func playedRecording(t *testing.T, e *adapter.Engine, m *Matchup, seed uint64) *events.Log {
	t.Helper()
	k, err := AdapterSide(e, Config{ArrangeRestOrder: true})(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: Config{ArrangeRestOrder: true}})
	if !res.Clean() {
		t.Fatalf("the game itself: %+v", res.Div)
	}
	return AdapterGame(k).Log()
}

func recordingMatchup(t *testing.T) (*adapter.Engine, *Matchup, uint64) {
	t.Helper()
	c, pool := testPoolCorpus(t, "fdn")
	e, err := adapter.New([]string{"fdn"}, c.Reg, dirac.SurfaceFull)
	if err != nil {
		t.Skip(err)
	}
	p := firstServedPair(t, c, pool)
	m, _, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 1)
	if err != nil {
		t.Fatal(err)
	}
	return e, m, p.Seed
}

func TestLockstepRecordingAgrees(t *testing.T) {
	e, m, seed := recordingMatchup(t)
	l := playedRecording(t, e, m, seed)
	res, err := lockstepRecording(e, m, l)
	if err != nil {
		t.Fatal(err)
	}
	if res.Div != nil || res.Decisions == 0 {
		t.Fatalf("%d decisions, %v", res.Decisions, res.Div)
	}
}

// TestLockstepRecordingNamesADivergence gives gorge the other seat's deck: the first
// board compared differs.
func TestLockstepRecordingNamesADivergence(t *testing.T) {
	e, m, seed := recordingMatchup(t)
	l := playedRecording(t, e, m, seed)
	wrong, err := NewMatchup(m.reg, m.Names, [2][]*cards.Card{m.Decks[1], m.Decks[0]}, m.Mulligans)
	if err != nil {
		t.Fatal(err)
	}
	lead, err := NewGorgeSide(wrong, l.Seed, Config{ArrangeRestOrder: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	k, err := AdapterSide(e, Config{ArrangeRestOrder: true})(m, l.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := LockstepLed(m, l.Seed, lead, k, FirstPolicy(), RunConfig{Config: Config{ArrangeRestOrder: true}})
	if res.Div == nil {
		t.Fatal("no divergence named")
	}
}

// TestRecordingStopsAtAConcession keeps the kernel out of a game only the host ended.
func TestRecordingStopsAtAConcession(t *testing.T) {
	e, m, seed := recordingMatchup(t)
	k, err := AdapterSide(e, Config{ArrangeRestOrder: true})(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	g := AdapterGame(k)
	for g.Pending() != nil {
		d := g.Pending()
		var conceded bool
		for i, o := range d.Options {
			if o.Kind == "concede" {
				if err := g.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}); err != nil {
					t.Fatal(err)
				}
				conceded = true
				break
			}
		}
		if conceded {
			break
		}
		k.Step()
		a := k.(*adapterSide)
		k.Apply(a.cur[0].Canon)
	}
	res, err := lockstepRecording(e, m, g.Log())
	if err != nil {
		t.Fatal(err)
	}
	if res.Div != nil {
		t.Fatalf("%v", res.Div)
	}
}
