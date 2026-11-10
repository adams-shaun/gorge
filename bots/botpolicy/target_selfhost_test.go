package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestUpToRemovalSkipsItsOwnHost: an "up to one per player" exile until its
// host leaves (Vault 13: Dweller's Journey's chapter I) offers the host
// itself among its controller's options. Topping the answer up with it
// exiles the host, which returns at once with everything it exiled and
// re-triggers -- a Commander pod looped that way to the object cap. The host
// is left out past Min; another own permanent still tops up as before.
func TestUpToRemovalSkipsItsOwnHost(t *testing.T) {
	t.Parallel()
	mk := func() *decision.Decision {
		return &decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 0, Max: 3,
			TargetEffect: &decision.TargetEffect{API: "ChangeZone",
				Removal: &decision.RemovalEffect{Kind: "exile", Destination: "Exile"}, Host: 108},
			Options: []decision.Option{
				{Index: 0, Kind: "permanent", Obj: 108, Player: 0, Group: "target-controller-0"},
				{Index: 1, Kind: "permanent", Obj: 200, Player: 1, Group: "target-controller-1"},
				{Index: 2, Kind: "permanent", Obj: 300, Player: 2, Group: "target-controller-2"},
			}}
	}
	got := Decide(Board{}, mk(), rng(1)).Choices
	for _, c := range got {
		if c == 0 {
			t.Fatalf("up-to removal choices = %v: the ability's own host was exiled", got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("up-to removal choices = %v, want both opposing permanents", got)
	}
	// Min 1 with only the host offered: the floor still answers.
	d := mk()
	d.Min, d.Max, d.Options = 1, 1, d.Options[:1]
	if got := Decide(Board{}, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("a forced pick of the host = %v, want [0]", got)
	}
}
