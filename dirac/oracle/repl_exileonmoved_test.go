package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect's "exile that spell instead of putting it into your graveyard as
// it leaves the stack" (ExileOnMoved$ Stack beside an Event$ Moved | Origin$
// Stack | Destination$ Graveyard replacement), played decision by decision
// against gorge: Lilah, Undefeated Slickshot (a multicolored instant cast
// from hand, exiled and plotted, resolved or countered, and cast again from
// its plot) and Nita, Forum Conciliator (an opponent's Lightning Bolt exiled
// from their graveyard and cast this turn, exiled instead of going back).
// TestStdSetReprosLockstep replays the repros; TestExileOnMovedReprosReach
// requires each recorded game to reach the card.
//
// The games are recorded by TestExileOnMovedRecord (EXILE_ON_MOVED_RECORD=1)
// on the kernel table built from the decks: the seats prefer the case's
// cards, hold the gated spell back until its enabler is out, and otherwise
// pick at random from the seed's stream.
type exileMovedCase struct {
	effectLongCase
	gated    string // cast only once needOut is on the battlefield
	needOut  string
	activate string // an activated ability to prefer
}

func exileMovedCases() []exileMovedCase {
	lilahDeck := func() []string {
		return append(append(append(deckOf(4, "Lilah, Undefeated Slickshot"), deckOf(6, "Lightning Helix")...),
			deckOf(4, "Island")...), append(deckOf(5, "Mountain"), deckOf(5, "Plains")...)...)
	}
	nitaDeck := func() []string {
		return append(append(deckOf(4, "Nita, Forum Conciliator"), deckOf(6, "Walking Corpse")...),
			append(deckOf(7, "Plains"), deckOf(7, "Swamp")...)...)
	}
	return []exileMovedCase{
		{
			effectLongCase: effectLongCase{
				name:    "lilah_resolved_instant_is_exiled_and_plotted",
				prefer:  []string{"Lilah, Undefeated Slickshot", "Lightning Helix"},
				decks:   [2][]string{lilahDeck(), append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...)},
				surface: "autopay",
				reached: stackLeft("Lightning Helix", state.ZExile, leaveResolved),
			},
			gated: "Lightning Helix", needOut: "Lilah, Undefeated Slickshot",
		},
		{
			effectLongCase: effectLongCase{
				name:    "lilah_countered_instant_goes_to_the_graveyard",
				prefer:  []string{"Lilah, Undefeated Slickshot", "Lightning Helix", "Counterspell"},
				decks:   [2][]string{lilahDeck(), append(deckOf(8, "Counterspell"), deckOf(16, "Island")...)},
				surface: "autopay",
				reached: func(g *GorgeSide) bool {
					// Fizzle$ False: only the resolution move is replaced, so a counter
					// leaves Lilah's spell in the graveyard, unplotted
					return stackLeft("Lightning Helix", state.ZGraveyard, leaveUnresolved)(g) && stackLeft("Counterspell", state.ZGraveyard, leaveResolved)(g)
				},
			},
			gated: "Lightning Helix", needOut: "Lilah, Undefeated Slickshot",
		},
		{
			effectLongCase: effectLongCase{
				name:    "lilah_plotted_instant_cast_again_goes_to_the_graveyard",
				prefer:  []string{"Lilah, Undefeated Slickshot", "Lightning Helix"},
				decks:   [2][]string{lilahDeck(), append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...)},
				surface: "autopay",
				reached: recastExiled("Lightning Helix", state.ZStack, state.ZGraveyard, leaveResolved),
			},
			gated: "Lightning Helix", needOut: "Lilah, Undefeated Slickshot",
		},
		{
			effectLongCase: effectLongCase{
				name:    "nita_exiled_bolt_cast_this_turn_is_exiled_again",
				prefer:  []string{"Nita, Forum Conciliator", "Walking Corpse", "Lightning Bolt"},
				decks:   [2][]string{nitaDeck(), append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...)},
				surface: "autopay",
				reached: recastExiled("Lightning Bolt", state.ZGraveyard, state.ZExile, leaveResolved),
			},
			activate: "Nita, Forum Conciliator",
		},
		{
			effectLongCase: effectLongCase{
				name:   "nita_exiled_bolt_countered_is_exiled_again",
				prefer: []string{"Nita, Forum Conciliator", "Walking Corpse", "Lightning Bolt", "Counterspell"},
				decks: [2][]string{nitaDeck(), append(append(deckOf(6, "Lightning Bolt"), deckOf(5, "Counterspell")...),
					append(deckOf(6, "Island"), deckOf(7, "Mountain")...)...)},
				surface: "autopay",
				reached: func(g *GorgeSide) bool {
					return recastExiled("Lightning Bolt", state.ZGraveyard, state.ZExile, leaveUnresolved)(g) &&
						stackLeft("Counterspell", state.ZGraveyard, leaveResolved)(g)
				},
			},
			activate: "Nita, Forum Conciliator",
		},
	}
}

