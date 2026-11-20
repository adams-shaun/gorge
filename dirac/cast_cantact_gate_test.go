package dirac

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// cantActByAll is actForbiddenBy's old body: eachStatic over every source.
// The per-card gate (pcard.cantAct) must return the same verdict for every
// source, activator, mana/loyalty pair and condition.
func cantActByAll(s *State, k *prep, sl Slot, p uint8, mana, loyal bool) bool {
	if mana && DebugCantActMana {
		return false
	}
	hit := false
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantActivate {
			return true
		}
		st := &a.Static
		if st.StFlags&kop.StRaiseYou != 0 && stc.ctrl != p || st.StFlags&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		if mana && st.A&3 == 1 || st.A&3 == 2 && !loyal {
			return true
		}
		if st.A&kop.CantActNamed != 0 {
			if c, ok := s.chosenNameOf(stc.sl); !ok || s.nameCardAt(sl) != s.nameKey(c) {
				return true
			}
		}
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, s.ctxStatic(stc)) {
			return true
		}
		if s.matchPermGen(k, st.Filter, sl, stc.ctrl, stc.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// cantActDeck is a deterministic deck list for the gate test, drawn from the
// cantact pool (every CantBeActivated carrier beside cards with activated
// abilities), 23 spells and three basics.
func cantActDeck(set cardset.Set, n int, seed uint64) [][]cardset.Entry {
	r := rand.New(rand.NewPCG(seed, 99))
	sup := set.Supported()
	var spells []int
	for i := 0; i < set.NumCards() && i < len(sup); i++ {
		if !sup[i] || set.Inert(i) {
			continue
		}
		if set.Def(i).Types&kop.TLand != 0 {
			continue
		}
		spells = append(spells, i)
	}
	var out [][]cardset.Entry
	for len(out) < n {
		cnt := map[string]int{}
		for k := 0; k < 23; k++ {
			cnt[set.Names()[spells[r.IntN(len(spells))]]]++
		}
		var es []cardset.Entry
		keys := make([]string, 0, len(cnt))
		for k := range cnt {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			es = append(es, cardset.Entry{Name: k, Count: cnt[k]})
		}
		es = append(es, cardset.Entry{Name: "Plains", Count: 6}, cardset.Entry{Name: "Island", Count: 6}, cardset.Entry{Name: "Forest", Count: 5})
		out = append(out, es)
	}
	return out
}

// checkCantActTable asserts the per-card flag matches the card's printed
// StatCantActivate (the same abilities actForbiddenBy's scan reads).
func checkCantActTable(t *testing.T, tbl *Table, k *prep, wantTrue bool) (nTrue int) {
	t.Helper()
	for c := range tbl.Cards {
		has := false
		for j := range tbl.Cards[c].Abilities {
			a := &tbl.Cards[c].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantActivate {
				has = true
				break
			}
		}
		if got := k.cards[c].cantAct; got != has {
			t.Fatalf("%s: pcard.cantAct = %v, printed statics say %v", tbl.Cards[c].Name, got, has)
		}
		if has {
			nTrue++
		}
	}
	if wantTrue && nTrue == 0 {
		t.Fatal("precondition: the table has no card with a StatCantActivate")
	}
	return nTrue
}

// checkCantActState compares the gated verdict with the old full scan for
// every source in force, every activator, and both mana/loyalty pairs.
func checkCantActState(t *testing.T, s *State, k *prep, dec int) (checks, hits int) {
	t.Helper()
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		for p := uint8(0); p < s.nSeats(); p++ {
			for _, mana := range [2]bool{false, true} {
				for _, loyal := range [2]bool{false, true} {
					got := s.actForbiddenBy(k, sl, p, mana, loyal)
					want := cantActByAll(s, k, sl, p, mana, loyal)
					if got != want {
						t.Fatalf("decision %d: source %s (slot %d) p %d mana %v loyal %v: gated %v, full scan %v",
							dec, k.t.Cards[s.cardAt(sl)].Name, sl, p, mana, loyal, got, want)
					}
					checks++
					if got {
						hits++
					}
				}
			}
		}
	}
	return checks, hits
}

func TestCantActGateEquivalence(t *testing.T) {
	oldMana, oldCount := DebugCantActMana, DebugCantActCount
	DebugCantActMana, DebugCantActCount = false, false
	defer func() { DebugCantActMana, DebugCantActCount = oldMana, oldCount }()

	set := cardsettest.Set(t, "cantact")
	decks := cantActDeck(set, 64, 7)
	const games = 8
	var checks, hits int
	for g := 0; g < games; g++ {
		a := decks[(g*7919)%len(decks)]
		b := decks[(g*104729+31)%len(decks)]
		tbl, da, db, err := set.Matchup(a, b)
		if err != nil {
			t.Fatalf("game %d: %v", g, err)
		}
		s := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
		k := s.k()
		checkCantActTable(t, tbl, k, false)
		pol := NewRandomPolicy(uint64(g)*7919 + 3)
		var d Decision
		for n := 0; n < 1500; n++ {
			s.StepInto(&d)
			c, h := checkCantActState(t, s, k, n)
			checks += c
			hits += h
			if d.Kind == DkNone {
				break
			}
			s.Apply(pol.Pick(&d))
		}
		ReleaseTable(tbl)
	}
	if checks == 0 || hits == 0 {
		t.Fatalf("precondition: %d checks, %d forbidding verdicts", checks, hits)
	}
	t.Logf("%d games, %d checks, %d forbidding verdicts", games, checks, hits)
}

// TestCantActGatePositiveControl puts Grand Abolisher's lock on one seat and
// Prodigal Sorcerer's activated ability on the other, so both paths must
// say the activation is forbidden; a gate that skips the lock's card shows here.
func TestCantActGatePositiveControl(t *testing.T) {
	set := cardsettest.Set(t, "cantact")
	deck := []cardset.Entry{
		{Name: "Grand Abolisher", Count: 1},
		{Name: "Prodigal Sorcerer", Count: 1},
		{Name: "Plains", Count: 30},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseTable(tbl)
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.s.active, sc.s.turn = 0, 1
	sc.perm(id["Grand Abolisher"], 0)
	ss := sc.perm(id["Prodigal Sorcerer"], 1)
	sl := sc.slot(ss)
	k := sc.k
	if !cantActByAll(sc.s, k, sl, 1, false, false) {
		t.Fatal("precondition: the full scan finds no lock on the opponent's activation")
	}
	if !sc.s.actForbiddenBy(k, sl, 1, false, false) {
		t.Fatal("gated scan misses the lock (Grand Abolisher's card flag)")
	}
	if sc.s.actForbiddenBy(k, sl, 0, false, false) {
		t.Fatal("gated scan forbids the lock's own controller")
	}
}
