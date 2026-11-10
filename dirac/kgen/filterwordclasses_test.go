package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The tier-G sweep: one test per closed class. The
// parse-level tests need no corpus; the card-level ones lower the BLB
// carriers end to end (generate: lowering and emit) and skip without
// .cards.

func TestFilterWordClasses(t *testing.T) {
	known := func(s string) bool { return s == "Squirrel" }
	for _, tc := range []struct {
		expr string
		want func(c lClause) bool
	}{
		{"Squirrel.YouCtrl+StrictlyOther", func(c lClause) bool { return c.Flags == fOther && c.Ctrl == whoYou }},
		{"Creature.toughnessGE4", func(c lClause) bool { return c.CmpStat == cmpToughness && c.CmpOp == opGE && c.CmpN == 4 }},
		{"Creature.hasXCost", func(c lClause) bool { return c.CmpStat == cmpHasX }},
		{"Creature.powerLTtoughness", func(c lClause) bool { return c.CmpStat == cmpPowerLTToughness }},
		{"Creature.YouOwn+NotDefinedTriggeredCard", func(c lClause) bool { return c.Flags == fNotTriggerObj && c.Owner == whoYou }},
		// Pawpatch Recruit's "other than that creature" (the
		// BecomesTarget watcher's trigger object is the targeted one)
		{"Creature.YouCtrl+NotDefinedTriggeredTarget", func(c lClause) bool { return c.Flags == fNotTriggerObj && c.Ctrl == whoYou }},
	} {
		f, err := parseFilter(tc.expr, known, known)
		if err != nil || len(f.Clauses) != 1 || !tc.want(f.Clauses[0]) {
			t.Errorf("%s: got %s, %v", tc.expr, f, err)
		}
	}
	// two comparisons in one clause still refuse (one predicate per clause)
	for _, bad := range []string{"Creature.toughnessGE4+powerGE2", "Creature.hasXCost+cmcGE4"} {
		if f, err := parseFilter(bad, known, known); err == nil {
			t.Errorf("%s: lowered to %s; want a refusal", bad, f)
		}
	}
	// a toughness bound that is not a constant refuses at emit
	cb := &cardBuilder{}
	if _, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opGE, CmpX: "X"}, 0); err == nil {
		t.Errorf("toughness compare against a value: want a refusal")
	}
	if _, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opEQ, CmpN: 3}, 0); err == nil {
		t.Errorf("toughness EQ compare: want a refusal")
	}
	// ... but toughness <= / < a value lowers on the battlefield (Zero Point Ballad's toughnessLEX)
	x := &lValue{Kind: vX}
	cb = &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	if kf, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opLE, CmpX: "X", CmpV: x}, kop.ZoneBattlefield); err != nil || kf.Pred != kop.PredToughnessLEV || kf.PredV == (kop.ValueRef{}) {
		t.Errorf("toughnessLEX on the battlefield: got %+v, %v; want PredToughnessLEV with PredV", kf, err)
	}
	lt, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opLT, CmpX: "X", CmpV: x}, kop.ZoneBattlefield)
	le, _ := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opLE, CmpX: "X", CmpV: x}, kop.ZoneBattlefield)
	if err != nil || lt.Pred != kop.PredToughnessLEV || lt.PredV == le.PredV {
		t.Errorf("toughnessLTX: got %+v, %v; want PredToughnessLEV with the bound lowered by one", lt, err)
	}
	if _, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opLE, CmpX: "X", CmpV: x}, kop.ZoneGraveyard); err == nil {
		t.Errorf("toughnessLEX off the battlefield: want a refusal")
	}
	if _, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opGE, CmpX: "X", CmpV: x}, kop.ZoneBattlefield); err == nil {
		t.Errorf("toughnessGEX: want a refusal")
	}
}

