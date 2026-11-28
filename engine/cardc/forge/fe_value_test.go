package forge

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// svarTable is a test lookup over a fixed table.
func svarTable(m map[string]string) SVarLookup {
	return func(n string) (string, bool) { b, ok := m[n]; return b, ok }
}

func TestLowerValueTable(t *testing.T) {
	svars := svarTable(map[string]string{"Y": "Count$CardPower", "Z": "2"})
	cases := []struct{ in, want string }{
		{"3", "(value const 3)"},
		{"-2", "(value const -2)"},
		{"X", "(value x)"},
		{"+X", "(value x)"},
		{"-X", "(value mul (value const -1) (value x))"},
		{"Y", "(value power :ref (ref self))"},
		{"Number$4", "(value const 4)"},
		{"SVar$Y/Plus.1", "(value add (value power :ref (ref self)) (value const 1))"},
		{"Count$xPaid", "(value x)"},
		{"Count$CardPower", "(value power :ref (ref self))"},
		{"Count$CardToughness", "(value toughness :ref (ref self))"},
		{"Count$CardManaCost", "(value mana-value :ref (ref self))"},
		{"Count$YourLifeTotal", "(value life :ref (ref you))"},
		{"Count$YourStartingLife", "(value starting-life :ref (ref you))"},
		{"Count$LifeYouGainedThisTurn", "(value life-gained-this-turn :ref (ref you))"},
		{"Count$ChosenNumber", "(value chosen-number)"},
		{"Count$RememberedSize", "(value count-receipt :ref (ref memory remembered))"},
		{"Count$RememberedNumber", "(value receipt-amount :ref (ref memory remembered))"},
		{"Count$TriggerRememberAmount", "(value event-amount)"},
		{"Count$Domain", "(value domain :ref (ref you))"},
		{"Count$Party", "(value party-size :ref (ref you))"},
		{"Count$Converge", "(value converge)"},
		{"Count$AttackersDeclared", "(value attackers-declared)"},
		{"Count$TimesKicked", "(value times-kicked)"},
		{"Count$YouDrewThisTurn", "(value cards-drawn-this-turn :ref (ref you))"},
		{"Count$CardCounters.P1P1", "(value counters-among :ref (ref self) :word \"+1/+1\")"},
		{"Count$Devotion.Black", "(value devotion :ref (ref you) :colors B)"},
		{"Count$DevotionDual.White.Blue", "(value devotion :ref (ref you) :colors WU)"},
		{"Count$xPaid/Twice", "(value mul (value const 2) (value x))"},
		{"Count$xPaid/Thrice", "(value mul (value const 3) (value x))"},
		{"Count$xPaid/HalfUp", "(value half-up (value x))"},
		{"Count$xPaid/HalfDown", "(value half-down (value x))"},
		{"Count$xPaid/Minus.Z", "(value sub (value x) (value const 2))"},
		{"Count$xPaid/Times.2", "(value mul (value x) (value const 2))"},
		{"Count$xPaid/LimitMax.5", "(value min (value x) (value const 5))"},
		{"Count$xPaid/LimitMin.1", "(value max (value x) (value const 1))"},
		{"Count$Compare Y GE3.Z.0", "(value if (value const 2) (value const 0) :test (cond compare :l (value power :ref (ref self)) :r (value const 3) :cmp ge))"},
	}
	for _, c := range cases {
		v, r := LowerValue(c.in, svars)
		if r != nil {
			t.Errorf("LowerValue(%q) refused: %v", c.in, r)
			continue
		}
		if err := ir.ValidateValue(v); err != nil {
			t.Errorf("LowerValue(%q) does not validate: %v", c.in, err)
		}
		if got := v.String(); got != c.want {
			t.Errorf("LowerValue(%q)\n got %s\nwant %s", c.in, got, c.want)
		}
		if v.Src == "" {
			t.Errorf("LowerValue(%q) sets no Src", c.in)
		}
	}
}

