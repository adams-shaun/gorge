package view

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07.
// CR 106.4a/106.4b: a player announces what mana is in their pool, which is
// public information (see view/cr106_pool_conformance_test.go for the full
// citation block). A spend restriction on that mana is derived from a public
// battlefield permanent's own ability, so it is public in exactly the same
// way -- the seat needs it to understand why a floating symbol was refused.
// CR 605: a mana ability's restriction ("spend this mana only to ...") binds
// the mana, not the pool display.
//
// This file pins the projection added for the Cavern of Souls feedback report
// (fb-20260922T145544Z): the engine correctly refuses to offer a cast against
// a {B} restricted to the chosen creature type, but the seat's view showed a
// bare chip with no trace of the restriction.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// restrictionBoard builds a 2-seat game whose seat 0 has a Cavern-of-Souls
// stand-in on the battlefield with a recorded ChosenType, one restricted {B}
// batch naming it, and one unrestricted {B} batch. The unrestricted batch is
// the control: it must NOT produce an annotation (its Valid is empty), which
// is what makes the "one entry per restricted batch" assertion meaningful
// rather than "one entry per pool entry".
func restrictionBoard(t *testing.T) (*state.Game, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob"})
	c, err := cards.ParseBytes("cavern.txt", []byte("Name:Cavern of Souls\nTypes:Land\nOracle:x\n"))
	if err != nil {
		t.Fatalf("parse card: %v", err)
	}
	c.Link()
	o := g.AddObject(c, 0)
	o.Zone = state.ZBattlefield
	o.ChosenType = "Demon"
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})

	// The pool holds one restricted {B} (Cavern's own) and one bare {B}
	// (another source), so the projection must distinguish them.
	g.Players[0].Pool[state.MB] = 2
	g.Players[0].RestrictedMana = []state.ManaRestriction{
		{Color: "B", Amount: 1, Valid: "Spell.Creature+ChosenType", NoCounter: "True", Source: o.ID},
		{Color: "B", Amount: 1},
	}
	return g, o.ID
}

// TestRestrictedManaIsProjected pins the projection of a restricted batch
// onto the wire. Each sub-case asserts its own precondition first: the batch
// really sits in state (so a projection that reads a different field cannot
// pass), and the produced text really differs from the raw-Valid fallback
// (so "the field is present but still raw" cannot pass).
func TestRestrictedManaIsProjected(t *testing.T) {
	g, src := restrictionBoard(t)

	// Precondition (1): the restricted batch is really in state, with the
	// exact shape this snapshot recorded, and the source carries Demon.
	if len(g.Players[0].RestrictedMana) != 2 {
		t.Fatalf("precondition: seat 0 holds %d restricted batches, want 2", len(g.Players[0].RestrictedMana))
	}
	if got := g.Players[0].RestrictedMana[0].Valid; got != "Spell.Creature+ChosenType" {
		t.Fatalf("precondition: batch[0].Valid = %q, want Spell.Creature+ChosenType", got)
	}
	if o := g.Obj(src); o == nil || o.ChosenType != "Demon" {
		t.Fatalf("precondition: source %d has no ChosenType Demon", src)
	}

	v := Project(g, flatChars{g}, 0, nil)
	var pv PlayerView
	for _, p := range v.Players {
		if p.ID == 0 {
			pv = p
		}
	}
	if len(pv.PoolRestrictions) != 1 {
		t.Fatalf("seat 0 projects %d pool restrictions, want 1 (the bare {B} carries no limit): %+v", len(pv.PoolRestrictions), pv.PoolRestrictions)
	}
	got := pv.PoolRestrictions[0]

	// Precondition (2): the text is NOT the raw Valid$ string, so a
	// formatter that silently fell through to the raw fallback fails here.
	if got.Text == "Spell.Creature+ChosenType" {
		t.Fatalf("precondition: the projection fell through to the raw Valid$ string instead of humanising it")
	}
	want := "spend only to cast a Demon creature spell"
	if got.Text != want {
		t.Fatalf("restricted {B} text = %q, want %q", got.Text, want)
	}
	if got.Color != "B" || got.Amount != 1 {
		t.Fatalf("restricted batch = %+v, want color B amount 1", got)
	}

	// The plain Spell.Creature shape must read naturally too, and the
	// source-relative resolution must not leak a chosen type into it.
	g2 := state.NewGame([]string{"alice", "bob"})
	g2.Players[0].Pool[state.MB] = 1
	g2.Players[0].RestrictedMana = []state.ManaRestriction{{Color: "B", Amount: 1, Valid: "Spell.Creature"}}
	v2 := Project(g2, flatChars{g2}, 0, nil)
	var pv2 PlayerView
	for _, p := range v2.Players {
		if p.ID == 0 {
			pv2 = p
		}
	}
	if len(pv2.PoolRestrictions) != 1 {
		t.Fatalf("seat 0 projects %d pool restrictions, want 1", len(pv2.PoolRestrictions))
	}
	if want := "spend only to cast a creature spell"; pv2.PoolRestrictions[0].Text != want {
		t.Fatalf("Spell.Creature text = %q, want %q", pv2.PoolRestrictions[0].Text, want)
	}
}

