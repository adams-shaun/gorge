package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runPTFilterScenario runs one inline oracle scenario and returns every
// checkpoint snapshot; the scenario's own failures fail the test.
func runPTFilterScenario(t *testing.T, raw string) []OracleSnapshot {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	return res.Snapshots
}

func ptPermAt(s OracleSnapshot, ref string) (OracleSnapPerm, bool) { return snapPerm(s, ref) }

// wantPT asserts ref is on the battlefield of s with current P/T pt: the
// precondition every filter test below leans on.
func wantPT(t *testing.T, s OracleSnapshot, ref, pt string) {
	t.Helper()
	p, ok := ptPermAt(s, ref)
	if !ok || p.PT != pt {
		t.Fatalf("precondition: %s = %+v (on battlefield %v), want P/T %s", ref, p, ok, pt)
	}
}

func wantAlive(t *testing.T, s OracleSnapshot, ref string, alive bool) {
	t.Helper()
	if _, ok := ptPermAt(s, ref); ok != alive {
		t.Fatalf("%s on battlefield = %v, want %v", ref, ok, alive)
	}
}

// Retribution of the Meek reads current power (CR 613.4, 708.2): Bears under
// two Glorious Anthems are 4/4 and die; a face-down Krosan Colossus is a 2/2
// and survives; a bare Craw Wurm (6/4) is the control that the spell ran.
func TestDestroyAllPowerFilterReadsCurrentPT(t *testing.T) {
	t.Parallel()
	snaps := runPTFilterScenario(t, `{"name":"retribution","setup":{
	  "p0":{"hand":["Krosan Colossus","Retribution of the Meek"]},
	  "p1":{"battlefield":["Grizzly Bears","Glorious Anthem","Glorious Anthem","Craw Wurm"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Krosan Colossus","mana":"CCC","cast_mode":"morphed"},{"op":"resolve"},
	  {"op":"cast","seat":0,"card":"p0:Retribution of the Meek","mana":"CCW"},{"op":"resolve"}]}`)
	pre, last := snaps[2], snaps[len(snaps)-1]
	wantPT(t, pre, "p1:Grizzly Bears", "4/4")
	fd := 0
	for _, p := range pre.Permanents {
		if p.FaceDown && p.PT == "2/2" {
			fd++
		}
	}
	if fd != 1 {
		t.Fatalf("precondition: want one face-down 2/2 before the spell, got %d: %+v", fd, pre.Permanents)
	}
	wantAlive(t, last, "p1:Grizzly Bears", false)
	wantAlive(t, last, "p1:Craw Wurm", false)
	fd = 0
	for _, p := range last.Permanents {
		if p.FaceDown {
			fd++
		}
	}
	if fd != 1 {
		t.Fatalf("the face-down 2/2 must survive Retribution of the Meek, got %d face-down permanents", fd)
	}
}

// Zero Point Ballad (X=2) destroys creatures with current toughness 2 or
// less: a Glorious Anthem'd Bears (3/3) lives, a bare Llanowar Elves dies.
func TestDamageAllToughnessFilterReadsCurrentPT(t *testing.T) {
	t.Parallel()
	snaps := runPTFilterScenario(t, `{"name":"zpb","setup":{
	  "p0":{"battlefield":["Grizzly Bears","Glorious Anthem"],"hand":["Zero Point Ballad"]},
	  "p1":{"battlefield":["Mons's Goblin Raiders"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Zero Point Ballad","mana":"CCB","answers":[{"kind":"choose","pick":["X = 2"]}]},{"op":"resolve"}]}`)
	pre, last := snaps[0], snaps[len(snaps)-1]
	wantPT(t, pre, "p0:Grizzly Bears", "3/3")
	wantPT(t, pre, "p1:Mons's Goblin Raiders", "1/1")
	wantAlive(t, last, "p0:Grizzly Bears", true)
	wantAlive(t, last, "p1:Mons's Goblin Raiders", false)
}

