package botpolicy

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// fact is one battlefield creature for the board helpers below: an object
// id and the facts the adapter halves would report for it.
type fact struct {
	id state.ObjID
	c  Creature
}

// boardOf builds a Board with the given creatures (Life empty unless a test
// sets it) -- the battlefield census the adapters would produce.
func boardOf(fs ...fact) Board {
	b := Board{Creatures: map[state.ObjID]Creature{}, Life: map[state.PlayerID]int32{}}
	for _, f := range fs {
		b.Creatures[f.id] = f.c
	}
	return b
}

// atk is a fact helper: a creature controlled by seat 0 with the given
// power/toughness and keywords.
func atk(id int, p int32, t int32, kw ...string) fact {
	return fact{state.ObjID(100 + id), Creature{Power: p, Toughness: t, Keywords: kw, Controller: 0}}
}

// def is the same helper for seat 1.
func def(id int, p int32, t int32, kw ...string) fact {
	return fact{state.ObjID(200 + id), Creature{Power: p, Toughness: t, Keywords: kw, Controller: 1}}
}

// attackDecision builds a KAttackers decision for seat 0 (the attacker)
// against seat 1 with one option per object id, and returns the option
// indices Decide chose.
func attackDecision(b Board, ids ...int) []int {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KAttackers, Min: 0, Max: len(ids),
		Options: []decision.Option{}}
	for _, id := range ids {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "attacker",
			Obj: state.ObjID(100 + id), Player: 1})
	}
	return Decide(b, &d, rng(1)).Choices
}

// blockDecision builds a KBlockers decision for seat 0 (the defender) with
// one option per (blocker, attacker) pair — ids 100+blocker, 200+attacker,
// matching the atk/def helpers — and returns the option indices Decide
// chose. An option's position in the returned slice is its position in
// pairs.
func blockDecision(b Board, pairs ...[2]int) []int {
	return blockDecisionFull(b, pairs...).choices
}

// blockDecisionFull is blockDecision plus the decision it answered, so a
// test can name which (blocker, attacker) a chosen option was.
type attackAnswer struct {
	d       *decision.Decision
	choices []int
}

func blockDecisionFull(b Board, pairs ...[2]int) attackAnswer {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KBlockers, Min: 0, Max: len(pairs),
		Options: []decision.Option{}}
	for _, p := range pairs {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "block",
			Obj: state.ObjID(100 + p[0]), Attacker: state.ObjID(200 + p[1]), Player: 0})
	}
	return attackAnswer{&d, Decide(b, &d, rng(1)).Choices}
}

// TestAttackNothingToGain is AR1: a creature with no power deals no damage
// and trades nothing, so it stays home even with an empty defender board.
func TestAttackNothingToGain(t *testing.T) {
	b := boardOf(atk(1, 0, 4))
	if got := attackDecision(b, 1); len(got) != 0 {
		t.Errorf("0-power attacker chosen: %v, want none", got)
	}
}

// TestAttackUnblockable is AR2: a flying attacker against a defender with
// no Flying/Reach creature attacks — nothing can block it. A Reach 1/1
// makes it blockable, which hands the decision to the deadliest-block
// analysis (AR3) and then the leave-a-blocker rule (AR4): the 1/1 cannot
// kill the 3/3, but a Reach 4/4 can and does, and a 3/3 that trades for a
// reach 1/1 is held back to let that 1/1 die on the counter instead.
func TestAttackUnblockable(t *testing.T) {
	b := boardOf(atk(1, 3, 3, "Flying"))
	if got := attackDecision(b, 1); len(got) != 1 || got[0] != 0 {
		t.Errorf("flier vs no blockers = %v, want chosen", got)
	}
	b = boardOf(atk(1, 3, 3, "Flying"), def(1, 4, 4))
	if got := attackDecision(b, 1); len(got) != 1 {
		t.Errorf("flier vs ground-only defender = %v, want chosen", got)
	}
	b = boardOf(atk(1, 3, 3, "Flying"), def(1, 4, 4, "Reach"))
	if got := attackDecision(b, 1); len(got) != 0 {
		t.Errorf("flier vs reach 4/4 = %v, want none (it dies for free)", got)
	}
	// A lone 3/3-flier against a reach 1/1: swinging into a blocker that
	// trades for it is worse than holding it back, where the 1/1 has to
	// come to it instead (AR4 holds back the only attacker).
	b = boardOf(atk(1, 3, 3, "Flying"), def(1, 1, 1, "Reach"))
	if got := attackDecision(b, 1); len(got) != 0 {
		t.Errorf("flier vs reach 1/1 = %v, want it held back (AR4: it trades down)", got)
	}
}

