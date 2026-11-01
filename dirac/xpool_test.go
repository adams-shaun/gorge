package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXAskCountsPoolAndWindow: with {R}{R}{R} floating (Chandra, Flameshaper's
// +2) and three Mountains untapped, Goblin Negotiation's X ask reaches what the
// pool and the window's sources pay together (CR 601.2b, 601.2g; rules
// cast_subcounter.go xAsk).
func TestXAskCountsPoolAndWindow(t *testing.T) {
	deck := []cardset.Entry{{Name: "Goblin Negotiation", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Mountain", Count: 8}}
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
	for i := 0; i < 3; i++ {
		sc.perm(id["Mountain"], 0)
	}
	sc.perm(id["Savannah Lions"], 1)
	s.pl[0].pool[kop.R] = 3
	gn := sc.hand(id["Goblin Negotiation"], 0)
	sc.do(t, AkCast, uint8(gn), 0xff)
	for i := 0; i < 6; i++ {
		d := s.Step()
		if d.Kind == DkChoose && d.Sub == ChX {
			if d.N != 5 || d.Options[4].Mode() != 4 {
				t.Fatalf("X options %v, want X = 0 to 4", d.Options[:d.N])
			}
			return
		}
		if d.Kind == DkNone {
			t.Fatal("the game ended")
		}
		s.Apply(d.Options[0])
	}
	t.Fatal("no X ask")
}

// TestTargetReducedFlashbackNeedsPool: Luminous Rebuke granted flashback
// (Sphinx of Forgotten Lore) costs {1}{W} against a tapped creature, but
// gorge's planners never price a target-conditional reduction
// ("shape:target_dependent_cost"); its potential planner prices the full
// {4}{W} only. So under gorge's payment surfaces the cast four untapped
// Plains pay only through the reduction is offered only when the floating
// pool pays it (pair 13571). Before, the kernel offered it from the lands.
func TestTargetReducedFlashbackNeedsPool(t *testing.T) {
	deck := []cardset.Entry{{Name: "Luminous Rebuke", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Plains", Count: 8}}
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
	for _, pool := range []bool{false, true} {
		sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
		s := sc.s
		for i := 0; i < 4; i++ {
			sc.perm(id["Plains"], 0)
		}
		lions := sc.perm(id["Savannah Lions"], 1)
		s.perms[sc.slot(lions)].flags |= pfTapped
		lr := sc.obj(id["Luminous Rebuke"], 0)
		s.place(lr, kop.ZoneGraveyard, false)
		s.grantFB(sc.k, lr)
		if pool {
			s.pl[0].pool[kop.W] = 2
		}
		d := s.Step()
		got := false
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkCast && a.Source() == uint8(lr) && a.Mode()&2 != 0 {
				got = true
			}
		}
		if got != pool {
			t.Fatalf("pool {W}{W} floating %v: flashback Luminous Rebuke offered %v, want %v", pool, got, pool)
		}
	}
}

// TestGreatwurmOwnPermissionUnpayableNotAmbiguous: with no counters among
// its creatures, Quilled Greatwurm's own graveyard permission cannot be
// used, so Muldrotha's creature permission is the only one and gorge's
// planner prices the cast (not "ambiguous"). Before, the kernel counted
// both permissions and offered the cast only when the pool paid it (pair
// 20520).
func TestGreatwurmOwnPermissionUnpayableNotAmbiguous(t *testing.T) {
	deck := []cardset.Entry{{Name: "Quilled Greatwurm", Count: 1}, {Name: "Muldrotha, the Gravetide", Count: 1}, {Name: "Forest", Count: 8}}
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
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true, GorgeUnpriced: true})
	s := sc.s
	for i := 0; i < 6; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Muldrotha, the Gravetide"], 0)
	qg := sc.obj(id["Quilled Greatwurm"], 0)
	s.place(qg, kop.ZoneGraveyard, false)
	d := s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == uint8(qg) {
			return
		}
	}
	t.Fatalf("the Greatwurm's cast through Muldrotha is not offered: %v", d.Options[:d.N])
}

// TestScriptSearchCountsXCast: gorge searches a scripted prefix (Bloom
// Tender's mana) for the first four insufficient potential plays in walk
// order only; an {X} cast (Goblin Negotiation) is one of them although the
// autopay surface never offers it. Here the hand's four casts take the
// four searches, so Goldvein Pick's equip is left "census_incomplete" and
// not offered (pair 16186). Before, the kernel's walk skipped the {X} cast
// and searched the equip.
func TestScriptSearchCountsXCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Bloom Tender", Count: 1}, {Name: "Goldvein Pick", Count: 1}, {Name: "Tatyova, Benthic Druid", Count: 1}, {Name: "Gorehorn Raider", Count: 1},
		{Name: "Goblin Negotiation", Count: 1}, {Name: "Axgard Cavalry", Count: 1}, {Name: "Mountain", Count: 4}}
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
	for _, withX := range []bool{false, true} {
		sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true, GorgeUnpriced: true})
		s := sc.s
		sc.perm(id["Bloom Tender"], 0)
		pick := sc.perm(id["Goldvein Pick"], 0)
		for _, n := range []string{"Tatyova, Benthic Druid", "Gorehorn Raider", "Goblin Negotiation", "Axgard Cavalry"} {
			if n == "Goblin Negotiation" && !withX {
				continue
			}
			sc.hand(id[n], 0)
		}
		d := s.Step()
		got := false
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkActivate && a.Source() == uint8(pick) {
				got = true
			}
		}
		if got == withX {
			t.Fatalf("with the {X} cast in hand %v: equip offered %v, want %v", withX, got, !withX)
		}
	}
}
