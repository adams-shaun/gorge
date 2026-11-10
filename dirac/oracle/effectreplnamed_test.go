package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Effect-carried replacements over a player's side of the board, played
// decision by decision against gorge (kop.ExtTurnRepl): Malicious Eclipse's
// "if a creature an opponent controls would die this turn, exile it instead"
// (which also catches an opposing creature that enters after it resolved) and
// Prairie Dog's "if you would put one or more +1/+1 counters on a creature you
// control, put that many plus one instead", alone and beside Brad Boimler's
// every-kind one. TestStdSetReprosLockstep replays the recorded games;
// TestEffectReplNamedReprosReach requires each to reach what it is for, and
// TestEffectReplNamedMutation that the replacement being ignored diverges.
//
// The games are recorded by TestEffectReplNamedRecord
// (EFFECT_REPL_NAMED_RECORD=1) on the table built from the decks.

type effectReplNamedCase struct {
	effectLongCase
	policy func(c effectReplNamedCase, m *Matchup, seed uint64) Policy
}

func effectReplNamedCases() []effectReplNamedCase {
	dog := [2][]string{
		append(deckOf(6, "Prairie Dog"), append(deckOf(3, "Brad Boimler, Eager Ensign"), append(deckOf(4, "Twiddle"),
			append(deckOf(4, "Battlegrowth"), append(deckOf(3, "Servant of the Scale"), append(deckOf(4, "Llanowar Elves"),
				append(deckOf(8, "Plains"), deckOf(8, "Forest")...)...)...)...)...)...)...),
		append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
	}
	return []effectReplNamedCase{
		{
			effectLongCase: effectLongCase{
				name:   "malicious_eclipse_exiles_an_opposing_flash_creature",
				prefer: []string{"Malicious Eclipse", "Disfigure"},
				decks: [2][]string{
					append(deckOf(6, "Malicious Eclipse"), append(deckOf(6, "Disfigure"), deckOf(14, "Swamp")...)...),
					append(deckOf(8, "Ambush Viper"), deckOf(16, "Forest")...),
				},
				surface: "autopay",
				reached: eclipseExilesLateEntrant,
			},
			policy: eclipsePolicy,
		},
		{
			effectLongCase: effectLongCase{
				name: "prairie_dog_puts_one_more_counter", decks: dog, surface: "autopay",
				reached: func(g *GorgeSide) bool { return plusCountersIn(g, false) },
			},
			policy: func(_ effectReplNamedCase, _ *Matchup, seed uint64) Policy { return castFirstPolicy(seed) },
		},
		{
			effectLongCase: effectLongCase{
				name: "prairie_dog_beside_boimler_puts_two_more", decks: dog, surface: "autopay",
				reached: func(g *GorgeSide) bool { return plusCountersIn(g, true) },
			},
			policy: func(_ effectReplNamedCase, _ *Matchup, seed uint64) Policy { return castFirstPolicy(seed) },
		},
	}
}

func objFace(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// eclipseResolvedThisTurn: a Malicious Eclipse left the stack for the
// graveyard after the last turn change.
func eclipseResolvedThisTurn(g *GorgeSide) bool {
	done := false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			done = false
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard && objFace(g, e.Obj) == "Malicious Eclipse":
			done = true
		}
	}
	return done
}

// eclipsePolicy: the opposing seat casts its flash Viper only once an Eclipse
// has resolved this turn, so the creature enters after the replacement
// exists; the Eclipse's seat prefers its two spells.
func eclipsePolicy(c effectReplNamedCase, m *Matchup, seed uint64) Policy {
	base := effectLongPolicy(m, c.effectLongCase, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && p.D.Player == 1 && p.G != nil {
			late := eclipseResolvedThisTurn(p.G)
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Ambush Viper" && late {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPass {
					return o
				}
			}
		}
		return base.Pick(p)
	})
}

// eclipseExilesLateEntrant: an Ambush Viper entered after an Eclipse
// resolved in the same turn, and that turn it went from the battlefield to
// exile, not the graveyard.
func eclipseExilesLateEntrant(g *GorgeSide) bool {
	resolved := false
	entered := map[state.ObjID]bool{}
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			resolved = false
			clear(entered)
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard && objFace(g, e.Obj) == "Malicious Eclipse":
			resolved = true
		case resolved && e.Kind == events.MoveZone && e.To == state.ZBattlefield && objFace(g, e.Obj) == "Ambush Viper":
			entered[e.Obj] = true
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZExile && entered[e.Obj]:
			return true
		}
	}
	return false
}

