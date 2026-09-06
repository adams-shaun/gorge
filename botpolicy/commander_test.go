package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// commanderBoard is the Board the commander rules read: the casting census
// (Cards) and the commander bookkeeping (Commanders) both adapters fill,
// with IsMain true so the cast group is reachable.
func commanderBoard(facts map[state.ObjID]Card, cmds map[state.ObjID]Commander) Board {
	return Board{IsMain: true, Cards: facts, Commanders: cmds}
}

// zoneCmd is a Commander fact standing for "in the command zone, cast N
// times already, having dealt the given per-player damage".
func zoneCmd(casts int32, dmg map[state.PlayerID]int32) Commander {
	return Commander{Casts: casts, InCommandZone: true, Damage: dmg}
}

// ---------------------------------------------------------------------------
// CR1: cast the commander from the command zone, taxed.

// TestCastCommanderFromZoneFirstCast is CR1's positive base: a creature
// commander sitting in the command zone, never cast before, is cast — the
// permanently-available threat the format's premise promises, score 30+4P
// with no tax to price.
func TestCastCommanderFromZoneFirstCast(t *testing.T) {
	b := commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 4, CMC: 4},
	}, map[state.ObjID]Commander{1: zoneCmd(0, nil)})
	got, d := castDecision(b, []decision.Option{
		castCreature(0, 1),
		{Index: 1, Kind: "pass"},
	})
	if got != 0 {
		t.Fatalf("cast = option %d (obj %d), want the first-cast commander (obj 1)", got, d.Options[got].Obj)
	}
}

// TestCastCommanderSecondCastStillCasts is CR1's recast side: a 4/4 cast
// once before (the second command-zone cast pays {2}) still ranks above
// every spell — 46 - 20 = 26 against a one-shot's mana value — because the
// recast is the bot's way back into a game its commander can win.
func TestCastCommanderSecondCastStillCasts(t *testing.T) {
	b := commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 4, CMC: 4},
		2: {CMC: 3},
	}, map[state.ObjID]Commander{1: zoneCmd(1, nil)})
	got, d := castDecision(b, []decision.Option{
		castSpell(0, 2),    // a three-mana spell listed first
		castCreature(1, 1), // the recast commander listed second
	})
	if got != 1 {
		t.Fatalf("cast = option %d (obj %d), want the taxed recast commander (obj 1) over the spell", got, d.Options[got].Obj)
	}
}

// TestCastCommanderRefusedWhenTaxLoses is CR1's "never recast forever"
// side: the same 4/4 on its FOURTH command-zone cast pays {6} extra, which
// prices the exchange below zero (46 - 60 = -14) — the option is not cast
// at all, and the decision falls through to pass. Deleting the tax
// subtraction, or the below-zero refusal, picks the commander and fails
// this test by name.
func TestCastCommanderRefusedWhenTaxLoses(t *testing.T) {
	b := commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 4, CMC: 4},
	}, map[state.ObjID]Commander{1: zoneCmd(3, nil)})
	got, d := castDecision(b, []decision.Option{
		castCreature(0, 1),
		{Index: 1, Kind: "pass"},
	})
	if got != 1 || d.Options[got].Kind != "pass" {
		t.Fatalf("priority = option %d (kind %q), want pass — the fourth command-zone cast is a losing exchange", got, d.Options[got].Kind)
	}
	// A 2/2 refuses its third command-zone cast (38 - 40 < 0), where a 4/4
	// still takes its third (46 - 40 = 6): the tax cap prices power.
	b = commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 2, CMC: 2},
	}, map[state.ObjID]Commander{1: zoneCmd(2, nil)})
	got, d = castDecision(b, []decision.Option{
		castCreature(0, 1),
		{Index: 1, Kind: "pass"},
	})
	if got != 1 || d.Options[got].Kind != "pass" {
		t.Fatalf("priority = option %d (kind %q), want pass — a 2/2 on its third command-zone cast", got, d.Options[got].Kind)
	}
}

// TestCastCommanderTaxedBelowBigSpell pins the tax's magnitude as a
// RANKING, not only as a refuse/no-refuse gate: a 4/4 on its THIRD
// command-zone cast (46 - 40 = 6) ranks below a CMC-8 spell, where an
// untaxed 4/4 (46) would crush it. Deleting the subtraction alone (keeping
// the refusal) fails this test by name.
func TestCastCommanderTaxedBelowBigSpell(t *testing.T) {
	b := commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 4, CMC: 4},
		2: {CMC: 8},
	}, map[state.ObjID]Commander{1: zoneCmd(2, nil)})
	got, d := castDecision(b, []decision.Option{
		castCreature(0, 1),
		castSpell(1, 2),
	})
	if got != 1 {
		t.Fatalf("cast = option %d (obj %d), want the CMC-8 spell over the third-taxed commander", got, d.Options[got].Obj)
	}
}

