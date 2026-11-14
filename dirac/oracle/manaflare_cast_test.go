package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mana Flare's reflected tap extra beside casts whose price the extra
// reaches: gorge's planner folds the extra into each single-colour
// production (pay.PaymentPlanReflectedTapMana), so a player whose one land
// tapped makes two mana of the pick and can pay a two-mana cast of that
// colour from it. Two shapes, both replayed by TestManaFlareCastLockstep:
//
//   - mana_flare_pain_land_cast: a painful any-colour land (City of Brass)
//     whose tap's extra is one mana of the pick. The kernel priced the tap
//     without it (srcExtraPain took the pain-only shortcut and dropped the
//     pick-dependent extra), so gorge offered a {3W} hand cast the kernel's
//     option list did not contain; it replays the shrunk pool repro, whose
//     Loyal Unicorn (the filing ticket's card, not lowered here) reads as
//     the same-cost Abzan Battle Priest.
//   - mana_flare_cast_paid_with_the_reflected_extra: a fixed-deck fixture
//     over the non-pain fold (the extra of a choice land, a basic's own
//     colour), recorded by TestManaFlareCastRecord.
func manaFlareCastCases() []effectLongCase {
	return []effectLongCase{
		{
			name:    "mana_flare_pain_land_cast",
			prefer:  []string{"Mana Flare"},
			decks:   [2][]string{},
			surface: "full",
			reached: manaFlarePainTapFloatedTwo,
		},
		{
			name:   "mana_flare_cast_paid_with_the_reflected_extra",
			prefer: []string{"Mana Flare", "Hill Giant", "Grizzly Bears", "Lightning Strike", "Giant Growth"},
			decks: [2][]string{
				deckOf(4, "Mana Flare", deckOf(4, "Hill Giant", deckOf(16, "Mountain")...)...),
				deckOf(4, "Grizzly Bears", deckOf(4, "Lightning Strike", deckOf(4, "Giant Growth", deckOf(8, "Forest", deckOf(4, "Rugged Highlands")...)...)...)...),
			},
			surface: "full",
			reached: manaFlareExtraCast,
		},
	}
}

// manaFlarePainTapFloatedTwo: Mana Flare was on the battlefield and a later
// land tap was followed by two mana of one colour before the next tap of
// another object — the tap's own mana and the reflected extra, the shape a
// boundary cast is priced on.
func manaFlarePainTapFloatedTwo(g *GorgeSide) bool {
	flare := false
	evs := g.E.L.Events
	for i, e := range evs {
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Mana Flare" {
				flare = true
			}
		}
		if !flare || e.Kind != events.Tap {
			continue
		}
		n := 0
		for j := i + 1; j < len(evs) && j <= i+6; j++ {
			f := evs[j]
			if f.Kind == events.Tap && f.Obj != e.Obj {
				break
			}
			if f.Kind == events.ManaAdd && f.Amount > 0 {
				n += int(f.Amount)
			}
		}
		if n >= 2 {
			return true
		}
	}
	return false
}

// manaFlareExtraCast: Mana Flare was on the battlefield and a later land tap
// was followed by a mana addition beyond the land's own (the reflected
// extra).
func manaFlareExtraCast(g *GorgeSide) bool {
	flare := false
	evs := g.E.L.Events
	for i, e := range evs {
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Mana Flare" {
				flare = true
			}
		}
		if !flare || e.Kind != events.Tap {
			continue
		}
		n := 0
		for j := i + 1; j < len(evs) && j <= i+6; j++ {
			f := evs[j]
			if f.Kind == events.Tap && f.Obj != e.Obj {
				break
			}
			if f.Kind == events.ManaAdd && f.Amount > 0 {
				n += int(f.Amount)
			}
		}
		if n >= 2 {
			return true
		}
	}
	return false
}

// TestManaFlareCastRecord plays the fixture case's policy over seeds until
// one reaches the extra, and writes the repro. It runs only with
// MANAFLARE_CAST_RECORD=1; the game it writes is the committed repro. A
// divergence fails loudly with its decision, kind and field.
func TestManaFlareCastRecord(t *testing.T) {
	if os.Getenv("MANAFLARE_CAST_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range manaFlareCastCases() {
		if c.name != "mana_flare_cast_paid_with_the_reflected_extra" {
			continue // the pool repro is a kfuzz shrink, not a fixture recording
		}
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
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

// TestManaFlareCastLockstep replays the recorded games: no divergence, every
// scripted action offered, the reflected extra reached.
func TestManaFlareCastLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range manaFlareCastCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := printedDelveSet(t, r.Cards)
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches the reflected extra of %s", c.name)
			}
		})
	}
}
