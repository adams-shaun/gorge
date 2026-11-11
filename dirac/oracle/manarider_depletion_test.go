package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The bounded mana-ability riders played decision by decision against gorge
// (kernel mana_rider.go): a depletion land (Peat Bog, Hickory Woodlot, Remote
// Farm) removes a counter, adds its two mana and is sacrificed once none are
// left; Gemstone Mine does the same with mining counters and one any-colour
// mana; Chromatic Sphere pays {1}, sacrifices itself, adds any colour and
// draws (CR 605.3b: the rider resolves with the mana). The table is built from
// each case's own decks (pool "custom"). TestManaRiderDepletionReprosReach
// requires the named permanent to leave the battlefield, which only the
// rider's sacrifice (a land) or the cost (the Sphere) does here, since a
// replay that never used the card would pass. The games are recorded by
// TestManaRiderDepletionRecord (MANA_RIDER_DEPLETION_RECORD=1) with both seats
// preferring the case's cards, and replayed here.
func manaRiderDepletionCases() []effectLongCase {
	land := func(name, spell string) effectLongCase {
		return effectLongCase{
			name:    "mana_rider_" + riderSlug(name) + "_sacrifices_on_its_last_counter",
			prefer:  []string{spell},
			decks:   [2][]string{append(deckOf(10, spell), deckOf(14, name)...), deckOf(24, "Plains")},
			surface: "full",
			reached: riderSacrificed(name),
		}
	}
	return []effectLongCase{
		land("Peat Bog", "Walking Corpse"),
		land("Hickory Woodlot", "Grizzly Bears"),
		land("Gemstone Mine", "Grizzly Bears"),
		land("Remote Farm", "Savannah Lions"),
		{
			name:    "mana_rider_chromatic_sphere_sacrifices_and_draws",
			prefer:  []string{"Grizzly Bears", "Chromatic Sphere"},
			decks:   [2][]string{append(deckOf(8, "Chromatic Sphere"), append(deckOf(8, "Grizzly Bears"), deckOf(8, "Forest")...)...), deckOf(24, "Plains")},
			surface: "full",
			reached: riderSacrificed("Chromatic Sphere"),
		},
		{
			name:    "mana_rider_shadowblood_egg_sacrifices_and_draws",
			prefer:  []string{"Black Knight", "Shadowblood Egg"},
			decks:   [2][]string{append(deckOf(8, "Shadowblood Egg"), append(deckOf(8, "Black Knight"), deckOf(8, "Swamp")...)...), deckOf(24, "Plains")},
			surface: "full",
			reached: riderSacrificed("Shadowblood Egg"),
		},
		{
			name:    "mana_rider_grove_gives_each_opponent_a_life",
			prefer:  []string{"Grizzly Bears"},
			decks:   [2][]string{append(deckOf(8, "Grizzly Bears"), deckOf(16, "Grove of the Burnwillows")...), deckOf(24, "Plains")},
			surface: "full",
			reached: lifeGained,
		},
	}
}

// lifeGained: some player gained life (nothing else in the case's decks does).
func lifeGained(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.LifeChange && e.Amount > 0 {
			return true
		}
	}
	return false
}

func riderSlug(n string) string {
	b := []byte(n)
	for i, c := range b {
		if c == ' ' {
			b[i] = '_'
		} else if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// riderSacrificed: a permanent named card moved battlefield -> graveyard
// after it had been on the battlefield (a sacrifice).
func riderSacrificed(card string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		for _, e := range g.E.L.Events {
			if e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == card {
					return true
				}
			}
		}
		return false
	}
}

// manaRiderDepletionPolicy is effectLongPolicy, except that the case's
// non-spell permanents' abilities are activated when offered.
func manaRiderDepletionPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() != ActActivate {
					continue
				}
				switch m.InstanceCard(int(o.Src())) {
				case "Chromatic Sphere", "Shadowblood Egg":
					return o
				}
			}
		}
		return inner.Pick(p)
	})
}

// TestManaRiderDepletionReprosReach replays the recorded games: no
// divergence, every scripted action offered, the card used up.
func TestManaRiderDepletionReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range manaRiderDepletionCases() {
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
				t.Fatal("the recorded game never uses the card up")
			}
		})
	}
}

// TestManaRiderDepletionRecord plays the cases' policies over seeds until one
// reaches the sacrifice, and writes the repros. It runs only with
// MANA_RIDER_DEPLETION_RECORD=1.
func TestManaRiderDepletionRecord(t *testing.T) {
	if os.Getenv("MANA_RIDER_DEPLETION_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range manaRiderDepletionCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 300; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, manaRiderDepletionPolicy(m, c, seed))
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

// TestManaRiderDepletionSeeds plays each case's policy over seeds 1..30 on
// both option surfaces' full form: no divergence, and the card is used up in
// at least one of them (so the games are not all idle).
func TestManaRiderDepletionSeeds(t *testing.T) {
	reg := synthReg(t)
	for _, c := range manaRiderDepletionCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			reachedN := 0
			for seed := uint64(1); seed <= 30; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, manaRiderDepletionPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if reached {
					reachedN++
				}
			}
			if reachedN == 0 {
				t.Fatal("no seed used the card up")
			}
		})
	}
}