// TestCastCommanderInHandIsUntaxed is CR1's gate: the tax applies only to
// a cast FROM the command zone (InCommandZone), never to a hand cast of
// the same commander — a bounced commander is just a card. The same object
// with the same cast history but outside the zone casts at full value.
func TestCastCommanderInHandIsUntaxed(t *testing.T) {
	b := commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 4, CMC: 4},
	}, map[state.ObjID]Commander{1: {Casts: 3, InCommandZone: false}})
	got, d := castDecision(b, []decision.Option{
		castCreature(0, 1),
		{Index: 1, Kind: "pass"},
	})
	if got != 0 {
		t.Fatalf("cast = option %d (obj %d), want the commander cast from hand (untaxed) over pass", got, d.Options[got].Obj)
	}
}

// TestCommanderRefusalFallsToLand is G0's interplay: a commander the tax
// has priced out of casting does not block the land drop — the decision
// still plays its land (free, unconditional, only raises the ceiling).
func TestCommanderRefusalFallsToLand(t *testing.T) {
	b := commanderBoard(map[state.ObjID]Card{
		1: {Creature: true, Power: 2, CMC: 2},
		2: {Basic: true},
	}, map[state.ObjID]Commander{1: zoneCmd(2, nil)})
	got, d := castDecision(b, []decision.Option{
		castCreature(0, 1),
		playLand(1, 2),
	})
	if got != 1 || d.Options[got].Kind != "play_land" {
		t.Fatalf("priority = option %d (kind %q), want the land drop when the commander is refused", got, d.Options[got].Kind)
	}
}

// ---------------------------------------------------------------------------
// AR5: the commander clock closes the fight — attack anyway.

// TestAttackCommanderClockCloseout is AR5: a commander whose unblocked
// swing would take the defender to 21+ from it attacks even into a block
// that kills it for less than it is worth (here a 1/1 Deathtouch that
// alone would send the 4/4 home under AR3). The swing ends the game on the
// second track if it gets through.
func TestAttackCommanderClockCloseout(t *testing.T) {
	b := boardOf(atk(1, 4, 4), def(1, 1, 1, "Deathtouch"))
	b.Commanders = map[state.ObjID]Commander{101: zoneCmd(1, map[state.PlayerID]int32{1: 18})}
	if got := attackDecision(b, 1); len(got) != 1 || got[0] != 0 {
		t.Errorf("4/4 commander at 18 vs 1/1 swiftdeath = %v, want it to attack (18+4 closes the clock)", got)
	}
	// The same commander at 16 stays home: 16 + 4 = 20 is not yet a
	// win, and dying to the free block only delays the closing while
	// paying the tax to recast.
	b.Commanders = map[state.ObjID]Commander{101: zoneCmd(1, map[state.PlayerID]int32{1: 16})}
	if got := attackDecision(b, 1); len(got) != 0 {
		t.Errorf("4/4 commander at 16 vs 1/1 swiftdeath = %v, want it held back (16+4 does not close)", got)
	}
}

// TestAttackCommanderClockCloserNotHeldBack is AR4's AR5 exemption: with a
// second, blockable attacker beside the closing commander, the leave-a-
// blocker rule holds back the OTHER one — the closer is the game-winning
// swing, never the spare.
func TestAttackCommanderClockCloserNotHeldBack(t *testing.T) {
	b := boardOf(atk(1, 4, 4), atk(2, 2, 2), def(1, 2, 2))
	b.Commanders = map[state.ObjID]Commander{101: zoneCmd(0, map[state.PlayerID]int32{1: 18})}
	got := attackDecision(b, 1, 2)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("closer 4/4 + spare 2/2 vs defender 2/2 = %v, want only the closer (option 0) to attack", got)
	}
}

// ---------------------------------------------------------------------------
// BoardFromGame: the game-shaped half's commander facts.

