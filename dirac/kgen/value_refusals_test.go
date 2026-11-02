package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestValueLoweringRefusesUnknownTokens is the silent-lowering ban: every
// modifier and value-head switch fed a token it cannot parse must refuse,
// never default the operand (the /Plus.Y lesson: Atoi("Y") lowered to 0).
func TestValueLoweringRefusesUnknownTokens(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{
		"Y":   "PlayerCountPropertyYou$LifeLostThisTurn",
		"Bad": "Count$NoSuchCount",
	}}}
	for _, body := range []string{
		// modifier switch: unknown modifier, non-int non-SVar operand,
		// missing operand, SVar operand with no compound form, bad SVar body
		"Count$YourLifeTotal/Bogus.1",
		"Count$YourLifeTotal/Plus.Q",
		"Count$YourLifeTotal/Minus.Q",
		"Count$YourLifeTotal/DivideEvenlyDown.Q",
		"Count$YourLifeTotal/DivideEvenlyDown.Y",
		"Count$YourLifeTotal/DivideEvenlyDown.0",
		"Count$YourLifeTotal/Plus",
		"Count$YourLifeTotal/Plus.",
		"Count$YourLifeTotal/Twice.Y",
		"Count$YourLifeTotal/Plus.Bad",
		// value heads
		"NoSuchHead$Foo",
		"PlayerCountPropertyYou$NoSuchProperty",
		"Count$NoSuchCount",
		"Number$Q",
		"TriggerCount$Q",
		"Remembered$Q",
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
	v, err := lc.svarValue("Count$LifeYouGainedThisTurn/Plus.Y", 0)
	if err != nil || v.Kind != vSum || len(v.Sub) != 2 || v.Sub[1].Kind != vLifeLostThisTurn || v.Sub[1].Neg {
		t.Errorf("/Plus.Y: got %s, %v; want Sum(LifeGainedThisTurn,LifeLostThisTurn)", v, err)
	}
	v, err = lc.svarValue("Count$LifeYouGainedThisTurn/Minus.Y", 0)
	if err != nil || v.Kind != vSum || !v.Sub[1].Neg {
		t.Errorf("/Minus.Y: got %s, %v; want Sum(..,-LifeLostThisTurn)", v, err)
	}
	v, err = lc.svarValue("Count$YourLifeTotal/Times.Y", 0)
	if err != nil || v.Kind != vProduct || len(v.Sub) != 2 || v.Sub[1].Kind != vLifeLostThisTurn {
		t.Errorf("/Times.Y: got %s, %v; want Product(LifeOf(You),LifeLostThisTurn)", v, err)
	}
	if v, err = lc.svarValue("Count$YourLifeTotal/Plus.2", 0); err != nil || v.Add != 2 {
		t.Errorf("/Plus.2: got %s, %v", v, err)
	}
}

