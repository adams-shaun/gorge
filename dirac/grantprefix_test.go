package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGrantedManaPrefixBeforeTargets: Imprisoned in the Moon's "{T}: Add
// {C}" is a gap in gorge's planner census (it holds printed mana abilities
// only), so Rogue's Passage's {4} with three printed sources is a scripted
// potential play: the gap is activated first and its {C} floats when the
// ability asks its target (rules potentialPrefixScript; pairs 344 480 671
// and 20 more in the 10k slice). Before, the kernel tapped nothing until
// the payment.
func TestGrantedManaPrefixBeforeTargets(t *testing.T) {
	deck := []cardset.Entry{{Name: "Rogue's Passage", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	s := sc.s
	var isl []Obj
	for i := 0; i < 3; i++ {
		isl = append(isl, sc.perm(id["Island"], 0))
	}
	pass := sc.perm(id["Rogue's Passage"], 0)
	gap := sc.perm(id["Island"], 0)
	moon := sc.perm(id["Imprisoned in the Moon"], 0)
	s.perms[sc.slot(moon)].attached = sc.slot(gap)
	s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(gap))
	sc.perm(id["Eager Trufflesnout"], 0)
	d := s.Step()
	found := false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(pass)) && a.Mode() == 1 {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Rogue's Passage's {4},{T} not offered: %v", d.Options[:d.N])
	}
	d = s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	if !sc.tapped(gap) || s.pl[0].pool[kop.C] != 1 {
		t.Fatalf("at the target ask: imprisoned Island tapped=%v, pool {C}=%d; want the gap tapped and {C} floating", sc.tapped(gap), s.pl[0].pool[kop.C])
	}
	if n := tappedN(sc, isl); n != 0 {
		t.Fatalf("%d printed Islands tapped before the payment, want 0", n)
	}
}

// TestGrantedManaAfterTreasure: gorge's planner never sees a granted mana
// ability, so with the Island and a Treasure able to pay {1}{U} it
// sacrifices the Treasure and leaves the imprisoned land untapped (pairs
// 442 3178 4489 7902). Before, the kernel ranked the granted {C} (no
// irreversible cost) above the Treasure.
func TestGrantedManaAfterTreasure(t *testing.T) {
	deck := []cardset.Entry{{Name: "Goldvein Pick", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tre := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "c_a_treasure_sac" || tbl.Cards[i].Name == "Treasure" {
			tre = CardID(i)
		}
	}
	if tre == 255 || int(v.Plains) >= len(tbl.Cards) {
		t.Skip("no Treasure token in the table, or the table is too small")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	isl := sc.perm(id["Island"], 0)
	gap := sc.perm(id["Island"], 0)
	moon := sc.perm(id["Imprisoned in the Moon"], 0)
	s.perms[sc.slot(moon)].attached = sc.slot(gap)
	s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(gap))
	tr := sc.obj(tre, 0)
	s.objs[tr].of |= ofToken
	sc.enterAs(tr, 0)
	c := kop.ManaCost{Generic: 1}
	c.Pips[kop.U] = 1
	s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(isl) || sc.tapped(gap) || sc.onBF(tr) {
		t.Fatalf("Island tapped=%v imprisoned land tapped=%v Treasure on the battlefield=%v; want the Island and the Treasure", sc.tapped(isl), sc.tapped(gap), sc.onBF(tr))
	}
}

// TestGrantedManaScriptCap: gorge searches a granted-source prefix for the
// first potentialScriptPlays plays its printed sources cannot pay (rules
// PotentialPaymentPlans), in walk order: with that many {2}{B} Gutless
// Plunderers in hand ahead of it, Squad Rallier's {2}{W} that only the
// imprisoned land pays is not offered; with one fewer it is (pair 5618).
// Before, the kernel offered it either way.
func TestGrantedManaScriptCap(t *testing.T) {
	deck := []cardset.Entry{{Name: "Squad Rallier", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Gutless Plunderer", Count: potentialScriptPlays}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	for _, tc := range []struct {
		plunderers int
		offered    bool
	}{{potentialScriptPlays - 1, true}, {potentialScriptPlays, false}} {
		sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
		s := sc.s
		for i := 0; i < tc.plunderers; i++ {
			sc.hand(id["Gutless Plunderer"], 0)
		}
		sc.perm(id["Plains"], 0)
		sc.perm(id["Swamp"], 0)
		gap := sc.perm(id["Swamp"], 0)
		moon := sc.perm(id["Imprisoned in the Moon"], 1)
		s.perms[sc.slot(moon)].attached = sc.slot(gap)
		s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(gap))
		ral := sc.perm(id["Squad Rallier"], 0)
		d := s.Step()
		got := false
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkActivate && a.Source() == TCode(uint8(ral)) {
				got = true
			}
		}
		if got != tc.offered {
			t.Errorf("%d Plunderers: Squad Rallier offered %v, want %v (%v)", tc.plunderers, got, tc.offered, d.Options[:d.N])
		}
	}
}

