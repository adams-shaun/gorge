package oracle

import (
	"sort"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Pump Duration$ AsLongAsControl cards the host-anchored override record
// lowers (dirac/filter_combatwords.go): Kyodai Soul of Kamigawa, Aegis Angel
// and Thancred Waters grant indestructible "for as long as you control
// CARDNAME", Possession Engine takes the creature it gained and holds it from
// combat while its controller keeps the Engine, There and Back Again's last
// chapter stops a creature from blocking while the Saga stays. The lockstep
// plays each card's synthetic pool: gorge and the kernel must agree on the
// grant and on its lifetime (the host's control change and phase-out end it).
var kpumpCtrlCards = []string{
	"Kyodai, Soul of Kamigawa", "Aegis Angel", "Thancred Waters", "Possession Engine", "There and Back Again",
}

// kpumpCtrlPool finds the synthetic pool that plays name, loading each pool
// once.
var kpumpCtrlMu sync.Mutex

func kpumpCtrlPool(t *testing.T, name string, loaded map[string]cardset.Set) cardset.Set {
	t.Helper()
	kpumpCtrlMu.Lock()
	defer kpumpCtrlMu.Unlock()
	if s, ok := loaded[name]; ok {
		return s
	}
	names, err := cardset.SynthNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, pn := range names {
		set := cardsettest.Synth(t, pn)
		loaded[pn] = set
		if i, ok := set.Lookup(name); ok && set.Supported()[i] {
			t.Logf("%s plays from pool %s", name, pn)
			return set
		}
	}
	t.Fatalf("%s is in no synthetic pool", name)
	return nil
}

// kpumpCtrlDeck is count copies of name over a five-colour basic mix that
// fills the deck to 40 cards.
func kpumpCtrlDeck(name string, count int) []cardset.Entry {
	d := []cardset.Entry{{Name: name, Count: count}}
	basics := []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	per, rem := (40-count)/5, (40-count)%5
	for i, b := range basics {
		n := per
		if i < rem {
			n++
		}
		d = append(d, cardset.Entry{Name: b, Count: n})
	}
	return d
}

// kpumpCtrlCompanion is the pool's cheapest castable creature (deterministic:
// corpus order, then name order), for Possession Engine's opponent: its enters
// trigger targets a creature an opponent controls, and the pump anchors to the
// Engine.
func kpumpCtrlCompanion(t *testing.T, reg *cards.Registry, set cardset.Set) string {
	t.Helper()
	var cands []string
	for _, c := range reg.AllCards() {
		f := c.Faces[0]
		if f == nil || !f.IsCreature() {
			continue
		}
		mv, ok := f.PrintedManaValue()
		if !ok || mv > 2 || mv == 0 {
			continue
		}
		cands = append(cands, f.Name)
	}
	sort.Strings(cands)
	for _, nm := range cands {
		if i, ok := set.Lookup(nm); ok && set.Supported()[i] {
			return nm
		}
	}
	return ""
}

func kpumpCtrlRun(t *testing.T, cn string, set cardset.Set, decks [2][]cardset.Entry, reg *cards.Registry) (divs, stalls int) {
	t.Helper()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var corp [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				corp[s] = append(corp[s], c)
			}
		}
	}
	for seed := uint64(1); seed <= 6; seed++ {
		m, err := NewMatchup(reg, [2]string{"pumpctrl-a", "pumpctrl-b"}, corp, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
			stalls++
		} else if res.Stall != "" && res.Stall != "turns" {
			t.Logf("seed %d: stalled: %s", seed, res.Stall)
			stalls++
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			break
		}
	}
	return divs, stalls
}

func TestPumpControlLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	loaded := map[string]cardset.Set{}
	for _, cn := range kpumpCtrlCards {
		cn := cn
		t.Run(cn, func(t *testing.T) {
			t.Parallel()
			set := kpumpCtrlPool(t, cn, loaded)
			decks := [2][]cardset.Entry{kpumpCtrlDeck(cn, 2), kpumpCtrlDeck(cn, 0)}
			if cn == "Possession Engine" {
				comp := kpumpCtrlCompanion(t, reg, set)
				if comp == "" {
					t.Skip("no cheap creature in the Engine's pool")
				}
				decks[1] = kpumpCtrlDeck(comp, 4)
			}
			divs, stalls := kpumpCtrlRun(t, cn, set, decks, reg)
			t.Logf("%s: 6 seeds, %d divergent, %d stalled", cn, divs, stalls)
		})
	}
}