// TestValueModifiers pins the modifiers and their refusals.
func TestValueModifiers(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}}
	for body, mul := range map[string]int{"Count$xPaid/Times.3": 3, "Count$xPaid/Thrice": 3} {
		if v, err := lc.svarValue(body, 0); err != nil || v.Mul != mul {
			t.Errorf("%q: got %s, %v; want *%d", body, v, err, mul)
		}
	}
	if v, err := lc.svarValue("Count$xPaid/HalfDown", 0); err != nil || v.Div != 2 {
		t.Errorf("HalfDown: got %s, %v", v, err)
	}
	for _, body := range []string{"Count$xPaid/Times.Q", "Count$xPaid/Times", "Count$xPaid/Plus.1/NMinus.2",
		"Count$YourLifeTotal/HalfDown", "Count$xPaid/Thrice.2", "Count$xPaid/Times.0"} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
	// a multiply after an add or divide wraps the value in a
	// sum with zero, which the kernel evaluates before the outer Mul.
	v, err := lc.svarValue("Count$xPaid/Plus.1/Times.2", 0)
	if err != nil || v.Kind != vSum || v.Mul != 2 || len(v.Sub) != 2 ||
		v.Sub[0].Kind != vX || v.Sub[0].Add != 1 || !v.Sub[1].isConst() {
		t.Errorf("Plus.1/Times.2: got %s, %v; want Sum(X+1,0)*2", v, err)
	}
	if v, err := lc.svarValue("Remembered$Amount/Plus.1", 0); err != nil || v.Kind != vRememberedCount || v.Add != 1 {
		t.Errorf("Remembered$Amount/Plus.1: got %s, %v; want RememberedCount+1", v, err)
	}
	if v, err := lc.svarValue("Remembered$CardManaCost/NMinus.10", 0); err != nil ||
		v.Kind != vMVOf || v.Add != -10 || !v.Neg {
		t.Errorf("Remembered$CardManaCost/NMinus.10: got %s, %v; want -(MV-10)", v, err)
	}
	if v, err := lc.svarValue("Remembered$Amount/DivideEvenlyDown.2", 0); err != nil ||
		v.Kind != vTokensCreated || v.Div != 2 {
		t.Errorf("Remembered$Amount/DivideEvenlyDown.2: got %s, %v; want TokensCreated/2", v, err)
	}
	// gorge evalRememberedOK cuts at the FIRST "/" and passes the whole
	// suffix to applyCountOp as one op, so a chain is not arithmetic it
	// reads: refuse rather than silently mis-lower it.
	for _, body := range []string{
		"Remembered$Amount/Plus.1/Times.2",
		"Remembered$CardManaCost/Plus.1/Plus.1",
		"Remembered$Amount/DivideEvenlyDown.2/Plus.1",
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
}

// TestValidGraveyardAggregateRefused is the ban on re-targeting a
// Count$ValidGraveyard aggregate to the battlefield: ExSumPower and
// ExDistinctCMC walk the battlefield occupancy (s.occ), and every validAggs
// entry is the ExValidAgg fold over battlefield permanents, so none reads the
// graveyard. kgen refuses instead of lowering the operand to something else;
// only the mana-value folds take their graveyard twin, ExGraveAgg.
func TestValidGraveyardAggregateRefused(t *testing.T) {
	// known: true -- the CardTypes head "Cave" (the reversed-zone-order
	// lowering below) is a subtype, which parseFilter resolves through
	// lc.known
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return true }}
	for _, body := range []string{
		"Count$ValidGraveyard Creature.YouOwn$CardPower",
		"Count$ValidGraveyard Card.YouOwn$DifferentCardNames",
		"Count$ValidGraveyard,Battlefield Card.YouOwn$CardTypes",
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
	for body, agg := range map[string]int{
		"Count$ValidGraveyard Card.YouOwn$DifferentCardManaCost": kop.AggDistinctMV,
		"Count$ValidGraveyard Card.YouCtrl$GreatestCardManaCost": kop.AggGreatestMV,
		"Count$ValidGraveyard Instant.YouOwn$CardManaCost":       kop.AggSumMV,
		"Count$ValidGraveyard Card.YouOwn$CardTypes":             kop.AggCardTypes,
		"Count$ValidGraveyard Creature.YouOwn$GreatestCardPower": kop.AggGreatestPower,
		"Count$ValidGraveyard Card$CardTypes":                    kop.AggCardTypes,
		"Count$ValidGraveyard Card.YouOwn$CardTypesPermanent":    kop.AggPermTypes,
	} {
		if v, err := lc.svarValue(body, 0); err != nil || v.Kind != vGraveAgg || v.N != agg {
			t.Errorf("%q: got %s, %v; want GraveAgg %d", body, v, err, agg)
		}
	}
	// the reversed zone order lowers to the same two-zone sum
	// (gorge multiCountZones: the union, in any order)
	v, err := lc.svarValue("Count$ValidGraveyard,Battlefield Cave.YouCtrl", 0)
	if err != nil || v.Kind != vSum || len(v.Sub) != 2 ||
		v.Sub[0].Kind != vCountGraveyard || v.Sub[1].Kind != vCount ||
		v.Sub[0].F == nil || v.Sub[0].F != v.Sub[1].F {
		t.Errorf("Count$ValidGraveyard,Battlefield Cave.YouCtrl: got %s, %v; want Sum(CountGraveyard,Count) over one filter", v, err)
	}
	v, err = lc.svarValue("Count$Valid Creature.YouCtrl$CardPower", 0)
	if err != nil || v.Kind != vSumPower {
		t.Errorf("Count$Valid Creature.YouCtrl$CardPower: got %s, %v; want vSumPower", v, err)
	}
}

