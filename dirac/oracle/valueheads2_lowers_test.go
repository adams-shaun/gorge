package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Three value heads in lockstep with gorge, each on its own fixture pool:
//
//   - PlayerCountPlayers$HighestLifeTotal (dirac/kgen value_filterheads.go:
//     ExOppExtreme OppExtLife over the OppExtAllPl group; gorge lifeExtreme
//     over AliveFrom): Arbiter of Knollridge's "each player's life total
//     becomes the highest", Scourge of the Skyclaves' 20 minus the highest
//     life total, Sorin, Grim Nemesis' -9 (fixtures/highlife).
//   - Targeted$CardNumColors (kgen value.go, kernel refNumColors' target
//     arm; gorge h.ObjectColors over the target): Breathe Your Last's life
//     for each colour of the creature it destroyed, Might of the Nephilim's
//     +2/+2 per colour (fixtures/refcolours).
//   - TriggeredSource$CardToughness (kgen lower_ninjutsu.go, kernel refPT
//     over RefTriggerSource): Ikra Shidiqi's life equal to the toughness of
//     the creature that dealt combat damage (fixtures/refcolours).

var highLifeWatch = map[string]bool{"Arbiter of Knollridge": true, "Scourge of the Skyclaves": true, "Sorin, Grim Nemesis": true}

var highLifeDeck = []cardset.Entry{
	{Name: "Arbiter of Knollridge", Count: 6}, {Name: "Scourge of the Skyclaves", Count: 6}, {Name: "Sorin, Grim Nemesis", Count: 3},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 8},
}

var highLifeDecks = []fixtureDecks{{highLifeDeck, highLifeDeck}}

var refColoursWatch = map[string]bool{"Breathe Your Last": true, "Might of the Nephilim": true, "Ikra Shidiqi, the Usurper": true}

var refColoursDeck = []cardset.Entry{
	{Name: "Breathe Your Last", Count: 6}, {Name: "Might of the Nephilim", Count: 6}, {Name: "Ikra Shidiqi, the Usurper", Count: 4},
	{Name: "Putrid Leech", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
}

var refColoursDecks = []fixtureDecks{{refColoursDeck, refColoursDeck}}

// mutateExprs rewrites every table expression edit accepts, on a copy, and
// fails when none matched.
func mutateExprs(t *testing.T, what string, edit func(*kop.Expr) bool) func(*kop.Table) {
	return func(tbl *kop.Table) {
		tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
		n := 0
		for i := range tbl.Exprs {
			if edit(&tbl.Exprs[i]) {
				n++
			}
		}
		if n == 0 {
			t.Fatalf("precondition: no table expression is %s", what)
		}
	}
}

func TestHighestLifeTotalLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "highlife", highLifeDecks, highLifeWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if n := run.casts["Scourge of the Skyclaves"]; n < 40 {
		t.Errorf("Scourge of the Skyclaves cast %d times", n)
	}
	if n := run.casts["Arbiter of Knollridge"]; n < 5 {
		t.Errorf("Arbiter of Knollridge cast %d times", n)
	}
	// the controller dropped from the group: the highest opponent life only
	mut := runFixtureLockstep(t, "highlife", highLifeDecks, highLifeWatch, 120, false,
		mutateExprs(t, "an every-player life extreme", func(x *kop.Expr) bool {
			if x.Kind != kop.ExOppExtreme || int(x.N)&0xff != kop.OppExtLife || int(x.N)&kop.OppExtAllPl == 0 {
				return false
			}
			x.N &^= kop.OppExtAllPl
			return true
		}))
	mut.requireDivergence(t, "the controller dropped from HighestLifeTotal")
}

func TestRefColoursLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "refcolours", refColoursDecks, refColoursWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for _, n := range []string{"Breathe Your Last", "Might of the Nephilim"} {
		if run.casts[n] < 40 {
			t.Errorf("%s cast %d times", n, run.casts[n])
		}
	}
	if n := run.casts["Ikra Shidiqi, the Usurper"]; n < 10 {
		t.Errorf("Ikra Shidiqi cast %d times", n)
	}
	// the target's colour count read as the source's (0: RefSelf has no arm)
	mut := runFixtureLockstep(t, "refcolours", refColoursDecks, refColoursWatch, 120, false,
		mutateExprs(t, "a target's colour count", func(x *kop.Expr) bool {
			if x.Kind != kop.ExNumColorsOf {
				return false
			}
			x.Ref = kop.RefSelf
			return true
		}))
	mut.requireDivergence(t, "Targeted$CardNumColors read off no target")
	// the damage source's toughness read as its power
	mut = runFixtureLockstep(t, "refcolours", refColoursDecks, refColoursWatch, 120, false,
		mutateExprs(t, "the damage source's toughness", func(x *kop.Expr) bool {
			if x.Kind != kop.ExToughnessOf || x.Ref != kop.RefTriggerSource {
				return false
			}
			x.Kind = kop.ExPowerOf
			return true
		}))
	mut.requireDivergence(t, "TriggeredSource$CardToughness read as power")
}

// ConditionDefined$ Self | ConditionPresent$ Card.!kicked (kgen
// lower_condnotpresent.go selfNotKicked: ConditionPresent$ Card.kicked with
// ConditionCompare$ EQ0): Desolation Giant's and Desolation Angel's unkicked
// "destroy all ... you control", Sphinx of Lost Truths' unkicked discard
// (fixtures/notkicked).

var notKickedWatch = map[string]bool{"Desolation Giant": true, "Desolation Angel": true, "Sphinx of Lost Truths": true}

var notKickedRWB = []cardset.Entry{
	{Name: "Desolation Giant", Count: 7}, {Name: "Desolation Angel", Count: 6}, {Name: "Savannah Lions", Count: 6},
	{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
}

var notKickedUW = []cardset.Entry{
	{Name: "Sphinx of Lost Truths", Count: 9}, {Name: "Elite Vanguard", Count: 8}, {Name: "Island", Count: 13}, {Name: "Plains", Count: 10},
}

var notKickedDecks = []fixtureDecks{{notKickedRWB, notKickedUW}, {notKickedUW, notKickedRWB}}

func TestNotKickedLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "notkicked", notKickedDecks, notKickedWatch, 160, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for n := range notKickedWatch {
		if run.casts[n] < 15 {
			t.Errorf("%s cast %d times", n, run.casts[n])
		}
	}
	// the EQ0 gate negated: the unkicked branch runs only when kicked
	mut := runFixtureLockstep(t, "notkicked", notKickedDecks, notKickedWatch, 160, false, func(tbl *kop.Table) {
		tbl.Conds = append([]kop.Cond(nil), tbl.Conds...)
		n := 0
		for i := range tbl.Conds {
			if c := &tbl.Conds[i]; c.Op == kop.CmpEQ {
				c.Neg = !c.Neg
				n++
			}
		}
		if n == 0 {
			t.Fatal("precondition: no table condition compares EQ")
		}
	})
	mut.requireDivergence(t, "the !kicked gate negated")
}