// TestAttackCheapKillBlockIsDeadly is AR3: an attacker the defender can
// kill while losing less than the attacker is worth stays home. A 2/2
// swinging into a lone 3/3 loses to a free block; a 4/4 swinging into a
// 2/2 plus a 3/3 dies to the team for just the 2/2. Each case is also run
// beside a spare attacker that my side could swing, so it is AR3 alone
// that keeps the poisoned attacker home — not AR4 (which would hold the
// whole lone board back anyway).
func TestAttackCheapKillBlockIsDeadly(t *testing.T) {
	b := boardOf(atk(1, 2, 2), def(1, 3, 3))
	if got := attackDecision(b, 1); len(got) != 0 {
		t.Errorf("2/2 vs 3/3 = %v, want none (3/3 kills it for free)", got)
	}
	b = boardOf(atk(1, 4, 4), def(1, 2, 2), def(2, 3, 3))
	if got := attackDecision(b, 1); len(got) != 0 {
		t.Errorf("4/4 vs 2/2+3/3 = %v, want none (team kills it for a 2/2)", got)
	}
	// Spare-creature variants: the 5/5 attacks, the poisoned 2/2 stays
	// home, and AR4 has nothing to hold back (the 5/5 covers the board).
	b = boardOf(atk(1, 2, 2), atk(2, 5, 5), def(1, 3, 3))
	if got := attackDecision(b, 1, 2); len(got) != 1 || got[0] != 1 {
		t.Errorf("2/2+5/5 vs 3/3 = %v, want only the 5/5 to attack", got)
	}
	b = boardOf(atk(1, 4, 4), atk(2, 6, 6), def(1, 2, 2), def(2, 3, 3))
	if got := attackDecision(b, 1, 2); len(got) != 1 || got[0] != 1 {
		t.Errorf("4/4+6/6 vs 2/2+3/3 = %v, want only the 6/6 to attack", got)
	}
}

// TestAttackFirstStrikeSweeps pins the First-Strike branch of the damage
// simulation the combat math is built on (blockCombat): a First-Strike
// attacker kills the first blocker in its own step and the rest hit it
// one-for-one on the way through, so a 3/3-FS against two 2/2s survives
// the team block the ground version would die to — the kind of fact the
// bot would be guessing at if the keywords were not visible, and a rule
// the deadliest-block analysis must honour.
func TestAttackFirstStrikeSweeps(t *testing.T) {
	b := boardOf(atk(1, 3, 3, "First Strike"), atk(2, 1, 1), def(1, 2, 2), def(2, 2, 2))
	if got := attackDecision(b, 1, 2); len(got) != 1 || got[0] != 0 {
		t.Errorf("3/3-FS+1/1 vs two 2/2s = %v, want only the 3/3-FS to attack", got)
	}
	// The same attacker without First Strike is team-killed for one 2/2 and
	// stays home — the control that proves the keyword, not the stats, is
	// what earned the swing.
	b = boardOf(atk(1, 3, 3), atk(2, 1, 1), def(1, 2, 2), def(2, 2, 2))
	if got := attackDecision(b, 1, 2); len(got) != 0 {
		t.Errorf("3/3+1/1 vs two 2/2s = %v, want nothing to attack (team-killed)", got)
	}
}

// TestAttackTradesUpIsFine is AR3's other side: attacking is right when
// every way the defender can kill the attacker costs the defender at least
// the attacker's worth, or cannot kill it at all. A 4/4 into two 2/2s
// kills both; a 3/3 into a lone 2/2 the 2/2 cannot even kill; a 3/3 into
// a lone 3/3 trades at worst evenly. Each sits beside a 1/1 that the
// defender can kill for free and which stays home, so the board is never
// left completely unguarded and AR4 has nothing to hold back.
func TestAttackTradesUpIsFine(t *testing.T) {
	b := boardOf(atk(1, 4, 4), atk(2, 1, 1), def(1, 2, 2), def(2, 2, 2))
	if got := attackDecision(b, 1, 2); len(got) != 1 || got[0] != 0 {
		t.Errorf("4/4+1/1 vs two 2/2s = %v, want only the 4/4 to attack (it kills both)", got)
	}
	b = boardOf(atk(1, 3, 3), atk(2, 1, 1), def(1, 2, 2))
	if got := attackDecision(b, 1, 2); len(got) != 1 || got[0] != 0 {
		t.Errorf("3/3+1/1 vs lone 2/2 = %v, want only the 3/3 to attack (the 2/2 cannot kill it)", got)
	}
	b = boardOf(atk(1, 3, 3), atk(2, 1, 1), def(1, 3, 3))
	if got := attackDecision(b, 1, 2); len(got) != 1 || got[0] != 0 {
		t.Errorf("3/3+1/1 vs lone 3/3 = %v, want only the 3/3 to attack (even trade)", got)
	}
}

