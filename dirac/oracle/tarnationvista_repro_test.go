package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tarnation Vista's "{1}, {T}: For each color among monocolored permanents you
// control, add one mana of that color" (CR 106.1, 605.1a) is a fee-paid census
// gap beside its chosen-colour {T}. gorge's prefix search reaches it only with
// the fee already floating (the census funds nothing: fees() is 0): the
// activation then asks which ability, and the {1} comes out of the pool. The
// search is depth first in battlefield order and lists the chosen {T} first.
//
// Both games hold one source of {W}: the Vista's each-colour set, once a
// Porcelain Legionnaire (a white artifact creature, cast for {2} and 2 life) is
// on the battlefield. The Vista chooses red, so a Bartolomé del Presidio ({W}{B})
// is paid only through the Vista's each-colour ability, its fee coming from
//
//   - other Vistas: each taps for its chosen {R} before the last asks which
//     ability to use (the search's chosen-first order), and one of those {R}
//     pays the last one's fee; or
//   - a Dark Ritual's floating {B}, with a single Vista.
//
// The games are recorded by TestTarnationVistaRecord
// (TARNATION_VISTA_RECORD=1) and replayed by TestStdSetReprosLockstep.
type vistaCase struct {
	effectLongCase
	ritual bool // the fee is a Dark Ritual's floating mana
}

func tarnationVistaCases() []vistaCase {
	return []vistaCase{
		{effectLongCase: effectLongCase{
			name:   "tarnation_vista_chosen_mana_of_other_vistas_pays_the_fee",
			prefer: []string{"Bartolomé del Presidio", "Porcelain Legionnaire"},
			decks: [2][]string{
				append(append(append(deckOf(11, "Swamp"), deckOf(4, "Tarnation Vista")...), deckOf(4, "Porcelain Legionnaire")...), deckOf(5, "Bartolomé del Presidio")...),
				deckOf(24, "Plains"),
			},
			surface: "full",
			reached: func(g *GorgeSide) bool { return vistaEachColourPaid(g, false) },
		}},
		{effectLongCase: effectLongCase{
			name:   "tarnation_vista_each_colour_paid_from_floating_mana",
			prefer: []string{"Bartolomé del Presidio", "Porcelain Legionnaire", "Dark Ritual"},
			decks: [2][]string{
				append(append(append(append(deckOf(6, "Swamp"), deckOf(1, "Tarnation Vista")...), deckOf(4, "Porcelain Legionnaire")...), deckOf(6, "Dark Ritual")...), deckOf(7, "Bartolomé del Presidio")...),
				deckOf(24, "Plains"),
			},
			surface: "full",
			reached: func(g *GorgeSide) bool { return vistaEachColourPaid(g, true) },
		}, ritual: true},
	}
}

// vistaEachColourPaid: after a Porcelain Legionnaire entered the battlefield,
// {W} was added (the Vista's each-colour mana: nothing else in the deck makes
// it, and the Vista chose red) and a Bartolomé del Presidio entered after it.
// With ritual, a Dark Ritual resolved before the {W}; without, at least two
// {R} were added before it: the search tapped two Vistas for their chosen {R}
// (the second's ask answered chosen first) and the third's each-colour
// ability paid its fee with one of them. A first-fit tap would stop at two.
func vistaEachColourPaid(g *GorgeSide, ritual bool) bool {
	name := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	legion, ritualDone, white := false, false, false
	reds := 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && name(e) == "Porcelain Legionnaire":
			legion, ritualDone, white, reds = true, false, false, 0
		case !legion:
		case e.Kind == events.Resolve && name(e) == "Dark Ritual":
			ritualDone = true
		case e.Kind == events.ManaAdd && e.Player == 0 && e.Amount > 0 && e.Counter == "R":
			reds++
		case e.Kind == events.ManaAdd && e.Player == 0 && e.Amount > 0 && e.Counter == "W":
			white = ritualDone || !ritual && reds >= 2
		case white && e.Kind == events.MoveZone && e.To == state.ZBattlefield && name(e) == "Bartolomé del Presidio":
			return true
		}
	}
	return false
}

// tarnationVistaPolicy plays the Vista, then a Swamp, casts Bartolomé, the
// Legionnaire and the Ritual in that order, picks red for the Vista, and
// answers the rest at random.
func tarnationVistaPolicy(m *Matchup, c vistaCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose && len(p.D.Options) > 3 {
			return p.D.Options[3] // red
		}
		if p.D.Kind == DPriority {
			for _, land := range []string{"Tarnation Vista", "Swamp"} {
				for _, o := range p.D.Options {
					if o.Kind() == ActPlayLand && m.InstanceCard(int(o.Src())) == land {
						return o
					}
				}
			}
			for _, want := range c.prefer {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestTarnationVistaReprosReach replays the recorded games: no divergence,
// every scripted action offered, and Bartolomé paid with the Vista's each-colour
// mana.
func TestTarnationVistaReprosReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range tarnationVistaCases() {
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
				t.Fatal("the recorded game never pays Bartolomé with the Vista's each-colour mana")
			}
		})
	}
}

// TestTarnationVistaRecord plays each policy over seeds until one reaches
// Bartolomé, and writes the repro. It runs only with TARNATION_VISTA_RECORD=1;
// the games it writes are the committed repros.
func TestTarnationVistaRecord(t *testing.T) {
	if os.Getenv("TARNATION_VISTA_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range tarnationVistaCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 2000; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, tarnationVistaPolicy(m, c, seed))
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
			t.Fatal("no seed paid Bartolomé with the Vista's each-colour mana")
		})
	}
}
