package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOnceAbilityPerIndex: a permanent with three activate-only-once
// abilities keeps one mark per ability (pfOnceGame for the first, the
// mechOnce2/3 arena masks for the second and third), so each is offered
// until its own activation and no other. A flickered permanent is a new
// object (CR 400.7) and may activate all three again.
func TestOnceAbilityPerIndex(t *testing.T) {
	once := func(n uint8) kop.Ability {
		return kop.Ability{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostOncePerGame}, Once: n}
	}
	tbl, _, ids := newModeTable(kop.CardDef{Name: "Test Once Thrice", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{once(1), once(2), once(3)}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	ix2, ix3 := sc.k.gen.ix[mechOnce2], sc.k.gen.ix[mechOnce3]
	if ix2 == arenaNone || ix3 == arenaNone || ix2 == ix3 {
		t.Fatalf("arena once masks: once2 %d once3 %d, want two assigned masks", ix2, ix3)
	}
	id := ids[0]
	p := sc.perm(id, 0)

	activateAll := func(p Obj) {
		t.Helper()
		for ab := 0; ab < 3; ab++ {
			if !sc.activateAb(p, uint8(ab)) {
				t.Fatalf("ability %d not offered", ab)
			}
			sc.resolveAll(t)
			for used := 0; used <= ab; used++ {
				if sc.activateOffered(p, uint8(used)) {
					t.Fatalf("ability %d offered again after its activation", used)
				}
			}
			for rest := ab + 1; rest < 3; rest++ {
				if !sc.activateOffered(p, uint8(rest)) {
					t.Fatalf("ability %d no longer offered after ability %d", rest, ab)
				}
			}
		}
	}
	activateAll(p)

	sl := sc.slot(p)
	sc.s.removePerm(sc.k, sl, kop.ZoneExile, false)
	p2 := sc.perm(id, 0)
	if sc.slot(p2) != sl {
		t.Fatalf("the flickered permanent took slot %d, want the vacated %d", sc.slot(p2), sl)
	}
	for ab := 0; ab < 3; ab++ {
		if !sc.activateOffered(p2, uint8(ab)) {
			t.Fatalf("ability %d not offered after the flicker", ab)
		}
	}
	activateAll(p2)
}

// TestOnceManaAbilityPerIndex: a first mana ability whose fee is one coloured
// pip and which is activate-only-once is a filter (Loot, the Pathfinder): its
// pip is paid from the pool, it makes its N mana of the chosen colour, and
// its own mark is spent so a later untap cannot activate it again.
func TestOnceManaAbilityPerIndex(t *testing.T) {
	cost := kop.Cost{Flags: kop.CostTap | kop.CostOncePerGame}
	cost.Mana.Pips[kop.G] = 1
	tbl, _, ids := newModeTable(kop.CardDef{Name: "Test Once Mana", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Once: 1, Cost: cost, ManaColors: kop.ColAny, ManaN: 3}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyFiltFirst || !sc.k.gen.anyFiltPip {
		t.Fatalf("prep: filtFirst %v filtPip %v, want both", sc.k.gen != nil && sc.k.gen.anyFiltFirst, sc.k.gen != nil && sc.k.gen.anyFiltPip)
	}
	id := ids[0]
	p := sc.perm(id, 0)
	sl, c := sc.slot(p), id
	a := &sc.k.t.Cards[c].Abilities[0]
	col, ok := sc.s.filtPip(sc.k, sl)
	if !ok || col != kop.G {
		t.Fatalf("filtPip = %d %v, want the {G} fee", col, ok)
	}
	sc.s.pl[0].pool[kop.G] = 1
	if !sc.s.filtPaid(sc.k, 0, sl) || !sc.s.filtUsable(sc.k, sl) || !sc.s.filtGateOn(sc.k, sl) {
		t.Fatal("the filter is not usable with its pip floating")
	}
	sc.s.filtDo(sc.k, 0, filtStep{sl: sl, ans: 1 + kop.U, gap: true})
	if got := sc.s.pl[0].pool[kop.U]; got != 3 {
		t.Fatalf("pool U = %d, want 3", got)
	}
	if sc.s.pl[0].pool[kop.G] != 0 {
		t.Fatalf("pool G = %d, want the pip spent", sc.s.pl[0].pool[kop.G])
	}
	if !sc.s.onceUsed(sc.k, sl, a) {
		t.Fatal("the mana ability's once mark is not set")
	}
	if sc.s.filtPaid(sc.k, 0, sl) || sc.s.filtUsable(sc.k, sl) || sc.s.filtGateOn(sc.k, sl) {
		t.Fatal("the used once-only filter is usable again")
	}
}