func TestValueHeadClasses(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(s string) bool { return false }}
	for body, want := range map[string]valKind{
		"PlayerCountOpponents$HighestLifeTotal":                      vOppExtreme,
		"PlayerCountOpponents$HighestCardsInHand":                    vOppExtreme,
		"PlayerCountOpponents$LowestLifeTotal":                       vOppExtreme,
		"PlayerCountOpponents$HighestValid Land.YouCtrl":             vOppExtreme,
		"PlayerCountOpponents$HighestCounters.Poison":                vHighestOppPoison,
		"PlayerCountRegisteredOpponents$HasPropertyLostLifeThisTurn": vOppsLostLife,
		"Count$CardsInYourHand":                                      vHandSizeOf,
		"Count$ValidExile Creature.YouOwn":                           vCountExile,
		"Count$ValidGraveyard,Exile Creature.YouOwn":                 vSum,
	} {
		v, err := lc.svarValue(body, 0)
		if err != nil || v.Kind != want {
			t.Errorf("%s: got %s, %v; want %s", body, v, err, valNames[want])
		}
	}
	// Keen-Eyed Curator's exile card-types count lowers inside a continuous
	// static (its own source) and refuses anywhere else
	st := &lowerCtx{face: lc.face, known: lc.known, exwStatic: true, out: &lCard{}}
	if v, err := st.svarValue("Count$ValidExile Card.ExiledWithSource$CardTypes", 0); err != nil || v.Kind != vExileTypes {
		t.Errorf("exile $CardTypes in a static: got %s, %v; want ExileTypes", v, err)
	}
	if v, err := (&lowerCtx{face: lc.face, known: lc.known, out: &lCard{}}).svarValue("Count$ValidExile Card.ExiledWithSource$CardTypes", 0); err == nil {
		t.Errorf("exile $CardTypes outside a static lowered to %s; want a refusal", v)
	}
	v, _ := lc.svarValue("PlayerCountOpponents$HighestValid Land.YouCtrl", 0)
	// YouCtrl stays the controller word: the kernel rebinds it to each
	// opponent in turn (value_oppextreme.go, gorge SpecContext(p))
	if v.F == nil || v.F.Clauses[0].Ctrl != whoYou || v.N != kop.OppExtValid {
		t.Errorf("HighestValid Land.YouCtrl: each opponent's lands, got %s", v)
	}
	for _, bad := range []string{
		"PlayerCountOpponents$HighestValid Creature.Other+YouCtrl", // source-relative from the opponent's side
		"PlayerCountOpponents$HighestCounters.Experience",          // only Counters.Poison is read
		"PlayerCountOpponents$LowestCounters.Poison",               // only the Highest extreme
		"PlayerCountRegisteredOpponents$HasPropertyGainedLife",
		"Count$ValidExile Card.ExiledWithSource$CardPower", // only $CardTypes is read
		"Targeted$CardManaCost",
	} {
		if v, err := lc.svarValue(bad, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", bad, v)
		}
	}
}

// filterwordclassesCarriers is each closed class's BLB carrier.
var filterwordclassesCarriers = map[string]string{
	"Agate-Blade Assassin":       "TriggeredDefendingPlayer",
	"Valley Rotcaller":           "StrictlyOther",
	"Repel Calamity":             "toughnessGE4",
	"Helga, Skittish Seer":       "hasXCost",
	"Jackdaw Savior":             "NotDefinedTriggeredCard, cmcLTX, TriggeredCard$CardManaCost",
	"Fabled Passage":             "ConditionDescription",
	"Might of the Meek":          "ConditionDescription",
	"Harnesser of Storms":        "trigger ActivationLimit, Effect RememberObjects$ RememberedCard MayPlay",
	"Moonstone Harbinger":        "trigger ActivationLimit",
	"Hired Claw":                 "activation ActivationLimit + CheckSVar, AttackingPlayer by ValidAttackers",
	"Thought Shucker":            "Activation$ Threshold",
	"Spellgyre":                  "Charm Defined$ You",
	"Clifftop Lookout":           "DigUntil onto the battlefield, Tapped",
	"Lumra, Bellow of the Woods": "ChangeZoneAll Tapped",
	"Fecund Greenshell":          "powerLTtoughness, Dig Tapped",
	"Eddymurk Crab":              "ConditionPlayerTurn ETB replacement",
	"Beza, the Bounding Spring":  "PlayerCountOpponents$",
	"Gev, Scaled Scorch":         "PlayerCountRegisteredOpponents$",
	"Huskburster Swarm":          "Count$ValidGraveyard,Exile",
}

func TestFilterAndValueCarriersAccepted(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for n := range filterwordclassesCarriers {
		names = append(names, n)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	cv, err := generate(p, "b1", out, filepath.Join(out, idsFile))
	if err != nil {
		t.Fatal(err)
	}
	refused := map[string]string{}
	for _, r := range cv.refused {
		if r.Node != supportNode {
			refused[r.Card] = r.Reason
		}
	}
	for n, class := range filterwordclassesCarriers {
		if class == "" {
			if !strings.Contains(refused[n], "RememberedCard") {
				t.Errorf("%s: want the M11 refusal only, got %q", n, refused[n])
			}
			continue
		}
		if r, ok := refused[n]; ok {
			t.Errorf("%s (%s): refused: %s", n, class, r)
		}
	}
	// spot-check the lowered reads
	lw := lowerPool(p)
	var sb strings.Builder
	for _, c := range lw.cards {
		dumpLIR(&sb, c)
	}
	lir := sb.String()
	for _, want := range []string{
		"activate turnonce=true cond=Value(op1,OppLifeLostThisTurn,1)", // Hired Claw
		"activate turnonce=false cond=Threshold",                       // Thought Shucker
		"repl EntersWith F=nil cond=!YourTurn()",                       // Eddymurk Crab
		"LoseLife obj=Defending",                                       // Agate-Blade Assassin
		"once=true",                                                    // Moonstone Harbinger
		"OppExtreme",                                                   // Beza
		"A=OppsLostLife",                                               // Gev
	} {
		if !strings.Contains(lir, want) {
			t.Errorf("lowered IR lacks %q", want)
		}
	}
}
