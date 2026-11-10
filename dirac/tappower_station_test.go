package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTapPowerStationStatic is the kernel half of the TapPowerValue static:
// Tapestry Warden's power<toughness creatures station with their toughness
// (a 1/3 for 3, a 3/1 for its own 3), and Sulu's tripled power crews a
// Vehicle needing 6 and stations a Spacecraft for 6, while a Planet gets no
// Station bonus (Sulu's Station qualifier names Spacecraft).
func TestTapPowerStationStatic(t *testing.T) {
	tbl := v.NewTable()
	tbl.Subtypes = []string{"Spacecraft", "Vehicle"}
	fWarden := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Pred: kop.PredPowerLTToughness})
	fSelf := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Flags: kop.FSelf})
	fOther := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther})
	fSpacecraft := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Sub: 1}) // Spacecraft index 0
	fVehicle := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Sub: 2}) // Vehicle index 1
	// Exprs[0] is unused; index 1 is Sulu's Count$CardPower/Twice, evaluated
	// with the tapped creature as the static value's source (RefSelf).
	tbl.Exprs = []kop.Expr{{}, {Kind: kop.ExPowerOf, Ref: kop.RefSelf, Mul: 2}}
	suluValue := kop.ValueRef{K: kop.VExpr, N: 1}

	warden := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Tapestry Warden", Types: kop.TCreature, Power: 3, Toughness: 4,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatTapPower, Filter: fWarden,
			StFlags: kop.StTapPowerStation | kop.StTapPowerTough,
		}}},
	})
	sulu := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Sulu, Heroic Helmsman", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatTapPower, Filter: fSelf, Filter2: fSpacecraft,
				StFlags: kop.StTapPowerStation | kop.StDynamic, VA: suluValue,
			}},
			{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatTapPower, Filter: fSelf, Filter2: fVehicle,
				StFlags: kop.StTapPowerCrew | kop.StDynamic, VA: suluValue,
			}},
		},
	})
	oneThree := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "One Three", Types: kop.TCreature, Power: 1, Toughness: 3})
	threeOne := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Three One", Types: kop.TCreature, Power: 3, Toughness: 1})
	vehicle := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Vehicle", Types: kop.TArtifact | kop.TVehicle, Subtypes: (kop.SubMask{}).With(1),
		Abilities: []kop.Ability{{Kind: kop.AbActivated,
			// the plain powers of Warden 3, One Three 1, Three One 3 and
			// Sulu 2 sum to 9: only Sulu's Crew bonus reaches 10
			Cost: kop.Cost{Flags: kop.CostTapPower | kop.CostCrew, N: 10, Filter: fOther}}},
	})
	spacecraft := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Spacecraft", Types: kop.TArtifact, Subtypes: (kop.SubMask{}).With(0)})
	planet := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Planet", Types: kop.TLand})

	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.anyTapPowerStatic {
		t.Fatal("prepare did not arm the TapPower static walk")
	}
	sc.perm(warden, 0)
	oneThreeObj := sc.perm(oneThree, 0)
	oneThreeSl := sc.slot(oneThreeObj)
	threeOneSl := sc.slot(sc.perm(threeOne, 0))
	suluSl := sc.slot(sc.perm(sulu, 0))
	vehicleSl := sc.slot(sc.perm(vehicle, 0))
	shipSl := sc.slot(sc.perm(spacecraft, 0))
	planetSl := sc.slot(sc.perm(planet, 0))

	// Warden: toughness for a power<toughness creature, plain power otherwise.
	if got := sc.s.tapPowerTotal(sc.k, oneThreeSl, 0, true, shipSl); got != 3 {
		t.Fatalf("a 1/3 stations for %d, want its toughness 3", got)
	}
	if got := sc.s.tapPowerTotal(sc.k, threeOneSl, 0, true, shipSl); got != 3 {
		t.Fatalf("a 3/1 stations for %d, want its power 3", got)
	}
	// The value head reads the same total through the Station trigger object.
	ctx := evalCtx{e: &stackEntry{trigObj: oneThreeObj}, srcSl: shipSl, ctrl: 0}
	if got := sc.s.tapPowerAt(sc.k, kop.RefTriggerObj, ctx); got != 3 {
		t.Fatalf("ExTapPower over the tapped 1/3 read %d, want 3", got)
	}

	// Sulu: 2 power, tripled when the host is the scoped permanent.
	if got := sc.s.tapPowerValue(sc.k, suluSl, kop.CostCrew, false, vehicleSl); got != 4 {
		t.Fatalf("Sulu's Crew bonus %d, want twice his power (4)", got)
	}
	if got := sc.s.tapPowerTotal(sc.k, suluSl, kop.CostCrew, false, vehicleSl); got != 6 {
		t.Fatalf("Sulu crews for %d, want 6", got)
	}
	va := &tbl.Cards[vehicle].Abilities[0]
	if !sc.s.tapPowerPayable(sc.k, va, vehicleSl) {
		t.Fatal("Sulu's tripled power did not make the Crew-10 Vehicle payable")
	}
	DebugIgnoreTapPower = true
	plainOK := sc.s.tapPowerPayable(sc.k, va, vehicleSl)
	DebugIgnoreTapPower = false
	if plainOK {
		t.Fatal("the Crew-10 Vehicle is payable without the bonus: the check proves nothing")
	}
	if got := sc.s.tapPowerTotal(sc.k, suluSl, 0, true, shipSl); got != 6 {
		t.Fatalf("Sulu stations a Spacecraft for %d, want 6", got)
	}
	if got := sc.s.tapPowerTotal(sc.k, suluSl, 0, true, planetSl); got != 2 {
		t.Fatalf("Sulu stations a Planet for %d, want his power 2", got)
	}
}