// TestLowerValueFilterHeads checks the filter-taking heads by shape: the
// kind, the filter's scope zones and, for ThisTurnEntered, the origin.
func TestLowerValueFilterHeads(t *testing.T) {
	bf, gy, hand := ir.MakeZoneSet(ir.ZoneBattlefield), ir.MakeZoneSet(ir.ZoneGraveyard), ir.MakeZoneSet(ir.ZoneHand)
	cases := []struct {
		in            string
		kind          ir.ValueKind
		scope, origin ir.ZoneSet
	}{
		{"Count$Valid Creature.YouCtrl", ir.VCount, bf, 0},
		{"Count$Valid Creature,Artifact", ir.VCount, bf, 0},
		{"Count$ValidGraveyard Card.YouOwn", ir.VCount, gy, 0},
		{"Count$ValidHand Card.YouOwn", ir.VCount, hand, 0},
		{"Count$ValidLibrary Card.YouOwn", ir.VCount, ir.MakeZoneSet(ir.ZoneLibrary), 0},
		{"Count$ValidExile Card.YouOwn", ir.VCount, ir.MakeZoneSet(ir.ZoneExile), 0},
		{"Count$CardsInYourHand", ir.VCount, hand, 0},
		{"Count$Valid Creature.YouCtrl$CardPower", ir.VTotalPower, bf, 0},
		{"Count$ThisTurnCast_Card.YouCtrl", ir.VCastThisTurn, 0, 0},
		{"Count$ThisTurnEntered_Battlefield_Creature.YouCtrl", ir.VEnteredThisTurn, bf, 0},
		{"Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature", ir.VEnteredThisTurn, gy, bf},
		{"Count$Valid Creature.inZoneBattlefield", ir.VCount, bf, 0},
		{"Count$ValidGraveyard Creature.inZoneGraveyard", ir.VCount, gy, 0},
	}
	for _, c := range cases {
		v, r := LowerValue(c.in, nil)
		if r != nil {
			t.Errorf("LowerValue(%q) refused: %v", c.in, r)
			continue
		}
		if err := ir.ValidateValue(v); err != nil {
			t.Errorf("LowerValue(%q) does not validate: %v", c.in, err)
		}
		if v.Kind != c.kind || v.Filter == nil || v.Filter.Zones != c.scope || v.Zones != c.origin {
			t.Errorf("LowerValue(%q) = %s, want kind %s, scope %q, origin %q", c.in, v, c.kind, c.scope, c.origin)
		}
	}
	// An alternative takes the scope too.
	v, _ := LowerValue("Count$ValidGraveyard Creature,Artifact", nil)
	for _, a := range v.Filter.Args {
		if a.Zones != gy {
			t.Errorf("alternative %s has scope %q, want graveyard", a, a.Zones)
		}
	}
}