// TestThisTurnEntered lowers the generic Count$ThisTurnEntered head onto
// kop.ExThisTurnEntered. gorge counts a per-ADD state.Entered list: a card
// that entered a zone twice counts twice, and a card that has since moved on
// is still listed, its validity read against the object wherever it now
// lives (CR 400.7 makes the moved card a new object; CR 603.10a supplies the
// departure-time creature status). The kernel's genState.entered ring carries
// the same entries. The one exact
// Graveyard_from_Battlefield_Creature spelling keeps the cheaper per-add
// `died` tally.
func TestThisTurnEntered(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(w string) bool { return w == "Zubera" }}
	for body, want := range map[string]struct {
		zone kop.Zone
		from int8
		prop uint8
		f2   bool
		f3   bool
		f4   bool
	}{
		"Count$ThisTurnEntered_Battlefield_Permanent.nonLand":                    {zone: kop.ZoneBattlefield, from: -1, f4: true},
		"Count$ThisTurnEntered_Battlefield_Creature":                             {zone: kop.ZoneBattlefield, from: -1, f2: true, f4: true},
		"Count$ThisTurnEntered_Battlefield_Creature.YouCtrl":                     {zone: kop.ZoneBattlefield, from: -1, f2: true, f4: true},
		"Count$ThisTurnEntered_Graveyard_Card.YouOwn":                            {zone: kop.ZoneGraveyard, from: -1},
		"Count$ThisTurnEntered_Exile_from_Battlefield_Creature":                  {zone: kop.ZoneExile, from: int8(kop.ZoneBattlefield), f2: true},
		"Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature,Artifact":     {zone: kop.ZoneGraveyard, from: int8(kop.ZoneBattlefield), f2: true, f3: true},
		"Count$ThisTurnEntered_Graveyard_from_Battlefield_Card.Zubera$CardPower": {zone: kop.ZoneGraveyard, from: int8(kop.ZoneBattlefield), prop: kop.PropCardPower},
		"Count$ThisTurnEntered_Hand_from_Battlefield_Permanent.YouOwn":           {zone: kop.ZoneHand, from: int8(kop.ZoneBattlefield)},
	} {
		v, err := lc.svarValue(body, 0)
		if err != nil {
			t.Errorf("%q: %v", body, err)
			continue
		}
		if v.Kind != vThisTurnEntered || v.Zone != uint8(want.zone) || v.From != want.from || v.Prop != want.prop {
			t.Errorf("%q: got %s zone=%d from=%d prop=%d; want zone=%d from=%d prop=%d", body, v, v.Zone, v.From, v.Prop, want.zone, want.from, want.prop)
			continue
		}
		if v.F == nil {
			t.Errorf("%q: no valid filter", body)
		}
		if (v.F2 != nil) != want.f2 || (v.F3 != nil) != want.f3 || (v.F4 != nil) != want.f4 {
			t.Errorf("%q: F2=%v F3=%v F4=%v; want %v/%v/%v", body, v.F2 != nil, v.F3 != nil, v.F4 != nil, want.f2, want.f3, want.f4)
		}
	}
	for _, body := range []string{
		"Count$ThisTurnEntered_",
		"Count$ThisTurnEntered_Nonsense_Card",
		"Count$ThisTurnEntered_Battlefield_",
		"Count$ThisTurnEntered_Graveyard_from_Nonsense_Card",
		"Count$ThisTurnEntered_Battlefield_Card_Too_Many_Parts",
		"Count$ThisTurnEntered_Stack_Card",
		"Count$ThisTurnEntered_Command_Card",
		"Count$ThisTurnEntered_Battlefield_from_Battlefield_Card",
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
	v, err := lc.svarValue("Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature", 0)
	if err != nil || v.Kind != vCreaturesDied {
		t.Errorf("exact died spelling: got %s, %v; want vCreaturesDied", v, err)
	}
}
