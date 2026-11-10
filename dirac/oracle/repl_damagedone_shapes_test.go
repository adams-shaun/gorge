package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Self-targeted DamageDone replacements played decision by decision against
// gorge:
//
//   - Anti-Venom, Horrifying Healer (kop.ReplPreventDamageCounters): damage
//     that would be dealt to it is prevented and it gets that many +1/+1
//     counters instead.
//   - Wolverine, Fierce Fighter (kop.ReplDamageHealFirst): damage that would
//     be dealt to him is dealt, but the damage already marked on him is
//     healed first (gorge: the HealDamage body, then the held event).
//
// TestStdSetReprosLockstep replays the repros; TestReplDamagedoneShapesReach
// requires each recorded game to reach the replacement, and
// TestReplDamagedoneShapesMutation that the kernel without it diverges. The
// games are recorded by TestReplDamagedoneShapesRecord
// (REPL_DAMAGEDONE_RECORD=1) on the kernel table built from the decks.

const (
	sdAntiVenom = "Anti-Venom, Horrifying Healer"
	sdWolverine = "Wolverine, Fierce Fighter"
)

// sdCase is an effectLongCase whose burn (burn, cast by seat 1) waits for
// host on the battlefield (damaged too, when damaged) and aims at it.
type sdCase struct {
	effectLongCase
	host, burn string
	damaged    bool
	// pile: the host takes its damage in combat (sdPilePolicy): only it
	// attacks and every blocker the options offer blocks it; no burn.
	pile bool
	// pileLast: the blockers are declared last-offered first, so their
	// declaration order (gorge deals their hits in it) differs from the
	// slot order the kernel adds them in.
	pileLast bool
}

func sdCases() []sdCase {
	burn := append(append(deckOf(8, "Lightning Strike"), deckOf(4, "Raging Goblin")...), deckOf(12, "Mountain")...)
	return []sdCase{
		{
			effectLongCase: effectLongCase{
				name:    "anti_venom_burn_becomes_counters",
				prefer:  []string{sdAntiVenom},
				decks:   [2][]string{append(deckOf(8, sdAntiVenom), deckOf(16, "Plains")...), burn},
				surface: "autopay",
				reached: sdCountersFromDamage,
			},
			host: sdAntiVenom, burn: "Lightning Strike",
		},
		{
			effectLongCase: effectLongCase{
				name:    "wolverine_damage_heals_what_was_marked",
				prefer:  []string{sdWolverine},
				decks:   [2][]string{append(deckOf(8, sdWolverine), append(deckOf(8, "Mountain"), deckOf(8, "Forest")...)...), burn},
				surface: "autopay",
				reached: sdHealedThenDealt,
			},
			host: sdWolverine, burn: "Lightning Strike", damaged: true,
		},
		{
			effectLongCase: effectLongCase{
				name:    "wolverine_double_blocked_heals_first_hit",
				prefer:  []string{sdWolverine, "Grizzly Bears"},
				decks:   [2][]string{append(deckOf(8, sdWolverine), append(deckOf(8, "Mountain"), deckOf(8, "Forest")...)...), append(deckOf(12, "Grizzly Bears"), deckOf(12, "Forest")...)},
				surface: "autopay",
				reached: sdDoubleBlockHealed,
			},
			host: sdWolverine, pile: true,
		},
		{
			effectLongCase: effectLongCase{
				name:    "wolverine_double_blocked_last_declared_survives",
				prefer:  []string{sdWolverine, "Grizzly Bears", "Typhoid Rats"},
				decks:   [2][]string{append(deckOf(8, sdWolverine), append(deckOf(8, "Mountain"), deckOf(8, "Forest")...)...), append(append(deckOf(6, "Grizzly Bears"), deckOf(6, "Typhoid Rats")...), append(deckOf(6, "Forest"), deckOf(6, "Swamp")...)...)},
				surface: "autopay",
				reached: sdDoubleBlockMixed,
			},
			host: sdWolverine, pile: true, pileLast: true,
		},
	}
}

