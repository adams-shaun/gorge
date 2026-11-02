package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 613 layer scenarios: one per guard row the layer walk lifted
// that has no other scenario. The expected values are gorge's: each test
// mirrors a dirac/oracle TestL7Gorge* scenario, which plays the real
// cards through gorge's oracle scenario runner and asserts these same
// characteristics (P/T there includes Dub's +2/+2; here the base P/T is
// compared). No FDN or BLB table has the cards, so the table is FDN's with
// their shapes (the overrides and grants kgen lowers them to) appended:
//   - Witness Protection's Aura slot is reused for Darksteel Mutation and
//     Reprobation (an override static over the enchanted creature);
//   - Quick-Draw Katana carries Dub's "is a Knight" (an attached subtype
//     grant);
//   - Savannah Lions carries Thran Lens's colourless, Enchanted Evening's
//     added Enchantment type, or Valentina's "other Villains you control
//     are Heroes";
//   - Ambush Wolf stands in for Craw Wurm (and, with Villain, for Kree
//     Commandos); the one-shots are Nameless Inversion's "loses all
//     creature types" and Omnibian's "becomes a 3/3 Frog" records.

type l7scen struct {
	tbl kop.Table
	id  map[string]CardID
	sub map[string]int
	sc  *scen
}

func newL7(t *testing.T) *l7scen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1},
		{Name: "Ambush Wolf", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Quick-Draw Katana", Count: 1},
		{Name: "Plains", Count: 10}}
	fdnSet := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := fdnSet.Lookup(e.Name); !ok || !fdnSet.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl0, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	l := &l7scen{tbl: *tbl0, id: map[string]CardID{}, sub: map[string]int{}}
	l.tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	l.tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	l.tbl.Overrides = append([]kop.Override(nil), tbl0.Overrides...)
	l.tbl.Subtypes = append([]string(nil), tbl0.Subtypes...)
	for _, c := range da {
		l.id[l.tbl.Cards[c].Name] = c
	}
	for i, s := range l.tbl.Subtypes {
		l.sub[s] = i
	}
	for _, s := range []string{"Knight", "Coward", "Insect", "Frog", "Villain", "Hero", "Citizen"} {
		if _, ok := l.sub[s]; !ok {
			l.tbl.Subtypes = append(l.tbl.Subtypes, s)
			l.sub[s] = len(l.tbl.Subtypes) - 1
		}
	}
	return l
}

func (l *l7scen) mask(subs ...string) kop.SubMask {
	var m kop.SubMask
	for _, s := range subs {
		m = m.With(l.sub[s])
	}
	return m
}

func (l *l7scen) filter(f kop.Filter) kop.FilterID {
	l.tbl.Filters = append(l.tbl.Filters, f)
	return kop.FilterID(len(l.tbl.Filters) - 1)
}

func (l *l7scen) override(o kop.Override) int {
	l.tbl.Overrides = append(l.tbl.Overrides, o)
	return len(l.tbl.Overrides) - 1
}

func (l *l7scen) addStatic(card string, st kop.Static) {
	cd := &l.tbl.Cards[l.id[card]]
	cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbStatic, Static: st})
}

// auraOverride makes card's override static (an Aura's) carry o instead.
func (l *l7scen) auraOverride(card string, o kop.Override) {
	idx := l.override(o)
	cd := &l.tbl.Cards[l.id[card]]
	cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
			a.Static.Override = uint16(idx + 1)
			return
		}
	}
	panic("no override static on " + card)
}

func (l *l7scen) start(t *testing.T) {
	l.sc = newScenTable(t, &l.tbl, Config{NoElide: true})
}

func (l *l7scen) attach(card string, host Obj) {
	a := l.sc.perm(l.id[card], 0)
	l.sc.s.attachTo(l.sc.k, l.sc.slot(a), l.sc.slot(host))
	l.sc.s.refreshTy(l.sc.k)
}

func (l *l7scen) perm(card string) Obj {
	o := l.sc.perm(l.id[card], 0)
	l.sc.s.refreshTy(l.sc.k)
	return o
}

func (l *l7scen) record(idx int, host Obj) {
	l.sc.s.applyOverride(l.sc.k, l.sc.slot(host), idx, l.id["Ambush Wolf"], true)
	l.sc.s.refreshTy(l.sc.k)
}

func (l *l7scen) hasSub(o Obj, s string) bool {
	return l.sc.s.hasSubGen(l.sc.k, l.sc.slot(o), uint8(l.sub[s]))
}

func (l *l7scen) types(o Obj) kop.TypeMask {
	return l.sc.s.typesAt(l.sc.k, l.sc.slot(o)) &^ (kop.TLegendary | kop.TBasic | kop.TToken)
}

