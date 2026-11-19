package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// The Shadowmoor/Eventide filter lands' "{W/B}, {T}: Add {W}{W}, {W}{B}, or
// {B}{B}" beside "{T}: Add {C}" (Fetid Heath), played decision by decision
// against gorge on the table built from the repro's own decks (pool
// "custom"). Gorge's census funds a hybrid fee like any other once-only paid
// gap: the prefix search taps a covered source, floats its mana, and the
// Heath's {W/B} fee then makes {W}{W} or {B}{B}, whose ask lists {C} then the
// Combo's one option per colour of the amount. A cast that needs a Heath's
// filter is therefore offered, and the recorded game reaches Aron, Benalia's
// Ruin ({W}{W}{B}) off a Heath's {B}{B} after another source's {W} pays the
// fee.
//
// TestFilterLandLockstep replays the recorded game and requires a black spell
// to be cast straight after a Heath's {B}{B} (blackOffAHeath): a lockstep entry
// alone would pass a game that never activated a filter. TestFilterLandRecord
// (PINS_RECORD=1) plays the case's policy over seeds until one reaches
// it and writes the repro.
type filterLandCase struct {
	effectLongCase
	float []string // the lands the policy floats, in order
}

func filterLandCases() []filterLandCase {
	a := append(deckOf(3, "Aron, Benalia's Ruin"), append(deckOf(3, "Vindicate"), append(deckOf(2, "Exsanguinate"), append(deckOf(8, "Fetid Heath"), append(deckOf(4, "Plains"), deckOf(4, "Swamp")...)...)...)...)...)
	return []filterLandCase{
		{effectLongCase{
			name:    "filter_land_fetid_heath_pays_wwb",
			prefer:  []string{"Aron, Benalia's Ruin", "Vindicate", "Exsanguinate"},
			decks:   [2][]string{a, deckOf(24, "Plains")},
			surface: "full",
			reached: twoOffAFilter("B"),
		}, []string{"Plains", "Fetid Heath"}},
	}
}

// twoOffAFilter: a spell was cast with a seat-0 filter's two mana of col the
// latest col mana added (no basic land's after it) and nothing else put on the
// stack between, so the spell's mana of col came out of the filter, its
// hybrid fee a floated mana of the other colour.
func twoOffAFilter(col string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		filtered := false
		evs := g.E.L.Events
		for i := range evs {
			e := &evs[i]
			switch {
			case e.Kind == events.ManaAdd && e.Player == 0 && e.Counter == col:
				filtered = e.Amount == 2
			case e.Kind == events.PutOnStack:
				if filtered {
					return true
				}
			}
		}
		return false
	}
}

func TestFilterLandLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range filterLandCases() {
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
				t.Fatal("the recorded game never casts a spell off a filter's two mana")
			}
		})
	}
}

func TestFilterLandRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range filterLandCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 150; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, filterLandPolicy(m, c, seed))
				if res.Div != nil {
					for i := max(0, res.Div.At-6); i < res.Div.At && i < len(res.Actions); i++ {
						a := res.Actions[i]
						t.Logf("  action %d: %v %s", i, a, m.InstanceCard(int(a.Src())))
					}
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					t.Logf("seed %d: %d decisions, not reached", seed, len(res.Actions))
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

// filterLandPolicy casts the preferred card whenever offered, plays lands,
// floats mana from the Plains before the Heaths, and answers anything else at
// random.
func filterLandPolicy(m *Matchup, c filterLandCase, seed uint64) Policy {
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
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, name := range c.float {
				for _, o := range p.D.Options {
					if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == name {
						return o
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}