// sdCountersFromDamage: Anti-Venom got +1/+1 counters and was never dealt
// damage (its only counter source is the replacement).
func sdCountersFromDamage(g *GorgeSide) bool {
	got := false
	for _, e := range g.E.L.Events {
		if !sdNamed(g, e.Obj, sdAntiVenom) {
			continue
		}
		switch {
		case e.Kind == events.Damage && e.Amount > 0:
			return false
		case e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount > 0:
			got = true
		}
	}
	return got
}

// sdHealedThenDealt: Wolverine's marked damage was healed (a negative
// Damage event) and the damage that healed it was then dealt to him.
func sdHealedThenDealt(g *GorgeSide) bool {
	healed := false
	for _, e := range g.E.L.Events {
		if e.Kind != events.Damage || !sdNamed(g, e.Obj, sdWolverine) {
			continue
		}
		if e.Amount < 0 {
			healed = true
		} else if healed && e.Amount > 0 {
			return true
		}
	}
	return false
}

// sdDoubleBlockHealed: in one damage step Wolverine was dealt damage, healed
// of it by each next event's heal (a negative Damage), and dealt that next
// event: three positive Damage events (6 damage a kernel that did not heal
// would mark on his 5 toughness) with a negative one before each of the
// later two and no step change, and the game goes on past the end of combat to the second
// main phase (the marked damage is observable at a decision before the game
// ends).
func sdDoubleBlockHealed(g *GorgeSide) bool {
	pos, healed, reached, later := 0, 0, false, 0
	for _, e := range g.E.L.Events {
		if e.Kind == events.StepChange || e.Kind == events.TurnChange {
			if reached {
				later++ // end of combat, then the second main phase
				if later == 2 {
					return true
				}
				continue
			}
			pos, healed = 0, 0
			continue
		}
		if e.Kind != events.Damage || !sdNamed(g, e.Obj, sdWolverine) {
			continue
		}
		switch {
		case e.Amount < 0:
			healed = pos
		case e.Amount > 0:
			pos++
			reached = reached || pos >= 3 && healed >= 2
		}
	}
	return false
}

// sdDoubleBlockMixed: in one damage step Wolverine was dealt two or more
// damage events of different amounts (a Bears' 2, a Rats' 1 with
// deathtouch), each healed by the next (a negative Damage between), and the
// game goes on past the end of combat. The last event dealt is the one whose
// damage and deathtouch mark stay.
func sdDoubleBlockMixed(g *GorgeSide) bool {
	var amts []int32
	healed, reached, later := false, false, 0
	for _, e := range g.E.L.Events {
		if e.Kind == events.StepChange || e.Kind == events.TurnChange {
			if reached {
				later++
				if later == 2 {
					return true
				}
				continue
			}
			amts, healed = amts[:0], false
			continue
		}
		if e.Kind != events.Damage || !sdNamed(g, e.Obj, sdWolverine) {
			continue
		}
		switch {
		case e.Amount < 0:
			healed = true
		case e.Amount > 0:
			if len(amts) > 0 && !healed {
				amts = amts[:0] // two events with no heal between: not the shape
			}
			amts = append(amts, e.Amount)
			healed = false
			reached = reached || len(amts) >= 2 && amts[len(amts)-1] != amts[len(amts)-2]
		}
	}
	return false
}

func sdNamed(g *GorgeSide, id state.ObjID, name string) bool {
	if id == 0 {
		return false
	}
	o := g.E.G.Obj(id)
	return o != nil && o.Face() != nil && o.Face().Name == name
}

// sdHost is the case's host on the battlefield (damaged, when c.damaged),
// nil when there is none or gorge's state is not at hand.
func sdHost(g *GorgeSide, c sdCase) *state.Object {
	if g == nil {
		return nil
	}
	for p := range g.E.G.Players {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == c.host && (!c.damaged || o.Damage > 0) {
				return o
			}
		}
	}
	return nil
}

