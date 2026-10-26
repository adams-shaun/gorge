package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// liliana6Board puts the corpus Liliana of the Veil (loyalty raised to 6) on
// seat 0's battlefield and four creatures on seat 1's, activates the -6 aimed
// at seat 1 and returns the engine, the four ids and the posed split ask.
func liliana6Board(t *testing.T, seed uint64) (*Engine, []state.ObjID, *decision.Decision) {
	t.Helper()
	deck1 := kr3Cards(t, kr3Creature("Alpha"), kr3Creature("Beta"), kr3Creature("Gamma"), kr3Creature("Delta"))
	e, _ := kr3Game(t, seed, []*cards.Card{corpusAlternativeCard(t, "Liliana of the Veil")}, deck1)
	lili := kr3Move(t, e, 0, "Liliana of the Veil", state.ZBattlefield)
	var ids []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma", "Delta"} {
		ids = append(ids, kr3Move(t, e, 1, n, state.ZBattlefield))
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: lili, Counter: "LOYALTY", Amount: 6 - e.G.Obj(lili).Counter("LOYALTY")})
	if got := e.G.Obj(lili).Counter("LOYALTY"); got != 6 {
		t.Fatalf("precondition: Liliana loyalty = %d, want 6", got)
	}
	for _, id := range ids {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Controller != 1 {
			t.Fatalf("precondition: %d is not seat 1's permanent: %+v", id, o)
		}
	}
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, lili, 2).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the target ask", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat 1 not offered as the target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	return e, ids, kr3Next(t, e)
}

// TestTwoPilesBattlefieldLiliana: Liliana of the Veil's -6 against seat 1,
// who controls four permanents. Seat 0 splits them 1/3, seat 1 picks a pile,
// and exactly the chosen pile is sacrificed.
func TestTwoPilesBattlefieldLiliana(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct {
		pick       string
		sacrificed []int
	}{
		{"pile-a", []int{0}},
		{"pile-b", []int{1, 2, 3}},
	} {
		t.Run(tc.pick, func(t *testing.T) {
			e, ids, d := liliana6Board(t, uint64(701+i))
			if d == nil || d.ResumeKind != "twopiles_split" || d.Player != 0 {
				t.Fatalf("split ask = %+v, want seat 0's twopiles_split", d)
			}
			if got := kr3OptionObjs(d); !slices.Equal(got, ids) {
				t.Fatalf("split options = %v, want seat 1's permanents %v", got, ids)
			}
			d = kr3Answer(t, e, kr3Indexes(t, d, ids[0])...)
			if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 1 {
				t.Fatalf("pick ask = %+v, want seat 1's twopiles_pick", d)
			}
			for _, id := range ids {
				if kr3Zone(e, id) != state.ZBattlefield {
					t.Fatalf("%d left the battlefield before the pick", id)
				}
			}
			if d := kr3Answer(t, e, kr3OptionKind(t, d, tc.pick)); d != nil {
				t.Fatalf("a fully answered TwoPiles posed %+v", d)
			}
			for n, id := range ids {
				want := state.ZBattlefield
				if slices.Contains(tc.sacrificed, n) {
					want = state.ZGraveyard
				}
				if got := kr3Zone(e, id); got != want {
					t.Fatalf("card %d zone = %v, want %v", n, got, want)
				}
			}
		})
	}
}
