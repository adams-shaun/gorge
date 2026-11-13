package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// xScen is a scenario on synthetic pool set under the autopay surface.
func xScen(t *testing.T, set string, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	tbl, da, _, err := cardsettest.Synth(t, set).Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.cfg |= cfgGorgeAutopay | cfgGorgeUnpriced
	return sc, id
}

// xChoices activates ability ab of permanent o and returns the X values the ask offers.
func (sc *scen) xChoices(t *testing.T, o Obj, ab uint8) []int {
	t.Helper()
	d := sc.s.Step()
	found := false
	for i := 0; i < int(d.N) && !found; i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) && a.Mode()&0x7f == ab {
			sc.s.Apply(a)
			found = true
		}
	}
	if !found {
		t.Fatalf("ability %d of object %d not offered: %v", ab, o, d.Options[:d.N])
	}
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	var xs []int
	for i := 0; i < int(d.N); i++ {
		xs = append(xs, int(d.Options[i].Mode()))
	}
	return xs
}

// TestXAnnouncementReachesTheWindow: X is announced before the mana window opens
// (CR 601.2b, 601.2g), so with an empty pool it is bounded by what the sources can
// make, and an X above the pool is paid through the window in battlefield order.
func TestXAnnouncementReachesTheWindow(t *testing.T) {
	sc, id := xScen(t, "std-fdn", []cardset.Entry{{Name: "Steel Hellkite", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8}})
	hk := sc.perm(id["Steel Hellkite"], 0)
	lands := []Obj{sc.perm(id["Island"], 0), sc.perm(id["Plains"], 0), sc.perm(id["Island"], 0)}
	if xs := fmt.Sprint(sc.xChoices(t, hk, 1)); xs != "[0 1 2 3]" {
		t.Fatalf("X options %s, want [0 1 2 3]", xs)
	}
	sc.do(t, AkChoose, 0xff, 0xff) // the first X, to check the ask closes at X = 0
	if sc.s.Step(); sc.s.ctl != ctlPriority || sc.tapped(lands[0]) {
		t.Fatalf("X = 0 paid: ctl %d, first land tapped %v", sc.s.ctl, sc.tapped(lands[0]))
	}
}

// TestXAnnouncementPaysThroughTheWindow: gorge's planner leaves an X cost to the
// manual window, which taps the first useful source in battlefield order, where the
// planner would rank the objects (Islands older than the Plains between them).
func TestXAnnouncementPaysThroughTheWindow(t *testing.T) {
	sc, id := xScen(t, "std-fdn", []cardset.Entry{{Name: "Steel Hellkite", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8}})
	hk := sc.perm(id["Steel Hellkite"], 0)
	older := sc.obj(id["Island"], 0)
	plains := sc.obj(id["Plains"], 0)
	newer := sc.obj(id["Island"], 0)
	for _, o := range []Obj{newer, plains, older} {
		sc.s.perms[sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)].flags &^= pfSick
		sc.s.npend = 0
	}
	sc.xChoices(t, hk, 1)
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 2))
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want priority with the ability on the stack", d.Kind)
	}
	if !sc.tapped(newer) || !sc.tapped(plains) || sc.tapped(older) {
		t.Fatalf("tapped first %v, second %v, third %v, want the first two on the battlefield", sc.tapped(newer), sc.tapped(plains), sc.tapped(older))
	}
}

// TestXAnnouncementCountsAGrantedSource: a land under New Horizons makes two, whoever
// controls the Aura.
func TestXAnnouncementCountsAGrantedSource(t *testing.T) {
	sc, id := xScen(t, "std-fdn", []cardset.Entry{{Name: "Steel Hellkite", Count: 1}, {Name: "New Horizons", Count: 1}, {Name: "Plains", Count: 8}})
	hk := sc.perm(id["Steel Hellkite"], 0)
	host := sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	aura := sc.perm(id["New Horizons"], 1)
	sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(host))
	sc.s.refreshTy(sc.k)
	if xs := fmt.Sprint(sc.xChoices(t, hk, 1)); xs != "[0 1 2 3]" {
		t.Fatalf("X options %s, want [0 1 2 3]", xs)
	}
}

