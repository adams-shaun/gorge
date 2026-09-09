package view

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// manaChars wraps flatChars and lets a test drive a chosen AvailableMana per
// player, so the projection test can assert the view wires the engine's
// answer through without also re-implementing the engine's folding.
type manaChars struct {
	flatChars
	avail func(state.PlayerID) state.Mana
}

func (c manaChars) AvailableMana(p state.PlayerID) state.Mana { return c.avail(p) }

// boardWithLands builds a two-seat game where player 0 controls an untapped
// Plains (its intrinsic tap-for-white) and player 1 controls nothing.
func boardWithLands(t *testing.T) *state.Game {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob"})
	plains, err := cards.ParseBytes("plains.txt",
		[]byte("Name:Plains\nManaCost:no cost\nTypes:Basic Land Plains\nOracle:x\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	plains.Link()
	for _, f := range plains.Faces {
		f.ApplyIntrinsics()
	}
	oid := g.AddObject(plains, 0).ID
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{oid})
	return g
}

// TestAvailableManaProjectedForEverySeat pins the public nature of the new
// field: available mana is derived from the battlefield, so it is projected
// for EVERY seat and under EVERY visibility (seat, public, omniscient) —
// never gated the way Hand/Pool are on "is this the viewer's own seat".
func TestAvailableManaProjectedForEverySeat(t *testing.T) {
	g := boardWithLands(t)
	ch := manaChars{flatChars{g}, func(p state.PlayerID) state.Mana {
		if p == 0 {
			return state.Mana{state.MW: 1}
		}
		return state.Mana{}
	}}

	for _, v := range []Visibility{Seat, Public, Omniscient} {
		view := ProjectFor(g, ch, 0, v, nil)
		// Player 0's own seat.
		if got := view.Players[0].Available["W"]; got != 1 {
			t.Errorf("%s: player 0 available[W] = %d, want 1", v, got)
		}
		// Player 1 may be the viewer's own seat or not; available is public
		// either way and here is empty, but must be present (not null).
		if up := view.Players[1].Available; up == nil {
			t.Errorf("%s: player 1 available is null; available is public and must be a non-nil map", v)
		} else if len(up) != 0 {
			t.Errorf("%s: player 1 available = %v, want empty map", v, up)
		}
	}
}

// TestAvailableManaNeverMarshalsNull pins both wire shapes against the
// Java-script-null that crashed the table view for every public spectator
// (71a03cc). Available is a public quantity and never null: it is absent when
// nothing is available (omitempty) and an object when something is. The
// floating pool is public too now (CR 106.4a/106.4b) and never null: it is
// always a non-nil object, "{}" when empty, because it carries no omitempty.
// The two are therefore still unambiguously different fields: available is
// absent-when-zero, pool is present-but-empty-when-zero.
func TestAvailableManaNeverMarshalsNull(t *testing.T) {
	g := boardWithLands(t)
	// Empty availability: the key must be omitted, never "null".
	ch := manaChars{flatChars{g}, func(state.PlayerID) state.Mana { return state.Mana{} }}
	v := ProjectFor(g, ch, 0, Public, nil)
	blob, err := json.Marshal(v.Players[0])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(blob, &m); err != nil {
		t.Fatal(err)
	}
	if ra, ok := m["available"]; ok && string(ra) == "null" {
		t.Errorf("empty availability marshalled as null: %s", blob)
	}
	// The floating pool is a DIFFERENT field and is public: player 0 floats
	// nothing in this fixture, so it marshals a present-but-empty object.
	if rp, ok := m["pool"]; !ok || string(rp) != `{}` {
		t.Errorf("pool = %s, want {} for a public, empty pool (not null)", rp)
	}

	// Non-empty availability: the key is present as an object, not null.
	ch2 := manaChars{flatChars{g}, func(state.PlayerID) state.Mana { return state.Mana{state.MW: 1} }}
	v2 := ProjectFor(g, ch2, 0, Public, nil)
	blob2, err := json.Marshal(v2.Players[0])
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]json.RawMessage
	if err := json.Unmarshal(blob2, &m2); err != nil {
		t.Fatal(err)
	}
	if ra, ok := m2["available"]; !ok || string(ra) != `{"W":1}` {
		t.Errorf("non-empty available marshalled %s, want {\"W\":1}", ra)
	}
}

// TestAvailableManaVisibleAlongsidePublicPool is the ordinary public-client
// case: a spectator reads every seat's available mana, and the pool on the
// same PlayerView is public too (CR 106.4a/106.4b) so it is present rather
// than hidden. Player 0 in this fixture floats nothing, so the pool is the
// present-but-empty object; the available readout is what carries the color.
func TestAvailableManaVisibleAlongsidePublicPool(t *testing.T) {
	g := boardWithLands(t)
	ch := manaChars{flatChars{g}, func(p state.PlayerID) state.Mana {
		return state.Mana{state.MW: 1, state.MB: 2}
	}}
	v := ProjectFor(g, ch, NoSeat, Public, nil)
	vp := v.Players[0]
	if vp.Available["W"] != 1 || vp.Available["B"] != 2 {
		t.Errorf("public available = %v, want W:1 B:2", vp.Available)
	}
	if vp.Pool == nil {
		t.Errorf("public pool is null; the pool is public (CR 106.4a/106.4b) and must be a present-but-empty object")
	} else if len(vp.Pool) != 0 {
		t.Errorf("public pool = %v, want the empty pool this fixture floats", vp.Pool)
	}
}