// Depressurize's second body (Defined$ Targeted.powerLE0) reads the power
// the Pump of the first body just gave: Bears 2/2 -> -1/2 die, a 4/4 -> 1/4
// lives. The second body is a new boundary, so it sees the pumped value.
// Defined$ Targeted.<pt> already binds derived P/T per candidate, so this is
// a regression guard, not a table consumer (TestChooseCardPowerFilterReadsCurrentPT
// is the table-dependent carrier).
func TestDefinedTargetedPowerGateReadsPump(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		bf    string
		alive bool
	}{
		{"bare-bears", `"Grizzly Bears"`, false},
		{"anthemed-bears", `"Grizzly Bears","Glorious Anthem","Glorious Anthem"`, true},
	} {
		snaps := runPTFilterScenario(t, `{"name":"`+tc.name+`","setup":{"p0":{"battlefield":[`+tc.bf+`],"hand":["Depressurize"]}},
		 "steps":[{"op":"cast","seat":0,"card":"p0:Depressurize","mana":"CB","targets":["p0:Grizzly Bears"]},{"op":"resolve"}]}`)
		if !tc.alive {
			wantPT(t, snaps[0], "p0:Grizzly Bears", "2/2")
		} else {
			wantPT(t, snaps[0], "p0:Grizzly Bears", "4/4")
		}
		wantAlive(t, snaps[len(snaps)-1], "p0:Grizzly Bears", tc.alive)
	}
}

// Cactusfolk Sureshot's beginning-of-combat PumpAll reads current power: an
// anthem-boosted Hill Giant (printed 3, now 4) gains trample and haste, a
// Craw Wurm (printed 6) shrunk to 1 power by two Depressurizes does not.
func TestPumpAllFilterReadsCurrentPT(t *testing.T) {
	t.Parallel()
	snaps := runPTFilterScenario(t, `{"name":"sureshot","setup":{
	  "p0":{"battlefield":["Cactusfolk Sureshot","Hill Giant","Craw Wurm","Glorious Anthem"],"hand":["Depressurize","Depressurize"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Depressurize","mana":"CB","targets":["p0:Craw Wurm"]},{"op":"resolve"},
	  {"op":"cast","seat":0,"card":"p0:Depressurize#2","mana":"CB","targets":["p0:Craw Wurm"]},{"op":"resolve"},
	  {"op":"pass_to","seat":0,"step":"begin-combat"},{"op":"resolve"}]}`)
	last := snaps[len(snaps)-1]
	wantPT(t, last, "p0:Hill Giant", "4/4")
	wantPT(t, last, "p0:Craw Wurm", "1/5")
	hg, _ := ptPermAt(last, "p0:Hill Giant")
	cw, _ := ptPermAt(last, "p0:Craw Wurm")
	has := func(p OracleSnapPerm, kw string) bool {
		for _, k := range p.Keywords {
			if k == kw {
				return true
			}
		}
		return false
	}
	if !has(hg, "Trample") || !has(hg, "Haste") {
		t.Fatalf("the 4-power Hill Giant must gain trample and haste, keywords %v", hg.Keywords)
	}
	if has(cw, "Trample") || has(cw, "Haste") {
		t.Fatalf("the 1-power Craw Wurm must not gain trample or haste, keywords %v", cw.Keywords)
	}
}

// Cloud, Ex-SOLDIER's attack trigger gates its Treasures on ConditionDefined$
// Self + power 7 or greater: a 4/4 Cloud under three Glorious Anthems is a
// 7/7 and makes them, though its printed power is 4.
func TestConditionDefinedSelfReadsCurrentPT(t *testing.T) {
	t.Parallel()
	treasures := func(extra string) int {
		snaps := runPTFilterScenario(t, `{"name":"cloud","setup":{"p0":{"battlefield":["Cloud, Ex-SOLDIER"`+extra+`]}},
		 "steps":[{"op":"attack","seat":0,"defender":"p1","attackers":["p0:Cloud, Ex-SOLDIER"]},{"op":"resolve"}]}`)
		last := snaps[len(snaps)-1]
		n := 0
		for _, p := range last.Permanents {
			if p.Name == "Treasure Token" {
				n++
			}
		}
		return n
	}
	if got := treasures(`,"Glorious Anthem","Glorious Anthem","Glorious Anthem"`); got != 2 {
		t.Fatalf("a 7/7 Cloud must make two Treasures, got %d", got)
	}
	if got := treasures(``); got != 0 {
		t.Fatalf("a 4/4 Cloud must make no Treasure, got %d", got)
	}
}

