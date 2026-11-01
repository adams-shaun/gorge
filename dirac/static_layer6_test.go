package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 613 layer 6 scenarios (layer 6: gorge's dependency order,
// timestamps of losses and removals, promoted until-end-of-turn grants).
// No FDN or BLB table has Sephara, Starforged Sword or Dress Down, so the
// table is FDN's with synthetic statics appended to a few of its cards
// (the shapes kgen lowers those cards to):
//   - Savannah Lions + "other creatures with flying have indestructible"
//     (Sephara, Sky's Blade: a keyword-gated grant);
//   - Quick-Draw Katana + "equipped creature has flying";
//   - Swiftfoot Boots + "equipped creature loses flying" (Starforged Sword:
//     a Continuous static whose Override carries RemoveKw);
//   - Authority of the Consuls + "creatures lose all abilities" (Dress
//     Down: a global lose-all override static);
//   - Dauntless Veteran as Ygra, Eater of All ("other creatures are Food
//     artifacts"; "whenever a Food dies, ...").

type l4scen struct {
	sc   *scen
	id   map[string]CardID
	food int
}

func newL4(t *testing.T) *l4scen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Ambush Wolf", Count: 1},
		{Name: "Serra Angel", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Quick-Draw Katana", Count: 1},
		{Name: "Swiftfoot Boots", Count: 1}, {Name: "Authority of the Consuls", Count: 1}, {Name: "Dauntless Veteran", Count: 1},
		{Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl0, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Overrides = append([]kop.Override(nil), tbl0.Overrides...)
	tbl.Subtypes = append([]string(nil), tbl0.Subtypes...)
	l := &l4scen{id: map[string]CardID{}, food: -1}
	for _, c := range da {
		l.id[tbl.Cards[c].Name] = c
	}
	for i, s := range tbl.Subtypes {
		if s == "Food" {
			l.food = i
		}
	}
	if l.food < 0 {
		tbl.Subtypes = append(tbl.Subtypes, "Food")
		l.food = len(tbl.Subtypes) - 1
	}
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	override := func(o kop.Override) uint16 {
		tbl.Overrides = append(tbl.Overrides, o)
		return uint16(len(tbl.Overrides))
	}
	addAb := func(name string, a kop.Ability) {
		cd := &tbl.Cards[l.id[name]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), a)
	}
	equipped := filter(kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc})
	addAb("Savannah Lions", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous,
		Filter: filter(kop.Filter{Types: kop.TCreature, Flags: kop.FOther, Flags2: kop.F2WithFlying}), Kw: kop.KwIndestructible}})
	addAb("Quick-Draw Katana", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: equipped, Kw: kop.KwFlying}})
	addAb("Swiftfoot Boots", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: equipped,
		Override: override(kop.Override{RemoveKw: kop.KwFlying})}})
	addAb("Authority of the Consuls", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride,
		Filter: filter(kop.Filter{Types: kop.TCreature}), Override: override(kop.Override{LoseAbilities: true})}})
	var food kop.SubMask
	food = food.With(l.food)
	addAb("Dauntless Veteran", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride,
		Filter: filter(kop.Filter{Types: kop.TCreature, Flags: kop.FOther}), Override: override(kop.Override{AddTypes: kop.TArtifact, AddSub: food})}})
	addAb("Dauntless Veteran", kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDies, Filter: filter(kop.Filter{Sub: uint8(l.food + 1)})},
		Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefSelf, A: kop.Const(2), C: int16(kop.CtrP1P1)}}})
	l.sc = newScenTable(t, &tbl, Config{NoElide: true})
	if l.sc.k.gen.fxNeed&(fxNeedL6|fxNeedPromo) != fxNeedL6|fxNeedPromo {
		t.Fatalf("fxNeed %#x: the synthetic layer-6 statics set neither the walk nor promotion", l.sc.k.gen.fxNeed)
	}
	return l
}

func (l *l4scen) perm(name string) Obj { return l.sc.perm(l.id[name], 0) }

func (l *l4scen) attach(eq, host Obj) {
	l.sc.s.attachTo(l.sc.k, l.sc.slot(eq), l.sc.slot(host))
	l.sc.s.refreshTy(l.sc.k)
}

// grant is a resolved "gains <m> until end of turn" (interp OpGrantKeyword).
func (l *l4scen) grant(o Obj, m kop.KwMask) {
	sl := l.sc.slot(o)
	l.sc.s.grantEOT(sl, m)
	l.sc.s.promoKw(l.sc.k, sl, m)
}

func (l *l4scen) kw(o Obj) kop.KwMask { return l.sc.s.kw(l.sc.k, l.sc.slot(o)) }

// TestL4GatedLordOlderThanGrant: a keyword-gated lord older than the
// flying grant it reads (gorge abilityDependencyOrder, CR 613.8): the lord
// applies after the grant, an EOT one or a static one, both newer.
func TestL4GatedLordOlderThanGrant(t *testing.T) {
	l := newL4(t)
	l.perm("Savannah Lions")
	wolf := l.perm("Ambush Wolf")
	if l.kw(wolf)&kop.KwIndestructible != 0 {
		t.Fatal("a wolf without flying is indestructible")
	}
	l.grant(wolf, kop.KwFlying)
	if m := l.kw(wolf); m&(kop.KwFlying|kop.KwIndestructible) != kop.KwFlying|kop.KwIndestructible {
		t.Errorf("EOT flying newer than the lord: keywords %#x, want flying and indestructible", m)
	}
	l = newL4(t)
	l.perm("Savannah Lions")
	wolf = l.perm("Ambush Wolf")
	l.attach(l.perm("Quick-Draw Katana"), wolf)
	if m := l.kw(wolf); m&(kop.KwFlying|kop.KwIndestructible) != kop.KwFlying|kop.KwIndestructible {
		t.Errorf("static flying newer than the lord: keywords %#x, want flying and indestructible", m)
	}
	// and a removal newer than the lord stops it (the dependency points
	// the other way): Serra Angel loses flying, so no indestructible
	l = newL4(t)
	l.perm("Savannah Lions")
	angel := l.perm("Serra Angel")
	if l.kw(angel)&kop.KwIndestructible == 0 {
		t.Fatal("a printed flier is not indestructible under the lord")
	}
	l.attach(l.perm("Swiftfoot Boots"), angel)
	if m := l.kw(angel); m&(kop.KwFlying|kop.KwIndestructible) != 0 {
		t.Errorf("removal newer than the lord: keywords %#x, want neither flying nor indestructible", m)
	}
}