// leaveCause says how a spell's stack exit is wanted: any, as it resolved, or
// without resolving (countered, or fizzled: no Resolve event for the object).
type leaveCause int

const (
	leaveAny leaveCause = iota
	leaveResolved
	leaveUnresolved
)

func (c leaveCause) admits(resolved bool) bool {
	return c == leaveAny || c == leaveResolved && resolved || c == leaveUnresolved && !resolved
}

// stackLeft: a card named name left the stack for zone to, the way cause says.
func stackLeft(name string, to state.Zone, cause leaveCause) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		resolved := map[state.ObjID]bool{}
		for _, e := range g.E.L.Events {
			switch {
			case e.Kind == events.Resolve:
				resolved[e.Obj] = true
			case e.Kind == events.PutOnStack:
				resolved[e.Obj] = false
			case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == to:
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name && cause.admits(resolved[e.Obj]) {
					return true
				}
			}
		}
		return false
	}
}

// recastExiled: an object named name went from zone first into exile, from
// exile onto the stack, and from the stack on to zone to, the way cause says.
func recastExiled(name string, first, to state.Zone, cause leaveCause) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		stage := map[state.ObjID]int{}
		resolved := map[state.ObjID]bool{}
		for _, e := range g.E.L.Events {
			if e.Kind == events.Resolve {
				resolved[e.Obj] = true
				continue
			}
			if e.Kind != events.MoveZone && e.Kind != events.PutOnStack {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Face() == nil || o.Face().Name != name {
				continue
			}
			switch {
			case stage[e.Obj] == 0 && e.Kind == events.MoveZone && e.From == first && e.To == state.ZExile:
				stage[e.Obj] = 1
			case stage[e.Obj] == 1 && e.Kind == events.PutOnStack && e.From == state.ZExile:
				stage[e.Obj] = 2
				resolved[e.Obj] = false
			case stage[e.Obj] == 2 && e.Kind == events.MoveZone && e.From == state.ZStack && e.To == to && cause.admits(resolved[e.Obj]):
				return true
			}
		}
		return false
	}
}

func exileMovedPolicy(m *Matchup, c exileMovedCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, want := range c.prefer {
				for _, o := range p.D.Options {
					if o.Kind() != ActCast || m.InstanceCard(int(o.Src())) != want {
						continue
					}
					if want == c.gated && !exileEnablerOut(p.G, c.needOut) {
						continue
					}
					return o
				}
			}
			if c.activate != "" {
				for _, o := range p.D.Options {
					if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == c.activate {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// cardOut: a card named name is on the battlefield (false when the kernel
// leads and gorge's state is not at hand).
func exileEnablerOut(g *GorgeSide, name string) bool {
	if g == nil {
		return false
	}
	for p := range g.E.G.Players {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
	}
	return false
}

func TestExileOnMovedReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range exileMovedCases() {
		t.Run(c.name, func(t *testing.T) {
			exileMovedReplay(t, reg, c)
		})
	}
}

func exileMovedReplay(t *testing.T, reg *cards.Registry, c exileMovedCase) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

func TestExileOnMovedRecord(t *testing.T) {
	if os.Getenv("EXILE_ON_MOVED_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range exileMovedCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, exileMovedPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
