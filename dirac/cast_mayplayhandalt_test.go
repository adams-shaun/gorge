package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// mayPlayHandAltTable is the vanilla table plus four non-granting
// alternative-cost may-play statics over the hand (MayPlayDontGrantZonePermissions$
// True with MayPlayAltManaCost$ 0, PayLife<ConvertedManaCost>, PayEnergy<8> or
// CollectEvidence<10>: As Foretold, Demon of Fate's Design, Nissa, Worldsoul
// Speaker and Conspiracy Unraveler), each once per turn. They open no zone
// (kop.StMayZeroAlt): the alternative is offered only beside the plain hand
// cast (offerMayPlayHandAlt).
func mayPlayHandAltTable() (t *kop.Table, zeroC, lifeC, energyC, evidenceC CardID) {
	t = v.NewTable()
	t.Exprs = []kop.Expr{}
	mk := func(name string, fl kop.StaticFlags, a int) CardID {
		id := CardID(len(t.Cards))
		t.Cards = append(t.Cards, kop.CardDef{Name: name, Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatMayPlayFrom, Zone: kop.ZoneHand, A: int16(a),
				StFlags: kop.StMayZeroAlt | kop.StWithoutManaCost | kop.StOncePerTurn | fl}}}})
		return id
	}
	zeroC = mk("Test Zero Alt", 0, 0)
	lifeC = mk("Test Life Alt", kop.StMayPayLife, 0)
	energyC = mk("Test Energy Alt", kop.StMayPayEnergy, 8)
	evidenceC = mk("Test Evidence Alt", kop.StMayPayEvidence, 10)
	return t, zeroC, lifeC, energyC, evidenceC
}

// handAltOpts steps to the next decision and reports whether it offers the
// hand card o's plain cast and its non-granting alternative (the action
// carries zeroAltTgt).
func handAltOpts(t *testing.T, sc *scen, o Obj) (plain, alt bool) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkCast || a.Source() != TCode(o) {
			continue
		}
		if a.Target() == TCode(zeroAltTgt) {
			alt = true
		} else {
			plain = true
		}
	}
	return plain, alt
}

// handAltSetup puts the named alternative's permission under p0, two of
// card in p0's hand (the second for the once-per-turn check) and two blue
// sources on the battlefield, and reports the two hand objects.
func handAltSetup(t *testing.T, tbl *kop.Table, permC CardID) (sc *scen, first, second Obj) {
	t.Helper()
	sc = newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(permC, 0)
	sc.perm(v.Island, 0)
	sc.perm(v.Island, 0)
	first = sc.hand(v.ThinkTwice, 0)
	second = sc.hand(v.ThinkTwice, 0)
	return sc, first, second
}