// TestAttackLeaveABlocker is AR4: an attack that would leave the board
// with nobody to block with, while the defender has a creature of its own,
// holds back the best blockable attacker. An unblockable attacker is never
// the one held back — it is the way a defensive board still wins.
func TestAttackLeaveABlocker(t *testing.T) {
	// Two 2/2s, defender has a 2/2: attacking with both leaves nothing to
	// block with, so one stays home. Ties break on object id, so the
	// higher id attacks.
	b := boardOf(atk(1, 2, 2), atk(2, 2, 2), def(1, 2, 2))
	got := attackDecision(b, 1, 2)
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("both 2/2s vs defender 2/2 = %v, want exactly the id-2 option (one held back)", got)
	}
	// A lone unblockable attacker is not held back even though attacking
	// leaves no blocker behind: nothing on the ground can block it.
	b = boardOf(atk(1, 3, 3, "Flying"), def(1, 2, 2))
	if got := attackDecision(b, 1); len(got) != 1 {
		t.Errorf("lone flier vs ground defender = %v, want it to attack (unblockable)", got)
	}
	// No defender creatures: nothing to hold the board undefended against.
	b = boardOf(atk(1, 2, 2), atk(2, 2, 2))
	if got := attackDecision(b, 1, 2); len(got) != 2 {
		t.Errorf("both 2/2s vs empty board = %v, want both to attack", got)
	}
	// A vigilant attacker stays untapped, so it still counts as a blocker
	// and nothing needs to be held back.
	b = boardOf(atk(1, 2, 2), atk(2, 2, 2, "Vigilance"), def(1, 2, 2))
	if got := attackDecision(b, 1, 2); len(got) != 2 {
		t.Errorf("2/2 + vigilant 2/2 vs defender 2/2 = %v, want both to attack", got)
	}
}

// TestBlockFavorableAndEvenTrades is BR1: a blocker blocks when it kills
// the attacker and survives the return swing, or trades even-or-up. A 4/4
// blocks a 3/3 and survives; a 3/3 takes an even trade with another 3/3;
// a 1/1 is never thrown at a 6/6 it cannot touch when no life is at stake.
func TestBlockFavorableAndEvenTrades(t *testing.T) {
	b := boardOf(atk(1, 4, 4), def(1, 3, 3))
	b.Life[0] = 20
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 1 {
		t.Errorf("4/4 vs 3/3 = %v, want the block (it survives, attacker dies)", got)
	}
	b = boardOf(atk(1, 3, 3), def(1, 3, 3))
	b.Life[0] = 20
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 1 {
		t.Errorf("3/3 vs 3/3 = %v, want the block (even trade)", got)
	}
	b = boardOf(atk(1, 1, 1), def(1, 6, 6))
	b.Life[0] = 20
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 0 {
		t.Errorf("1/1 vs 6/6 = %v, want no block (it dies for nothing at 20 life)", got)
	}
	// Trading down is refused too: a 5/5 already carrying 4 damage dies to a
	// 4/4 while killing it, a trade the blocker loses, so it stays home when
	// no life is at stake.
	b = boardOf(fact{state.ObjID(101), Creature{Power: 5, Toughness: 5, Damage: 4, Controller: 0}},
		def(1, 4, 4))
	b.Life[0] = 20
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 0 {
		t.Errorf("5/5-damaged vs 4/4 = %v, want no block (both die and it trades down)", got)
	}
}

// TestBlockSpendsTheCheapestKiller is BR1's spending rule: among the
// blockers that kill the attacker acceptably, the cheapest is spent. A
// 1/1 Deathtouch trading for a 4/4 beats giving up a whole 4/4 in the
// same block.
func TestBlockSpendsTheCheapestKiller(t *testing.T) {
	b := boardOf(atk(1, 4, 4), atk(2, 1, 1, "Deathtouch"), def(1, 4, 4))
	b.Life[0] = 20
	got := blockDecision(b, [2]int{1, 1}, [2]int{2, 1}) // both options block the 4/4
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("4/4 vs {4/4, 1/1-deathtouch} = %v, want only the 1/1 option spent", got)
	}
}

