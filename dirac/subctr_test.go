package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGreatwurmAsksEachCounter: Quilled Greatwurm's graveyard cast
// "by removing six counters from among creatures you control" is chosen
// one counter at a time, as gorge asks it (rules cast_subcounter.go
// wildcardCounterAsk: a KChoose "subcounter" over every (creature,
// counter kind) unit left, a lone unit taken unasked), and the counters
// stay on the creatures until the cast is paid. Before, the kernel removed
// six counters itself at once, in slot order (pair 7476).
func TestGreatwurmAsksEachCounter(t *testing.T) {
	deck := []cardset.Entry{{Name: "Quilled Greatwurm", Count: 1}, {Name: "Cackling Prowler", Count: 1}, {Name: "Goblin Boarders", Count: 1}, {Name: "Forest", Count: 8}}
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
	sc := newScenTable(t, tbl, Config{NoElide: true})
	for i := 0; i < 6; i++ {
		sc.perm(id["Forest"], 0)
	}
	prowler := sc.perm(id["Cackling Prowler"], 0)
	boarders := sc.perm(id["Goblin Boarders"], 0)
	sc.s.perms[sc.slot(prowler)].setCtrN(sc.s, int(kop.CtrP1P1), 7)
	sc.s.perms[sc.slot(boarders)].setCtrN(sc.s, int(kop.CtrP1P1), 1)
	wurm := sc.obj(id["Quilled Greatwurm"], 0)
	sc.s.place(wurm, kop.ZoneGraveyard, false)
	sc.do(t, AkCast, TCode(wurm), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostSubCtr || d.N != 2 {
		t.Fatalf("decision kind %d sub %d with %d options; want the counter ask over both creatures", d.Kind, d.Sub, d.N)
	}
	// gorge's wildcardCounterAsk names the kind on each option, so the
	// canon must carry it for this spell build too (review).
	if !sc.s.FeatSubCtrWildcard() {
		t.Fatal("Quilled Greatwurm's wildcard ask not reported as a kind-carrying pick")
	}
	if sc.s.perms[sc.slot(prowler)].ctrLow(int(kop.CtrP1P1)) != 7 || sc.s.perms[sc.slot(boarders)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatal("counters removed before the cast is paid")
	}
	picked := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(boarders)) {
			sc.s.Apply(d.Options[i])
			picked = true
		}
	}
	if !picked {
		t.Fatal("Goblin Boarders not offered")
	}
	// the Prowler's five are the only units left: taken unasked
	for i := 0; i < 8 && sc.s.ctl == ctlBuild; i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChCostSubCtr {
			t.Fatalf("asked again with %d options after the Boarders' counter", d.N)
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[wurm].zone != kop.ZoneStack {
		t.Fatalf("Greatwurm zone %d, want the stack", sc.s.objs[wurm].zone)
	}
	if p, b := sc.s.perms[sc.slot(prowler)].ctrLow(int(kop.CtrP1P1)), sc.s.perms[sc.slot(boarders)].ctrLow(int(kop.CtrP1P1)); p != 2 || b != 0 {
		t.Fatalf("counters left Prowler %d Boarders %d, want 2 and 0", p, b)
	}
}

// TestGreatwurmThroughMuldrothaNoCounters: with fewer than six counters
// among p0's creatures, Quilled Greatwurm's own permission can't be used,
// and its graveyard cast is Muldrotha's: no counters are removed and none
// is asked for (gorge casts it through Muldrotha; pair 20520). Before, the
// kernel's cast took the card's own permission first and asked the
// counter picks.
func TestGreatwurmThroughMuldrothaNoCounters(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Quilled Greatwurm", Count: 1}, {Name: "Muldrotha, the Gravetide", Count: 1}, {Name: "Goblin Boarders", Count: 1}, {Name: "Forest", Count: 8}})
	for i := 0; i < 6; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Muldrotha, the Gravetide"], 0)
	boarders := sc.perm(id["Goblin Boarders"], 0)
	sc.s.perms[sc.slot(boarders)].setCtrN(sc.s, int(kop.CtrP1P1), 3)
	wurm := sc.obj(id["Quilled Greatwurm"], 0)
	sc.s.place(wurm, kop.ZoneGraveyard, false)
	sc.do(t, AkCast, TCode(wurm), scAny)
	for i := 0; i < 8 && sc.s.ctl == ctlBuild; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChCostSubCtr {
			t.Fatalf("asked a counter pick with %d options: want Muldrotha's cast", d.N)
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[wurm].zone != kop.ZoneStack || sc.s.perms[sc.slot(boarders)].ctrLow(int(kop.CtrP1P1)) != 3 {
		t.Fatalf("Greatwurm zone %d, Boarders' counters %d: want it cast with the counters kept", sc.s.objs[wurm].zone, sc.s.perms[sc.slot(boarders)].ctrLow(int(kop.CtrP1P1)))
	}
}