// TestMayPlayHandAltZeroCast: a hand card's {0} alternative (MayPlayAltManaCost$
// 0) is offered beside its plain cast, takes no mana, and spends its
// once-per-turn turn -- the next cast of the same or another card lists the
// plain cast alone.
func TestMayPlayHandAltZeroCast(t *testing.T) {
	tbl, zeroC, _, _, _ := mayPlayHandAltTable()
	sc, first, second := handAltSetup(t, tbl, zeroC)

	plain, alt := handAltOpts(t, sc, first)
	if !plain || !alt {
		t.Fatalf("the first cast: plain %v alt %v, want both (options %+v)", plain, alt, sc.s.Step().Options)
	}
	tappedIsland := func() int {
		n := 0
		for sl := Slot(0); sl < Slot(sc.s.permHi); sl++ {
			if sc.s.cardAt(sl) == v.Island && sc.s.perms[sl].flags&pfTapped != 0 {
				n++
			}
		}
		return n
	}
	if n := tappedIsland(); n != 0 {
		t.Fatalf("precondition: %d Islands tapped at offer time", n)
	}
	sc.do(t, AkCast, TCode(first), TCode(zeroAltTgt))
	for i := 0; i < 8 && sc.s.objs[first].zone != kop.ZoneStack; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[first].zone != kop.ZoneStack {
		t.Fatalf("the {0} cast did not reach the stack (zone %d)", sc.s.objs[first].zone)
	}
	if sc.s.pl[0].life != 20 {
		t.Fatalf("life after the {0} cast is %d, want 20", sc.s.pl[0].life)
	}
	if n := tappedIsland(); n != 0 {
		t.Fatalf("the {0} cast tapped %d mana sources", n)
	}

	// priority rides back to p0 (the opponent passes, the caster never does,
	// so the spell does not resolve) and the second cast lists the plain cast
	// alone: the once-per-turn turn is spent
	for i := 0; ; i++ {
		if i == 40 {
			t.Fatal("seat 0's next priority never came")
		}
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d before seat 0's next priority", d.Kind)
		}
		if d.Player == 0 {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	plain, alt = handAltOpts(t, sc, second)
	if !plain {
		t.Fatalf("the second cast: plain %v alt %v, want the plain cast still offered", plain, alt)
	}
	if alt {
		t.Fatal("the second cast lists the {0} alternative again; the once-per-turn turn was not spent")
	}
}

// TestMayPlayHandAltLifeCast: Demon of Fate's Design's alternative (the mana
// value in life) is withheld while the life cannot cover it, offered at or
// above it, and takes exactly the mana value -- no mana, no energy.
func TestMayPlayHandAltLifeCast(t *testing.T) {
	tbl, _, lifeC, _, _ := mayPlayHandAltTable()
	mv := int(v.Table.Cards[v.ThinkTwice].Cost.MV())
	if mv != 2 {
		t.Fatalf("precondition: Think Twice has mana value %d, want 2", mv)
	}
	sc, first, _ := handAltSetup(t, tbl, lifeC)
	sc.s.pl[0].life = int16(mv - 1)
	if _, alt := handAltOpts(t, sc, first); alt {
		t.Fatalf("a cast owing %d life is offered at %d life", mv, sc.s.pl[0].life)
	}
	sc.s.pl[0].life = 10
	sc.s.g.energy[0] = 3
	if plain, alt := handAltOpts(t, sc, first); !plain || !alt {
		t.Fatalf("at %d life: plain %v alt %v, want both", sc.s.pl[0].life, plain, alt)
	}
	sc.do(t, AkCast, TCode(first), TCode(zeroAltTgt))
	for i := 0; i < 8 && sc.s.objs[first].zone != kop.ZoneStack; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[first].zone != kop.ZoneStack {
		t.Fatalf("the pay-life cast did not reach the stack (zone %d)", sc.s.objs[first].zone)
	}
	if got := int(sc.s.pl[0].life); got != 10-mv {
		t.Fatalf("life after the pay-life cast is %d, want %d", got, 10-mv)
	}
	if sc.s.g.energy[0] != 3 {
		t.Fatalf("energy after the pay-life cast is %d, want 3", sc.s.g.energy[0])
	}
}

// TestMayPlayHandAltEnergyCast: Nissa, Worldsoul Speaker's alternative (the
// static's energy) is withheld under the energy, offered at it, and takes
// exactly that much energy -- no life, no mana.
func TestMayPlayHandAltEnergyCast(t *testing.T) {
	tbl, _, _, energyC, _ := mayPlayHandAltTable()
	sc, first, _ := handAltSetup(t, tbl, energyC)
	sc.s.g.energy[0] = 7
	if _, alt := handAltOpts(t, sc, first); alt {
		t.Fatal("a cast owing 8 energy is offered at 7 energy")
	}
	sc.s.g.energy[0] = 8
	sc.s.pl[0].life = 14
	if plain, alt := handAltOpts(t, sc, first); !plain || !alt {
		t.Fatalf("at 8 energy: plain %v alt %v, want both", plain, alt)
	}
	sc.do(t, AkCast, TCode(first), TCode(zeroAltTgt))
	for i := 0; i < 8 && sc.s.objs[first].zone != kop.ZoneStack; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[first].zone != kop.ZoneStack {
		t.Fatalf("the pay-energy cast did not reach the stack (zone %d)", sc.s.objs[first].zone)
	}
	if sc.s.g.energy[0] != 0 {
		t.Fatalf("energy after the cast is %d, want 0", sc.s.g.energy[0])
	}
	if sc.s.pl[0].life != 14 {
		t.Fatalf("life after the cast is %d, want 14", sc.s.pl[0].life)
	}
}

// TestMayPlayHandAltEvidenceCast: Conspiracy Unraveler's alternative (the
// static's evidence, CR 701.59a) is withheld while the graveyard's total
// mana value does not reach it, offered at it, and is paid by exiling the
// picked graveyard cards -- no life, no energy, no mana.
func TestMayPlayHandAltEvidenceCast(t *testing.T) {
	tbl, _, _, _, evidenceC := mayPlayHandAltTable()
	mv := int(v.Table.Cards[v.ThinkTwice].Cost.MV())
	if mv != 2 {
		t.Fatalf("precondition: Think Twice has mana value %d, want 2", mv)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(evidenceC, 0)
	sc.perm(v.Island, 0)
	sc.perm(v.Island, 0)
	first := sc.hand(v.ThinkTwice, 0)
	sc.grave(v.ThinkTwice, 0, 4) // total mana value 8 < 10
	if plain, alt := handAltOpts(t, sc, first); !plain || alt {
		t.Fatalf("at grave total 8: plain %v alt %v, want the plain cast only", plain, alt)
	}
	grave := sc.grave(v.ThinkTwice, 0, 1) // total mana value 10
	tappedIsland := func() int {
		n := 0
		for sl := Slot(0); sl < Slot(sc.s.permHi); sl++ {
			if sc.s.cardAt(sl) == v.Island && sc.s.perms[sl].flags&pfTapped != 0 {
				n++
			}
		}
		return n
	}
	if plain, alt := handAltOpts(t, sc, first); !plain || !alt {
		t.Fatalf("at grave total 10: plain %v alt %v, want both", plain, alt)
	}
	sc.do(t, AkCast, TCode(first), TCode(zeroAltTgt))
	// the card is on the stack from the push on (CR 601.2a), so the loop
	// drives the build stages to the next priority, not to the push
	for i := 0; i < 24; i++ {
		d := sc.s.Step()
		if d.N == 0 || d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChCostEvidence {
			// the build's evidence pick: take the Done once the picked total
			// reaches the static's evidence, else the first grave card
			done := -1
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Kind() == AkDone {
					done = j
				}
			}
			if done >= 0 {
				sc.s.Apply(d.Options[done])
			} else {
				sc.s.Apply(d.Options[0])
			}
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[first].zone != kop.ZoneStack {
		t.Fatalf("the evidence cast did not reach the stack (zone %d)", sc.s.objs[first].zone)
	}
	if sc.s.pl[0].life != 20 {
		t.Fatalf("life after the evidence cast is %d, want 20", sc.s.pl[0].life)
	}
	if sc.s.g.energy[0] != 0 {
		t.Fatalf("energy after the evidence cast is %d, want 0", sc.s.g.energy[0])
	}
	if n := tappedIsland(); n != 0 {
		t.Fatalf("the evidence cast tapped %d mana sources", n)
	}
	for _, o := range grave {
		if sc.s.objs[o].zone != kop.ZoneExile {
			t.Fatalf("a picked evidence card is in zone %d, want exile", sc.s.objs[o].zone)
		}
	}
	if n := int(sc.s.pl[0].graveN); n != 0 {
		t.Fatalf("the graveyard holds %d cards after the payment, want none", n)
	}
}

// TestMayPlaySelfGrantTwoZones: Squee, the Immortal's self grant ("You may
// cast CARDNAME from your graveyard or from exile") lowers to ONE static
// with both function flags, so the class gates reach it from both zones and
// the cast is offered and pays its printed cost from either. A copy in hand
// lists only the plain hand cast.
func TestMayPlaySelfGrantTwoZones(t *testing.T) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	cd := &tbl.Cards[v.HillGiant]
	cd.Abilities = append([]kop.Ability(nil), cd.Abilities...) // a copy: vanilla's table is shared
	cd.Abilities = append(cd.Abilities, kop.Ability{Kind: kop.AbStatic, Static: kop.Static{
		Kind:    kop.StatMayPlayFrom,
		Zone:    kop.ZoneGraveyard, // the self walk keys on the flags, not the byte
		StFlags: kop.StFunctionsInGraveyard | kop.StFunctionsInExile}})
	castable := func(z kop.Zone) *scen {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(v.Plains, 0)
		sc.perm(v.Plains, 0)
		sc.perm(v.Plains, 0)
		sc.perm(v.Mountain, 0) // the {R} pip; the Plains pay the generic three
		o := sc.obj(v.HillGiant, 0)
		sc.s.place(o, z, false)
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(o) {
				return sc
			}
		}
		t.Fatalf("the %v-zone self-grant cast is not offered; opts %v", z, d.Options[:d.N])
		return nil
	}
	for _, z := range []kop.Zone{kop.ZoneGraveyard, kop.ZoneExile} {
		sc := castable(z)
		o := Obj(0)
		// find the giant: it is the only non-land card the table's seat 0 owns
		// outside the battlefield
		var d Decision
		d = sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() != TCode(v.Plains) {
				o = Obj(a.Source())
			}
		}
		sc.do(t, AkCast, TCode(o), scAny)
		for i := 0; i < 8 && sc.s.objs[o].zone != kop.ZoneStack; i++ {
			dd := sc.s.Step()
			if dd.N == 0 {
				break
			}
			sc.s.Apply(dd.Options[0])
		}
		if sc.s.objs[o].zone != kop.ZoneStack {
			t.Fatalf("the %v-zone cast did not reach the stack (zone %d)", z, sc.s.objs[o].zone)
		}
		tapped := 0
		for sl := Slot(0); sl < Slot(sc.s.permHi); sl++ {
			if sc.s.perms[sl].flags&pfTapped != 0 {
				tapped++
			}
		}
		if tapped != 4 {
			t.Fatalf("the %v-zone cast tapped %d sources, want 4 (the printed {3}{R})", z, tapped)
		}
	}
}
