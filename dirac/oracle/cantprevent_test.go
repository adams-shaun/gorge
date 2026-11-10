package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Damage can't be prevented" played decision by decision against gorge: the
// Effect form (Impractical Joke, Pyrewood Gearhulk's enters trigger, Flaring
// Pain, A-Ready to Rumble's first mode) and the printed static (Leyline of
// Punishment), each against Swooping Protector, whose SHIELD counter would
// otherwise prevent the damage. TestStdSetReprosLockstep replays the repros;
// TestCantPreventReprosReach requires each recorded game to reach the card,
// since a lockstep entry alone would pass a script that never cast it.
//
// The games are recorded by TestCantPreventRecord (CANT_PREVENT_RECORD=1) on
// the kernel table built from the decks, with effectLongPolicy: both seats
// prefer the case's cards and otherwise pick at random from the seed's stream.
func cantPreventCases() []effectLongCase {
	vs := append(deckOf(8, "Swooping Protector"), deckOf(16, "Plains")...)
	return []effectLongCase{
		{
			name:    "impractical_joke_damages_a_shielded_creature",
			prefer:  []string{"Impractical Joke", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(8, "Impractical Joke"), deckOf(16, "Mountain")...), vs},
			surface: "autopay",
			reached: damagedShielded("Impractical Joke"),
		},
		{
			name:    "a_ready_to_rumble_damages_a_shielded_creature",
			prefer:  []string{"A-Ready to Rumble", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(8, "A-Ready to Rumble"), deckOf(16, "Mountain")...), vs},
			surface: "autopay",
			reached: damagedShielded("A-Ready to Rumble"),
		},
		{
			name:    "pyrewood_gearhulk_enters_damage_cant_be_prevented",
			prefer:  []string{"Pyrewood Gearhulk", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(8, "Pyrewood Gearhulk"), append(deckOf(8, "Mountain"), deckOf(8, "Forest")...)...), vs},
			surface: "autopay",
			reached: movedFrom("Pyrewood Gearhulk", state.ZStack, state.ZBattlefield),
		},
		{
			name:    "flaring_pain_resolves",
			prefer:  []string{"Flaring Pain", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(8, "Flaring Pain"), deckOf(16, "Mountain")...), vs},
			surface: "autopay",
			reached: movedFrom("Flaring Pain", state.ZStack, state.ZGraveyard),
		},
		{
			name:    "leyline_of_punishment_on_the_battlefield",
			prefer:  []string{"Leyline of Punishment", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(8, "Leyline of Punishment"), deckOf(16, "Mountain")...), vs},
			surface: "autopay",
			reached: movedFrom("Leyline of Punishment", state.ZStack, state.ZBattlefield),
		},
	}
}

// damagedShielded: a Swooping Protector was dealt damage. The other seat has
// no creature, so the source is the case's spell, and the Protector still had
// the SHIELD counter it entered with: without the static the counter would
// have prevented the damage and no Damage event would be logged.
func damagedShielded(spell string) func(g *GorgeSide) bool {
	_ = spell // the only damage source in the deck facing the Protectors
	return func(g *GorgeSide) bool {
		for _, e := range g.E.L.Events {
			if e.Kind == events.Damage && e.Amount > 0 && e.Obj != 0 {
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
					return true
				}
			}
		}
		return false
	}
}

// movedFrom: a card named name moved from zone from to zone to.
func movedFrom(name string, from, to state.Zone) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		for _, e := range g.E.L.Events {
			if e.Kind == events.MoveZone && e.From == from && e.To == to {
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
					return true
				}
			}
		}
		return false
	}
}

// cantPreventPolicy is effectLongPolicy, except that a damage spell of a
// damagedShielded case (prefer[0]) waits until a Swooping Protector is on the
// battlefield, so the damage meets the SHIELD counter.
func cantPreventPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	if !strings.Contains(c.name, "shielded") {
		return effectLongPolicy(m, c, seed)
	}
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || protectorOut(p.G) {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.prefer[0] {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// protectorOut: a Swooping Protector is on the battlefield (false when the
// kernel leads and gorge's state is not at hand).
func protectorOut(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	for p := range g.E.G.Players {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
				return true
			}
		}
	}
	return false
}

// TestCantPreventReprosReach replays the recorded games: no divergence,
// every scripted action offered, the card reached.
func TestCantPreventReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range cantPreventCases() {
		t.Run(c.name, func(t *testing.T) {
			cantPreventReplay(t, reg, c)
		})
	}
}

func cantPreventReplay(t *testing.T, reg *cards.Registry, c effectLongCase) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
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

// TestCantPreventRecord plays the cases' policies over seeds until one
// reaches the card, and writes the repro. It runs only with
// CANT_PREVENT_RECORD=1; the games it writes are the committed repros.
func TestCantPreventRecord(t *testing.T) {
	if os.Getenv("CANT_PREVENT_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range cantPreventCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, cantPreventPolicy(m, c, seed))
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
