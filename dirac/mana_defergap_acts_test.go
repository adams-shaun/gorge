package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// deferGapBoard is a scenario over the dr-mbc-bbba draft split: p0 controls
// the named permanents in order (a count first), Saprazzan Skerry among them
// holding ctrs depletion counters, untapped. gorge's planner defers a depletion
// land, so one prefix search over every census gap prices an activation it
// helps pay (CR 601.2g-h, 605.3b).
func deferGapBoard(t *testing.T, src string, ctrs int, perms ...string) (*scen, Obj) {
	t.Helper()
	set := cardsettest.Synth(t, "dr-mbc-bbba")
	names := append([]string{src, "Saprazzan Skerry", "Swamp", "Plains"}, perms...)
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s is not a supported card of dr-mbc-bbba", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true})
	o := sc.perm(id[src], 0)
	for _, n := range perms {
		switch n {
		case "Swamp", "Plains":
			continue
		}
		sc.perm(id[n], 0)
	}
	sk := sc.perm(id["Saprazzan Skerry"], 0)
	sl := sc.slot(sk)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Saprazzan Skerry is not the depletion-rider shape")
	}
	sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
	if !sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), ctrs) {
		t.Fatal("precondition: could not put the depletion counters on")
	}
	sc.s.perms[sl].flags &^= pfTapped // it entered tapped; it untaps on its controller's turn
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the depletion land is not a planner-deferred gap")
	}
	return sc, o
}

// addLands puts n of the named basic on p0's battlefield.
func (sc *scen) addLands(t *testing.T, name string, n int) {
	t.Helper()
	for c := range sc.k.t.Cards {
		if sc.k.t.Cards[c].Name == name {
			for i := 0; i < n; i++ {
				sc.perm(CardID(c), 0)
			}
			return
		}
	}
	t.Fatalf("%s is not in the table", name)
}

// A depletion land's {U}{U} beside a granted mana ability (Gemhide Sliver's
// {T}: Add one mana of any color, its own) pays Bloodrite Invoker's {8}: five
// Swamps, the Sliver and the Skerry make eight. The grant search is gorge's one
// search over the grants and the deferred gap, so the kernel offers the
// activation where it once priced the Skerry or the grant alone.
func TestDepletionGapBesideAGrantFundsAnActivation(t *testing.T) {
	for _, tc := range []struct {
		swamps, want int
	}{{4, 0}, {5, 1}} {
		sc, inv := deferGapBoard(t, "Bloodrite Invoker", 2, "Gemhide Sliver")
		sc.addLands(t, "Swamp", tc.swamps)
		sc.s.ctl = ctlPriority
		if n := sc.graveActivations(inv); n != tc.want {
			t.Errorf("%d Swamps, a Gemhide Sliver and a Skerry: Bloodrite Invoker's {8} offered %d times, want %d", tc.swamps, n, tc.want)
		}
	}
}

// The same land beside a tap-two-Elves source (Birchlore Rangers, with a second
// Elf to tap) pays Ebony Fly's {4} (the Fly tapped: its own {C} is no part of the
// sum): a Plains, the Skerry's two mana and the Rangers's one are exactly four,
// and without the Plains the board is a mana short.
func TestDepletionGapBesideATapXSourceFundsAnActivation(t *testing.T) {
	for _, tc := range []struct {
		plains, want int
	}{{0, 0}, {1, 1}} {
		sc, fly := deferGapBoard(t, "Ebony Fly", 2, "Birchlore Rangers", "Pollenbright Druid")
		sc.addLands(t, "Plains", tc.plains)
		sc.s.perms[sc.slot(fly)].flags |= pfTapped
		sc.s.ctl = ctlPriority
		if n := sc.graveActivations(fly); n != tc.want {
			t.Errorf("%d Plains, Rangers, Druid and a Skerry: Ebony Fly's {4} offered %d times, want %d", tc.plains, n, tc.want)
		}
	}
}