func (l *l7scen) withDub() {
	l.addStatic("Quick-Draw Katana", kop.Static{Kind: kop.StatContinuous,
		Filter: l.filter(kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc}), AddSub: l.mask("Knight")})
}

func (l *l7scen) inversion() int {
	return l.override(kop.Override{HasSub: true})
}

// TestL7SubtypeGrantVsRecordSetter mirrors TestL7GorgeSubtypeGrantVsRecordSetter:
// Dub then Nameless Inversion leaves no creature type; Inversion then Dub
// leaves a Knight (CR 613.1d, timestamp order; was the guard's "static
// subtype grant next to a subtype setter" row).
func TestL7SubtypeGrantVsRecordSetter(t *testing.T) {
	for _, dubFirst := range []bool{true, false} {
		l := newL7(t)
		l.withDub()
		inv := l.inversion()
		l.start(t)
		if l.sc.k.gen.fxNeed&fxNeedSubSet == 0 {
			t.Fatal("a subtype grant beside a subtype setter leaves fxNeedSubSet clear")
		}
		wolf := l.perm("Ambush Wolf")
		if dubFirst {
			l.attach("Quick-Draw Katana", wolf)
			l.record(inv, wolf)
		} else {
			l.record(inv, wolf)
			l.attach("Quick-Draw Katana", wolf)
		}
		if got, want := l.hasSub(wolf, "Knight"), !dubFirst; got != want {
			t.Errorf("dub first %v: Knight %v, want %v", dubFirst, got, want)
		}
		if l.hasSub(wolf, "Wolf") {
			t.Errorf("dub first %v: still a Wolf", dubFirst)
		}
	}
}

// TestL7TwoStaticSetters mirrors TestL7GorgeTwoStaticSetters: Witness
// Protection and Darksteel Mutation on one creature; the newer wins layer 4
// and layer 7b (guard rows "two base-P/T setters" and "a static type setter
// next to another type change").
func TestL7TwoStaticSetters(t *testing.T) {
	for _, wpFirst := range []bool{true, false} {
		l := newL7(t)
		l.auraOverride("Imprisoned in the Moon", kop.Override{HasPT: true, SetP: 0, SetT: 1, HasTypes: true,
			SetTypes: kop.TArtifact | kop.TCreature, HasSub: true, SetSub: l.mask("Insect"), LoseAbilities: true})
		l.start(t)
		wolf := l.perm("Ambush Wolf")
		if wpFirst {
			l.attach("Witness Protection", wolf)
			l.attach("Imprisoned in the Moon", wolf)
		} else {
			l.attach("Imprisoned in the Moon", wolf)
			l.attach("Witness Protection", wolf)
		}
		sl := l.sc.slot(wolf)
		p, tt := l.sc.s.basePTGen(l.sc.k, sl)
		ty := l.types(wolf)
		if wpFirst {
			if p != 0 || tt != 1 || ty != kop.TArtifact|kop.TCreature || !l.hasSub(wolf, "Insect") || l.hasSub(wolf, "Citizen") {
				t.Errorf("WP then Mutation: %d/%d types %#x Insect %v Citizen %v; want a 0/1 Insect artifact creature",
					p, tt, ty, l.hasSub(wolf, "Insect"), l.hasSub(wolf, "Citizen"))
			}
			if c := l.sc.s.colorsAt(l.sc.k, sl); c != kop.ColG|kop.ColW {
				t.Errorf("WP then Mutation: colours %#x, want green-white", c)
			}
		} else if p != 1 || tt != 1 || ty != kop.TCreature || l.hasSub(wolf, "Insect") {
			t.Errorf("Mutation then WP: %d/%d types %#x Insect %v; want a 1/1 creature, no Insect", p, tt, ty, l.hasSub(wolf, "Insect"))
		}
	}
}

