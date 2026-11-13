package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestActivateSacAttachedAuraCostFromGrantedMana: "{1}, Sacrifice an Aura
// attached to it" (Faunsbane Troll's fight, in the WOE pool) is payable when
// the only untapped mana is a Food that a Continuous static lets tap for {G}
// (Night of the Sweets' Revenge). The granted source is a prefix
// (CR 601.2g-h), and the printed plan must leave the Aura attached to the
// source as the sacrifice (CR 602.2b). The Sac part's filter reads the
// ability's source (Attached = attached to it), so the printed-plan check
// prices that part against the source's slot, not against "no source"
// (affordsUngrantedPlay's costPartDisjoint): without it the Aura is never a
// candidate there, the granted prefix is refused as unable to leave the part
// payable, and grantCap drops the activation gorge offers.
func TestActivateSacAttachedAuraCostFromGrantedMana(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Subtypes = append([]string(nil), v.Table.Subtypes...)
	tbl.Conds = []kop.Cond{} // a generated table (prepareGen's gate)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	sub := func(name string) uint8 {
		for i, n := range tbl.Subtypes {
			if n == name {
				return uint8(i)
			}
		}
		tbl.Subtypes = append(tbl.Subtypes, name)
		return uint8(len(tbl.Subtypes) - 1)
	}
	aura := sub("Aura")
	creature := filter(kop.Filter{Types: kop.TCreature})
	oppCreature := filter(kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp})
	artifactYou := filter(kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou})
	auraAttached := filter(kop.Filter{Sub: aura + 1, Pred: kop.PredAttached})
	troll := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Troll", Types: kop.TCreature, Power: 4, Toughness: 4,
		Abilities: []kop.Ability{{
			Kind: kop.AbActivated,
			Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}, Flags: kop.CostSac, N: 1, Filter: auraAttached},
			Targets: [kop.MaxTargets]kop.TargetSpec{
				{Kind: kop.TgtObject, Filter: oppCreature}},
			NTargets: 1,
		}}})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rite", Types: kop.TEnchantment,
		Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: artifactYou, Grant: 2}},
			{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1},
		}})
	role := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Role", Types: kop.TEnchantment,
		Subtypes: kop.SubMask{}.With(int(aura)), Enchant: kop.TargetSpec{Kind: kop.TgtObject, Filter: creature}})
	chow := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Food", Types: kop.TArtifact})
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})

	sc := newScenTableLib(t, &tbl, v.Plains, Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.sacMana {
		t.Fatal("precondition: the table does not arm the sacrifice-census guard (sacMana)")
	}
	trollObj := sc.perm(troll, 0)
	sc.perm(rite, 0)
	chowObj := sc.perm(chow, 0)
	bearObj := sc.perm(bear, 1) // the fight's "creature you don't control"
	roleObj := sc.perm(role, 0)
	s.attachTo(k, sc.slot(roleObj), sc.slot(trollObj))
	tsl, rsl := sc.slot(trollObj), sc.slot(roleObj)
	// preconditions: the Aura is on the battlefield attached to the source,
	// the cost filter reads that attachment from the source, and its
	// sacrifice candidate is counted; the granted Food is the only untapped
	// mana and the printed census cannot pay the {1}
	if s.perms[rsl].attached != tsl {
		t.Fatalf("Aura attached to slot %d, want the source's slot %d", s.perms[rsl].attached, tsl)
	}
	ab := &k.t.Cards[troll].Abilities[0]
	if !s.matchPerm(k, ab.Cost.Filter, rsl, 0, tsl) || s.matchPerm(k, ab.Cost.Filter, rsl, 0, NoSlot) {
		t.Fatal("the Sac filter does not read the ability's source")
	}
	if n := s.countSac(k, 0, ab.Cost.Filter, tsl); n != 1 {
		t.Fatalf("sacrifice candidates = %d, want the Aura", n)
	}
	if s.grantIdx(k, sc.slot(chowObj)) < 0 || !s.canTapForMana(k, sc.slot(chowObj)) {
		t.Fatal("the Food's granted {G} is not available")
	}
	if !s.matchPerm(k, creature, sc.slot(bearObj), 0, NoSlot) {
		t.Fatal("the fight's target is not a creature the source's controller does not control")
	}
	a := MakeAction(AkActivate, TCode(s.perms[tsl].obj), 0, 0)
	cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("grantPlayCost refused the activation")
	}
	if s.affordsUngrantedPlay(k, 0, a, &cost, ex, s.grantPayDesc(k, a)) {
		t.Fatal("the printed sources price the activation: the granted mana is not the board's only funding")
	}
	var g grantSrch
	if !s.grantSearch(k, 0, &cost, ex, s.grantPayDesc(k, a), need, &g) {
		t.Fatal("the grant search does not pay the activation its granted Food mana can fund")
	}
	// and the priority decision must offer it
	d := s.Step()
	if d.Kind != DkPriority || d.Player != 0 {
		t.Fatalf("Step = kind %d player %d, want p0's priority", d.Kind, d.Player)
	}
	if !hasOption(&d, a) {
		t.Fatal("the activation is not offered")
	}
}
