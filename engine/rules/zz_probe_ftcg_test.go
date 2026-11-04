package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// zz_probe_ftcg_test.go: scratch diagnostic, never committed.

const zzProbeBunny = "Name:Probe Bunny\nTypes:Creature\nPT:1/1\nOracle:x\n"

func probeEngine(t *testing.T, pool string, withTarget bool) *Engine {
	t.Helper()
	reg, err := cards.OpenCorpus("/home/sadams/projects/gorge/.worktrees/ls-blb-for-the-common-good/.cards")
	if err != nil {
		t.Fatal(err)
	}
	ftcg, ok := reg.Lookup("For the Common Good")
	if !ok {
		t.Fatal("no card")
	}
	e := New(seatZeroStart(Config{
		Seed:  42,
		Names: []string{"you", "me"},
		Decks: [][]*cards.Card{append(mountainDeck(t, 40), ftcg), mountainDeck(t, 40)},
	}))
	e.Advance()
	toMain1(t, e)
	moveSeeded(t, e, 0, zzProbeFTCGSrc(), state.ZHand)
	addMana(t, e, 0, pool)
	if withTarget {
		putToken(t, e, 0, zzProbeBunny, state.ZBattlefield)
		addMana(t, e, 0, "")
	}
	return e
}

func zzProbeFTCGSrc() string {
	return "Name:For the Common Good\nOracle:x\n"
}

func TestZZProbeFTCG(t *testing.T) {
	for _, tc := range []struct{ pool string; target bool }{
		{"G", true}, {"GG", true}, {"G", false}, {"GGG", true},
	} {
		e := probeEngine(t, tc.pool, tc.target)
		opts := e.legalActions(0)
		fmt.Printf("pool=%q target=%v: casts:\n", tc.pool, tc.target)
		found := false
		for _, o := range opts {
			if o.Kind == "cast" {
				fmt.Printf("  cast %+v\n", o)
				found = true
			}
		}
		if !found {
			fmt.Printf("  (no cast offered)\n")
		}
	}
	_ = state.PlayerID(0)
}

const zzProbeTreasure = "Name:Treasure Token\nManaCost:no cost\nTypes:Artifact Treasure\nA:AB$ Mana | Cost$ T Sac<1/CARDNAME/this token> | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any color.\nOracle:{T}, Sacrifice this token: Add one mana of any color.\n"

func zzOffered(t *testing.T, pool string, treasures int) {
	t.Helper()
	e := probeEngine(t, "", false)
	for i := 0; i < treasures; i++ {
		putToken(t, e, 0, zzProbeTreasure, state.ZBattlefield)
	}
	addMana(t, e, 0, pool)
	var kinds []string
	for _, o := range e.legalActions(0) {
		k := o.Label
		if o.Mode != "" {
			k = fmt.Sprintf("%s|%s", o.Label, o.Mode)
		}
		kinds = append(kinds, k)
	}
	fmt.Printf("ZZCASE pool=%q treasures=%d: %v\n", pool, treasures, kinds)
}

func TestZZProbeFTCG2(t *testing.T) {
	zzOffered(t, "G", 1)
	zzOffered(t, "", 1)
	zzOffered(t, "", 2)
	zzOffered(t, "G", 2)
	zzOffered(t, "", 0)
}

func TestZZProbeFTCG4(t *testing.T) {
	const plains = "Name:Plains\nManaCost:no cost\nTypes:Land Plains\nA:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Add {W}.\nOracle:{T}: Add {W}.\n"
	for _, tc := range []struct{ pool string; treasures int; plains bool }{
		{"", 2, false}, {"", 2, true}, {"", 1, true},
	} {
		e := probeEngine(t, "", false)
		for i := 0; i < tc.treasures; i++ {
			putToken(t, e, 0, zzProbeTreasure, state.ZBattlefield)
		}
		if tc.plains {
			putToken(t, e, 0, plains, state.ZBattlefield)
		}
		addMana(t, e, 0, tc.pool)
		var ftcg state.ObjID
		for _, id := range e.G.Zone(state.ZHand, 0) {
			if ob := e.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "For the Common Good" {
				ftcg = id
			}
		}
		got, witness := e.potentialPlayVerdict(0, decision.Option{Kind: "cast", Obj: ftcg})
		fmt.Printf("ZZ4 pool=%q treasures=%d plains=%v witness=%v reason=%q detail=%q\n", tc.pool, tc.treasures, tc.plains, witness, got.Reason, got.Detail)
		if got.Plan != nil {
			fmt.Printf("ZZ4   plan=%+v\n", *got.Plan)
			fmt.Printf("ZZ4   reach=%v\n", e.potentialWitnessReaches(0, decision.Option{Kind: "cast", Obj: ftcg}, *got.Plan))
		}
		offered := false
		for _, oo := range e.legalActions(0) {
			if oo.Kind == "cast" && oo.Obj == ftcg {
				offered = true
			}
		}
		fmt.Printf("ZZ4   offered=%v\n", offered)
	}
}

func TestZZProbeFTCG3(t *testing.T) {

	offered := probeEngine(t, "G", true)
	var opt decision.Option
	for _, o := range offered.legalActions(0) {
		if o.Kind == "cast" {
			opt = o
		}
	}
	fmt.Printf("ZZ3 option kind=%s obj=%d ability=%+v mode=%q alt=%d label=%q\n", opt.Kind, opt.Obj, opt.Ability, opt.Mode, opt.AltCostIndex, opt.Label)

	for _, tc := range []struct{ pool string; treasures int }{
		{"", 2}, {"G", 2}, {"", 1},
	} {
		e := probeEngine(t, "", false)
		for i := 0; i < tc.treasures; i++ {
			putToken(t, e, 0, zzProbeTreasure, state.ZBattlefield)
		}
		addMana(t, e, 0, tc.pool)
		o := opt
		for _, id := range e.G.Zone(state.ZHand, 0) {
			if ob := e.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "For the Common Good" {
				o.Obj = id
			}
		}
		fmt.Printf("ZZ3 ftcg id=%d\n", o.Obj)
		got, witness := e.potentialPlayVerdict(0, o)
		fmt.Printf("ZZ3 pool=%q treasures=%d witness=%v reason=%q detail=%q plan=%+v\n", tc.pool, tc.treasures, witness, got.Reason, got.Detail, got.Plan)
		if got.Plan != nil {
			reach := e.potentialWitnessReaches(0, o, *got.Plan)
			fmt.Printf("ZZ3   reach=%v\n", reach)
		}
		for _, pp := range e.potentialPlans(0, 0) {
			if pp.Action.Kind == "cast" {
				fmt.Printf("ZZ3   plans kind=%s obj=%d label=%q reason=%q detail=%q plan=%v\n", pp.Action.Kind, pp.Action.Obj, pp.Action.Label, pp.Reason, pp.Detail, pp.Plan != nil)
			}
		}
		for _, oo := range e.legalActions(0) {
			if oo.Kind == "cast" {
				fmt.Printf("ZZ3   menu cast obj=%d label=%q\n", oo.Obj, oo.Label)
			}
		}
	}
}
