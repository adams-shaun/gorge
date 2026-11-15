package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// scopeFilterScen is a scenario whose table carries the two filtered scoped
// prevention cards: a Heavy Fog (damage by attacking creatures to its
// controller) and a Pack Leader (combat damage to its controller's Dogs).
type scopeFilterScen struct {
	*scen
	fog, leader, dog, bear, opp, anyp CardID
}

func newScopeFilterScen(t *testing.T) *scopeFilterScen {
	t.Helper()
	var ids scopeFilterScen
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
		tbl.Filters = append(tbl.Filters,
			kop.Filter{Types: kop.TCreature, Flags: kop.FAttacking},
			kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Sub: 1})
		atk, dogs := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
		ext := func(c int, f kop.FilterID, b int) []kop.Ability {
			in := kop.ExtInstr(kop.ExtPreventDamageScope)
			in.C, in.F, in.B = int16(c), f, kop.Const(b)
			return []kop.Ability{{Prog: []kop.Instr{in}}}
		}
		add := func(d kop.CardDef) CardID {
			tbl.Cards = append(tbl.Cards, d)
			return CardID(len(tbl.Cards) - 1)
		}
		if len(tbl.Subtypes) != 0 {
			panic("the scenario table's subtype 0 is not free")
		}
		tbl.Subtypes = append(tbl.Subtypes, "Dog")
		var dog kop.SubMask
		dog[0] = 1
		ids.fog = add(kop.CardDef{Name: "Test Fog", Types: kop.TInstant, Abilities: ext(kop.PrevScopeFiltered|kop.PrevScopePlayer, atk, 0)})
		ids.leader = add(kop.CardDef{Name: "Test Leader", Types: kop.TCreature, Subtypes: dog, Abilities: ext(kop.PrevScopeFiltered|kop.PrevScopeCombat, 0, int(dogs))})
		ids.opp = add(kop.CardDef{Name: "Test Opp Fog", Types: kop.TInstant, Abilities: ext(kop.PrevScopeFiltered|kop.PrevScopeOppPlayer|kop.PrevScopeCombat, 0, 0)})
		ids.anyp = add(kop.CardDef{Name: "Test Player Fog", Types: kop.TInstant, Abilities: ext(kop.PrevScopeFiltered|kop.PrevScopeAnyPlayer|kop.PrevScopeCombat, 0, 0)})
		ids.dog = add(kop.CardDef{Name: "Test Dog", Types: kop.TCreature, Subtypes: dog, Power: 2, Toughness: 2})
		ids.bear = add(kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	})
	ids.scen = sc
	return &ids
}

// resolve registers the Ext PreventDamageScope of card c's object for seat 0,
// as the Effect's resolution does, and returns the record.
func (f *scopeFilterScen) resolve(c CardID) *prevRec {
	o := f.obj(c, 0)
	f.s.res.e.obj, f.s.res.e.ctrl = o, 0
	f.s.prevScopeAdd(f.k, &f.k.t.Cards[c].Abilities[0].Prog[0])
	for i := range f.s.g.prevCol {
		if r := &f.s.g.prevCol[i]; r.obj == o && r.sc != 0 {
			return r
		}
	}
	return nil
}

