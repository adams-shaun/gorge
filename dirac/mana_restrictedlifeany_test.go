package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A restricted "{T}, Pay N life" unit of any one colour (Thieving Varmint's
// two mana for a spell you don't own): the sole-life unit's choice
// production (dirac mana_restrictedcost.go, kop.RestrictedLifeShape). gorge's
// planner prices it as a last-resort unit with the life a consequence
// (pay.PaymentPlanShapeTierOf) for exactly the payments its restriction
// admits (CR 106.6, 119.4); the two mana are one chosen colour (CR 106.1a)
// and what a payment leaves floats restricted (CR 106.6).

// restrLifeAnyScen is the restrictedmana pool's table with Thieving Varmint
// and the named cards in the deck; the scenario starts empty.
func restrLifeAnyScen(t *testing.T, names ...string) (sc *scen, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "restrictedmana")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the restrictedmana table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	return sc, id
}

// mayExile grants seat 0 a plain play permission record over o, another
// player's card in their exile (the version-tied record of
// cast_maypermission.go).
func mayExile(sc *scen, o Obj) {
	sc.s.g.may[0] = mayPlay{obj: o, ver: sc.s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(sc.s.turn)}
	sc.s.g.nmay = 1
}

// mayCast drives the offered cast of o through its decisions (targets,
// payment) to its resolution, reporting whether the offer listed it.
func mayCast(sc *scen, o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			break
		}
	}
	for i := 0; i < 12; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			return false
		}
		sc.s.Apply(d.Options[0])
		if z := sc.s.objs[o].zone; z != kop.ZoneStack && z != kop.ZoneExile {
			return true // resolved: battlefield, graveyard or hand
		}
	}
	return false
}

