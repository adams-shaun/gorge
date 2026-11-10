package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// The TapPowerValue static lockstep: Tapestry Warden makes a power<toughness
// creature station with its toughness (Biotech Specialist, 1/3, puts three
// charge counters on Entropic Battlecruiser), and Sulu, Heroic Helmsman
// stations a Spacecraft with his tripled power (six counters on U.S.S.
// Prometheus, Prototype). TestStdSetReprosLockstep replays both recorded
// games; TestTapPowerStationReprosReach also requires each script to reach
// the station amount its card changed.
//
// The games are recorded by TestTapPowerStationRecord
// (TAPPOWER_STATION_RECORD=1) on the std-eoe and std-trc synth pools.

type tapPowerStationCase struct {
	name     string
	set      string
	card     string
	prefer   []string
	decks    [2][]string
	chargeOn int
	tapped   string
}

func tapPowerStationCases() []tapPowerStationCase {
	return []tapPowerStationCase{
		{
			name:   "tapestry_warden_stations_with_toughness",
			set:    "std-eoe",
			card:   "Tapestry Warden",
			prefer: []string{"Tapestry Warden", "Entropic Battlecruiser", "Biotech Specialist"},
			decks: [2][]string{
				append(deckOf(4, "Tapestry Warden"), append(deckOf(4, "Biotech Specialist"), append(deckOf(4, "Entropic Battlecruiser"), append(deckOf(14, "Forest"), deckOf(14, "Swamp")...)...)...)...),
				deckOf(40, "Forest"),
			},
			chargeOn: 4,
			tapped:   "Tapestry Warden",
		},
		{
			name:   "sulu_stations_a_spacecraft_with_tripled_power",
			set:    "std-trc",
			card:   "Sulu, Heroic Helmsman",
			prefer: []string{"Sulu, Heroic Helmsman", "U.S.S. Prometheus, Prototype"},
			decks: [2][]string{
				append(deckOf(4, "Sulu, Heroic Helmsman"), append(deckOf(4, "U.S.S. Prometheus, Prototype"), deckOf(32, "Arcane Sanctum")...)...),
				deckOf(40, "Arcane Sanctum"),
			},
			chargeOn: 6,
			tapped:   "Sulu, Heroic Helmsman",
		},
	}
}

// stationAfterTap reports whether the named creature tapped and a station
// resolution then put n charge counters on a permanent.
func stationAfterTap(tapped string, n int) func(*GorgeSide) bool {
	return func(g *GorgeSide) bool {
		evs := g.E.L.Events
		for i, e := range evs {
			if e.Kind != events.Tap {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Face() == nil || o.Face().Name != tapped {
				continue
			}
			for j := i + 1; j < len(evs) && j <= i+20; j++ {
				if evs[j].Kind == events.CounterChange && evs[j].Counter == "CHARGE" && int(evs[j].Amount) == n {
					return true
				}
			}
		}
		return false
	}
}

func tapPowerStationPolicy(m *Matchup, c tapPowerStationCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, want := range c.prefer {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActSpecial || o.Kind() == ActActivate {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

func TestTapPowerStationReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range tapPowerStationCases() {
		t.Run(c.name, func(t *testing.T) {
			set := cardsettest.Synth(t, c.set)
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			ec := effectLongCase{name: c.name, prefer: c.prefer, decks: c.decks, surface: "autopay",
				reached: stationAfterTap(c.tapped, c.chargeOn)}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, ec, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never tapped %s for %d charge counters", c.tapped, c.chargeOn)
			}
		})
	}
}

func TestTapPowerStationRecord(t *testing.T) {
	if os.Getenv("TAPPOWER_STATION_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range tapPowerStationCases() {
		t.Run(c.name, func(t *testing.T) {
			set := cardsettest.Synth(t, c.set)
			ec := effectLongCase{name: c.name, prefer: c.prefer, decks: c.decks, surface: "autopay",
				reached: stationAfterTap(c.tapped, c.chargeOn)}
			for seed := uint64(1); seed <= 300; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
					Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "synth " + c.set}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, ec, r, set, tapPowerStationPolicy(m, c, seed))
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