// TestPreventScopeFilteredSourceAndTarget: Heavy Fog prevents only the
// damage attacking creatures would deal to its controller; Pack Leader's
// Effect only the combat damage dealt to the Dogs its controller controls.
func TestPreventScopeFilteredSourceAndTarget(t *testing.T) {
	f := newScopeFilterScen(t)
	s, k := f.s, f.k
	atk := f.perm(f.bear, 1)
	s.perms[f.slot(atk)].flags |= pfAttacking
	s.perms[f.slot(atk)].atk = TCode(PlayerTarget0)
	idle := f.perm(f.bear, 1)
	myDog, theirDog, myBear := f.perm(f.dog, 0), f.perm(f.dog, 1), f.perm(f.bear, 0)
	if !s.isCreature(k, f.slot(atk)) || s.perms[f.slot(atk)].flags&pfAttacking == 0 {
		t.Fatal("the attacker is not an attacking creature")
	}

	fog := f.resolve(f.fog)
	if fog == nil || fog.sc&kop.PrevScopeFiltered == 0 {
		t.Fatalf("Heavy Fog made no filtered record: %+v", fog)
	}
	hit := func(r *prevRec, src Obj, dst Slot, rcp uint8, combat bool) bool {
		return s.prevScopeHit(k, r, f.slot(src), s.objs[src].card, 0, prevTgt{rcp, dst}, combat)
	}
	for _, c := range []struct {
		name   string
		src    Obj
		dst    Slot
		rcp    uint8
		combat bool
		want   bool
	}{
		{"an attacker's combat damage to the controller", atk, NoSlot, 0, true, true},
		{"an attacker's noncombat damage to the controller", atk, NoSlot, 0, false, true},
		{"a non-attacker's damage to the controller", idle, NoSlot, 0, true, false},
		{"an attacker's damage to the other player", atk, NoSlot, 1, true, false},
		{"an attacker's damage to the controller's creature", atk, f.slot(myBear), 0, true, false},
	} {
		if got := hit(fog, c.src, c.dst, c.rcp, c.combat); got != c.want {
			t.Errorf("Heavy Fog, %s: prevented %v, want %v", c.name, got, c.want)
		}
	}

	leader := f.resolve(f.leader)
	if leader == nil || leader.sc&kop.PrevScopeFiltered == 0 {
		t.Fatalf("Pack Leader made no filtered record: %+v", leader)
	}
	for _, c := range []struct {
		name   string
		dst    Slot
		rcp    uint8
		combat bool
		want   bool
	}{
		{"combat damage to the controller's Dog", f.slot(myDog), 0, true, true},
		{"noncombat damage to the controller's Dog", f.slot(myDog), 0, false, false},
		{"combat damage to the controller's non-Dog", f.slot(myBear), 0, true, false},
		{"combat damage to the opponent's Dog", f.slot(theirDog), 1, true, false},
		{"combat damage to the controller", NoSlot, 0, true, false},
	} {
		if got := hit(leader, atk, c.dst, c.rcp, c.combat); got != c.want {
			t.Errorf("Pack Leader, %s: prevented %v, want %v", c.name, got, c.want)
		}
	}

	// end to end through the damage hook: both records are in the turn's set
	if !s.prevColHit(k, f.slot(atk), s.objs[atk].card, prevTgt{0, NoSlot}, true) {
		t.Error("prevColHit: the attacker's damage to the controller is not prevented")
	}
	if s.prevColHit(k, f.slot(idle), s.objs[idle].card, prevTgt{0, NoSlot}, true) {
		t.Error("prevColHit: a non-attacker's damage to the controller is prevented")
	}
}

// TestPreventScopeFilteredNeedsItsInstruction: a filtered record whose source
// card holds no filtered instruction prevents nothing.
func TestPreventScopeFilteredNeedsItsInstruction(t *testing.T) {
	f := newScopeFilterScen(t)
	r := &prevRec{obj: f.obj(f.bear, 0), sc: kop.PrevScopeFiltered | kop.PrevScopePlayer}
	if f.s.prevScopeHit(f.k, r, NoSlot, f.bear, 0, prevTgt{0, NoSlot}, true) {
		t.Fatal("a filtered record without its instruction prevented damage")
	}
}

// TestPreventScopeFilteredPlayerRecipients: the instruction-only recipient
// bits (ValidTarget$ Opponent / Player) leave the record's source-colour
// filter empty, so a coloured source's damage to those players is prevented.
func TestPreventScopeFilteredPlayerRecipients(t *testing.T) {
	f := newScopeFilterScen(t)
	s, k := f.s, f.k
	src := f.perm(f.bear, 1)
	white := kop.ColorMask(1)
	for _, c := range []struct {
		name string
		card CardID
		rcp  uint8
		bear bool
		want bool
	}{
		{"opponents, to the opponent", f.opp, 1, false, true},
		{"opponents, to the controller", f.opp, 0, false, false},
		{"opponents, noncombat, never", f.opp, 1, true, false},
		{"any player, to the controller", f.anyp, 0, false, true},
		{"any player, to the opponent", f.anyp, 1, false, true},
	} {
		r := f.resolve(c.card)
		if r == nil {
			t.Fatalf("%s: no record", c.name)
		}
		if r.col != 0 {
			t.Errorf("%s: record colour filter %#x, want none", c.name, r.col)
		}
		combat := !c.bear
		if got := s.prevScopeHit(k, r, f.slot(src), s.objs[src].card, white, prevTgt{c.rcp, NoSlot}, combat); got != c.want {
			t.Errorf("%s: prevented %v, want %v", c.name, got, c.want)
		}
		*r = prevRec{}
	}
}