// commanderGameState builds a two-seat state.Game with one creature per
// seat, each seat's creature named its commander (roster, cast counts and
// the dense-indexed damage clocks all set directly, the way a live
// Commander game would hold them): seat 0's commander A has been cast
// twice and sits in the command zone, having dealt seat 0 7 damage; seat
// 1's commander B has never been cast and is on the battlefield, having
// dealt seat 1 3 damage. Returns the game and A's and B's object ids.
func commanderGameState(t *testing.T) (*state.Game, state.ObjID, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"a", "b"})
	A := g.AddObject(cardFace(t, "A", "Legendary Creature Bear", 4, 4), 0)
	B := g.AddObject(cardFace(t, "B", "Legendary Creature Bear", 3, 3), 1)
	g.Players[0].Commanders = []state.ObjID{A.ID}
	g.Players[0].CmdCasts = []int32{2}
	g.Players[0].CmdDamage = []int32{7, 0} // took 7 from A (dense 0)
	g.Players[1].Commanders = []state.ObjID{B.ID}
	g.Players[1].CmdCasts = []int32{0}
	g.Players[1].CmdDamage = []int32{0, 3} // took 3 from B (dense 1)
	g.SetZone(state.ZCommand, 0, []state.ObjID{A.ID})
	g.SetZone(state.ZBattlefield, 1, []state.ObjID{B.ID})
	return g, A.ID, B.ID
}

// TestBoardFromGameCommanderFacts pins the game-shaped half's commander
// bookkeeping: the roster transposes the dense-indexed CmdDamage slices
// into per-commander, per-damaged-player tallies; Casts comes from the
// parallel CmdCasts; InCommandZone is zone-LIST membership (A sits in the
// zone, B is on the battlefield); and a commander with no damage on a
// given player's card gets no Damage entry at all — nil maps and zero
// entries are the halves' shared "nothing happened yet" shape.
func TestBoardFromGameCommanderFacts(t *testing.T) {
	g, A, B := commanderGameState(t)
	got := BoardFromGame(g, stubChars{}, 0)
	cmA, ok := got.Commanders[A]
	if !ok {
		t.Fatalf("seat 0's commander A (%d) missing from the board", A)
	}
	if cmA.Casts != 2 || !cmA.InCommandZone {
		t.Errorf("commander A facts = %+v, want Casts 2 in the command zone", cmA)
	}
	if cmA.Damage[0] != 7 || cmA.Damage[1] != 0 || len(cmA.Damage) != 1 {
		t.Errorf("commander A damage = %v, want {seat 0: 7} exactly", cmA.Damage)
	}
	cmB, ok := got.Commanders[B]
	if !ok {
		t.Fatalf("seat 1's commander B (%d) missing from the board", B)
	}
	if cmB.Casts != 0 || cmB.InCommandZone {
		t.Errorf("commander B facts = %+v, want Casts 0 on the battlefield (not in the zone)", cmB)
	}
	if cmB.Damage[1] != 3 || len(cmB.Damage) != 1 {
		t.Errorf("commander B damage = %v, want {seat 1: 3} exactly", cmB.Damage)
	}
}

// TestBoardFromGameCommandZoneCensus is the Cards-census extension: a
// commander sitting in the command zone carries card facts (creature,
// power, mana value) exactly like a hand or battlefield card — the fact
// the casting rule's tax arithmetic reads. Without the command zone in the
// census a commander reads as a zero-fact 0/0 and its cast is never
// scored.
func TestBoardFromGameCommandZoneCensus(t *testing.T) {
	g, A, _ := commanderGameState(t)
	got := BoardFromGame(g, stubChars{}, 0)
	c, ok := got.Cards[A]
	if !ok {
		t.Fatalf("command-zone commander A (%d) absent from the casting census", A)
	}
	// A is this fixture's first object (id 1), so stubChars reports the
	// derived power 2 for it — the point is the commander is a CREATURE in
	// the census, not a zero-fact unreadable card.
	if !c.Creature || c.Power != 2 {
		t.Errorf("command-zone commander facts = %+v, want a creature with the derived power", c)
	}
}