// TestGrantedManaKickedCast: under the autopay surface gorge leaves out the
// plain Grow from the Ashes that only imprisoned lands pay, but offers the
// kicked cast as a lowered potential play (pair 8638). Before, the kernel
// skipped the kicked cast whenever the plain one was left out.
func TestGrantedManaKickedCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Grow from the Ashes", Count: 1}, {Name: "Imprisoned in the Moon", Count: 4}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	s := sc.s
	g := sc.hand(id["Grow from the Ashes"], 0)
	sc.perm(id["Forest"], 0)
	for i := 0; i < 4; i++ {
		gap := sc.perm(id["Forest"], 0)
		moon := sc.perm(id["Imprisoned in the Moon"], 1)
		s.perms[sc.slot(moon)].attached = sc.slot(gap)
		s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(gap))
	}
	d := s.Step()
	plain, kicked := false, false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(g)) {
			if a.Mode()&1 != 0 {
				kicked = true
			} else {
				plain = true
			}
		}
	}
	if plain || !kicked {
		t.Fatalf("plain offered %v kicked offered %v, want only the kicked cast: %v", plain, kicked, d.Options[:d.N])
	}
}

// TestGrantedArtifactManaKeepsPlan: an imprisoned artifact's {C} is typed
// (Artifact) mana in gorge (effects ManaProducerTag reads the printed face), and
// typed mana sits in its colour's bucket for the planner (pay.PlanPool). So after
// the gap's prefix the play stays planned: only the gap is tapped before the
// target ask, and the printed sources wait for the payment (Sower of Chaos with
// an imprisoned Campus Guide).
func TestGrantedArtifactManaKeepsPlan(t *testing.T) {
	deck := []cardset.Entry{{Name: "Rogue's Passage", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Campus Guide", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	s := sc.s
	var isl []Obj
	for i := 0; i < 3; i++ {
		isl = append(isl, sc.perm(id["Island"], 0))
	}
	pass := sc.perm(id["Rogue's Passage"], 0)
	gap := sc.perm(id["Campus Guide"], 0)
	moon := sc.perm(id["Imprisoned in the Moon"], 0)
	s.perms[sc.slot(moon)].attached = sc.slot(gap)
	s.loseAllOnset(sc.k, sc.slot(moon), sc.slot(gap))
	sc.perm(id["Eager Trufflesnout"], 0)
	d := s.Step()
	found := false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(pass)) && a.Mode() == 1 {
			s.Apply(a)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Rogue's Passage's {4},{T} not offered: %v", d.Options[:d.N])
	}
	d = s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	if !sc.tapped(gap) || s.pl[0].pool[kop.C] != 1 || s.pl[0].pool[kop.U] != 0 {
		t.Fatalf("at the target ask: imprisoned Campus Guide tapped=%v, pool %v; want {C} floating", sc.tapped(gap), s.pl[0].pool)
	}
	if n := tappedN(sc, isl); n != 0 {
		t.Fatalf("%d printed Islands tapped before the targets, want 0 (the plan pays after them)", n)
	}
}

// TestGrantedFlashbackInstanceLowered: gorge never plans the granted
// flashback instance beside a printed one (pair 1841), so the oracle
// lowers it: Bulk Up's granted {1}{R} has both Mountains tapped into the
// pool before its target is asked (pair 87192). Before, beginCast asked
// FeatPlannedCast after castFlagsGen had ended the grant, read the
// printed instance as planned, and tapped nothing until the payment.
func TestGrantedFlashbackInstanceLowered(t *testing.T) {
	deck := []cardset.Entry{{Name: "Bulk Up", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Mountain", Count: 8}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	m := [2]Obj{sc.perm(id["Mountain"], 0), sc.perm(id["Mountain"], 0)}
	sc.perm(id["Eager Trufflesnout"], 0)
	bu := sc.obj(id["Bulk Up"], 0)
	sc.s.place(bu, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, bu)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(bu)) && a.Mode()&2 != 0 {
			sc.s.Apply(a)
			if d = sc.s.Step(); d.Kind != DkTarget {
				t.Fatalf("decision kind %d, want the target ask", d.Kind)
			}
			if !sc.tapped(m[0]) || !sc.tapped(m[1]) || sc.s.pl[0].pool[kop.R] != 2 {
				t.Fatalf("at the target ask: Mountains tapped %v %v, pool %v; want both tapped, {R}{R} floating", sc.tapped(m[0]), sc.tapped(m[1]), sc.s.pl[0].pool)
			}
			return
		}
	}
	t.Fatalf("the granted {1}{R} flashback is not offered: %v", d.Options[:d.N])
}
