package botpolicy

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func on(x CombatRules) AttackSimParams {
	p := DefaultAttackSimParams()
	p.Combat = x
	return p
}

func tapped(f fact) fact {
	f.c.Tapped = true
	return f
}

// simBlock answers a KBlockers decision for seat 0 whose options are the
// (blocker, attacker) pairs (ids 100+blk, 200+atk) by the attack-sim.
func simBlock(b Board, p AttackSimParams, pairs ...[2]int) []int {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KBlockers, Min: 0, Max: len(pairs)}
	for _, pr := range pairs {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "block",
			Obj: state.ObjID(100 + pr[0]), Attacker: state.ObjID(200 + pr[1]), Player: 0})
	}
	return AttackSimDecide(b, &d, rng(1), p).Choices
}

func simAttackP(b Board, p AttackSimParams, ids ...int) []int {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KAttackers, Min: 0, Max: len(ids)}
	for _, id := range ids {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "attacker",
			Obj: state.ObjID(100 + id), Player: 1})
	}
	return AttackSimDecide(b, &d, rng(1), p).Choices
}

func gangBlockBoard() Board {
	b := boardOf(atk(1, 2, 2), atk(2, 3, 3), tapped(def(1, 4, 4)))
	b.Life.Set(0, 20)
	b.Life.Set(1, 20)
	return b
}

func gangPredictBoard() Board {
	b := boardOf(atk(1, 4, 4), def(1, 2, 2), def(2, 3, 3))
	b.Life.Set(0, 20)
	b.Life.Set(1, 6)
	return b
}

func dsBlockBoard() Board {
	b := boardOf(atk(1, 2, 2), tapped(def(1, 2, 2, "Double Strike")))
	b.Life.Set(0, 20)
	b.Life.Set(1, 20)
	return b
}

func gangBlock(p AttackSimParams) []int {
	return simBlock(gangBlockBoard(), p, [2]int{1, 1}, [2]int{2, 1})
}
func gangPredict(p AttackSimParams) []int { return simAttackP(gangPredictBoard(), p, 1) }
func dsBlock(p AttackSimParams) []int {
	return simBlock(dsBlockBoard(), p, [2]int{1, 1})
}

func wantChoices(t *testing.T, what string, got, want []int) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestCombatRulesGangBlockSeed(t *testing.T) {
	for _, plies := range []int{0, 1, 2} {
		for _, greedy := range []bool{false, true} {
			for _, margin := range []int64{0, -50} {
				t.Run(fmt.Sprintf("plies%d/greedy%v/margin%d", plies, greedy, margin), func(t *testing.T) {
					p := DefaultAttackSimParams()
					p.BlockPlies, p.BlockGreedy, p.Margin = plies, greedy, margin
					wantChoices(t, "OFF", gangBlock(p), nil)
					p.Combat = CombatRules{GangBlocks: true}
					wantChoices(t, "ON", gangBlock(p), []int{0, 1})
				})
			}
		}
	}
}

func TestCombatRulesGangPredict(t *testing.T) {
	if got := attackDecision(gangPredictBoard(), 1); len(got) != 0 {
		t.Fatalf("default attacker = %v, want none (the premise)", got)
	}
	wantChoices(t, "OFF", gangPredict(DefaultAttackSimParams()), []int{0})
	wantChoices(t, "ON", gangPredict(on(CombatRules{GangPredict: true})), nil)
}

func TestCombatRulesDoubleStrikeBlock(t *testing.T) {
	wantChoices(t, "OFF", dsBlock(DefaultAttackSimParams()), []int{0})
	wantChoices(t, "ON", dsBlock(on(CombatRules{DoubleStrike: true})), nil)
}

func TestCombatRulesIndependent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules CombatRules
		want  [3][]int
	}{
		{"GangBlocks", CombatRules{GangBlocks: true}, [3][]int{{0, 1}, {0}, {0}}},
		{"GangPredict", CombatRules{GangPredict: true}, [3][]int{nil, nil, {0}}},
		{"DoubleStrike", CombatRules{DoubleStrike: true}, [3][]int{nil, {0}, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := on(tc.rules)
			wantChoices(t, "test 1", gangBlock(p), tc.want[0])
			wantChoices(t, "test 2", gangPredict(p), tc.want[1])
			wantChoices(t, "test 3", dsBlock(p), tc.want[2])
		})
	}
}

func TestCombatRulesDeterministic(t *testing.T) {
	p := on(CombatRules{true, true, true})
	b := boardOf(atk(1, 2, 2), atk(2, 3, 1), atk(3, 1, 4, "Flying"), atk(4, 4, 4, "Trample"),
		def(1, 3, 3), def(2, 2, 2, "Deathtouch"), def(3, 1, 1, "Reach"))
	b.Life.Set(0, 11)
	b.Life.Set(1, 9)
	first := simAttackP(b, p, 1, 2, 3, 4)
	for i := 0; i < 50; i++ {
		if s := simAttackP(b, p, 1, 2, 3, 4); !reflect.DeepEqual(s, first) {
			t.Fatalf("attack call %d = %v, first = %v", i, s, first)
		}
	}
	blk := decision.Decision{Seq: 1, Player: 1, Kind: decision.KBlockers, Max: 12}
	for _, bl := range []state.ObjID{201, 202, 203} {
		for _, a := range []state.ObjID{101, 102, 104} {
			blk.Options = append(blk.Options, decision.Option{Index: len(blk.Options), Kind: "block", Obj: bl, Attacker: a, Player: 1})
		}
	}
	for _, a := range []state.ObjID{101, 102, 104} {
		c := b.Creatures.Get(a)
		c.Tapped = true
		b.Creatures.Set(a, c)
	}
	bFirst := AttackSimDecide(b, &blk, rng(1), p).Choices
	for i := 0; i < 50; i++ {
		if s := AttackSimDecide(b, &blk, rng(1), p).Choices; !reflect.DeepEqual(s, bFirst) {
			t.Fatalf("block call %d = %v, first = %v", i, s, bFirst)
		}
	}
}

func TestCombatRulesZeroValueIsDefault(t *testing.T) {
	if DefaultAttackSimParams().Combat != (CombatRules{}) {
		t.Fatalf("DefaultAttackSimParams().Combat = %+v, want the zero value", DefaultAttackSimParams().Combat)
	}
}
