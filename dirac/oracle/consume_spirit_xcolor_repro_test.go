package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Consume Spirit's XColor$ Black ("spend only black mana on X", CR 107.3,
// 601.2b) played decision by decision against gorge: the caster holds Swamps
// and Mountains, so a kernel that priced X as generic would plan its taps
// (and its largest X) from the Mountains where gorge needs the Swamps, and the
// tapped lands and floating mana would diverge. The bare NumDmg$ X and
// LifeAmount$ X are the paid X.
//
// TestStdSetReprosLockstep replays the repro; TestConsumeSpiritXColorReprosReach
// requires the recorded game to resolve the spell with an X of two or more
// (the shape where the restriction is visible).
//
// The game is recorded by TestConsumeSpiritXColorRecord
// (CONSUME_SPIRIT_XCOLOR_RECORD=1) on the kernel table built from the decks.
func consumeSpiritXColorCases() []effectLongCase {
	return []effectLongCase{{
		name:   "x_color_black_consume_spirit_pays_x_with_black_only",
		prefer: []string{"Consume Spirit"},
		decks: [2][]string{
			append(append(deckOf(8, "Consume Spirit"), deckOf(8, "Swamp")...), deckOf(8, "Mountain")...),
			append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...),
		},
		surface: "full", // an {X} cast is offered only on the full surface
		reached: consumeSpiritResolvedBigX,
	}, {
		// Crypt Rats's activated {X} carries the same XColor$ on its ability: the
		// opponent holds no damage, so any damage of two or more is the Rats'.
		name:   "x_color_black_crypt_rats_activation_pays_x_with_black_only",
		prefer: []string{"Crypt Rats"},
		decks: [2][]string{
			append(append(deckOf(8, "Crypt Rats"), deckOf(8, "Swamp")...), deckOf(8, "Mountain")...),
			append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
		},
		surface: "full",
		reached: func(g *GorgeSide) bool {
			for _, e := range g.E.L.Events {
				if e.Kind == events.Damage && e.Amount >= 2 {
					return true
				}
			}
			return false
		},
	}}
}

// consumeSpiritResolvedBigX: a Consume Spirit left the stack for the graveyard
// and its caster gained two or more life (its LifeAmount$ X with X >= 2).
func consumeSpiritResolvedBigX(g *GorgeSide) bool {
	if !movedFrom("Consume Spirit", state.ZStack, state.ZGraveyard)(g) {
		return false
	}
	for _, e := range g.E.L.Events {
		if e.Kind == events.LifeChange && e.Player == 0 && e.Amount >= 2 {
			return true
		}
	}
	return false
}

// consumeSpiritPolicy is effectLongPolicy, except that an {X} announcement
// takes the largest value offered, so the colour restriction (not the random
// pick) bounds the spell.
func consumeSpiritPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	xKind := ActChoose + CanonKind(mustChooseCode("x"))
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == c.prefer[0] && c.prefer[0] == "Crypt Rats" {
					return o
				}
			}
		}
		if p.D.Kind == DChoose && len(p.D.Options) > 0 && p.D.Options[0].Kind() == xKind {
			return p.D.Options[len(p.D.Options)-1]
		}
		return inner.Pick(p)
	})
}

func TestConsumeSpiritXColorReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range consumeSpiritXColorCases() {
		t.Run(c.name, func(t *testing.T) {
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
		})
	}
}

// TestConsumeSpiritXColorRecord plays the case's policy over seeds until one
// reaches the resolution, and writes the repro. It runs only with
// CONSUME_SPIRIT_XCOLOR_RECORD=1.
func TestConsumeSpiritXColorRecord(t *testing.T) {
	if os.Getenv("CONSUME_SPIRIT_XCOLOR_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range consumeSpiritXColorCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, consumeSpiritPolicy(m, c, seed))
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