// Scaled Destruction's "small" mode (totalPT <= 4) reads current P/T: Bears
// under a Glorious Anthem (3/3, total 6) live, a bare Bears (total 4) dies.
func TestDestroyAllTotalPTReadsCurrentPT(t *testing.T) {
	t.Parallel()
	snaps := runPTFilterScenario(t, `{"name":"scaled","setup":{
	  "p0":{"battlefield":["Grizzly Bears","Glorious Anthem"],"hand":["Scaled Destruction"]},
	  "p1":{"battlefield":["Mons's Goblin Raiders"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Scaled Destruction","mana":"CCCCWW","answers":[{"kind":"modes","pick":["Destroy all small creatures."]}]},{"op":"resolve"}]}`)
	wantPT(t, snaps[0], "p0:Grizzly Bears", "3/3")
	last := snaps[len(snaps)-1]
	wantAlive(t, last, "p0:Grizzly Bears", true)
	wantAlive(t, last, "p1:Mons's Goblin Raiders", false)
}

// The BattlefieldPTs table is sparse and on demand: nil on a plain board, one
// entry carrying the derived values for a pumped creature, and not requested
// by a body whose parameters read no P/T (that last half is
// TestLayerBattlefieldPTWantBit in engine/effects).
func TestBattlefieldPTTableIsSparse(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(`{"name":"sparse","setup":{"p0":{"battlefield":["Grizzly Bears","Hill Giant"],"hand":["Giant Growth"]}},"steps":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _, run := runOracleScenario(reg, sc)
	e := run.e
	if got := e.LayerTables(effects.LayerBattlefieldPT).BattlefieldPTs; got != nil {
		t.Fatalf("plain board: BattlefieldPTs = %+v, want nil", got)
	}
	// Pump the Bears with an Anthem-free until-end-of-turn effect: Giant Growth.
	sc2, err := decodeOracleScenario([]byte(`{"name":"sparse2","setup":{"p0":{"battlefield":["Grizzly Bears","Hill Giant"],"hand":["Giant Growth"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Giant Growth","mana":"G","targets":["p0:Grizzly Bears"]},{"op":"resolve"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _, run2 := runOracleScenario(reg, sc2)
	e2 := run2.e
	tab := e2.LayerTables(effects.LayerBattlefieldPT).BattlefieldPTs
	if len(tab) != 1 || tab[0].Power != 5 || tab[0].Toughness != 5 || tab[0].BasePower != 2 || tab[0].BaseToughness != 2 {
		t.Fatalf("pumped board: BattlefieldPTs = %+v, want exactly the Bears at 5/5 over base 2/2", tab)
	}
	if got := e2.LayerTables(0).BattlefieldPTs; got != nil {
		t.Fatalf("table published without the want bit: %+v", got)
	}
}

// Unstable Glyphbridge's ChooseCard (Choices$ Creature.powerLE2) is the
// carrier that has no per-candidate P/T bind, so it must read the board
// table: p0's Bears under a Glorious Anthem are a 3/3, not a legal choice,
// and die; p1's bare 1/1 is chosen and survives.
func TestChooseCardPowerFilterReadsCurrentPT(t *testing.T) {
	t.Parallel()
	snaps := runPTFilterScenario(t, `{"name":"glyphbridge","setup":{
	  "p0":{"battlefield":["Grizzly Bears","Glorious Anthem"],"hand":["Unstable Glyphbridge"]},
	  "p1":{"battlefield":["Mons's Goblin Raiders"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Unstable Glyphbridge","mana":"CCCWW"},{"op":"resolve"}]}`)
	wantPT(t, snaps[0], "p0:Grizzly Bears", "3/3")
	wantPT(t, snaps[0], "p1:Mons's Goblin Raiders", "1/1")
	last := snaps[len(snaps)-1]
	wantAlive(t, last, "p0:Grizzly Bears", false)
	wantAlive(t, last, "p1:Mons's Goblin Raiders", true)
}
