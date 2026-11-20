package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The census's paid-arm limit (relaxedPaidLimit, potential_plan.go) binds the
// funding of PLAIN filter fees: the fourth "{1}, {T}: Add one mana of any
// color" gap breaks census.relaxable and nils out.paid, so gorge's
// census.fees() funds nothing and its prefix search cannot tap an Island for a
// gate's fee -- a {1}{W} cast off a gate's any-colour mana stays unoffered.
// dirac's mirror (filtFunding) must gate on the intact gap mask for the same
// bound; with the gate read on the mask the funding loop already consumed, the
// bound never fired and dirac offered a payment gorge prices unpayable.
//
// The recorded witness is four Omni-Cheese Pizzas ("{1}, {T}, Sacrifice this
// artifact: Add one mana of any color") rather than four Heap Gates: a Heap
// Gate also has "{T}: Add {C}", and the search classifies gaps by object, so
// the gates fund each other's fees as gap activations and the bound never
// binds there. The pizza has one mana ability, so its fee needs a non-gap tap
// -- exactly what the bound forbids. On the recorded board gorge prices the
// {1}{W} Whitesun's Passage cast unpayable (funding 0) and offers pass, while
// the kernel offers the cast off a pizza's any-colour mana.
//
// TestMBCPlainFilterFeeLimitReprosLockstep replays the recorded game and
// requires the four-pizza board to be reached (four pizzas entered, the spell
// never cast), since a lockstep entry alone would pass a script that never
// gets there. TestPlainFilterFeeLimitRecord (PINS_RECORD=1) records it.
func plainGapLimitCases() []effectLongCase {
	a := append(deckOf(4, "Omni-Cheese Pizza"), append(deckOf(5, "Island"), deckOf(9, "Whitesun's Passage")...)...)
	return []effectLongCase{
		{
			name:    "plain_filter_fee_limit",
			decks:   [2][]string{a, deckOf(20, "Mountain")},
			surface: "full",
			reached: plainGapLimitReached,
		},
	}
}

// plainGapLimitReached reads the finished gorge log: four pizzas entered and
// the {1}{W} spell was never put on the stack -- the board whose priority
// decisions the census bound governs (the policy casts the pizzas and declines
// the spell, so a cast event would mean the witness state never held).
func plainGapLimitReached(g *GorgeSide) bool {
	name := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	pizzas, cast := 0, false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && name(e) == "Omni-Cheese Pizza":
			pizzas++
		case e.Kind == events.PutOnStack && name(e) == "Whitesun's Passage":
			cast = true
		}
	}
	return pizzas >= 4 && !cast
}

// plainGapLimitPolicy casts the pizzas, plays a land otherwise, and passes
// without ever casting, floating or activating anything else, so the board
// reaches the four-pizza state with a {1}{W} spell still in hand and an empty
// pool.
func plainGapLimitPolicy(m *Matchup, c effectLongCase) Policy {
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Omni-Cheese Pizza" {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		for _, o := range p.D.Options {
			if o.Kind() == ActPass {
				return o
			}
		}
		return p.D.Options[0]
	})
}

func TestMBCPlainFilterFeeLimitReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "plain_filter_fee_limit.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, synthReg(t), plainGapLimitCases()[0], r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never reaches the four-pizza board")
	}
}

// TestPlainFilterFeeLimitRecord plays the pass-and-land policy over seeds
// until one reaches the four-gate board and diverges there, and writes the
// repro. It runs only with PINS_RECORD=1; the game it writes is the committed
// repro (it encodes the pre-fix divergence, as hybrid_filter_fee does).
func TestPlainFilterFeeLimitRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := plainGapLimitCases()[0]
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 80; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, plainGapLimitPolicy(m, c))
		if res.Div == nil {
			t.Logf("seed %d: %d decisions, no divergence (reached %v)", seed, len(res.Actions), reached)
			continue
		}
		if res.Div.What != "options" || res.Div.Field != "options" {
			t.Fatalf("seed %d: unexpected divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d actions, divergence at %d: %s %s %v", seed, len(res.Actions), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		return
	}
	t.Fatal("no seed reached the four-gate board and diverged")
}