// sdPolicy is effectLongPolicy, except that the burn waits for the host and
// a target choice takes the host when it is offered.
func sdPolicy(m *Matchup, c sdCase, seed uint64) Policy {
	if c.pile {
		return sdPilePolicy(m, c, seed)
	}
	inner := effectLongPolicy(m, c.effectLongCase, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		host := sdHost(p.G, c)
		switch p.D.Kind {
		case DTarget:
			if host != nil {
				ref := p.G.InstRef(host.ID)
				for _, o := range p.D.Options {
					if o.Kind() == ActTarget && o.Tgt() == ref {
						return o
					}
				}
			}
		case DPriority:
			if host != nil {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.burn {
						return o
					}
				}
				break
			}
			d := *p.D
			d.Options = nil
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.burn {
					continue
				}
				d.Options = append(d.Options, o)
			}
			return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
		}
		return inner.Pick(p)
	})
}

// sdPilePolicy casts the case's preferred cards and lands, attacks with the
// host alone, and blocks with every creature the options offer onto the
// attacker the first blocker blocked.
func sdPilePolicy(m *Matchup, c sdCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c.effectLongCase, seed)
	var tgt uint32
	var have bool
	return PolicyFunc(func(p *PickCtx) Canon {
		var block, done, more Canon
		var atk []Canon
		haveBlock, haveDone, haveMore := false, false, false
		for _, o := range p.D.Options {
			switch o.Kind() {
			case ActBlock:
				if have && uint32(o.Tgt()) == tgt {
					if !c.pileLast {
						return o
					}
					more, haveMore = o, true
				}
				if !haveBlock || c.pileLast {
					block, haveBlock = o, true
				}
			case ActAttack:
				if m.InstanceCard(int(o.Src())) == c.host {
					atk = append(atk, o)
				}
			case ActDone:
				done, haveDone = o, true
			}
		}
		if haveMore {
			return more
		}
		if haveBlock {
			if !have {
				tgt, have = uint32(block.Tgt()), true
			}
			return block
		}
		have = false
		if len(atk) > 0 {
			return atk[0]
		}
		if haveDone && p.D.Kind != DPriority {
			return done
		}
		return inner.Pick(p)
	})
}

func sdReplay(t *testing.T, c sdCase) Result {
	t.Helper()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
	if res.Div == nil {
		if script.Skipped != 0 {
			t.Errorf("%d scripted actions were not offered", script.Skipped)
		}
		if !reached {
			t.Errorf("the recorded game never reaches %s", c.name)
		}
	}
	return res
}

// TestReplDamagedoneShapesReach replays the recorded games: no divergence,
// every scripted action offered, the replacement reached.
func TestReplDamagedoneShapesReach(t *testing.T) {
	for _, c := range sdCases() {
		t.Run(c.name, func(t *testing.T) {
			if d := sdReplay(t, c).Div; d != nil {
				t.Fatalf("divergence at %d: %s %s %v", d.At, d.What, d.Field, d.Diff)
			}
		})
	}
}

// TestReplDamagedoneShapesMutation: with both replacements off in the
// kernel, each recorded game diverges from gorge.
func TestReplDamagedoneShapesMutation(t *testing.T) {
	dirac.DebugSelfDamageReplOff = true
	defer func() { dirac.DebugSelfDamageReplOff = false }()
	for _, c := range sdCases() {
		t.Run(c.name, func(t *testing.T) {
			d := sdReplay(t, c).Div
			if d == nil {
				t.Fatal("the mutation did not diverge")
			}
			t.Logf("diverged at %d: %s %s %v", d.At, d.What, d.Field, d.Diff)
		})
	}
}

// TestReplDamagedoneShapesRecord plays the cases' policies over seeds until
// one reaches the replacement, and writes the repro. It runs only with
// REPL_DAMAGEDONE_RECORD=1; the games it writes are the committed repros.
func TestReplDamagedoneShapesRecord(t *testing.T) {
	if os.Getenv("REPL_DAMAGEDONE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range sdCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, sdPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
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
