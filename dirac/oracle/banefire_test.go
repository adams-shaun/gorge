package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Banefire played decision by decision against gorge: "If X is 5 or more,
// this spell can't be countered and the damage can't be prevented". Both
// cases announce the largest X offered, so the gate is met when six mana
// pay. banefire_x5_damage_cant_be_prevented aims it at a Swooping Protector,
// whose SHIELD counter would prevent the damage; banefire_x5_cant_be_countered
// faces a Cancel, which would counter the spell. TestReproBanefire replays the
// recorded games and requires each to reach its observable (a lockstep entry
// alone would pass a game that never met the gate). TestBanefireRecord
// (BANEFIRE_RECORD=1) writes them.
func banefireCases() []effectLongCase {
	vs := append(deckOf(3, "Swooping Protector"), deckOf(21, "Plains")...)
	return []effectLongCase{
		{
			name:    "banefire_x5_damage_cant_be_prevented",
			prefer:  []string{"Banefire", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(5, "Banefire"), deckOf(19, "Mountain")...), vs},
			surface: "full",
			reached: damagedShielded("Banefire"),
		},
		{
			name:    "banefire_x5_cant_be_countered",
			prefer:  []string{"Banefire", "Cancel"},
			decks:   [2][]string{append(deckOf(5, "Banefire"), deckOf(19, "Mountain")...), append(deckOf(5, "Cancel"), deckOf(19, "Island")...)},
			surface: "full",
			reached: banefireSurvivedCancel,
		},
	}
}

// banefireSurvivedCancel: a Cancel resolved (left the stack for the
// graveyard) and a Banefire's damage of five or more was dealt anyway: the
// decks hold no other damage source, and a countered Banefire deals none.
func banefireSurvivedCancel(g *GorgeSide) bool {
	cancelled, dealt := false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Cancel" {
				cancelled = true
			}
		case e.Kind == events.Damage && e.Amount >= 5:
			dealt = true
		}
	}
	return cancelled && dealt
}

// banefirePolicy is cantPreventPolicy's play plus the largest X every time
// an X is asked.
func banefirePolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := cantPreventPolicy(m, effectLongCase{name: "shielded " + c.name, prefer: c.prefer, decks: c.decks}, seed)
	if c.name == "banefire_x5_cant_be_countered" {
		inner = effectLongPolicy(m, c, seed)
	}
	xCode, _ := chooseCode("x")
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && mountainsOut(p.G) < 6 {
			d := *p.D
			d.Options = nil
			for _, o := range p.D.Options { // no Banefire before six lands: the gate needs X of five
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Banefire" {
					continue
				}
				d.Options = append(d.Options, o)
			}
			return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
		}
		if p.D.Kind == DChoose {
			best, found := Canon(0), false
			for _, o := range p.D.Options {
				if o.Kind() == ActChoose+CanonKind(xCode) && (!found || o.Mode() > best.Mode()) {
					best, found = o, true
				}
			}
			if found {
				return best
			}
		}
		return inner.Pick(p)
	})
}

// TestReproBanefire replays the recorded games: no divergence, every
// scripted action offered, the observable reached.
func TestReproBanefire(t *testing.T) {
	reg := synthReg(t)
	for _, c := range banefireCases() {
		t.Run(c.name, func(t *testing.T) {
			replayEffectLongCase(t, reg, c)
		})
	}
}

// TestBanefireRecord plays banefirePolicy over seeds until one reaches the
// observable and writes the repro. It runs only with BANEFIRE_RECORD=1.
func TestBanefireRecord(t *testing.T) {
	if os.Getenv("BANEFIRE_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range banefireCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, banefirePolicy(m, c, seed))
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

// mountainsOut counts the Mountains on the battlefield (0 when the kernel
// leads and gorge's state is not at hand).
func mountainsOut(g *GorgeSide) int {
	if g == nil {
		return 0
	}
	n := 0
	for p := range g.E.G.Players {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
				n++
			}
		}
	}
	return n
}