// TestL7KeptSubtypes: a replacement of the card types leaves the subtypes of a
// card type the effect adds (CR 205.1a). Imprisoned in the Moon's land stays a
// Plains and still taps only for the Aura's {C}: the kept type brings no mana
// ability back. An Aura made an enchantment stays an Aura.
func TestL7KeptSubtypes(t *testing.T) {
	l := newL7(t)
	if _, ok := l.sub["Plains"]; !ok {
		l.tbl.Subtypes = append(l.tbl.Subtypes, "Plains")
		l.sub["Plains"] = len(l.tbl.Subtypes) - 1
	}
	plainsCard := &l.tbl.Cards[l.id["Plains"]]
	plainsCard.Subtypes = l.mask("Plains")
	var moon kop.Override
	for _, a := range l.tbl.Cards[l.id["Imprisoned in the Moon"]].Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
			moon = l.tbl.Overrides[a.Static.Override-1]
		}
	}
	if !moon.HasSub || !moon.LoseAbilities {
		t.Fatalf("Imprisoned in the Moon's override: %+v", moon)
	}
	moon.KeepSub = l.mask("Plains")
	l.auraOverride("Imprisoned in the Moon", moon)
	asEnchantment := l.override(kop.Override{HasTypes: true, SetTypes: kop.TEnchantment, HasSub: true, KeepTypes: kop.TAura})
	l.start(t)

	plains := l.perm("Plains")
	l.attach("Imprisoned in the Moon", plains)
	if !l.hasSub(plains, "Plains") {
		t.Error("the enchanted land is no longer a Plains")
	}
	if pc := l.sc.s.manaPCGen(l.sc.k, l.sc.slot(plains)); pc.manaCol != kop.ColC {
		t.Errorf("the enchanted Plains taps for colours %#x, want only {C}", pc.manaCol)
	}

	aura := l.perm("Witness Protection")
	l.record(asEnchantment, aura)
	if ty := l.types(aura); ty != kop.TEnchantment|kop.TAura {
		t.Errorf("the Aura made an enchantment has types %#x, want an enchantment Aura", ty)
	}

	k := l.sc.k
	c := dchars{types: kop.TLand | kop.TCreature, subs: l.mask("Plains", "Frog")}
	lfxApply4(k, &lfx{o: &moon}, &c)
	if c.types != kop.TLand || !c.subs.Has(l.sub["Plains"]) || c.subs.Has(l.sub["Frog"]) {
		t.Errorf("the layer walk gives types %#x subs %v, want a land that is a Plains and no Frog", c.types, c.subs)
	}
}

// TestL7SubtypeGrantVsStaticSetter mirrors TestL7GorgeSubtypeGrantVsStaticSetter:
// Reprobation's Coward setter and Dub's Knight grant on a walked slot.
func TestL7SubtypeGrantVsStaticSetter(t *testing.T) {
	for _, repFirst := range []bool{true, false} {
		l := newL7(t)
		l.withDub()
		l.auraOverride("Imprisoned in the Moon", kop.Override{HasPT: true, SetP: 0, SetT: 1, HasTypes: true,
			SetTypes: kop.TCreature, HasSub: true, SetSub: l.mask("Coward"), LoseAbilities: true})
		l.start(t)
		wolf := l.perm("Ambush Wolf")
		if repFirst {
			l.attach("Imprisoned in the Moon", wolf)
			l.attach("Quick-Draw Katana", wolf)
		} else {
			l.attach("Quick-Draw Katana", wolf)
			l.attach("Imprisoned in the Moon", wolf)
		}
		if !l.hasSub(wolf, "Coward") || l.hasSub(wolf, "Knight") != repFirst {
			t.Errorf("Reprobation first %v: Coward %v Knight %v; want Coward, Knight %v", repFirst,
				l.hasSub(wolf, "Coward"), l.hasSub(wolf, "Knight"), repFirst)
		}
		if p, tt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf)); p != 0 || tt != 1 {
			t.Errorf("Reprobation first %v: base %d/%d, want 0/1", repFirst, p, tt)
		}
	}
}

// TestL7ColourSetters mirrors TestL7GorgeColourSetters: Thran Lens and
// Witness Protection (guard row "two colour setters").
func TestL7ColourSetters(t *testing.T) {
	for _, lensFirst := range []bool{true, false} {
		l := newL7(t)
		idx := l.override(kop.Override{HasColors: true, SetColors: kop.ColC})
		l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride, Filter: l.filter(kop.Filter{}), Override: uint16(idx + 1)})
		l.start(t)
		wolf := l.perm("Ambush Wolf")
		if lensFirst {
			l.perm("Savannah Lions")
			l.attach("Witness Protection", wolf)
		} else {
			l.attach("Witness Protection", wolf)
			l.perm("Savannah Lions")
		}
		c := l.sc.s.colorsAt(l.sc.k, l.sc.slot(wolf))
		if want := map[bool]kop.ColorMask{true: kop.ColG | kop.ColW, false: kop.ColC}[lensFirst]; c != want && !(want == kop.ColC && c == 0) {
			t.Errorf("lens first %v: colours %#x, want %#x", lensFirst, c, want)
		}
	}
}