// TestL4LoseAllAndGrantTimestamps: a global lose-all static (Dress Down)
// and an until-end-of-turn grant: the newer one wins (CR 613.7; gorge's
// layer-6 walk in timestamp order).
func TestL4LoseAllAndGrantTimestamps(t *testing.T) {
	l := newL4(t)
	l.perm("Authority of the Consuls")
	wolf := l.perm("Ambush Wolf")
	angel := l.perm("Serra Angel")
	if l.kw(angel)&kop.KwFlying != 0 || !l.sc.s.genLost(l.sc.k, l.sc.slot(angel)) {
		t.Errorf("Serra Angel under the lose-all keeps flying or its abilities")
	}
	l.grant(wolf, kop.KwFlying)
	if l.kw(wolf)&kop.KwFlying == 0 {
		t.Error("lose-all then a newer grant: the grant is lost")
	}
	l = newL4(t)
	wolf = l.perm("Ambush Wolf")
	l.grant(wolf, kop.KwFlying)
	l.perm("Authority of the Consuls")
	if l.kw(wolf)&kop.KwFlying != 0 {
		t.Error("a grant then a newer lose-all: the grant survives")
	}
	// the lose-all's source is not a creature: its own abilities stay
	if l.sc.s.genLost(l.sc.k, l.sc.slot(l.perm("Swiftfoot Boots"))) {
		t.Error("a noncreature lost its abilities to a creature-only lose-all")
	}
}

// TestL4RemoveKeywordVsEOTGrant: a static RemoveKeyword (Starforged
// Sword) and an until-end-of-turn flying grant made before and after the
// equip (the attach restamps the Equipment, CR 613.7e): the newer wins,
// which needs the grant's promoted timestamp.
func TestL4RemoveKeywordVsEOTGrant(t *testing.T) {
	l := newL4(t)
	wolf := l.perm("Ambush Wolf")
	boots := l.perm("Swiftfoot Boots")
	l.grant(wolf, kop.KwFlying)
	l.attach(boots, wolf)
	if l.kw(wolf)&kop.KwFlying != 0 {
		t.Error("grant then equip: flying survives the newer removal")
	}
	l = newL4(t)
	wolf = l.perm("Ambush Wolf")
	boots = l.perm("Swiftfoot Boots")
	l.attach(boots, wolf)
	l.grant(wolf, kop.KwFlying)
	if l.kw(wolf)&kop.KwFlying == 0 {
		t.Error("equip then grant: the newer grant is removed")
	}
	// the Boots' own grants still apply; the printed flier loses flying
	// only while equipped
	angel := l.perm("Serra Angel")
	l.attach(boots, angel)
	if m := l.kw(angel); m&kop.KwFlying != 0 || m&kop.KwHaste == 0 {
		t.Errorf("equipped Serra Angel: keywords %#x, want haste without flying", m)
	}
	if l.kw(wolf)&kop.KwFlying == 0 {
		t.Error("the wolf's grant is gone once the removal moved away")
	}
	// the cleanup ends the grant and its timestamp record
	if !l.sc.s.fxM.has(l.sc.slot(wolf)) {
		t.Error("the promoted grant left no record")
	}
}

// TestL4YgraFoodDies: a creature made a Food by a global override static
// dies; the Food trigger sees the walked subtype (the leave triggers look
// back at the permanent still on the battlefield; gorge reads its LKI
// derived characteristics), including when the static's source dies in
// the same event.
func TestL4YgraFoodDies(t *testing.T) {
	l := newL4(t)
	s, k := l.sc.s, l.sc.k
	ygra := l.perm("Dauntless Veteran")
	wolf := l.perm("Ambush Wolf")
	if !s.hasSubGen(k, l.sc.slot(wolf), uint8(l.food)) || s.typesAt(k, l.sc.slot(wolf))&kop.TArtifact == 0 {
		t.Fatal("the wolf is not a Food artifact under the global override")
	}
	if s.hasSubGen(k, l.sc.slot(ygra), uint8(l.food)) {
		t.Fatal("Ygra's \"other creatures\" made Ygra a Food")
	}
	n0 := s.npend
	s.destroy(k, l.sc.slot(wolf))
	if s.npend != n0+1 {
		t.Errorf("a Food wolf died: %d triggers queued, want 1", s.npend-n0)
	}
	// both die to one state-based check
	l = newL4(t)
	s, k = l.sc.s, l.sc.k
	ygra = l.perm("Dauntless Veteran")
	wolf = l.perm("Ambush Wolf")
	s.perms[l.sc.slot(ygra)].damage = 100
	s.perms[l.sc.slot(wolf)].damage = 100
	n0 = s.npend
	s.sba(k)
	if s.npend != n0+1 {
		t.Errorf("Ygra and a Food wolf died together: %d triggers queued, want 1", s.npend-n0)
	}
}