// TestBlockChumpOnlyForLethal is BR2: a blocker that dies without killing
// blocks only when the unblocked damage would otherwise take the defender
// to zero or below. The trample case saves only the blocker's own
// toughness, which is exactly what the life bookkeeping subtracts.
func TestBlockChumpOnlyForLethal(t *testing.T) {
	// 4 damage at 5 life: not lethal, the 1/1 stays home.
	b := boardOf(atk(1, 1, 1), def(1, 4, 4))
	b.Life[0] = 5
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 0 {
		t.Errorf("1/1 vs 4/4 at 5 life = %v, want no chump (not lethal)", got)
	}
	// The same block at 4 life: lethal, chump.
	b.Life[0] = 4
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 1 {
		t.Errorf("1/1 vs 4/4 at 4 life = %v, want the chump (lethal)", got)
	}
	// Trample: at 3 life the unblocked 4 is lethal, and a 1/1 chump saves
	// the only life it can (1); still the right block.
	b = boardOf(atk(1, 1, 1), def(1, 4, 4, "Trample"))
	b.Life[0] = 3
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 1 {
		t.Errorf("1/1 vs trampling 4/4 at 3 life = %v, want the chump", got)
	}
	// Non-lethal incoming total with two attackers: no chump on either.
	b = boardOf(atk(1, 1, 1), def(1, 2, 2), def(2, 2, 2))
	b.Life[0] = 5
	if got := blockDecision(b, [2]int{1, 1}, [2]int{1, 2}); len(got) != 0 {
		t.Errorf("1/1 vs two 2/2s at 5 life = %v, want no chump", got)
	}
}

// TestBlockNeverThrowsAway is BR1+BR2's floor: a blocker that cannot kill
// its attacker and whose sacrifice saves no life is never spent, and the
// same blocker is never given to two attackers. A 2/2 against a {5/5,
// 1/1} pair blocks only the 1/1 (which it kills and survives) — the 5/5
// gets through untouched.
func TestBlockNeverThrowsAway(t *testing.T) {
	b := boardOf(atk(1, 2, 2), def(1, 5, 5), def(2, 1, 1))
	b.Life[0] = 20
	got := blockDecision(b, [2]int{1, 1}, [2]int{1, 2}) // options: (2/2,5/5), (2/2,1/1)
	if len(got) != 1 {
		t.Fatalf("2/2 vs {5/5, 1/1} = %v, want exactly one block", got)
	}
	if got[0] != 1 {
		t.Errorf("2/2 blocked option %d, want option 1 (the 1/1 attacker, which it kills and survives)", got[0])
	}
}

// TestBlockCombatIsDeterministic pins the two determinism properties the
// combat branches must keep: the same decision with the same board answers
// identically every time, and with one 4/4 blocker against a 4/4 and a
// 3/3 attacker the single block always goes to the biggest threat (the
// 4/4), whichever order the options arrive in — only the option index
// that names it shifts.
func TestBlockCombatIsDeterministic(t *testing.T) {
	b := boardOf(atk(1, 4, 4), atk(2, 3, 3), def(1, 4, 4))
	b.Life[0] = 20
	// Forward: option 0 is (blocker, 4/4 attacker).
	fa := blockDecisionFull(b, [2]int{1, 1}, [2]int{1, 2})
	if len(fa.choices) != 1 {
		t.Fatalf("want exactly one block with one blocker, got %v", fa.choices)
	}
	if fa.d.Options[fa.choices[0]].Attacker != state.ObjID(201) {
		t.Errorf("forward block = %+v, want the 4/4 attacker blocked", fa.d.Options[fa.choices[0]])
	}
	// Backward: option 1 is now (blocker, 4/4 attacker); same fact choice.
	ba := blockDecisionFull(b, [2]int{1, 2}, [2]int{1, 1})
	if len(ba.choices) != 1 {
		t.Fatalf("want exactly one block, got %v", ba.choices)
	}
	if ba.d.Options[ba.choices[0]].Attacker != state.ObjID(201) {
		t.Errorf("backward block = %+v, want the 4/4 attacker blocked", ba.d.Options[ba.choices[0]])
	}
	// Same input twice: identical output.
	same := blockDecisionFull(b, [2]int{1, 1}, [2]int{1, 2})
	for i := range fa.choices {
		if fa.choices[i] != same.choices[i] {
			t.Fatalf("same input gave different choices: %v vs %v", fa.choices, same.choices)
		}
	}
}

