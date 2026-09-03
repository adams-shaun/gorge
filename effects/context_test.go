package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// fakeHost is the smallest thing satisfying Host: a real Game plus a captured
// event list, so effect tests assert on emitted events rather than internals.
type fakeHost struct {
	g   *state.Game
	log []events.Event
	n   int
}

func (h *fakeHost) Game() *state.Game { return h.g }
func (h *fakeHost) Emit(e events.Event) {
	h.log = append(h.log, e)
	events.Apply(h.g, e)
}
func (h *fakeHost) Rand(n int) int { h.n++; return 0 }

func newHost(t *testing.T, seats int) *fakeHost {
	t.Helper()
	return &fakeHost{g: state.NewGame(names(seats))}
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a' + i))
	}
	return out
}

func sa(t *testing.T, line string) *cards.SA {
	t.Helper()
	src := "Name:T\nTypes:Sorcery\nA:" + line + "\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	return c.Faces[0].Abilities[0]
}

func TestResolveWalksTheSubAbilityChain(t *testing.T) {
	var order []string
	Register("TestA", func(h Host, c *Ctx, s *cards.SA) { order = append(order, "A") })
	Register("TestB", func(h Host, c *Ctx, s *cards.SA) { order = append(order, "B") })
	Register("TestC", func(h Host, c *Ctx, s *cards.SA) { order = append(order, "C") })
	t.Cleanup(func() { unregister("TestA", "TestB", "TestC") })

	src := "Name:T\nTypes:Sorcery\nA:SP$ TestA | SubAbility$ X\nSVar:X:DB$ TestB | SubAbility$ Y\nSVar:Y:DB$ TestC\nOracle:x\n"
	c, _ := cards.ParseBytes("t.txt", []byte(src))
	c.Link()
	Resolve(newHost(t, 2), &Ctx{}, c.Faces[0].Abilities[0])

	if len(order) != 3 || order[0] != "A" || order[1] != "B" || order[2] != "C" {
		t.Fatalf("resolution order = %v", order)
	}
}

func TestResolveNotesUnimplementedAPIsInsteadOfPanicking(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ NoSuchApiExists | NumDmg$ 1"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("expected one Note event, got %+v", h.log)
	}
	if want := "unimplemented API NoSuchApiExists"; h.log[0].Text != want {
		t.Fatalf("Text = %q, want %q", h.log[0].Text, want)
	}
}

func TestDefinedResolvesEachForm(t *testing.T) {
	h := newHost(t, 4)
	ctx := &Ctx{Source: 7, Controller: 1,
		Targets:    []state.Target{{Player: 3, IsPlayer: true}},
		Remembered: []state.Target{{Obj: 9}}}

	check := func(line string, want []state.Target) {
		t.Helper()
		got := Defined(h, ctx, sa(t, line))
		if len(got) != len(want) {
			t.Fatalf("%s -> %v, want %v", line, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s -> %v, want %v", line, got, want)
			}
		}
	}
	check("SP$ X | Defined$ You", []state.Target{{Player: 1, IsPlayer: true}})
	check("SP$ X | Defined$ Self", []state.Target{{Obj: 7}})
	check("SP$ X | Defined$ Remembered", []state.Target{{Obj: 9}})
	check("SP$ X | Defined$ Targeted", []state.Target{{Player: 3, IsPlayer: true}})
	// No Defined$ at all falls back to the spell's chosen targets.
	check("SP$ X | NumDmg$ 1", []state.Target{{Player: 3, IsPlayer: true}})
	// Opponent expands to every other living seat, in APNAP order from the
	// controller, so the resulting event order is deterministic.
	check("SP$ X | Defined$ Opponent", []state.Target{
		{Player: 2, IsPlayer: true}, {Player: 3, IsPlayer: true}, {Player: 0, IsPlayer: true}})
}

func TestDefinedOpponentSkipsEliminatedSeats(t *testing.T) {
	h := newHost(t, 4)
	h.g.Players[2].Lost = true
	got := Defined(h, &Ctx{Controller: 1}, sa(t, "SP$ X | Defined$ Opponent"))
	if len(got) != 2 || got[0].Player != 3 || got[1].Player != 0 {
		t.Fatalf("Defined$ Opponent = %v", got)
	}
}

func TestSupportedListsRegisteredAPIs(t *testing.T) {
	Register("TestZ", func(Host, *Ctx, *cards.SA) {})
	t.Cleanup(func() { unregister("TestZ") })
	if !Supported()["api:TestZ"] {
		t.Fatal("Supported did not list a registered API")
	}
	if Supported()["api:NotRegistered"] {
		t.Fatal("Supported listed an API that was never registered")
	}
}