// TestBoardFromGameIntoClearsCommanders is the ownership contract's third
// map: BoardFromGameInto must clear and reuse the caller's Commanders map
// like Creatures/Life/Cards — a refill over a second game leaves only that
// game's commanders, with only that game's casts and damage. Deleting the
// clear(b.Commanders) fails this test by name, as does reallocating the
// map instead of clearing it.
func TestBoardFromGameIntoClearsCommanders(t *testing.T) {
	gA, A, _ := commanderGameState(t)
	boardA := BoardFromGame(gA, stubChars{}, 0)
	cmdPtr := mapPtr(boardA.Commanders)

	// Game B: seat 1 alone has a commander. A filler object takes object id
	// 1 so B's commander id (2) cannot collide with game A's commander id
	// (also 1) — the leakage check below must compare identities, not
	// coincidences of the id arena.
	gB := state.NewGame([]string{"a", "b"})
	gB.AddObject(cardFace(t, "filler", "Creature Bear", 1, 1), 0)
	Bo := gB.AddObject(cardFace(t, "B", "Legendary Creature Bear", 3, 3), 1)
	gB.Players[1].Commanders = []state.ObjID{Bo.ID}
	gB.Players[1].CmdCasts = []int32{0}
	gB.Players[1].CmdDamage = []int32{3} // took 3 from B (dense 0)
	gB.SetZone(state.ZBattlefield, 1, []state.ObjID{Bo.ID})
	boardB := BoardFromGameInto(gB, stubChars{}, 0, &boardA)

	if mapPtr(boardB.Commanders) != cmdPtr {
		t.Fatalf("BoardFromGameInto reallocated the Commanders map: reuse is not happening")
	}
	if len(boardB.Commanders) != 1 {
		t.Fatalf("refilled Commanders = %d entries, want 1 (game A's commander failed to clear)", len(boardB.Commanders))
	}
	if _, stale := boardB.Commanders[A]; stale {
		t.Errorf("game A's commander A leaked into the refilled board")
	}
	cmB := boardB.Commanders[Bo.ID]
	if cmB.Casts != 0 || cmB.InCommandZone || cmB.Damage[1] != 3 || len(cmB.Damage) != 1 {
		t.Errorf("game B's commander facts = %+v, want the clean B facts", cmB)
	}
}

// ---------------------------------------------------------------------------
// BR3/BR4: the commander clock is a second lethal line on defense.

// TestBlockCommanderClockChump is BR3: a commander whose unblocked swing
// would take THIS defender to 21+ from it is chumped even at a healthy
// life total — the clock is a second track that ignores life. The 1/1
// cannot kill the 3/3 (BR1 declines); the life gate alone (20 - 3 > 0)
// would also decline; only the clock forces the block.
func TestBlockCommanderClockChump(t *testing.T) {
	b := boardOf(atk(1, 1, 1), def(1, 3, 3))
	b.Life[0] = 20
	b.Commanders = map[state.ObjID]Commander{201: zoneCmd(0, map[state.PlayerID]int32{0: 18})}
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 1 {
		t.Errorf("1/1 vs 3/3 commander at 18 = %v, want the chump (18+3 closes the clock)", got)
	}
	// The same 1/1 vs a commander at 16 stays home: 19 < 21, and 20 life
	// can take the hit.
	b.Commanders = map[state.ObjID]Commander{201: zoneCmd(0, map[state.PlayerID]int32{0: 16})}
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 0 {
		t.Errorf("1/1 vs 3/3 commander at 16 = %v, want no chump (16+3 does not close)", got)
	}
	// And a NON-commander never fires the clock: without the Commanders
	// entry the same attacker and same life read as a plain 3/3.
	b.Commanders = map[state.ObjID]Commander{}
	if got := blockDecision(b, [2]int{1, 1}); len(got) != 0 {
		t.Errorf("1/1 vs plain 3/3 at 20 life = %v, want no chump (no commander clock)", got)
	}
}

// TestBlockCommanderCloserRankedFirst is BR4: the defender with one 4/4
// blocker and two incoming attackers — a 3/3 commander at 18 (one swing
// from closing the clock) and a plain 4/4 — must spend the blocker on the
// commander: the 4/4 blocks the 3/3 and survives, the plain 4/4 lands four
// life, and the clock stays at 18. Under a power-only ordering the blocker
// would trade with the plain 4/4 first and the commander would land,
// closing the clock at 21 — a loss the clock ordering prevents.
func TestBlockCommanderCloserRankedFirst(t *testing.T) {
	b := boardOf(atk(1, 4, 4), def(1, 3, 3), def(2, 4, 4))
	b.Life[0] = 20
	b.Commanders = map[state.ObjID]Commander{201: zoneCmd(0, map[state.PlayerID]int32{0: 18})}
	fa := blockDecisionFull(b, [2]int{1, 1}, [2]int{1, 2}) // options: (4/4, closer 3/3), (4/4, plain 4/4)
	if len(fa.choices) != 1 {
		t.Fatalf("one blocker vs two attackers = %v, want exactly one block", fa.choices)
	}
	blocked := fa.d.Options[fa.choices[0]].Attacker
	if blocked != state.ObjID(201) {
		t.Errorf("blocked attacker = %d, want the closer commander (201); power-only order would block the plain 4/4 (202) and lose to the clock", blocked)
	}
}
