package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 106.1a, 601.2f: Cloud's Limit Break's mode ask prices each tier's total
// against the caster's potential mana for THIS cast. A Chosen-colour source
// (Coldsteel Heart: one mana of the chosen colour) makes one mana, not any
// amount, so with two Hearts and three Plains the seat can pay five mana at
// most: Blade Beam ({2}{W}) is affordable and Omnislash ({4}{W}{W}) is not,
// with or without a restricted mana source on the board (the sibling case is
// TestMBCCloudSLimitBreakReprosLockstep). Seat 0 casts Cloud's Limit Break at
// a tapped Savannah Lions of seat 1 with both Hearts untapped, so one is
// still untapped at the mode ask after the base cost is paid.
//
// The game is recorded by TestMBCTieredChosenManaRecord (PINS_RECORD=1, the
// writer switch the test runner allows) and replayed by
// TestMBCTieredChosenManaReprosLockstep.
func tieredChosenManaCase() effectLongCase {
	return effectLongCase{
		name: "tiered_chosen_mana_bounds_exactly",
		decks: [2][]string{
			append(append(deckOf(8, "Cloud's Limit Break"), deckOf(10, "Coldsteel Heart")...), deckOf(22, "Plains")...),
			append(deckOf(14, "Savannah Lions"), deckOf(26, "Plains")...),
		},
		surface: "full",
		reached: tieredChosenManaCast,
	}
}

// tieredChosenManaCast: Cloud's Limit Break was put on the stack.
func tieredChosenManaCast(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.PutOnStack {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Cloud's Limit Break" {
			return true
		}
	}
	return false
}

// tieredChosenManaBoard counts seat 0's lands and untapped Coldsteel Hearts,
// and reports whether seat 1 holds a tapped Savannah Lions.
func tieredChosenManaBoard(g *GorgeSide) (lands, hearts int, tappedLions bool) {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Plains":
			lands++
		case "Coldsteel Heart":
			if !o.Tapped {
				hearts++
			}
		}
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 1) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Savannah Lions" && o.Tapped {
			tappedLions = true
		}
	}
	return lands, hearts, tappedLions
}

// tieredChosenManaPolicy plays seat 0's lands (three at most), casts its
// Coldsteel Hearts, and casts Cloud's Limit Break once both are untapped
// and a Lions is tapped. Seat 1 plays lands, casts its Lions and attacks with
// them; the seed's random stream answers the rest.
func tieredChosenManaPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		if p.D.Kind != DPriority || p.G == nil {
			return rnd.Pick(p)
		}
		lands, hearts, tapped := tieredChosenManaBoard(p.G)
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			name := m.InstanceCard(int(o.Src()))
			switch {
			case o.Kind() == ActActivate:
				continue // no floating mana: the mode ask prices untapped sources
			case p.D.Player == 0 && o.Kind() == ActPlayLand && lands >= 3:
				continue // the bound must stay under Omnislash's six
			case p.D.Player == 0 && o.Kind() == ActCast && name == "Cloud's Limit Break":
				if hearts < 2 || !tapped || lands < 3 {
					continue
				}
				return o
			}
			d.Options = append(d.Options, o)
		}
		for _, want := range []string{"Coldsteel Heart", "Savannah Lions"} {
			for _, o := range d.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want && (want == "Coldsteel Heart") == (p.D.Player == 0) {
					return o
				}
			}
		}
		if p.D.Player == 1 {
			for _, o := range d.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// TestMBCTieredChosenManaRecord plays tieredChosenManaPolicy over seeds until
// one reaches the cast and writes the repro. It runs only with PINS_RECORD=1.
func TestMBCTieredChosenManaRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := tieredChosenManaCase()
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Pool: "one-off", Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", MaxTurns: 40}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, tieredChosenManaPolicy(m, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s last=%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.LastSourced)
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
}

// TestMBCTieredChosenManaReprosLockstep replays the recorded game: no
// divergence, every scripted action offered, Cloud's Limit Break cast.
func TestMBCTieredChosenManaReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := tieredChosenManaCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := reproSet(t, "custom", r)
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, set, script)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stopped: %q", res.Stall)
	}
	if !reached {
		t.Fatalf("the recorded game never casts Cloud's Limit Break")
	}
}