// TestTapPowerStaticNoGen: a table whose only extension is a StatTapPower
// static has no generated prep; the walk must still read the static (and not
// dereference the missing gen).
func TestTapPowerStaticNoGen(t *testing.T) {
	tbl := v.NewTable()
	fWarden := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	warden := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Tapestry Warden", Types: kop.TCreature, Power: 3, Toughness: 4,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatTapPower, Filter: fWarden,
			StFlags: kop.StTapPowerStation | kop.StTapPowerTough,
		}}},
	})
	oneThree := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "One Three", Types: kop.TCreature, Power: 1, Toughness: 3})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen != nil {
		t.Fatal("the table has a generated extension: the nil-gen path is not exercised")
	}
	if !sc.k.anyTapPowerStatic {
		t.Fatal("prepare did not arm the TapPower static walk")
	}
	sc.perm(warden, 0)
	sl := sc.slot(sc.perm(oneThree, 0))
	if got := sc.s.tapPowerTotal(sc.k, sl, 0, true, NoSlot); got != 3 {
		t.Fatalf("a 1/3 stations for %d, want its toughness 3", got)
	}
}

// TestTapPowerStationLiteralAll is Stoic Star-Captain's shape: a literal +2
// for every creature its controller has (not only itself), scoped to Crew and
// Station with no host qualifier. The opponent's creature gets nothing, and a
// Spell-scoped read (no flags, not a station) sees no bonus.
func TestTapPowerStationLiteralAll(t *testing.T) {
	tbl := v.NewTable()
	fYours := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	captain := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Stoic Star-Captain", Types: kop.TCreature, Power: 2, Toughness: 3,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatTapPower, Filter: fYours, A: 2,
			StFlags: kop.StTapPowerStation | kop.StTapPowerCrew,
		}}},
	})
	bear := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	ship := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Spacecraft", Types: kop.TArtifact})

	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(captain, 0)
	mine := sc.slot(sc.perm(bear, 0))
	theirs := sc.slot(sc.perm(bear, 1))
	shipSl := sc.slot(sc.perm(ship, 0))

	if got := sc.s.tapPowerTotal(sc.k, mine, 0, true, shipSl); got != 4 {
		t.Fatalf("my 2/2 stations for %d, want 4", got)
	}
	if got := sc.s.tapPowerTotal(sc.k, mine, kop.CostCrew, false, shipSl); got != 4 {
		t.Fatalf("my 2/2 crews for %d, want 4", got)
	}
	if got := sc.s.tapPowerTotal(sc.k, mine, kop.CostSaddle, false, shipSl); got != 2 {
		t.Fatalf("my 2/2 saddles for %d, want its power 2 (Crew and Station only)", got)
	}
	if got := sc.s.tapPowerTotal(sc.k, theirs, 0, true, shipSl); got != 2 {
		t.Fatalf("the opponent's 2/2 stations for %d, want its power 2", got)
	}
}
