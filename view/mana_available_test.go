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

// TestAvailableManaNeverMarshalsNull pins the new field's shape on the wire:
// Pool can be a literal JSON null (a hidden zone), and that exact mismatch
// (protocol.ts types it non-nullable while the server sends null) crashed the
// table view for every public spectator (71a03cc). Available is a public
// quantity and never null: it is absent when nothing is available (omitempty)
// and an object when something is, never the Java-script-null a client would
// have to special-case. The floating pool on the same public PlayerView stays
// null — so available and pool are unambiguously different fields.
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
	// The floating pool is a DIFFERENT field and DOES go null on a public
	// view (player 0 is not the viewer).
	if rp, ok := m["pool"]; !ok || string(rp) != "null" {
		t.Errorf("pool = %s, want null for a hidden zone on a public view", rp)
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

// TestAvailableManaVisibleEvenWhenPoolHidden is the ordinary public-client
// case the feature exists to serve: a spectator has a null pool for every
// seat, so the seat box line 3 would be blank — but available mana is public
// and so still renders. This is what makes the line population the feature,
// not the empty floating pool.
func TestAvailableManaVisibleEvenWhenPoolHidden(t *testing.T) {
	g := boardWithLands(t)
	ch := manaChars{flatChars{g}, func(p state.PlayerID) state.Mana {
		return state.Mana{state.MW: 1, state.MB: 2}
	}}
	v := ProjectFor(g, ch, NoSeat, Public, nil)
	vp := v.Players[0]
	if vp.Available["W"] != 1 || vp.Available["B"] != 2 {
		t.Errorf("public available = %v, want W:1 B:2", vp.Available)
	}
	if vp.Pool != nil {
		t.Errorf("public pool = %v, want nil (hidden)", vp.Pool)
	}
}
