package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// hexOwnScen is a table with a StatHexproofOwnColors static (Tam, Mindful
// First-Year's "each other creature you control has hexproof from each of
// its colors") and colour probes: a red, a blue and a colourless source.
func hexOwnScen(t *testing.T) (sc *scen, tam, red, blue, redSpell, blueSpell, colourlessSpell CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
	tbl.Conds = []kop.Cond{} // the generated-table gate (prepareGen)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther})
	f := kop.FilterID(len(tbl.Filters) - 1)
	next := func(c kop.CardDef) CardID {
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, c)
		return id
	}
	tam = next(kop.CardDef{Name: "Test Tam", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatHexproofOwnColors, Filter: f}}}})
	red = next(kop.CardDef{Name: "Test Red Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColR})
	blue = next(kop.CardDef{Name: "Test Blue Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColU})
	redSpell = next(kop.CardDef{Name: "Test Red Spell", Types: kop.TInstant, Colors: kop.ColR})
	blueSpell = next(kop.CardDef{Name: "Test Blue Spell", Types: kop.TInstant, Colors: kop.ColU})
	colourlessSpell = next(kop.CardDef{Name: "Test Colourless Spell", Types: kop.TInstant})
	sc = newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyHexOwn {
		t.Fatal("precondition: the table does not arm the own-colours hexproof scan (gen.anyHexOwn)")
	}
	return sc, tam, red, blue, redSpell, blueSpell, colourlessSpell
}

// hexOwnTargetable asks whether a spell (the probe) controlled by p may
// target the battlefield permanent o.
func hexOwnTargetable(sc *scen, card CardID, p uint8, o Obj) bool {
	sc.s.g.probe = probe{kind: probeSpell, card: card, ctrl: p}
	defer func() { sc.s.g.probe = probe{} }()
	return sc.s.objTargetable(sc.k, sc.slot(o), p)
}

// TestStaticHexproofOwnColorsTargeting: an opponent's red spell can't target
// a red creature beside Tam while a blue or a colourless one can; Tam itself
// is targetable and its controller's own spells still target.
func TestStaticHexproofOwnColorsTargeting(t *testing.T) {
	sc, tam, red, _, redSpell, blueSpell, colourlessSpell := hexOwnScen(t)
	sc.perm(tam, 0)
	redObj := sc.perm(red, 0)
	if sc.s.kw(sc.k, sc.slot(redObj))&kop.KwHexproof != 0 {
		t.Fatal("precondition: the target has printed hexproof")
	}
	if hexOwnTargetable(sc, redSpell, 1, redObj) {
		t.Fatal("an opponent's red spell targeted a red creature with hexproof from red")
	}
	if !hexOwnTargetable(sc, blueSpell, 1, redObj) {
		t.Fatal("an opponent's blue spell could not target a red creature")
	}
	if !hexOwnTargetable(sc, colourlessSpell, 1, redObj) {
		t.Fatal("an opponent's colourless spell could not target a red creature")
	}
	if !hexOwnTargetable(sc, redSpell, 0, redObj) {
		t.Fatal("the controller's own red spell could not target their creature")
	}
	tamObj := sc.perm(tam, 0)
	if !hexOwnTargetable(sc, redSpell, 1, tamObj) {
		t.Fatal("Tam itself was hexproof though the static says other creatures")
	}
}

// TestStaticHexproofOwnColorsCurrentColours: the static reads the target's
// CURRENT colours (CR 702.11d), so a colour change to all colours makes it
// hexproof from every coloured source.
func TestStaticHexproofOwnColorsCurrentColours(t *testing.T) {
	sc, tam, red, blue, redSpell, blueSpell, colourlessSpell := hexOwnScen(t)
	sc.perm(tam, 0)
	redObj := sc.perm(red, 0)
	blueObj := sc.perm(blue, 0)
	tbl := sc.k.t
	all := kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG
	tbl.Overrides = append(tbl.Overrides, kop.Override{HasColors: true, SetColors: all})
	sc.s.applyOverride(sc.k, sc.slot(redObj), len(tbl.Overrides)-1, red, false)
	if got := sc.s.colorsGen(sc.k, sc.slot(redObj)); got != all {
		t.Fatalf("precondition: the recolour set %v, want all colours", got)
	}
	if hexOwnTargetable(sc, redSpell, 1, redObj) || hexOwnTargetable(sc, blueSpell, 1, redObj) {
		t.Fatal("an opponent's coloured spell targeted an all-colours creature with hexproof from each colour")
	}
	if !hexOwnTargetable(sc, colourlessSpell, 1, redObj) {
		t.Fatal("a colourless source was stopped by a colour quality")
	}
	if hexOwnTargetable(sc, blueSpell, 1, blueObj) {
		t.Fatal("the blue creature was not hexproof from blue")
	}
}

// TestStaticHexproofOwnColorsLayerSix: the grant is a layer-6 effect. A loss
// of the target's abilities newer than Tam's static removes it; a grant
// newer than the loss survives (CR 613.7).
func TestStaticHexproofOwnColorsLayerSix(t *testing.T) {
	t.Run("a newer loss removes the grant", func(t *testing.T) {
		sc, tam, red, _, redSpell, _, _ := hexOwnScen(t)
		sc.perm(tam, 0)
		redObj := sc.perm(red, 0)
		ts := sc.s.tick()
		if !sc.s.setLossFx(sc.slot(redObj), ts, false) {
			t.Fatal("precondition: the loss record did not fit")
		}
		sc.s.perms[sc.slot(redObj)].ovr.flags |= ovLoseAll
		if !sc.s.genLost(sc.k, sc.slot(redObj)) {
			t.Fatal("precondition: the target did not lose its abilities")
		}
		if lts, lost := sc.s.lossAt(sc.k, sc.slot(redObj)); !lost || lts != ts {
			t.Fatalf("precondition: loss timestamp %d, lost %v, want %d, true", lts, lost, ts)
		}
		if !hexOwnTargetable(sc, redSpell, 1, redObj) {
			t.Fatal("a hexproof granted before the newer ability loss still stopped the spell")
		}
	})
	t.Run("a newer grant survives the loss", func(t *testing.T) {
		sc, tam, red, _, redSpell, _, _ := hexOwnScen(t)
		redObj := sc.perm(red, 0)
		if !sc.s.setLossFx(sc.slot(redObj), sc.s.tick(), false) {
			t.Fatal("precondition: the loss record did not fit")
		}
		sc.s.perms[sc.slot(redObj)].ovr.flags |= ovLoseAll
		tamObj := sc.perm(tam, 0) // a static newer than the loss
		if lts, _ := sc.s.lossAt(sc.k, sc.slot(redObj)); sc.s.grantTs(statSrc{sl: sc.slot(tamObj)}) < lts {
			t.Fatal("precondition: the static is not newer than the loss")
		}
		if hexOwnTargetable(sc, redSpell, 1, redObj) {
			t.Fatal("a hexproof granted after the ability loss did not stop the spell")
		}
	})
}

// TestStaticHexproofOwnColorsSourceLosesAbilities: a source that lost its
// abilities has no static, so the creatures are targetable again.
func TestStaticHexproofOwnColorsSourceLosesAbilities(t *testing.T) {
	sc, tam, red, _, redSpell, _, _ := hexOwnScen(t)
	tamObj := sc.perm(tam, 0)
	redObj := sc.perm(red, 0)
	if !sc.s.setLossFx(sc.slot(tamObj), sc.s.tick(), false) {
		t.Fatal("precondition: the loss record did not fit")
	}
	sc.s.perms[sc.slot(tamObj)].ovr.flags |= ovLoseAll
	if !sc.s.genLost(sc.k, sc.slot(tamObj)) {
		t.Fatal("precondition: the source did not lose its abilities")
	}
	if !hexOwnTargetable(sc, redSpell, 1, redObj) {
		t.Fatal("a source that lost its abilities still granted hexproof")
	}
}
