package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// selfFlashTgtTable is vanilla's table plus a Timely Ward fixture: a {W} Aura
// "enchant creature" that grants itself flash while it can target a creature
// the caster controls (Filter2 = Creature.YouCtrl, kgen selfFlashTargetsFilter),
// and a vanilla bear.
func selfFlashTgtTable() (tbl *kop.Table, ward, bear CardID) {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	t.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	t.Conds = []kop.Cond{} // a generated table (prepareGen's gate)
	filter := func(f kop.Filter) kop.FilterID {
		t.Filters = append(t.Filters, f)
		return kop.FilterID(len(t.Filters) - 1)
	}
	creature := filter(kop.Filter{Types: kop.TCreature})
	mine := filter(kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	cost := kop.ManaCost{}
	cost.Pips[kop.W] = 1
	ward = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Ward", Cost: cost, Types: kop.TEnchantment | kop.TAura,
		Colors: kop.ColW, SelfFlash: true, Enchant: kop.TargetSpec{Kind: kop.TgtObject, Filter: creature},
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCastWithFlash, Filter2: mine}}}})
	bear = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	return &t, ward, bear
}

// wardCastOffered runs the priority walk and reports the hand cast of o.
func wardCastOffered(sc *scen, o Obj) (Action, bool) {
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(o) {
			return a, true
		}
	}
	return 0, false
}

// TestSelfFlashIsTargetingCensus: a self flash grant with a
// `Spell.IsTargeting Valid <spec>` clause (Timely Ward, Flash Photography) is
// offered at instant speed only while the spell can announce a target
// matching the spec; the target ask then withholds every other candidate, and
// the CR 601.2e recheck reverses an announced target that does not match.
// Sorcery-speed casts are unaffected.
func TestSelfFlashIsTargetingCensus(t *testing.T) {
	tbl, ward, bear := selfFlashTgtTable()
	sc := newScenTableLib(t, tbl, v.Plains, Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.anySelfFlash {
		t.Fatal("precondition: the table opens no self flash gate")
	}
	sc.perm(v.Plains, 0)
	theirs := sc.perm(bear, 1)
	o := sc.hand(ward, 0)
	// a legal target exists (the opponent's bear) but none the caster controls
	if !s.spellTargetsFeasible(k, ward, 0) {
		t.Fatal("precondition: the Aura has no legal target")
	}
	s.step = kop.StepUpkeep
	if _, ok := wardCastOffered(sc, o); ok {
		t.Fatal("offered at instant speed with only a non-matching target")
	}
	s.step = kop.StepMain1
	if _, ok := wardCastOffered(sc, o); !ok {
		t.Fatal("not offered at sorcery speed: the grant must not touch a sorcery-speed cast")
	}

	mine := sc.perm(bear, 0)
	s.step = kop.StepUpkeep
	act, ok := wardCastOffered(sc, o)
	if !ok {
		t.Fatal("not offered at instant speed with a matching target")
	}

	// the ask: the matching target alone is offered, the opponent's bear withheld
	s.Apply(act)
	d := s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	var got []TCode
	for i := 0; i < int(d.N); i++ {
		got = append(got, d.Options[i].Target())
	}
	if len(got) != 1 || got[0] != TCode(mine) {
		t.Fatalf("target options %v, want only the caster's bear %d (not %d)", got, mine, theirs)
	}

	// the recheck: an announced non-matching target is reversed, a matching one stands
	e := &s.bld().e
	e.tgt[0], e.nt = TCode(theirs), 1
	if !s.flashTgtRecheck(k, e) {
		t.Fatal("recheck passed a non-matching announced target")
	}
	e.tgt[0] = TCode(mine)
	if s.flashTgtRecheck(k, e) {
		t.Fatal("recheck reversed a matching announced target")
	}
	// at sorcery speed the same non-matching announcement is no business of the grant
	s.step = kop.StepMain1
	e.tgt[0] = TCode(theirs)
	if s.flashTgtRecheck(k, e) {
		t.Fatal("recheck bound a sorcery-speed cast")
	}
	// the stage wiring: the mana stage reverses the cast, the card back in hand
	s.step = kop.StepUpkeep
	if s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("precondition: the spell is in zone %v, want the stack", s.objs[o].zone)
	}
	if !s.manaStage(k) || s.objs[o].zone != kop.ZoneHand {
		t.Fatalf("manaStage left the non-matching cast standing: zone %v", s.objs[o].zone)
	}
}
