package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// histAb is the index of card c's first triggered ability on event ev.
func histAb(t *testing.T, sc *scen, c CardID, ev kop.TrigEvent) uint8 {
	t.Helper()
	for j, a := range sc.k.t.Cards[c].Abilities {
		if a.Kind == kop.AbTriggered && a.Trig.Event == ev {
			return uint8(j)
		}
	}
	t.Fatalf("%s has no %v trigger", sc.k.t.Cards[c].Name, ev)
	return 0
}

// histCastAb folds a spell cast by p that spent mv mana into the history bank
// the way finishBuild does, and returns the pending triggers it queued.
func histCastAb(sc *scen, p uint8, card CardID, mv int) int {
	sc.s.npend = 0
	e := stackEntry{kind: seSpell, obj: NoObj, card: card, ctrl: p}
	cost := kop.ManaCost{Generic: uint8(mv)}
	sc.s.histCast(sc.k, &e, &cost)
	sc.s.onCast(sc.k, &e)
	return int(sc.s.npend)
}

// TestExpendFiresOnceAtTheNth: Bakersbane Duo's "whenever you expend 4"
// fires on the cast whose mana takes the turn's total from below 4 to 4 or
// more, once (gorge ManaExpendMatches: prev < n && total >= n); an
// opponent's spending never fires it; the total resets at the turn's start.
func TestExpendFiresOnceAtTheNth(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Bakersbane Duo", Count: 1}, {Name: "Plains", Count: 8}})
	bd, pl := id["Bakersbane Duo"], id["Plains"]
	if !sc.k.gen.anyHist {
		t.Fatal("a table with an expend card must gate the history bank on")
	}
	exp := histAb(t, sc, bd, kop.TrigManaExpend)
	sc.perm(bd, 0)
	fired := func(want bool, what string) {
		t.Helper()
		got := false
		for i := 0; i < int(sc.s.npend); i++ {
			if sc.s.pend[i].ab == exp {
				got = true
			}
		}
		if got != want {
			t.Fatalf("%s: expend 4 fired=%v, want %v (spent %d)", what, got, want, sc.s.g.spent[0])
		}
	}
	histCastAb(sc, 0, pl, 3)
	fired(false, "3 spent")
	histCastAb(sc, 1, pl, 5)
	fired(false, "the opponent spent 5")
	histCastAb(sc, 0, pl, 2)
	fired(true, "3 -> 5 spent")
	histCastAb(sc, 0, pl, 4)
	fired(false, "5 -> 9 spent")
	if sc.s.g.hist[0][histSpells] != 3 || sc.s.g.hist[1][histSpells] != 1 {
		t.Fatalf("spells cast this turn %v, want 3 and 1", sc.s.g.hist)
	}
	sc.s.newTurnGen()
	if sc.s.g.spent != [MaxSeats]uint16{} || sc.s.g.hist != [MaxSeats][3]uint8{} {
		t.Fatalf("the history bank survived the turn: %v %v", sc.s.g.spent, sc.s.g.hist)
	}
	histCastAb(sc, 0, pl, 4)
	fired(true, "0 -> 4 spent on a new turn")
}

// TestNthSpellTrigger: Hearthborn Battler's "whenever a player casts their
// second spell each turn" (ActivatorThisTurnCast$ EQ2) fires on each
// player's second spell only, the count resetting at the turn's start.
func TestNthSpellTrigger(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Hearthborn Battler", Count: 1}, {Name: "Plains", Count: 8}})
	hb := id["Hearthborn Battler"]
	sc.perm(hb, 0)
	want := []int{0, 0, 1, 1, 0}
	for i, p := range []uint8{0, 1, 0, 1, 0} {
		if n := histCastAb(sc, p, hb, 1); n != want[i] {
			t.Fatalf("cast %d (player %d): %d triggers, want %d", i, p, n, want[i])
		}
	}
	sc.s.newTurnGen()
	histCastAb(sc, 0, hb, 1)
	if n := histCastAb(sc, 0, hb, 1); n != 1 {
		t.Fatalf("the second spell of a new turn: %d triggers, want 1", n)
	}
}

