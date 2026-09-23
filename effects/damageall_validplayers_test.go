package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// putOnBattlefield places a real corpus card for owner in owner's battlefield
// zone list (AddObject alone does not join the zone list the sweeps read).
func putOnBattlefield(t *testing.T, g *state.Game, reg *cards.Registry, name string, owner state.PlayerID) *state.Object {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	o := g.AddObject(c, owner)
	o.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
	return o
}

// svarSA resolves one of a real corpus face's SVars and fails loudly when the
// name is gone (a corpus re-pin must rewrite this test, not silently pass).
func svarSA(t *testing.T, f *cards.Face, name string) *cards.SA {
	t.Helper()
	sa := cards.ResolveSVar(f.SVars, name)
	if sa == nil {
		t.Fatalf("face %q has no SVar %q", f.Name, name)
	}
	return sa
}

// TestDamageAllValidPlayersOpponentIsRemembered closes Snort's exotic
// ValidPlayers$ spelling (brief rv2b-validplayers): the DamageAll player
// sweep reads the remember set -- the source object's persistent player list,
// falling back to the resolution's Remembered, exactly definedSpec's
// Player.IsRemembered two-tier read -- and narrows it to the spec's base
// constraint. Before the fix the dotted form fell through to the bare player
// filter, which binds no source and carries no remember fallback, so the
// whole player half matched NOBODY and Snort dealt no damage at all.
func TestDamageAllValidPlayersOpponentIsRemembered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	snort, ok := reg.Lookup("Snort")
	if !ok {
		t.Fatal("corpus has no Snort")
	}
	face := snort.Faces[0]
	// Precondition on the real script: the line still spells the exotic
	// selector this test closes.
	if line := face.SVars["DBDmg"]; !strings.Contains(line, "ValidPlayers$ Opponent.IsRemembered") {
		t.Fatalf("corpus moved: Snort's DBDmg no longer carries ValidPlayers$ Opponent.IsRemembered: %q", line)
	}
	dbDmg := svarSA(t, face, "DBDmg")

	h := newHost(t, 4)
	src := putOnBattlefield(t, h.g, reg, "Snort", 0)
	// The SP$ Discard's RememberDiscardingPlayers$ rider records the
	// discarding players in the RESOLUTION's Remembered (discardAndRemember),
	// one per seat that discarded its hand: opponents 1 and 3 here, seats 0
	// and 2 declined. The caster itself must stay out of the sweep even
	// though it is in the remembered pool's spelling's neighbourhood -- the
	// Opponent base excludes it.
	c := &Ctx{Source: src.ID, Controller: 0, SVars: face.SVars,
		Remembered: []state.Target{
			{Player: 1, IsPlayer: true}, {Player: 3, IsPlayer: true},
			{Player: 0, IsPlayer: true},
		}}
	if h.g.Players[1].Life != 20 || h.g.Players[3].Life != 20 || h.g.Players[0].Life != 20 {
		t.Fatalf("precondition: starting life is not the expected 20 (%d/%d/%d)",
			h.g.Players[0].Life, h.g.Players[1].Life, h.g.Players[3].Life)
	}
	Resolve(h, c, dbDmg)
	if h.g.Players[1].Life != 15 || h.g.Players[3].Life != 15 {
		t.Fatalf("remembered opponents 1 and 3 must take 5 damage each (life %d and %d), want 15/15",
			h.g.Players[1].Life, h.g.Players[3].Life)
	}
	if h.g.Players[0].Life != 20 {
		t.Fatalf("the caster (remembered, but not an Opponent) took damage: life %d, want 20", h.g.Players[0].Life)
	}
	if h.g.Players[2].Life != 20 {
		t.Fatalf("the unremembered opponent took damage: life %d, want 20", h.g.Players[2].Life)
	}
	for _, e := range h.log {
		// The chain's own DBCleanup legitimately Notes ("clears
		// remembered/imprinted objects"); only a census/unimplemented note
		// would mean the selector did not resolve.
		if e.Kind == events.Note && (strings.Contains(e.Text, "unresolved") || strings.Contains(e.Text, "unimplemented")) {
			t.Fatalf("no unresolved/unimplemented note may accompany a resolved selector, got %+v", e)
		}
	}
}

// TestDamageAllValidPlayersRememberedPersistentTierWins pins the other half
// of the two-tier read the dotted spelling reuses: when the source object's
// persistent player-remember list is non-empty it WINS over the resolution's
// Remembered (the definedSpec Player.IsRemembered convention -- Sower of
// Discord is why the persistent list wins there).
func TestDamageAllValidPlayersRememberedPersistentTierWins(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	snort, ok := reg.Lookup("Snort")
	if !ok {
		t.Fatal("corpus has no Snort")
	}
	face := snort.Faces[0]
	dbDmg := svarSA(t, face, "DBDmg")

	h := newHost(t, 3)
	src := putOnBattlefield(t, h.g, reg, "Snort", 0)
	// Persistent list names seat 2, the in-flight resolution remembered seat
	// 1: the persistent tier wins, so only seat 2 takes the sweep.
	src.Remembered = []state.Target{{Player: 2, IsPlayer: true}}
	c := &Ctx{Source: src.ID, Controller: 0, SVars: face.SVars,
		Remembered: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, dbDmg)
	if h.g.Players[2].Life != 15 {
		t.Fatalf("persistent tier: seat 2 must take the 5 damage, life %d want 15", h.g.Players[2].Life)
	}
	if h.g.Players[1].Life != 20 {
		t.Fatalf("persistent tier wins: resolution-remembered seat 1 must take nothing, life %d want 20", h.g.Players[1].Life)
	}
}