// plusCountersIn: in one turn a Prairie Dog's activation resolved and a
// +1/+1 placement of at least two (three with Boimler's trigger too) followed.
// With both false a turn holding Boimler's trigger does not count.
func plusCountersIn(g *GorgeSide, boimler bool) bool {
	dog, boim := false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			dog, boim = false, false
		case e.Kind == events.AbilityPush && objFace(g, e.Obj) == "Prairie Dog":
			dog = true
		case e.Kind == events.TriggerPush && objFace(g, e.Obj) == "Brad Boimler, Eager Ensign":
			boim = true
		case e.Kind == events.CounterChange && e.Counter == "P1P1" && dog && boim == boimler:
			if boimler && e.Amount >= 3 || !boimler && e.Amount >= 2 {
				return true
			}
		}
	}
	return false
}

func TestEffectReplNamedReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range effectReplNamedCases() {
		t.Run(c.name, func(t *testing.T) {
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
				t.Fatalf("the recorded game never reaches what %s is for", c.name)
			}
		})
	}
}

// TestEffectReplNamedMutation: each recorded game diverges from gorge when the
// kernel ignores the replacements (dirac.DebugTurnReplOff).
func TestEffectReplNamedMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugTurnReplOff = true
	defer func() { dirac.DebugTurnReplOff = false }()
	for _, c := range effectReplNamedCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			res, _ := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), &ScriptPolicy{Actions: r.Actions})
			if res.Div == nil {
				t.Fatal("the game did not diverge with the replacements ignored: it does not exercise them")
			}
		})
	}
}

func TestEffectReplNamedRecord(t *testing.T) {
	if os.Getenv("EFFECT_REPL_NAMED_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range effectReplNamedCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 1500; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, c.policy(c, m, seed))
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
			t.Fatalf("no seed reached what %s is for", c.name)
		})
	}
}

// TestEffectReplNamedPoolLockstep plays both replacements, Prairie Dog beside
// Brad Boimler and a counter doubler (Dr. Joseph M'Benga) on one side and
// Malicious Eclipse with creature removal on the other, over many seeds: the
// kernel agrees with gorge at every decision, and the pool reaches both (the
// plus-one made a placement larger, a creature went to exile for the scope).
func TestEffectReplNamedPoolLockstep(t *testing.T) {
	reg := synthReg(t)
	cat := func(ds ...[]string) []string {
		var o []string
		for _, d := range ds {
			o = append(o, d...)
		}
		return o
	}
	decks := [2][]string{
		cat(deckOf(5, "Prairie Dog"), deckOf(3, "Brad Boimler, Eager Ensign"), deckOf(3, "Twiddle"), deckOf(4, "Battlegrowth"),
			deckOf(3, "Servant of the Scale"), deckOf(3, "Dr. Joseph M'Benga"), deckOf(3, "Contentious Plan"), deckOf(8, "Plains"), deckOf(8, "Forest")),
		cat(deckOf(4, "Doom Blade"), deckOf(4, "Malicious Eclipse"), deckOf(8, "Grizzly Bears"), deckOf(6, "Swamp"), deckOf(6, "Forest")),
	}
	c := effectReplNamedCases()[1].effectLongCase
	set := printedDelveSet(t, decks)
	plus, exiled := dirac.TurnReplStats.Plus.Load(), dirac.TurnReplStats.Exiled.Load()
	for seed := uint64(1); seed <= 250; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"pool_0", "pool_1"}, Cards: decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
		res, _ := effectLongPlay(t, reg, c, r, set, castFirstPolicy(seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
	}
	if dirac.TurnReplStats.Plus.Load() == plus {
		t.Error("no placement got Prairie Dog's one more: the pool does not exercise it")
	}
	if dirac.TurnReplStats.Exiled.Load() == exiled {
		t.Error("no creature was exiled by Malicious Eclipse's scope: the pool does not exercise it")
	}
}