// TestRestrictedLifeOwnerWordSeatOne: the owner word on the Varmint's
// restriction ("spells you don't own") reads the caster's ownership for a
// seat-1 caster too (CR 109.2): an origin literal that carries no owner is
// the caster's own, never seat 0's, so seat 1's own Pacifism is no payment of
// the unit's and the cast is withheld; an owner the origin carries
// explicitly reads through (a Pacifism seat 0 owns is one the caster does
// not own, the unit's payment).
func TestRestrictedLifeOwnerWordSeatOne(t *testing.T) {
	sc, id := restrLifeAnyScen(t, "Thieving Varmint", "Pacifism")
	varm := sc.perm(id["Thieving Varmint"], 1)
	if sc.s.perms[sc.slot(varm)].flags&pfTapped != 0 {
		t.Fatal("Thieving Varmint is tapped; its unit is not available")
	}
	paci := sc.hand(id["Pacifism"], 1)
	k := sc.k
	ab := &k.t.Cards[id["Thieving Varmint"]].Abilities[k.gen.restrRecOf(id["Thieving Varmint"]).ab]
	own := payDesc{class: pdSpell, card: id["Pacifism"], src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
	if sc.s.restrSrcAdmits(k, 1, sc.slot(varm), ab, &own) {
		t.Fatal("the unit admits seat 1's own Pacifism; want it refused (an owner word reads the caster, not seat 0)")
	}
	foreign := own
	foreign.og.owner = 0
	foreign.og.ownSet = true
	if !sc.s.restrSrcAdmits(k, 1, sc.slot(varm), ab, &foreign) {
		t.Fatal("the unit refuses a Pacifism its caster does not own; want it admitted")
	}
	for i := 0; i < 20; i++ {
		d := sc.s.Step()
		for j := 0; j < int(d.N); j++ {
			if a := d.Options[j]; a.Kind() == AkCast && a.Source() == TCode(uint8(paci)) {
				t.Fatalf("seat 1's own Pacifism offered on the Varmint's unit; want it withheld (options %v)", d.Options[:d.N])
			}
		}
		if d.Player == 1 {
			return
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("never reached seat 1's priority")
}

// TestRestrictedLifeAnyUnitAdmitsAndPays: Thieving Varmint's unit is a
// planner unit for exactly the payments its restriction admits, pays the
// life it costs, makes one chosen colour, and what a payment leaves floats
// restricted.
func TestRestrictedLifeAnyUnitAdmitsAndPays(t *testing.T) {
	// (a) admission at the payment (CR 106.6, 601.2g-h): a spell the caster
	// owns is no payment of the unit's; a spell they do not own is one.
	sc, id := restrLifeAnyScen(t, "Thieving Varmint", "Pacifism", "Giant Growth")
	varm := sc.perm(id["Thieving Varmint"], 0)
	if sc.s.perms[sc.slot(varm)].flags&pfTapped != 0 {
		t.Fatal("Thieving Varmint is tapped; its unit is not available")
	}
	k := sc.k
	ab := &k.t.Cards[id["Thieving Varmint"]].Abilities[k.gen.restrRecOf(id["Thieving Varmint"]).ab]
	own := payDesc{class: pdSpell, card: id["Pacifism"], src: NoSlot, og: restrOrigin{from: kop.ZoneHand, owner: 0, ownSet: true}}
	foreign := own
	foreign.og.owner = 1
	if sc.s.restrSrcAdmits(k, 0, sc.slot(varm), ab, &own) {
		t.Fatal("(a) the unit admits a spell the caster owns; want it refused")
	}
	if !sc.s.restrSrcAdmits(k, 0, sc.slot(varm), ab, &foreign) {
		t.Fatal("(a) the unit refuses a spell the caster does not own; want it admitted")
	}

	// (b) another player's Pacifism ({1}{W}), offered on the unit alone (a
	// may-play of a card in their exile), is paid with it: one life and both
	// whites go into the payment, the Varmint spends its tap.
	sc2, id2 := restrLifeAnyScen(t, "Thieving Varmint", "Pacifism")
	varm2 := sc2.perm(id2["Thieving Varmint"], 0)
	paci := sc2.obj(id2["Pacifism"], 1)
	sc2.s.place(paci, kop.ZoneExile, false)
	mayExile(sc2, paci)
	if !mayCast(sc2, paci) {
		t.Fatal("(b) the Pacifism you don't own withheld with only the Varmint's unit: want it offered")
	}
	if sc2.s.objs[paci].zone == kop.ZoneStack || sc2.s.objs[paci].zone == kop.ZoneExile {
		t.Fatalf("(b) the Pacifism stuck in zone %d; want it through the stack", sc2.s.objs[paci].zone)
	}
	if sc2.s.g.nmay != 0 {
		t.Fatalf("(b) %d play permissions left; want the cast to spend the record", sc2.s.g.nmay)
	}
	if life := sc2.s.pl[0].life; life != 19 {
		t.Fatalf("(b) life %d after the unit's activation, want 19", life)
	}
	if sc2.s.restrN(0) != 0 || sc2.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("(b) after the payment: restricted %v pool %v, want both spent ({1}{W} takes both whites)", sc2.s.rb[0], sc2.s.pl[0].pool)
	}
	if sc2.s.perms[sc2.slot(varm2)].flags&pfTapped == 0 {
		t.Fatal("(b) Thieving Varmint untapped after its activation; the unit spent a tap")
	}

	// (c) one activation per tap: with the unit spent, another player's
	// Giant Growth is withheld although any colour would cover it.
	sc3, id3 := restrLifeAnyScen(t, "Thieving Varmint", "Pacifism", "Giant Growth")
	sc3.perm(id3["Thieving Varmint"], 0)
	paci3 := sc3.obj(id3["Pacifism"], 1)
	sc3.s.place(paci3, kop.ZoneExile, false)
	mayExile(sc3, paci3)
	if !mayCast(sc3, paci3) || sc3.s.pl[0].life != 19 {
		t.Fatal("(c) precondition: the Pacifism not paid by the unit")
	}
	gg := sc3.obj(id3["Giant Growth"], 1)
	sc3.s.place(gg, kop.ZoneExile, false)
	mayExile(sc3, gg)
	if d := sc3.s.Step(); mayCast(sc3, gg) {
		t.Fatalf("(c) Giant Growth offered on the spent unit: want it withheld (options %v)", d.Options[:d.N])
	}

	// (d) the colour is chosen per activation: another player's Giant
	// Growth ({G}) is paid by the same unit choosing green (CR 106.1a).
	sc4, id4 := restrLifeAnyScen(t, "Thieving Varmint", "Giant Growth")
	varm4 := sc4.perm(id4["Thieving Varmint"], 0)
	gg4 := sc4.obj(id4["Giant Growth"], 1)
	sc4.s.place(gg4, kop.ZoneExile, false)
	mayExile(sc4, gg4)
	if !mayCast(sc4, gg4) {
		t.Fatal("(d) Giant Growth you don't own withheld on the unit's green: want it offered")
	}
	if sc4.s.pl[0].life != 19 || sc4.s.perms[sc4.slot(varm4)].flags&pfTapped == 0 {
		t.Fatalf("(d) life %d tap %v after the growth's payment, want 19 and the Varmint spent", sc4.s.pl[0].life, sc4.s.perms[sc4.slot(varm4)].flags&pfTapped != 0)
	}

	// (e) what a payment leaves floats restricted: another player's
	// Savannah Lions ({W}) takes one white, one white stays floating
	// restricted.
	sc5, id5 := restrLifeAnyScen(t, "Thieving Varmint", "Savannah Lions")
	varm5 := sc5.perm(id5["Thieving Varmint"], 0)
	lions := sc5.obj(id5["Savannah Lions"], 1)
	sc5.s.place(lions, kop.ZoneExile, false)
	mayExile(sc5, lions)
	if !mayCast(sc5, lions) {
		t.Fatal("(e) the Lions you don't own withheld on the unit: want them offered")
	}
	if sc5.s.pl[0].life != 19 || sc5.s.perms[sc5.slot(varm5)].flags&pfTapped == 0 {
		t.Fatalf("(e) life %d tap %v after the Lions' payment, want 19 and the Varmint spent", sc5.s.pl[0].life, sc5.s.perms[sc5.slot(varm5)].flags&pfTapped != 0)
	}
	if sc5.s.restrN(0) != 1 || sc5.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("(e) after the payment: restricted %v pool %v, want one white floating restricted and an empty pool", sc5.s.rb[0], sc5.s.pl[0].pool)
	}

	// (f) control: beside a Forest the growth is paid from the land and the
	// unit stays unused.
	sc6, id6 := restrLifeAnyScen(t, "Thieving Varmint", "Giant Growth", "Forest")
	varm6 := sc6.perm(id6["Thieving Varmint"], 0)
	f := sc6.perm(id6["Forest"], 0)
	gg6 := sc6.obj(id6["Giant Growth"], 1)
	sc6.s.place(gg6, kop.ZoneExile, false)
	mayExile(sc6, gg6)
	if !mayCast(sc6, gg6) {
		t.Fatal("(f) Giant Growth withheld beside a Forest: want it offered")
	}
	if sc6.s.pl[0].life != 20 || sc6.s.perms[sc6.slot(varm6)].flags&pfTapped != 0 {
		t.Fatalf("(f) life %d Varmint tap %v beside the Forest, want 20 and the unit unused", sc6.s.pl[0].life, sc6.s.perms[sc6.slot(varm6)].flags&pfTapped != 0)
	}
	if sc6.s.perms[sc6.slot(f)].flags&pfTapped == 0 {
		t.Fatal("(f) Forest untapped; the growth's payment left no trace")
	}
}