// TestXAnnouncementNeedsATarget: Rydia's X is the mana value of a Saga card in the
// graveyard, so only that value is offered (CR 601.2c), whatever else the lands pay.
func TestXAnnouncementNeedsATarget(t *testing.T) {
	sc, id := xScen(t, "std-fin", []cardset.Entry{{Name: "Rydia, Summoner of Mist", Count: 1}, {Name: "Summon: G.F. Ifrit", Count: 1}, {Name: "Mountain", Count: 8}})
	ry := sc.perm(id["Rydia, Summoner of Mist"], 0)
	for i := 0; i < 5; i++ {
		sc.perm(id["Mountain"], 0)
	}
	sc.s.place(sc.obj(id["Summon: G.F. Ifrit"], 0), kop.ZoneGraveyard, false)
	if xs := fmt.Sprint(sc.xChoices(t, ry, 0)); xs != "[3]" {
		t.Fatalf("a Saga of mana value 3: X options %s, want [3]", xs)
	}
}

// TestPlainXAskCountsPrintedAbilityOfGrantedSource: Rok-Tahk grants every
// creature "{T}: Add {G}{G}"; a Savvy Profiteer beside it keeps its printed
// "{T}: Add {R}". The plain configuration's payment plans the printed
// ability, so the X ask must not count the granted {G}{G} for it.
func TestPlainXAskCountsPrintedAbilityOfGrantedSource(t *testing.T) {
	tbl, err := cardsettest.Synth(t, "std-trk").Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	lands := []string{"Forest", "Forest", "Forest", "Forest", "Mountain", "Mountain", "Mountain", "Rok-Tahk, Budding Scientist"}
	for _, b := range []struct {
		name  string
		savvy int
		maxX  int
	}{
		{"one dual source", 1, 9},
		{"two dual sources", 2, 10},
	} {
		board := append([]string{}, lands...)
		for i := 0; i < b.savvy; i++ {
			board = append(board, "Savvy Profiteer")
		}
		for _, c := range []struct {
			name string
			cfg  Config
		}{
			{"plain", Config{NoElide: true}},
			{"lowering", Config{NoElide: true, GorgeLowering: true}},
		} {
			name := b.name + "/" + c.name
			sc := newScenTableLib(t, tbl, id["Forest"], c.cfg)
			for _, n := range board {
				if _, ok := id[n]; !ok {
					t.Fatalf("%s: card %q not in the std-trk pool", name, n)
				}
				sc.perm(id[n], 0)
			}
			sc.perm(id["Mugato"], 1) // the target
			sc.hand(id["Khaaaaaaaaaaaannn!"], 0)
			d := sc.s.Step()
			cast := false
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Kind() == AkCast {
					sc.s.Apply(d.Options[i])
					cast = true
					break
				}
			}
			if !cast {
				t.Fatalf("%s: the cast is not offered", name)
			}
			d = sc.s.Step()
			if d.Kind != DkChoose || d.Sub != ChX {
				t.Fatalf("%s: decision kind %d sub %d, want the X ask", name, d.Kind, d.Sub)
			}
			if got := int(d.N) - 1; got != b.maxX {
				t.Errorf("%s: X offered up to %d, want %d", name, got, b.maxX)
			}
			sc.s.Apply(d.Options[d.N-1]) // the widest X: the payment must not panic "autopay found no plan"
			for i := 0; i < 12; i++ {
				d = sc.s.Step() // the target, then the payment
				if d.Kind == DkPriority {
					break
				}
				sc.s.Apply(d.Options[0])
			}
			if d.Kind != DkPriority || sc.s.sp != 1 {
				t.Errorf("%s: stack %d after the widest X, want the Khaaaaaaaaaaaannn! on it", name, sc.s.sp)
			}
		}
	}
}
