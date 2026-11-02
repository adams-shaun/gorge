package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Station self-grant lockstep (CR 702.184, 613.1f, 603.4;
// ): EOE Spacecraft whose STATION N+ line grants the
// Spacecraft itself a triggered ability (AddTrigger$) or a static ability
// (AddStaticAbility$) while it holds N charge counters. gorge queues the
// grant while the outer static is live; a granted trigger is matched when
// its event happens and is not rechecked on resolution. The games reach:
//
//   - the granted triggers resolving (attack, begin combat, attackers
//     declared) once the Spacecraft holds its counters;
//   - the events those triggers watch happening while the Spacecraft is
//     below its threshold (no trigger);
//   - Lumen-Class Frigate's granted anthem live in combat.
//
// Entropic Battlecruiser lowers its Discarded and Attacks grants but its
// STATION 8+ trigger body (a TgtChoose discard, Defined$
// OppNonRememberedController) still refuses, so the card stays out of the
// decks.

var kstationgrantDecks = [2][]cardset.Entry{
	{
		{Name: "Sledge-Class Seedship", Count: 3}, {Name: "Synthesizer Labship", Count: 3}, {Name: "Specimen Freighter", Count: 2},
		{Name: "Dawnsire, Sunstar Dreadnought", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Lumen-Class Frigate", Count: 3}, {Name: "The Seriema", Count: 3}, {Name: "Infinite Guideline Station", Count: 2},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Llanowar Elves", Count: 1}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 2},
		{Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Dawnsire, Sunstar Dreadnought", Count: 3},
	},
}

// kstationgrantThreshold is each gated carrier's granted-trigger threshold.
var kstationgrantThreshold = map[string]int32{
	"Sledge-Class Seedship": 7, "Synthesizer Labship": 2, "Specimen Freighter": 9,
	"Dawnsire, Sunstar Dreadnought": 10, "Infinite Guideline Station": 12,
}

type kstationgrantTally struct {
	evAt      int
	granted   int             // resolutions of a granted trigger (a carrier's resolving trigger with its counters)
	carriers  map[string]bool // carriers whose granted trigger resolved
	below     int             // watched events (begin combat, attackers declared) with a gated carrier below its threshold
	frigate   int             // beginning-of-combat steps with Lumen-Class Frigate's anthem live
	attackers int             // attacks by a Spacecraft with its granted trigger live
}

func (w *kstationgrantTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < w.evAt {
		w.evAt = 0
	}
	for j := w.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Resolve:
			o := g.Obj(e.Obj)
			if o == nil || o.Ability == nil {
				continue
			}
			src := g.Obj(o.Source)
			if src == nil || src.Zone != state.ZBattlefield {
				continue
			}
			f := src.Face()
			if f == nil {
				continue
			}
			if n, ok := kstationgrantThreshold[f.Name]; ok && src.Counter("CHARGE") >= n {
				w.granted++
				w.carriers[f.Name] = true
			}
		case events.DeclareAttackers:
			for _, id := range e.IDs {
				if o := g.Obj(id); o != nil && o.Face() != nil {
					if n, ok := kstationgrantThreshold[o.Face().Name]; ok && o.Counter("CHARGE") >= n {
						w.attackers++
					}
				}
			}
			fallthrough
		case events.StepChange:
			if e.Kind == events.StepChange && e.Step != state.StepBeginCombat {
				continue
			}
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					o := g.Obj(id)
					if o == nil || o.Face() == nil || o.Controller != g.Active {
						continue
					}
					name := o.Face().Name
					n, ok := kstationgrantThreshold[name]
					c := o.Counter("CHARGE")
					switch {
					case e.Kind == events.StepChange && name == "Lumen-Class Frigate" && c >= 2:
						w.frigate++
					case ok && c < n && (e.Kind == events.StepChange && name == "Synthesizer Labship" ||
						e.Kind == events.DeclareAttackers && name == "Dawnsire, Sunstar Dreadnought"):
						w.below++
					}
				}
			}
		}
	}
	w.evAt = len(evs)
	return nil
}

func TestStationgrantLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stationgrant")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kstationgrantDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kstationgrantDecks[0], kstationgrantDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	// DebugStationGrantNoGate is the mutation: a test that sets it must fail
	_ = dirac.DebugStationGrantNoGate
	w := &kstationgrantTally{carriers: map[string]bool{}}
	games, stalls := 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstationgrant-a", "kstationgrant-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		w.evAt = 0
		pol := stationPolicy(seed)
		if seed%4 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 25, Check: w.check})
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d granted-trigger resolutions over %d carriers, %d attacks with the grant live, %d below-threshold watched events, %d Frigate anthem combats, %d stalls",
		games, w.granted, len(w.carriers), w.attackers, w.below, w.frigate, stalls)
	if stalls != 0 {
		t.Errorf("%d games stalled", stalls)
	}
	if w.granted < 30 || len(w.carriers) < 4 {
		t.Errorf("%d granted-trigger resolutions over %d carriers, want at least 30 over 4", w.granted, len(w.carriers))
	}
	if w.below < 10 {
		t.Errorf("%d below-threshold watched events, want at least 10", w.below)
	}
	if w.frigate < 20 {
		t.Errorf("%d Frigate anthem combats, want at least 20", w.frigate)
	}
}