// TestResolvedThisTurn: Harvestrite Host's Count$ResolvedThisTurn counts its
// trigger's resolutions this turn, the current one included, per source
// object (a zone change keeps the count), reset at the turn's start.
func TestResolvedThisTurn(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Harvestrite Host", Count: 2}, {Name: "Plains", Count: 8}})
	hh := id["Harvestrite Host"]
	ab := histAb(t, sc, hh, kop.TrigETB)
	if sc.k.t.Cards[hh].Abilities[ab].AFlags&kop.AfResolveCount == 0 {
		t.Fatal("Harvestrite Host's trigger does not count its resolutions")
	}
	a, b := sc.perm(hh, 0), sc.perm(hh, 0)
	resolve := func(o Obj) int {
		sc.s.res.active = true
		sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: sc.s.objs[o].ver, card: hh, ab: ab, ctrl: 0}
		if !sc.s.trigResolveOK(sc.k) {
			t.Fatal("the trigger has no intervening-if")
		}
		n := sc.s.resolvedThisTurn(sc.k)
		sc.s.res.active = false
		return n
	}
	for i, want := range []int{1, 2, 3} {
		if n := resolve(a); n != want {
			t.Fatalf("resolution %d of the first Host reads %d", i+1, n)
		}
	}
	if n := resolve(b); n != 1 {
		t.Fatalf("the second Host's first resolution reads %d, want 1", n)
	}
	sc.s.newTurnGen()
	if n := resolve(a); n != 1 {
		t.Fatalf("the first resolution of a new turn reads %d, want 1", n)
	}
}

// TestWasDamagedFilter: Downwind Ambusher's "creature an opponent controls
// that was dealt damage this turn" matches a creature with damage marked.
func TestWasDamagedFilter(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 1}, {Name: "Plains", Count: 8}})
	da := id["Downwind Ambusher"]
	var f kop.FilterID
	for i := range sc.k.t.Filters {
		if sc.k.t.Filters[i].Flags2&kop.F2WasDamaged != 0 {
			f = kop.FilterID(i)
		}
	}
	if f == 0 {
		t.Fatal("no wasDealtDamageThisTurn filter in the table")
	}
	o := sc.perm(da, 1)
	sl := sc.slot(o)
	if sc.s.matchPerm(sc.k, f, sl, 0, NoSlot) {
		t.Fatal("an undamaged creature matched")
	}
	sc.s.perms[sl].damage = 1
	sc.s.g.dmgTurn.set(sc.s.perms[sl].obj)
	if !sc.s.matchPerm(sc.k, f, sl, 0, NoSlot) {
		t.Fatal("a damaged opposing creature did not match")
	}
}

// TestFDNHasNoHistoryReader: the FDN pool reads no history bank, so its
// cast path never takes the branch (the bench's hot path is unchanged).
func TestFDNHasNoHistoryReader(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	i, ok := set.Lookup("Plains")
	if !ok {
		t.Fatal("no Plains in FDN")
	}
	var lib [20]CardID
	for j := range lib {
		lib[j] = set.PoolID(i)
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	if k := s.k(); k.gen == nil || k.gen.anyHist {
		t.Fatal("the FDN pool gates the history bank on")
	}
}

// TestResolvedThisTurnSharedBody: Victor's one Eerie ability has two trigger
// lines, an enchantment entering and a Room fully unlocked, that share one
// body, so a resolution through either line counts into the same tally (gorge
// keys it by the body, events.ResolvedAbilityKey; the second resolution of the
// turn is the opponent's discard, not another surveil).
func TestResolvedThisTurnSharedBody(t *testing.T) {
	sc, id := newSetScenTweak(t, "gybf", []cardset.Entry{{Name: "Victor, Valgavoth's Seneschal", Count: 2}, {Name: "Plains", Count: 8}}, nil)
	v := id["Victor, Valgavoth's Seneschal"]
	etb, unlock := histAb(t, sc, v, kop.TrigETB), histAb(t, sc, v, kop.TrigFullyUnlock)
	if etb == unlock {
		t.Fatal("Victor's two Eerie lines are one ability")
	}
	o := sc.perm(v, 0)
	resolve := func(ab uint8) int {
		sc.s.res.active = true
		sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: sc.s.objs[o].ver, card: v, ab: ab, ctrl: 0}
		if !sc.s.trigResolveOK(sc.k) {
			t.Fatal("the trigger has no intervening-if")
		}
		n := sc.s.resolvedThisTurn(sc.k)
		sc.s.res.active = false
		return n
	}
	for i, ab := range []uint8{etb, unlock, etb} {
		if n := resolve(ab); n != i+1 {
			t.Fatalf("resolution %d of Victor's Eerie reads %d", i+1, n)
		}
	}
}
