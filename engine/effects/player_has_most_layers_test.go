package effects

// Player.withMostTypeCreature counts the layer-4 (CR 613.1d, 205.1) card-type
// set: a permanent that is a Creature only through a continuous effect (a
// solved Case) counts. A published derived entry is authoritative, so these
// tests build a tie on printed types and break it with one LayerTables entry.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

type withMostBoard struct {
	h      *fakeHost
	caseID state.ObjID
	layers LayerTables
}

// newWithMostBoard seats two printed creatures on each side plus, for seat 0,
// a Case that the returned layers turn into a Creature.
func newWithMostBoard(t *testing.T) withMostBoard {
	t.Helper()
	h := newHost(t, 2)
	g := h.g
	var ids [2][]state.ObjID
	for seat := 0; seat < 2; seat++ {
		for i := 0; i < 2; i++ {
			o := g.AddObject(mkCard(t, "Name:Test Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), state.PlayerID(seat))
			ids[seat] = append(ids[seat], o.ID)
		}
	}
	cs := g.AddObject(mkCard(t, "Name:Test Case\nTypes:Enchantment Case\nOracle:x\n"), 0)
	ids[0] = append(ids[0], cs.ID)
	for seat := 0; seat < 2; seat++ {
		for _, id := range ids[seat] {
			o := g.Obj(id)
			if o == nil {
				t.Fatalf("fixture object %d is missing", id)
			}
			o.Zone = state.ZBattlefield
		}
		g.SetZone(state.ZBattlefield, state.PlayerID(seat), ids[seat])
	}
	return withMostBoard{
		h: h, caseID: cs.ID,
		layers: LayerTables{DerivedTypes: []ObjectTypes{{ID: cs.ID, Types: []string{"Enchantment", "Case", "Creature", "Gorgon"}}}},
	}
}

func seatsMatching(b withMostBoard, spec string, pc PlayerSpecCtx) []state.PlayerID {
	var out []state.PlayerID
	for p := range b.h.g.Players {
		if MatchesPlayerSpecCtx(b.h.g, spec, state.PlayerID(p), 0, pc) {
			out = append(out, state.PlayerID(p))
		}
	}
	return out
}

func onlySeat0(got []state.PlayerID) bool { return len(got) == 1 && got[0] == 0 }

func TestPlayerWithMostTypeCreatureCountsLayeredType(t *testing.T) {
	b := newWithMostBoard(t)
	// Precondition: printed types tie 2-2, so both seats have the most.
	if got := seatsMatching(b, "Player.withMostTypeCreature", PlayerSpecCtx{}); len(got) != 2 {
		t.Fatalf("no layers: matches = %v; want the 2-2 tie (both seats)", got)
	}
	if got := seatsMatching(b, "Player.withMostTypeCreature", PlayerSpecCtx{Layers: b.layers}); !onlySeat0(got) {
		t.Fatalf("layers: matches = %v; want seat 0 only (3 creatures to 2)", got)
	}
	if got := seatsMatching(b, "Player.withMostTypeCreatureOnly", PlayerSpecCtx{}); len(got) != 0 {
		t.Fatalf("Only, no layers: matches = %v; want none (tie)", got)
	}
	if got := seatsMatching(b, "Player.withMostTypeCreatureOnly", PlayerSpecCtx{Layers: b.layers}); !onlySeat0(got) {
		t.Fatalf("Only, layers: matches = %v; want seat 0 only", got)
	}
}

// TestDefinedWithMostTypeCreatureResolutionReadsLayers is No Witnesses'
// resolution path: definedSpec -> MatchesPlayerSpecWithSVars.
func TestDefinedWithMostTypeCreatureResolutionReadsLayers(t *testing.T) {
	b := newWithMostBoard(t)
	if ts, ok := definedSpec(b.h, &Ctx{Controller: 1}, "Player.withMostTypeCreature"); !ok || len(ts) != 2 {
		t.Fatalf("no layers: definedSpec = %v, %v; want the 2-2 tie (both seats)", ts, ok)
	}
	ts, ok := definedSpec(b.h, &Ctx{Controller: 1, Layers: b.layers}, "Player.withMostTypeCreature")
	if !ok || len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 0 {
		t.Fatalf("layers: definedSpec = %v, %v; want exactly seat 0", ts, ok)
	}
}

// TestControlReferentPlayerSpecReadsLayers drives the object filter's
// ControlledBy Player.<qualifier> arm (controlReferentPlayers' default case)
// with Player.controlsCreature.Creature.Gorgon, which the control-referent
// grammar accepts (withMostType* is not in that arm's known set). The only
// Gorgon is the Case, a Gorgon creature through a continuous effect.
func TestControlReferentPlayerSpecReadsLayers(t *testing.T) {
	b := newWithMostBoard(t)
	g := b.h.g
	const ref = "Player.controlsCreature.Gorgon"
	if got, ok := controlReferentPlayers(g, SpecContext{You: 1}, "ControlledBy", ref); ok || len(got) != 0 {
		t.Fatalf("no layers: referent = %v, %v; want nobody (the Case is a printed Enchantment)", got, ok)
	}
	got, ok := controlReferentPlayers(g, SpecContext{You: 1, Layers: b.layers}, "ControlledBy", ref)
	if !ok || !onlySeat0(got) {
		t.Fatalf("layers: referent = %v, %v; want seat 0 only", got, ok)
	}
	// The same arm through the public object filter: a seat-0 bear is
	// controlled by the player who controls a Gorgon creature only with layers.
	var bear *state.Object
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		if id != b.caseID {
			bear = g.Obj(id)
			break
		}
	}
	if bear == nil {
		t.Fatal("fixture: no seat-0 bear")
	}
	const spec = "Creature.ControlledBy " + ref
	if MatchesObjectCtx(g, spec, bear, SpecContext{You: 1}) {
		t.Fatalf("no layers: %q must not match a seat-0 bear", spec)
	}
	if !MatchesObjectCtx(g, spec, bear, SpecContext{You: 1, Layers: b.layers}) {
		t.Fatalf("layers: %q must match a seat-0 bear", spec)
	}
}

func TestSpellIsTargetingPlayerSpecReadsLayers(t *testing.T) {
	b := newWithMostBoard(t)
	g := b.h.g
	spell := g.AddObject(mkCard(t, "Name:Test Bolt\nTypes:Instant\nOracle:x\n"), 1)
	const spec = "Player.withMostTypeCreature"
	// Target seat 1 instead: it ties without layers and loses with them.
	spell.Targets = []state.Target{{IsPlayer: true, Player: 1}}
	if !spellIsTargetingMatchesPtr(g, spec, spell, &SpecContext{You: 0}) {
		t.Fatalf("no layers: seat 1 should tie for the most creatures")
	}
	if spellIsTargetingMatchesPtr(g, spec, spell, &SpecContext{You: 0, Layers: b.layers}) {
		t.Fatalf("layers: seat 1 (2 creatures to 3) must not match withMostTypeCreature")
	}
	spell.Targets = []state.Target{{IsPlayer: true, Player: 0}}
	if !spellIsTargetingMatchesPtr(g, spec, spell, &SpecContext{You: 0, Layers: b.layers}) {
		t.Fatalf("layers: seat 0 (3 creatures to 2) must match withMostTypeCreature")
	}
}
