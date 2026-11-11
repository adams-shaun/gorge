package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Crypt Rats' "{X}: deals X damage to each creature and each player. Spend
// only black mana on X": an activated ability's XColor$. gorge folds the
// announced X into {B} pips (rules/pay AbilityXColor, Cost.WithX), so the X
// ask offers no value the black mana cannot pay; the kernel bounds the ask by
// the same colour and folds the charge (dirac/cost_xcolor.go). The recorded
// game's seat holds Swamps and Mountains, so the ceiling is the black mana,
// not every land; the policy announces the largest X offered.
//
// The game is recorded by TestCryptRatsRecord (CRYPT_RATS_RECORD=1) and
// replayed by TestCryptRatsReprosLockstep.
func cryptRatsCases() []effectLongCase {
	return []effectLongCase{{
		name: "crypt_rats_x_black_mana_only",
		decks: [2][]string{
			append(deckOf(4, "Crypt Rats"), append(deckOf(12, "Swamp"), deckOf(12, "Mountain")...)...),
			deckOf(30, "Plains"),
		},
		surface: "autopay",
		reached: cryptRatsReached,
	}}
}

// cryptRatsReached: seat 0, controlling a Mountain, was dealt at least 2
// noncombat damage (only Crypt Rats damages seat 0 in this matchup).
func cryptRatsReached(g *GorgeSide) bool {
	hit := false
	for i := range g.E.L.Events {
		if e := &g.E.L.Events[i]; e.Kind == events.Damage && e.Obj == 0 && e.Player == 0 && e.Amount >= 2 {
			hit = true
		}
	}
	return hit && untappedNamed(g, "Mountain", false) > 0
}

// untappedNamed counts seat 0's battlefield permanents named name (untapped
// ones only when untapped).
func untappedNamed(g *GorgeSide, name string, untapped bool) int {
	n := 0
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name && (!untapped || !o.Tapped) {
			n++
		}
	}
	return n
}

// cryptRatsPolicy plays lands and casts Crypt Rats; it activates the Rats
// once two Swamps are untapped beside a Mountain and answers the X ask with
// the largest value; the random tail answers the rest, never activating the
// Rats itself.
func cryptRatsPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose && p.G != nil {
			if d := p.G.E.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "x" {
				best := p.D.Options[0]
				for _, o := range p.D.Options {
					if o.Mode() > best.Mode() {
						best = o
					}
				}
				return best
			}
		}
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Crypt Rats" {
					return o
				}
			}
			if p.G != nil && untappedNamed(p.G, "Swamp", true) >= 2 && untappedNamed(p.G, "Mountain", true) > 0 {
				for _, o := range p.D.Options {
					if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Crypt Rats" {
						return o
					}
				}
			}
		}
		c := rnd.Pick(p)
		if p.D.Kind == DPriority && c.Kind() == ActActivate && m.InstanceCard(int(c.Src())) == "Crypt Rats" {
			// a free X = 0 activation repeated would run the kernel's
			// per-turn activation count out (StallActs): pass instead
			for _, o := range p.D.Options {
				if o.Kind() == ActPass {
					return o
				}
			}
		}
		return c
	})
}

// TestCryptRatsRecord plays the case's policy over seeds until one reaches
// its condition, and writes the repro. It runs only with CRYPT_RATS_RECORD=1;
// the game it writes is the committed repro.
func TestCryptRatsRecord(t *testing.T) {
	if os.Getenv("CRYPT_RATS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range cryptRatsCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 40; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", MaxTurns: 40}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, cryptRatsPolicy(m, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached || strings.HasPrefix(res.Stall, KernelStallPrefix) {
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

// TestCryptRatsReprosLockstep replays the recorded game: no divergence,
// every scripted action offered, the condition reached.
func TestCryptRatsReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range cryptRatsCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := reproSet(t, "custom", r)
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("the kernel stopped: %q", res.Stall)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}