// TestDamageAllValidPlayersUnmodelledIsFailClosedAndLoud pins The Fallen's
// exotic compound: `Player.Opponent+wasDealtDamageThisGameBy Self` needs a
// game-long damage-by-source history the Damage event does not carry (no
// source field on the event, no game-long record in state), so the selector
// is genuinely unmodelled. The contract: damage NOBODY (fail closed) and emit
// a Note naming the selector (loud) -- before the fix the compound matched
// nobody SILENTLY, which is the silent no-op this ticket closes.
func TestDamageAllValidPlayersUnmodelledIsFailClosedAndLoud(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	fallen, ok := reg.Lookup("The Fallen")
	if !ok {
		t.Fatal("corpus has no The Fallen")
	}
	face := fallen.Faces[0]
	if line := face.SVars["TrigDamage"]; !strings.Contains(line, "Player.Opponent+wasDealtDamageThisGameBy Self") {
		t.Fatalf("corpus moved: The Fallen's TrigDamage no longer carries the exotic compound: %q", line)
	}
	trig := svarSA(t, face, "TrigDamage")

	h := newHost(t, 3)
	src := putOnBattlefield(t, h.g, reg, "The Fallen", 0)
	c := &Ctx{Source: src.ID, Controller: 0, SVars: face.SVars}
	Resolve(h, c, trig)

	// Fail closed: no player lost a point of life.
	for i, p := range h.g.Players {
		if p.Life != 20 {
			t.Errorf("seat %d lost life (%d) under an unmodelled ValidPlayers$ selector -- it must fail closed", i, p.Life)
		}
	}
	// Loud: exactly one Note naming the unresolved selector, and NOT an
	// "unimplemented API" note -- DamageAll itself is registered; the gap is
	// the selector grammar.
	var notes []events.Event
	for _, e := range h.log {
		if e.Kind == events.Note {
			notes = append(notes, e)
		}
	}
	if len(notes) != 1 {
		t.Fatalf("want exactly one Note naming the unresolved selector, got %d notes (%v)", len(notes), notes)
	}
	if !strings.Contains(notes[0].Text, "wasDealtDamageThisGameBy Self") {
		t.Fatalf("the Note must name the unresolved selector, got %q", notes[0].Text)
	}
	if strings.Contains(notes[0].Text, "unimplemented API") {
		t.Fatalf("the Note must be the selector census, not the unimplemented-API fallback: %q", notes[0].Text)
	}
}

// TestDamageAllValidPlayersKnownBasesStillSweep guards the census arm against
// over-firing: a KNOWN base with an exotic QUALIFIER (Disorder's real
// `Player.controlsCreature.White_GE1`) must keep sweeping through the shared
// player filter -- damage to the seat that controls a white creature, none to
// the seat that does not, no census Note, and the ValidCards$ white-creature
// half untouched.
func TestDamageAllValidPlayersKnownBasesStillSweep(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	disorder, ok := reg.Lookup("Disorder")
	if !ok {
		t.Fatal("corpus has no Disorder")
	}
	face := disorder.Faces[0]
	if got := face.Abilities[0].Params["ValidPlayers"]; got != "Player.controlsCreature.White_GE1" {
		t.Fatalf("corpus moved: Disorder's ValidPlayers$ is %q, want Player.controlsCreature.White_GE1", got)
	}

	h := newHost(t, 2)
	putOnBattlefield(t, h.g, reg, "Savannah Lions", 0) // white creature: its controller qualifies
	putOnBattlefield(t, h.g, reg, "Grizzly Bears", 1)  // green creature: seat 1 does not qualify
	c := &Ctx{Source: putOnBattlefield(t, h.g, reg, "Disorder", 0).ID, Controller: 0}
	Resolve(h, c, face.Abilities[0])

	if h.g.Players[0].Life != 18 {
		t.Fatalf("seat 0 controls a white creature and must take Disorder's 2 damage, life %d want 18", h.g.Players[0].Life)
	}
	if h.g.Players[1].Life != 20 {
		t.Fatalf("seat 1 controls no white creature and must take nothing, life %d want 20", h.g.Players[1].Life)
	}
	if dmg := h.g.Obj(h.g.Zone(state.ZBattlefield, 0)[0]).Damage; dmg != 2 {
		t.Fatalf("the ValidCards$ white-creature half must still mark 2 on the lion, got %d", dmg)
	}
	if dmg := h.g.Obj(h.g.Zone(state.ZBattlefield, 1)[0]).Damage; dmg != 0 {
		t.Fatalf("the green bear must be unmarked, got %d", dmg)
	}
	for _, e := range h.log {
		if e.Kind == events.Note {
			t.Fatalf("a known-base qualified selector must not trip the census Note, got %+v", e)
		}
	}
}