// TestCombatShapeTotality is the totality guard for the combat branches:
// option lists referencing objects with no board facts at all must answer
// without panicking and validate, the same contract
// TestTotalityUnderArbitraryMinMax holds for Decide as a whole — and a
// factless attacker stays home while a factless blocker never blocks.
func TestCombatShapeTotality(t *testing.T) {
	b := boardOf()
	att := decision.Decision{Seq: 1, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 3,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 0},
			{Index: 1, Kind: "attacker", Obj: 101},
			{Index: 2, Kind: "attacker", Obj: 102},
		}}
	in := Decide(b, &att, rng(1))
	if err := att.Validate(in); err != nil {
		t.Errorf("attackers intent %+v failed Validate: %v", in, err)
	}
	if len(in.Choices) != 0 {
		t.Errorf("factless attackers chosen: %v, want none (they read as 0 power)", in.Choices)
	}
	blk := decision.Decision{Seq: 2, Player: 0, Kind: decision.KBlockers, Min: 0, Max: 4,
		Options: []decision.Option{
			{Index: 0, Kind: "block", Obj: 101, Attacker: 201},
			{Index: 1, Kind: "block", Obj: 101, Attacker: 202},
			{Index: 2, Kind: "block", Obj: 102, Attacker: 201},
			{Index: 3, Kind: "block", Obj: 103, Attacker: 203},
		}}
	in = Decide(b, &blk, rng(1))
	if err := blk.Validate(in); err != nil {
		t.Errorf("blockers intent %+v failed Validate: %v", in, err)
	}
	if len(in.Choices) != 0 {
		t.Errorf("factless blockers chosen: %v, want none", in.Choices)
	}
}

// TestBoardFromGameCensusSkipsNonCreatures pins the game-shaped adapter's
// filter: the census must contain exactly the battlefield creatures and
// nothing else — a land sitting on the battlefield must not reach the
// combat heuristic as a blocker the seat could not actually use.
func TestBoardFromGameCensusSkipsNonCreatures(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	whelp := g.AddObject(cardFace(t, "R Whelp", "Creature Whelp", 2, 2), 0)
	land := g.AddObject(cardFace(t, "Mountain", "Basic Land Mountain", 0, 0), 0)
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{whelp.ID, land.ID})
	got := BoardFromGame(g, stubChars{}, 0)
	if len(got.Creatures) != 1 {
		t.Fatalf("census has %d creatures, want 1 (the land must be dropped)", len(got.Creatures))
	}
	cr := got.Creatures[whelp.ID]
	if cr.Power != 2 || cr.Toughness != 2 {
		t.Errorf("creature facts = %d/%d, want 2/2", cr.Power, cr.Toughness)
	}
	if _, ok := got.Creatures[land.ID]; ok {
		t.Errorf("land object leaked into the creature census")
	}
	// A brand-new game sits in the untap step: not a main phase, and both
	// players at 20 life.
	if got.IsMain {
		t.Errorf("IsMain = true on a fresh game (step untap), want false")
	}
	if got.Life[0] != 20 || got.Life[1] != 20 {
		t.Errorf("life = %v, want 20/20", got.Life)
	}
}

// stubChars is a Chars stand-in for the census test: everything reads as
// the object's printed 0/0 except object 1, which this test's source knows
// is the 2/2 Whelp — enough to prove the adapter derives the facts
// through the Chars it is handed rather than off the printed face.
type stubChars struct{}

func (stubChars) Power(id state.ObjID) int32 {
	if id == 1 {
		return 2
	}
	return 0
}
func (stubChars) Toughness(id state.ObjID) int32 {
	if id == 1 {
		return 2
	}
	return 0
}
func (stubChars) Keywords(id state.ObjID) []string { return nil }

func cardFace(t *testing.T, name, types string, p, tn int32) *cards.Card {
	t.Helper()
	src := fmt.Sprintf("Name:%s\nTypes:%s\nPT:%d/%d\nOracle:x\n", name, types, p, tn)
	c, diags := cards.ParseBytes("combat_test.go", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parsing %q: %v", src, diags)
	}
	c.Link()
	return c
}