func TestLowerValueRefusals(t *testing.T) {
	svars := svarTable(map[string]string{"DBDraw": "DB$ Draw | NumCards$ 1", "F": "Creature.YouCtrl", "L": "L"})
	cases := []struct {
		in   string
		kind RefusalKind
		atom string
	}{
		{"", RefuseEmpty, ""},
		{"Count$WidgetsSeen", RefuseUnknownHead, "WidgetsSeen"},
		{"Count$ThisTurnActivated_Activated", RefuseUnknownHead, "ThisTurnActivated"},
		{"Count$Devotion.Plaid", RefuseUnknownHead, "Devotion.Plaid"},
		{"Count$ThisTurnEntered_Moon_Creature", RefuseUnknownHead, "ThisTurnEntered_Moon"},
		{"Count$Compare Y", RefuseUnknownHead, "Compare"},
		{"Count$Valid Creature.YouCtrl$GreatestCardPower", RefuseUnknownHead, "Valid$GreatestCardPower"},
		{"Count$xPaid/Squared", RefuseUnknownOperator, "Squared"},
		{"Count$xPaid/Plus", RefuseUnknownOperator, "Plus"},
		{"Count$xPaid/Twice.2", RefuseUnknownOperator, "Twice."},
		{"Count$Kicked.3.1", RefuseNeedsCondition, "Kicked"},
		{"Count$Morbid.2.0", RefuseNeedsCondition, "Morbid"},
		{"Targeted$CardPower", RefuseNeedsReference, "Targeted$CardPower"},
		{"Remembered$Amount/Plus.1", RefuseNeedsReference, "Remembered$Amount"},
		{"Count$Devotion.Chosen", RefuseNeedsReference, "Devotion.Chosen"},
		{"Count$CardCounters.WIDGET", RefuseUnknownCounter, "CardCounters.WIDGET"},
		{"Count$Valid Creature.blorp", RefuseUnknownPredicate, "Valid"},
		{"Count$ValidGraveyard Creature.blorp", RefuseUnknownPredicate, "ValidGraveyard"},
		{"Count$ThisTurnCast_Creature.blorp", RefuseUnknownPredicate, "ThisTurnCast"},
		{"Count$ThisTurnEntered_Battlefield_Creature.blorp", RefuseUnknownPredicate, "ThisTurnEntered"},
		{"Count$ValidGraveyard Creature.inZoneBattlefield", RefuseUnknownPredicate, "ValidGraveyard.inZone"},
		{"Count$Valid Card.inZoneExile", RefuseUnknownPredicate, "Valid.inZone"},
		{"Count$ValidGraveyard Creature.inZoneBattlefield,Artifact", RefuseUnknownPredicate, "ValidGraveyard.inZone"},
		{"Count$ThisTurnEntered_Battlefield_Creature.inZoneGraveyard", RefuseUnknownPredicate, "ThisTurnEntered.inZone"},
		{"Count$ThisTurnCast_Card.namedFoo$CardPower", RefuseUnknownHead, "ThisTurnCast$CardPower"},
		{"Count$ThisTurnEntered_Battlefield_Card.namedFoo$CardPower", RefuseUnknownHead, "ThisTurnEntered$CardPower"},
		{"Count$Random.0.2", RefuseUnknownHead, "Random"},
		{"L", RefuseSVarCycle, "L"},
		{"Result", RefuseSVarMissing, "Result"},
		{"Count$xPaid/Plus.Q7", RefuseSVarMissing, "QN"},
		{"DBDraw", RefuseNotANumber, "DB"},
		{"F", RefuseNotANumber, "Creature"},
		{"Number$many", RefuseNotANumber, "Number$"},
	}
	for k := firstValueRefusal; k <= lastValueRefusal; k++ {
		if !slices.ContainsFunc(cases, func(c struct {
			in   string
			kind RefusalKind
			atom string
		}) bool {
			return c.kind == k
		}) {
			t.Errorf("refusal kind %s has no case", k)
		}
	}
	for _, c := range cases {
		v, r := LowerValue(c.in, svars)
		if r == nil {
			t.Errorf("LowerValue(%q) = %s, want refusal %s", c.in, v, c.kind)
			continue
		}
		if r.Kind != c.kind || r.Atom != c.atom {
			t.Errorf("LowerValue(%q) refused %v, want %s: %s", c.in, r, c.kind, c.atom)
		}
	}
}

func TestLowerValueFollowsSVars(t *testing.T) {
	chain := svarTable(map[string]string{
		"A": "B", "B": "SVar$C/Plus.1", "C": "D", "D": "7", // four names deep
		"E": "A",                            // five
		"P": "Q", "Q": "Count$xPaid/Plus.P", // a cycle
		"S": "S",
		"M": "Count$CardPower/Plus.Nope",
		"X": "Count$CardPower", // a named X is its body, not VX
	})
	v, r := LowerValue("A", chain)
	if r != nil || v.String() != "(value add (value const 7) (value const 1))" {
		t.Errorf("four deep: %v %v", v, r)
	}
	for _, c := range []struct {
		in   string
		kind RefusalKind
		atom string
	}{
		{"E", RefuseSVarCycle, "D"},
		{"P", RefuseSVarCycle, "P"},
		{"S", RefuseSVarCycle, "S"},
		{"M", RefuseSVarMissing, "Nope"},
		{"Missing", RefuseSVarMissing, "Missing"},
	} {
		v, r := LowerValue(c.in, chain)
		if r == nil || r.Kind != c.kind || r.Atom != c.atom {
			t.Errorf("LowerValue(%q) = %v, %v; want %s: %s", c.in, v, r, c.kind, c.atom)
		}
	}
	if v, r := LowerValue("X", chain); r != nil || v.Kind != ir.VPower {
		t.Errorf("a named X: %v %v, want its body", v, r)
	}
	if v, r := LowerValue("X", nil); r != nil || v.Kind != ir.VX {
		t.Errorf("an unnamed X: %v %v, want VX", v, r)
	}
}