// TestL7SubtypeLordVsSetter mirrors TestL7GorgeSubtypeLordVsSetter: a
// Villain is a Hero under Valentina until Omnibian makes it a Frog, in
// either order (CR 613.8 dependency; guard row "a subtype-reading type
// grant next to a type change").
func TestL7SubtypeLordVsSetter(t *testing.T) {
	for _, order := range []int{0, 1, 2} { // lord only, lord then Omnibian, Omnibian then lord
		l := newL7(t)
		wolfCard := &l.tbl.Cards[l.id["Ambush Wolf"]]
		wolfCard.Subtypes = l.mask("Villain")
		l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatContinuous, AddSub: l.mask("Hero"),
			Filter: l.filter(kop.Filter{Types: kop.TCreature, Sub: uint8(l.sub["Villain"] + 1), Flags: kop.FOther, Ctrl: kop.CtrlYou})})
		omni := l.override(kop.Override{HasPT: true, SetP: 3, SetT: 3, HasSub: true, SetSub: l.mask("Frog")})
		l.start(t)
		if l.sc.k.gen.fxNeed&fxNeedSubGrant == 0 {
			t.Fatal("a subtype-reading grant leaves fxNeedSubGrant clear")
		}
		wolf := l.perm("Ambush Wolf")
		switch order {
		case 0:
			l.perm("Savannah Lions")
		case 1:
			l.perm("Savannah Lions")
			l.record(omni, wolf)
		case 2:
			l.record(omni, wolf)
			l.perm("Savannah Lions")
		}
		hero, villain, frog := l.hasSub(wolf, "Hero"), l.hasSub(wolf, "Villain"), l.hasSub(wolf, "Frog")
		if order == 0 && (!hero || !villain) || order != 0 && (hero || villain || !frog) {
			t.Errorf("order %d: Hero %v Villain %v Frog %v", order, hero, villain, frog)
		}
	}
}

// TestL7TypeAddVsRecordSetter mirrors TestL7GorgeTypeAddVsRecordSetter:
// Enchanted Evening's added Enchantment type and Omnibian's Frog record
// both apply, in either order (guard row "a static type grant next to a
// type-setting record").
func TestL7TypeAddVsRecordSetter(t *testing.T) {
	for _, eveFirst := range []bool{true, false} {
		l := newL7(t)
		idx := l.override(kop.Override{AddTypes: kop.TEnchantment})
		l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride, Filter: l.filter(kop.Filter{}), Override: uint16(idx + 1)})
		omni := l.override(kop.Override{HasPT: true, SetP: 3, SetT: 3, HasSub: true, SetSub: l.mask("Frog")})
		l.start(t)
		wolf := l.perm("Ambush Wolf")
		if eveFirst {
			l.perm("Savannah Lions")
			l.record(omni, wolf)
		} else {
			l.record(omni, wolf)
			l.perm("Savannah Lions")
		}
		p, tt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf))
		if ty := l.types(wolf); ty != kop.TCreature|kop.TEnchantment || !l.hasSub(wolf, "Frog") || l.hasSub(wolf, "Wolf") || p != 3 || tt != 3 {
			t.Errorf("evening first %v: types %#x Frog %v Wolf %v base %d/%d; want a 3/3 Frog enchantment creature",
				eveFirst, ty, l.hasSub(wolf, "Frog"), l.hasSub(wolf, "Wolf"), p, tt)
		}
	}
}

// TestL7SubSetOffOnFDNAndBLB: neither pool has a subtype grant that can
// meet a setter (FDN's Skyknight Squire grants itself; the setters that
// reach it remove its abilities), so no FDN or BLB table walks a slot for
// fxNeedSubSet.
func TestL7SubSetOffOnFDNAndBLB(t *testing.T) {
	blbSet := cardsettest.Set(t, "blb")
	fdnSet := cardsettest.Set(t, "fdn")
	for name, pool := range map[string]func() (*kop.Table, error){"fdn": fdnSet.Pool, "blb": blbSet.Pool} {
		tbl, err := pool()
		if err != nil {
			t.Fatal(err)
		}
		g := prepare(tbl).gen
		if g.fxNeed&fxNeedSubSet != 0 {
			t.Errorf("%s: fxNeedSubSet is set", name)
		}
	}
}

// TestL7SelfGrantUnderTwoAurasNoRecursion: Skyknight Squire (a self subtype
// grant) under Witness Protection and Imprisoned in the Moon is a walk
// slot; collecting its own grant asked whether it lost its abilities by
// walking it again, without end (a BenchmarkTransitionFDN stack overflow).
func TestL7SelfGrantUnderTwoAurasNoRecursion(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1},
		{Name: "Skyknight Squire", Count: 1}, {Name: "Plains", Count: 10}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sq := sc.perm(id["Skyknight Squire"], 0)
	for _, a := range []string{"Witness Protection", "Imprisoned in the Moon"} {
		o := sc.perm(id[a], 1)
		sc.s.attachTo(sc.k, sc.slot(o), sc.slot(sq))
		sc.s.refreshTy(sc.k)
	}
	for i := range tbl.Subtypes {
		sc.s.hasSubGen(sc.k, sc.slot(sq), uint8(i))
	}
	sc.s.kwGen(sc.k, sc.slot(sq))
}