// TestPoolRestrictionsOmittedWhenEmpty pins that a seat with no restricted
// mana omits the field entirely (the Available omitempty convention), so
// every existing view serialises byte-identically.
func TestPoolRestrictionsOmittedWhenEmpty(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	g.Players[0].Pool[state.MR] = 1
	v := Project(g, flatChars{g}, 0, nil)
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(blob); strings.Contains(got, `"pool_restrictions"`) {
		t.Fatalf("a seat with no restricted mana still serialises a pool_restrictions key: %s", got)
	}
	for _, pv := range v.Players {
		if pv.PoolRestrictions != nil {
			t.Fatalf("seat %d carries a non-nil PoolRestrictions with no restricted mana: %+v", pv.ID, pv.PoolRestrictions)
		}
	}
}

// TestRestrictedManaExoticValidFallsBackRaw pins the honest floor: a Valid$
// shape the formatter does not model is projected as its raw string rather
// than guessed at into prose, and a comma-separated all-common shape reads as
// an "or" list.
func TestRestrictedManaExoticValidFallsBackRaw(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	g.Players[0].Pool[state.MB] = 1
	g.Players[0].RestrictedMana = []state.ManaRestriction{
		{Color: "B", Amount: 1, Valid: "Spell.MultiColor"},
	}
	v := Project(g, flatChars{g}, 0, nil)
	var pv PlayerView
	for _, p := range v.Players {
		if p.ID == 0 {
			pv = p
		}
	}
	if len(pv.PoolRestrictions) != 1 {
		t.Fatalf("projects %d restrictions, want 1", len(pv.PoolRestrictions))
	}
	if pv.PoolRestrictions[0].Text != "Spell.MultiColor" {
		t.Fatalf("exotic Valid$ text = %q, want the raw fallback", pv.PoolRestrictions[0].Text)
	}

	// Two common alternatives join with "or" (comma is OR semantics).
	g2 := state.NewGame([]string{"alice", "bob"})
	g2.Players[0].Pool[state.MB] = 1
	g2.Players[0].RestrictedMana = []state.ManaRestriction{
		{Color: "B", Amount: 1, Valid: "Spell.Instant,Spell.Sorcery"},
	}
	v2 := Project(g2, flatChars{g2}, 0, nil)
	var pv2 PlayerView
	for _, p := range v2.Players {
		if p.ID == 0 {
			pv2 = p
		}
	}
	if want := "spend only to cast an instant spell or cast a sorcery spell"; pv2.PoolRestrictions[0].Text != want {
		t.Fatalf("two-alternative text = %q, want %q", pv2.PoolRestrictions[0].Text, want)
	}
}

// TestPoolRestrictionIsPublicForEveryViewer pins that, like Pool, the
// restriction is projected for every seat under every visibility: it is
// derived from a public battlefield permanent's ability, and the player who
// needs it most may be an opponent reasoning about a seat's mana.
func TestPoolRestrictionIsPublicForEveryViewer(t *testing.T) {
	g, _ := restrictionBoard(t)
	ch := flatChars{g}
	for _, vis := range []Visibility{Seat, Public, Omniscient} {
		for viewer := state.PlayerID(0); viewer < 2; viewer++ {
			v := ProjectFor(g, ch, viewer, vis, nil)
			var seat0 *PlayerView
			for i := range v.Players {
				if v.Players[i].ID == 0 {
					seat0 = &v.Players[i]
				}
			}
			if seat0 == nil {
				t.Fatalf("visibility %s viewer %d: seat 0 missing", vis, viewer)
			}
			if len(seat0.PoolRestrictions) != 1 {
				t.Fatalf("visibility %s viewer %d: seat 0 projects %d restrictions, want 1", vis, viewer, len(seat0.PoolRestrictions))
			}
			if want := "spend only to cast a Demon creature spell"; seat0.PoolRestrictions[0].Text != want {
				t.Fatalf("visibility %s viewer %d: seat 0 text = %q, want %q", vis, viewer, seat0.PoolRestrictions[0].Text, want)
			}
		}
	}
}
